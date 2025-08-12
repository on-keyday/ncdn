package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/netip"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/yzp0n/ncdn/controller/chunk"
	"github.com/yzp0n/ncdn/controller/file"
	"github.com/yzp0n/ncdn/controller/lbconn"
	"github.com/yzp0n/ncdn/controller/protocol"
	"github.com/yzp0n/ncdn/controller/remoteshell"
	"github.com/yzp0n/ncdn/controller/transport"
	wstransport "github.com/yzp0n/ncdn/controller/transport/websocket"
	"github.com/yzp0n/ncdn/l4lb/l4lbdrv"
	"github.com/yzp0n/ncdn/tool/util"
	"golang.org/x/net/websocket"
	"golang.org/x/sys/unix"
)

var controlPlaneAddr = flag.String("controlPlane", "ws://localhost:8080", "Control plane url for the load balancer")
var lbBin = flag.String("lbBin", "c/lb.o", "Path to XDP lb binary")
var cryptoBin = flag.String("cryptoBin", "c/init_crypto.o", "Path to XDP crypto binary")
var xdpcapHookPath = flag.String("xdpcapHookPath", "/sys/fs/bpf/xdpcap_hook", "Path to XDPCap hook")
var xdpif = flag.String("interface", "", "Interface to attach lb prog to")
var vip = flag.String("vip", "192.0.2.10", "VIP address to load balance")
var deststr = flag.String("dests", "", "Comma separated list of destination IP and MAC addresses. (Example: 192.168.88.10;00:00:5e:00:53:01,)")
var sharedKey = flag.String("sharedSecret", "shared_secret", "Shared secret for QUIC LB connection ID generation (TODO: move into secure place)")
var pidFile = flag.String("pidFile", "/run/ncdn/l4lb.pid", "Path to PID file for l4lb process")
var mtu = flag.Uint("mtu", 1500, "Maximum Transmission Unit (MTU) for the interface")
var asn = flag.Uint("asn", 65000, "Autonomous System Number (ASN) for iBGP")

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

type l4lbStatCounters struct {
	lock     sync.Mutex
	ebpfData *l4lbdrv.StatCounters
}

func (s *l4lbStatCounters) Set(data *l4lbdrv.StatCounters) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.ebpfData = data
}

func (s *l4lbStatCounters) GetStat() (*protocol.L4UpdateInfo, error) {
	s.lock.Lock()
	counter := s.ebpfData
	s.lock.Unlock()
	if counter == nil {
		return &protocol.L4UpdateInfo{}, nil
	}
	marshalled, err := binary.Append(nil, binary.BigEndian, counter)
	if err != nil {
		return nil, err
	}
	return &protocol.L4UpdateInfo{
		EbpfData: marshalled,
	}, nil
}

