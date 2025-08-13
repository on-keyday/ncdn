package vip

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/yzp0n/ncdn/controller/protocol"
)

type Remote struct {
	Remote *protocol.L4LBData
	Link   *netlink.Iptun
}

type VIPManager struct {
	VIPDevice  string
	RemoteList []*Remote
	VIP        netip.Prefix
	LocalAddr  netip.Addr
	PhyDev     int
}

func DisableRPFilter(ifName string) error {
	return os.WriteFile(fmt.Sprintf("/proc/sys/net/ipv4/conf/%s/rp_filter", ifName), []byte("0"), 0644)
}

func NewVIPManager(localAddr netip.Addr, phyDevIndex int) (*VIPManager, error) {
	if err := DisableRPFilter("lo"); err != nil {
		return nil, fmt.Errorf("failed to disable rp_filter on %s: %w", "lo", err)
	}
	return &VIPManager{VIPDevice: "lo", LocalAddr: localAddr, PhyDev: phyDevIndex}, nil
}

func (m *VIPManager) UpdateRemote(remotes []*protocol.L4LBData, logger *slog.Logger) error {
	m.RemoteList = make([]*Remote, len(remotes))
	for i, remote := range remotes {
		hexMac := hex.EncodeToString(remote.MacAddress[:])
		ipTun := &netlink.Iptun{
			LinkAttrs: netlink.LinkAttrs{
				Name:        hexMac,
				ParentIndex: m.PhyDev,
			},
			Local:  m.LocalAddr.AsSlice(),
			Remote: remote.Address.AsSlice(),
		}
		if err := netlink.LinkAdd(ipTun); err != nil {
			if !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("failed to add iptun link %s: %w", hexMac, err)
			}
			existingLink, err := netlink.LinkByName(hexMac)
			if err != nil {
				return fmt.Errorf("failed to find existing iptun link %s: %w", hexMac, err)
			}
			// delete and recreate
			if err := netlink.LinkDel(existingLink); err != nil {
				return fmt.Errorf("failed to delete existing iptun link %s: %w", hexMac, err)
			}
			time.Sleep(100 * time.Millisecond) // Give some time for the link to be removed
			if err := netlink.LinkAdd(ipTun); err != nil {
				return fmt.Errorf("failed to recreate iptun link %s: %w", hexMac, err)
			}
		}
		if err := DisableRPFilter(hexMac); err != nil {
			return fmt.Errorf("failed to disable rp_filter on %s: %w", hexMac, err)
		}
		if err := netlink.LinkSetUp(ipTun); err != nil {
			return fmt.Errorf("failed to set up iptun link %s: %w", hexMac, err)
		}
		m.RemoteList[i] = &Remote{
			Remote: remote,
			Link:   ipTun,
		}
	}
	logger.Info("Updated remote list", "count", len(m.RemoteList))
	return nil
}

func (m *VIPManager) UpdateVIP(vip netip.Prefix, logger *slog.Logger) error {
	link, err := netlink.LinkByName(m.VIPDevice)
	if err != nil {
		return err
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return err
	}
	var alreadySet bool
	for _, addr := range addrs {
		curAddr, ok := netip.AddrFromSlice(addr.IP)
		if !ok {
			continue
		}
		if vip.Addr() == curAddr {
			alreadySet = true
			continue
		}
		if err := netlink.AddrDel(link, &netlink.Addr{IPNet: &net.IPNet{
			IP:   curAddr.AsSlice(),
			Mask: net.CIDRMask(curAddr.BitLen(), curAddr.BitLen()),
		}}); err != nil {
			logger.Error("Failed to remove addr", "addr", addr, "dev", m.VIPDevice, "error", err)
		}
	}
	if alreadySet {
		return nil
	}
	if err := netlink.AddrAdd(link, &netlink.Addr{IPNet: &net.IPNet{
		IP:   vip.Addr().AsSlice(),
		Mask: net.CIDRMask(vip.Bits(), vip.Addr().BitLen()),
	}}); err != nil {
		return fmt.Errorf("failed to add VIP %s to device %s: %w", vip, m.VIPDevice, err)
	}

	m.VIP = vip
	logger.Info("VIP updated", "vip", vip, "dev", m.VIPDevice)
	return nil
}
