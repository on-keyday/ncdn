package util

import (
	"errors"
	"net"
	"net/netip"
)

func GetSelfIPv4Address(name string) (netip.Addr, net.HardwareAddr, error) {
	var iface *net.Interface
	if name != "" {
		ifaceN, err := net.InterfaceByName(name)
		if err != nil {
			return netip.Addr{}, nil, err
		}
		iface = ifaceN
	} else {
		ifaces, err := net.Interfaces()
		if err != nil {
			return netip.Addr{}, nil, err
		}
		for _, i := range ifaces {
			if i.Flags&net.FlagLoopback == 0 && i.Flags&net.FlagUp != 0 {
				iface = &i
				break
			}
		}
	}
	if iface == nil {
		return netip.Addr{}, nil, errors.New("no suitable network interface found")
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return netip.Addr{}, nil, err
	}
	var ipv4Addrs []net.Addr
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
			ipv4Addrs = append(ipv4Addrs, addr)
		}
	}
	if len(ipv4Addrs) != 1 {
		return netip.Addr{}, nil, errors.New("no unique IPv4 address found for interface")
	}
	return netip.AddrFrom4([4]byte(ipv4Addrs[0].(*net.IPNet).IP.To4())), iface.HardwareAddr, nil
}
