package util

import "net"

type InterfaceAddr struct {
	Interface string
	IP        net.IP
	Net       *net.IPNet
}

func GetAllAddresses() []*InterfaceAddr {
	rt := []*InterfaceAddr{}
	ifcs, err := net.Interfaces()
	if err != nil {
		return rt
	}

	for _, ifc := range ifcs {
		ifname := ifc.Name
		ifaddrs, err := ifc.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range ifaddrs {
			netip, netmask, err := net.ParseCIDR(addr.String())
			if err != nil {
				continue
			}
			rt = append(rt, &InterfaceAddr{Interface: ifname, IP: netip, Net: netmask})
		}
	}
	return rt
}

func FindAllAddresses(ipaddr net.IP) []*InterfaceAddr {
	rt := []*InterfaceAddr{}

	if ipaddr.Equal(net.IPv4zero) {
		ifaddrs := GetAllAddresses()
		for _, ifa := range ifaddrs {
			if ifa.IP.To4() != nil {
				rt = append(rt, ifa)
			}
		}
	} else if ipaddr.Equal(net.IPv6zero) {
		ifaddrs := GetAllAddresses()
		for _, ifa := range ifaddrs {
			if ifa.IP.To4() != nil {
				rt = append(rt, ifa)
			}
		}
	} else {
		ifaddrs := GetAllAddresses()
		for _, ifa := range ifaddrs {
			if ifa.Net.Contains(ipaddr) {
				rt = append(rt, ifa)
			}
		}
	}

	return rt
}
