package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/yzp0n/ncdn/l4lb/l4lbdrv"
	"github.com/yzp0n/ncdn/tool/util"
	"golang.org/x/sys/unix"
)

var lbBin = flag.String("lbBin", "c/lb.o", "Path to XDP lb binary")
var cryptoBin = flag.String("cryptoBin", "c/init_crypto.o", "Path to XDP crypto binary")
var xdpcapHookPath = flag.String("xdpcapHookPath", "/sys/fs/bpf/xdpcap_hook", "Path to XDPCap hook")
var xdpif = flag.String("interface", "net0", "Interface to attach lb prog to")
var vip = flag.String("vip", "192.0.2.10", "VIP address to load balance")
var deststr = flag.String("dests", "", "Comma separated list of destination IP and MAC addresses. (Example: 192.168.88.10;00:00:5e:00:53:01,)")
var sharedKey = flag.String("sharedSecret", "shared_secret", "Shared secret for QUIC LB connection ID generation (TODO: move into secure place)")

func parseDest(deststr string) ([]l4lbdrv.DestinationEntry, error) {
	commas := strings.Split(deststr, ",")
	dests := make([]l4lbdrv.DestinationEntry, 0, len(commas))
	for _, c := range commas {
		if c == "" {
			continue
		}

		parts := strings.Split(c, ";")
		if len(parts) != 2 {
			return nil, fmt.Errorf("Invalid destination entry: %s", c)
		}
		ip4 := netip.MustParseAddr(parts[0])
		if ip4.Is6() {
			return nil, fmt.Errorf("Destination must be ipv4 address, but was %s", ip4)
		}

		mac, err := net.ParseMAC(parts[1])
		if err != nil {
			return nil, fmt.Errorf("Invalid MAC address: %s", parts[1])
		}

		dests = append(dests, l4lbdrv.DestinationEntry{
			IPAddr:       ip4,
			HardwareAddr: mac,
		})
	}
	log.Printf("dests: %+v", dests)
	return dests, nil
}

func FSType(path string) (int64, error) {
	var statfs unix.Statfs_t
	if err := unix.Statfs(path, &statfs); err != nil {
		return 0, err
	}

	fsType := int64(statfs.Type)
	if unsafe.Sizeof(statfs.Type) == 4 {
		// We're on a 32 bit arch, where statfs.Type is int32. bpfFSType is a
		// negative number when interpreted as int32 so we need to cast via
		// uint32 to avoid sign extension.
		fsType = int64(uint32(statfs.Type))
	}
	return fsType, nil
}

func main() {
	flag.Parse()
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		log.Panicf("Failed to read /proc/self/mountinfo: %v", err)
	}
	fmt.Printf("Mountinfo:\n%s\n", data)
	linked, err := os.Readlink("/proc/self/ns/mnt")
	if err != nil {
		log.Panicf("Failed to read /proc/self/ns/mnt: %v", err)
	}
	fmt.Printf("Mount name: %s\n", linked)
	///*
	typ, err := FSType("/sys/fs/bpf/") // Ensure that the BPF filesystem is mounted
	if err != nil {
		log.Panicf("Failed to get filesystem type: %v", err)
	}
	if typ != 0xcafe4a11 {
		log.Panicf("Expected BPF filesystem type %d, got %d", 0xcafe4a11, typ)
	}
	//*/

	dests, err := parseDest(*deststr)
	if err != nil {
		log.Panicf("Failed to parse dest string: %v", err)
	}

	derivedKey, err := util.DeriveKey([]byte(*sharedKey), "quic-lb")
	if err != nil {
		log.Panicf("Failed to derive key: %v", err)
	}

	cfg := &l4lbdrv.Config{
		BinPath:        *lbBin,
		CryptoBin:      *cryptoBin,
		EBPFPinDir:     "/sys/fs/bpf/",
		XdpCapHookPath: *xdpcapHookPath,
		InterfaceName:  *xdpif,
		VIP:            netip.MustParseAddr(*vip),
		Dests:          dests,
		SharedKey:      derivedKey,
	}
	lb, err := l4lbdrv.New(cfg)
	if err != nil {
		log.Panicf("Failed to create l4lb instance: %v", err)
	}
	slog.Info("L4LB started.")
	defer lb.Close()

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			if err := lb.DumpCounters(); err != nil {
				slog.Error("Failed to dump counters", slog.String("err", err.Error()))
			}
			continue

		case <-done:
			break
		}
		break
	}
	slog.Info("Shutting down.")
}
