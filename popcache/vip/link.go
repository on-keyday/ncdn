package vip

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"

	"github.com/vishvananda/netlink"
)

type VIPManager struct {
	VIPDevice string
}

func NewVIPManager(dev string) (*VIPManager, error) {
	link := &netlink.Iptun{}
	link.Name = dev
	link.Remote
	err = netlink.LinkAdd(link)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	err = netlink.LinkSetUp(link)
	if err != nil {
		return nil, err
	}
	return &VIPManager{VIPDevice: dev}, nil
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

}
