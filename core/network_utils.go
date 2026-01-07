package core

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/vishvananda/netlink"
)

func UpdateRouteTable(action string, link netlink.Link, dst, src, gw string, scope netlink.Scope, table int) error {
	dstNet, err := netlink.ParseIPNet(dst)
	if err != nil {
		return err
	}

	r := netlink.Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dstNet,
		Src:       net.ParseIP(src),
		Gw:        net.ParseIP(gw),
		Scope:     scope,
		Table:     table,
	}

	switch action {
	case "add":
		if err := netlink.RouteAdd(&r); err != nil {
			return os.NewSyscallError("rtnetlink", err)
		}
	case "replace":
		if err := netlink.RouteReplace(&r); err != nil {
			return os.NewSyscallError("rtnetlink", err)
		}
	case "del":
		if err := netlink.RouteDel(&r); err != nil {
			return os.NewSyscallError("rtnetlink", err)
		}
	default:
		return fmt.Errorf("invalid action: %s", action)
	}

	return nil
}

func SetInterfaceLinkUp(ifname string) error {
	iface := &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{Name: ifname},
	}

	if err := netlink.LinkSetUp(iface); err != nil {
		return os.NewSyscallError("rtnetlink", err)
	}

	return nil
}

func SetInterfaceLinkDown(ifname string) error {
	iface := &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{Name: ifname},
	}

	if err := netlink.LinkSetDown(iface); err != nil {
		return os.NewSyscallError("rtnetlink", err)
	}

	return nil
}

func UpdateAddrList(action, ifname, ipaddr string) error {
	iface, err := netlink.LinkByName(ifname)
	if err != nil {
		return os.NewSyscallError("rtnetlink", err)
	}

	addr, err := netlink.ParseAddr(ipaddr)
	if err != nil {
		return err
	}

	switch action {
	case "add":
		if err := netlink.AddrAdd(iface, addr); err != nil {
			return os.NewSyscallError("rtnetlink", err)
		}
	case "del":
		if err := netlink.AddrDel(iface, addr); err != nil {
			return os.NewSyscallError("rtnetlink", err)
		}
	default:
		return fmt.Errorf("invalid action: %s", action)
	}

	return nil
}

func ParseCIDR(s string) (net.IP, *net.IPNet, error) {
	if !strings.Contains(s, "/") {
		if net.ParseIP(s).To4() != nil {
			s += "/32"
		} else {
			s += "/128"
		}
	}

	return net.ParseCIDR(s)
}