func main() {
	flag.Parse()
	if *pidFile != "" {
		if err := os.MkdirAll("/run/ncdn", 0755); err != nil {
			log.Panicf("Failed to create /run/ncdn directory: %v", err)
		}
		if err := os.WriteFile(*pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
			log.Panicf("Failed to write pid file %q: %v", *pidFile, err)
		}
		defer func() {
			if err := os.Remove(*pidFile); err != nil {
				log.Panicf("Failed to remove pid file %q: %v", *pidFile, err)
			}
		}()
	}
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
	if *mtu < 68 {
		log.Panicf("MTU must be at least 68 bytes, got %d", *mtu)
	}
	if *mtu > 9000 {
		log.Printf("Warning: MTU is set to %d bytes, which is larger than the typical Ethernet MTU of 1500 bytes. Ensure that your network supports this MTU.", *mtu)
	}
	if *mtu > 65535 {
		log.Panicf("MTU must not exceed 65535 bytes, got %d", *mtu)
	}

	//dests, err := parseDest(*deststr)
	//if err != nil {
	//	log.Panicf("Failed to parse dest string: %v", err)
	//}
	ifname, _, addr, hardAddr, err := util.GetSelfIPv4Address(*xdpif)
	if err != nil {
		log.Panicf("Failed to get IPv4 address for interface %s: %v", *xdpif, err)
	}

	derivedKey, err := util.DeriveKey([]byte(*sharedKey), "quic-lb")
	if err != nil {
		log.Panicf("Failed to derive key: %v", err)
	}

	var random [4]byte
	if _, err := rand.Read(random[:]); err != nil {
		log.Panicf("Failed to generate random bytes: %v", err)
	}

	defaultGW, err := util.GetDefaultGateway()
	if err != nil {
		log.Panicf("Failed to get default gateway: %v", err)
	}

	controlPlaneURL, err := url.Parse(*controlPlaneAddr)
	if err != nil {
		log.Panicf("Failed to parse control plane address %q: %v", *controlPlaneAddr, err)
	}
	if controlPlaneURL.Scheme != "ws" && controlPlaneURL.Scheme != "wss" {
		log.Panicf("Control plane address must use ws or wss scheme, got %s", controlPlaneURL.Scheme)
	}
	httpOrigin := *controlPlaneURL
	httpOrigin.Scheme = "http"

	bgpServe, err := SetupIBGP(uint32(*asn), addr, defaultGW)
	if err != nil {
		log.Panicf("Failed to setup iBGP: %v", err)
	}

	firstEntry := l4lbdrv.DestinationEntry{
		IPAddr:       addr,
		HardwareAddr: hardAddr,
		ServerID:     uint32(1), // TODO: Make this configurable
	}
	cfg := &l4lbdrv.Config{
		BinPath:        *lbBin,
		CryptoBin:      *cryptoBin,
		EBPFPinDir:     "/sys/fs/bpf/",
		XdpCapHookPath: *xdpcapHookPath,
		InterfaceName:  ifname,
		VIP:            netip.MustParseAddr(*vip),
		Dests:          []l4lbdrv.DestinationEntry{firstEntry},
		SharedKey:      derivedKey,
		MTU:            uint16(*mtu),
		RoutingRandom:  random,
	}
	lb, err := l4lbdrv.New(cfg)
	if err != nil {
		log.Panicf("Failed to create l4lb instance: %v", err)
	}
	slog.Info("L4LB started.")
	defer lb.Close()

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	counters := &l4lbStatCounters{}

	retryConn := lbconn.ConnectRetriable(slog.Default(), 5*time.Second, lbconn.ConnectL4LB,
		&protocol.L4LBData{
			ServerID:   uint32(1), // TODO: Make this configurable
			Address:    addr.As4(),
			MacAddress: [6]byte(hardAddr),
		}, 20*time.Second, func() (transport.Connection, error) {
			return wstransport.Connect(context.Background(), &websocket.Config{
				Location: controlPlaneURL,
				Origin:   &httpOrigin,
				Version:  websocket.ProtocolVersionHybi13,
			})
		}, counters.GetStat)
	log.Printf("Connecting to control plane at %s with origin %s", controlPlaneURL.String(), httpOrigin.String())

	updateDestsChan := make(chan *protocol.L4Lbl7Lbupdate, 1)
	updateConfig := make(chan *protocol.Vipupdate, 1)

	cmdMgr := remoteshell.NewManager()

	chunkedMap := chunk.NewChunkMap()

	go func() {
		for {
			msg, err := retryConn.Receive()
			if err != nil {
				slog.Error("Failed to receive connection", slog.String("error", err.Error()))
				return // this is fatal, we cannot continue without a connection
			}
			if l7lbUpdate := msg.L4LbL7LbUpdate(); l7lbUpdate != nil {
				updateDestsChan <- l7lbUpdate
				continue
			} else if l4lbUpdate := msg.VipUpdate(); l4lbUpdate != nil {
				updateConfig <- l4lbUpdate
				continue
			}
			if handled, err := remoteshell.DispatchMessage(cmdMgr, msg); err != nil {
				retryConn.Send(&lbconn.LogMsg{
					Level:   protocol.LogLevel_Error,
					Message: fmt.Sprintf("Failed to dispatch command line message: %v", err),
				})
			} else if handled {
				continue // Handled by cmdline, no need to process further
			}
			chunked, err := chunkedMap.ReadChunked(retryConn, msg)
			if err != nil {
				retryConn.Send(&lbconn.LogMsg{
					Level:   protocol.LogLevel_Error,
					Message: fmt.Sprintf("Failed to read chunked data: %v", err),
				})
				continue
			}
			if chunked != nil {
				if handled, path, perm, err := file.MaySaveFile(chunked); err != nil {
					retryConn.Send(&lbconn.LogMsg{
						Level:   protocol.LogLevel_Error,
						Message: fmt.Sprintf("Failed to save file: %v", err),
					})
				} else if handled {
					retryConn.Send(&lbconn.LogMsg{
						Level:   protocol.LogLevel_Info,
						Message: fmt.Sprintf("File saved to %s with permission %o", path, perm),
					})
				} else {
					retryConn.Send(&lbconn.LogMsg{
						Level:   protocol.LogLevel_Error,
						Message: "Received chunked data that is not a file",
					})
				}
			}
		}
	}()

	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			if counter, err := lb.GetCounters(); err != nil {
				slog.Error("Failed to dump counters", slog.String("err", err.Error()))
				continue
			} else {
				slog.Info(counter.String())
				counters.Set(counter)
			}
			continue
		case update := <-updateDestsChan:
			slog.Info("Received L7 load balancer update", slog.Any("update", update.Info))
			var destEntries []l4lbdrv.DestinationEntry
			destEntries = append(destEntries, firstEntry) // first is self address
			for _, dest := range update.Info {
				destEntries = append(destEntries, l4lbdrv.DestinationEntry{
					IPAddr:       netip.AddrFrom4(dest.Address),
					HardwareAddr: net.HardwareAddr(dest.MacAddress[:]),
					ServerID:     dest.ServerId,
				})
			}
			if err := lb.UpdateDestinations(destEntries); err != nil {
				retryConn.Send(&lbconn.LogMsg{
					Level:   protocol.LogLevel_Error,
					Message: fmt.Sprintf("Failed to update load balancer configuration: %v", err),
				})
				slog.Error("Failed to update load balancer configuration", slog.Any("error", err))
			} else {
				retryConn.Send(&lbconn.LogMsg{
					Level:   protocol.LogLevel_Info,
					Message: "Load balancer configuration updated successfully",
				})
				slog.Info("Load balancer configuration updated successfully")
			}
			continue
		case update := <-updateConfig:
			slog.Info("Received L4 load balancer update", slog.Any("update", update))
			if err := lb.UpdateVIP(netip.AddrFrom4(update.VirtualAddress)); err != nil {
				retryConn.Send(&lbconn.LogMsg{
					Level:   protocol.LogLevel_Error,
					Message: fmt.Sprintf("Failed to update VIP: %v", err),
				})
				slog.Error("Failed to update VIP", slog.Any("error", err))
			} else {
				if err := bgpServe.UpdateVIP(netip.PrefixFrom(netip.AddrFrom4(update.VirtualAddress), int(update.Prefix))); err != nil {
					retryConn.Send(&lbconn.LogMsg{
						Level:   protocol.LogLevel_Error,
						Message: fmt.Sprintf("Failed to update BGP VIP: %v", err),
					})
					slog.Error("Failed to update BGP VIP", slog.Any("error", err))
				} else {
					retryConn.Send(&lbconn.LogMsg{
						Level:   protocol.LogLevel_Info,
						Message: "VIP updated successfully",
					})
					slog.Info("VIP updated successfully")
				}
			}
			continue
		case cmd := <-cmdMgr.Output():
			if err := retryConn.SendCommandline(cmd); err != nil {
				slog.Error("Failed to send command line message", slog.String("error", err.Error()))
			}
			continue
		case d := <-done:
			slog.Info("Received shutdown signal", slog.String("signal", d.String()))
		}
		break
	}
	slog.Info("Shutting down.")
}
