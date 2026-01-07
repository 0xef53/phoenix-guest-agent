package core

import (
	"context"
	"net"
	"os"

	"github.com/vishvananda/netlink"
)

func (s *Server) GetRouteList(ctx context.Context, family InetFamily) ([]*RouteInfo, error) {
	tmp, err := netlink.RouteList(nil, int(family))
	if err != nil {
		return nil, os.NewSyscallError("rtnetlink", err)
	}

	routes := make([]*RouteInfo, 0, len(tmp))

	for _, x := range tmp {
		link, _ := net.InterfaceByIndex(x.LinkIndex)

		r := RouteInfo{
			LinkIndex: x.LinkIndex,
			LinkName:  link.Name,
			Scope:     x.Scope,
			Table:     x.Table,
		}

		if x.Src != nil {
			r.Src = x.Src.String()
		}

		if x.Dst == nil {
			r.Dst = "0.0.0.0"
		} else {
			r.Dst = x.Dst.String()
		}

		if x.Gw != nil {
			r.Gw = x.Gw.String()
		}

		routes = append(routes, &r)
	}

	return routes, nil
}

func (s *Server) AddRoute(ctx context.Context, attrs *RouteAttrs) (*RouteInfo, error) {
	return s.updateRouteTable(ctx, "add", attrs)
}

func (s *Server) DelRoute(ctx context.Context, attrs *RouteAttrs) (*RouteInfo, error) {
	return s.updateRouteTable(ctx, "del", attrs)
}

func (s *Server) updateRouteTable(ctx context.Context, action string, attrs *RouteAttrs) (*RouteInfo, error) {
	link, err := netlink.LinkByName(attrs.LinkName)
	if err != nil {
		return nil, os.NewSyscallError("rtnetlink", err)
	}

	if err := UpdateRouteTable(action, link, attrs.Dst, attrs.Src, attrs.Gw, attrs.Scope, attrs.Table); err != nil {
		return nil, err
	}

	return &RouteInfo{
		LinkIndex: link.Attrs().Index,
		LinkName:  attrs.LinkName,
		Src:       attrs.Src,
		Dst:       attrs.Dst,
		Gw:        attrs.Gw,
		Table:     attrs.Table,
		Scope:     attrs.Scope,
	}, nil
}

func (s *Server) GetInterfaces(ctx context.Context) ([]*InterfaceInfo, error) {
	tmp, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ifs := make([]*InterfaceInfo, 0, len(tmp))

	for _, x := range tmp {
		addrs, err := x.Addrs()
		if err != nil {
			return nil, err
		}

		strAddrs := make([]string, 0, len(addrs))

		for _, v := range addrs {
			strAddrs = append(strAddrs, v.(*net.IPNet).String())
		}

		ifs = append(ifs, &InterfaceInfo{
			Index:  x.Index,
			Name:   x.Name,
			Flags:  x.Flags,
			MTU:    x.MTU,
			HwAddr: x.HardwareAddr.String(),
			Addrs:  strAddrs,
		})
	}

	return ifs, nil
}

func (s *Server) SetInterfaceLinkUp(ctx context.Context, ifname string) error {
	if err := SetInterfaceLinkUp(ifname); err != nil {
		return err
	}

	return nil
}

func (s *Server) SetInterfaceLinkDown(ctx context.Context, ifname string) error {
	if err := SetInterfaceLinkDown(ifname); err != nil {
		return err
	}

	return nil
}

func (s *Server) AddIPAddr(ctx context.Context, ifname, addr string) error {
	if err := UpdateAddrList("add", ifname, addr); err != nil {
		return err
	}

	return nil
}

func (s *Server) DelIPAddr(ctx context.Context, ifname, addr string) error {
	if err := UpdateAddrList("del", ifname, addr); err != nil {
		return err
	}

	return nil
}
