package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	pb "github.com/0xef53/phoenix-guest-agent/protobufs/agent"

	empty "github.com/golang/protobuf/ptypes/empty"
	"github.com/vishvananda/netlink"
)

func (s *AgentServiceServer) GetRouteList(ctx context.Context, req *pb.RouteListRequest) (*pb.RouteList, error) {
	tmp, err := netlink.RouteList(nil, int(req.Family))
	if err != nil {
		return nil, os.NewSyscallError("rtnetlink", err)
	}

	routes := make([]*pb.RouteInfo, 0, len(tmp))

	for _, x := range tmp {
		link, _ := net.InterfaceByIndex(x.LinkIndex)
		r := pb.RouteInfo{
			LinkIndex: int32(x.LinkIndex),
			LinkName:  link.Name,
			Table:     int32(x.Table),
		}

		if _, ok := pb.RouteScope_name[int32(x.Scope)]; ok {
			r.Scope = pb.RouteScope(x.Scope)
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

	return &pb.RouteList{Routes: routes}, nil
}

func (s *AgentServiceServer) AddRoute(ctx context.Context, req *pb.RouteRequest) (*pb.RouteInfo, error) {
	return s.updateRouteTable(ctx, "add", req)
}

func (s *AgentServiceServer) DelRoute(ctx context.Context, req *pb.RouteRequest) (*pb.RouteInfo, error) {
	return s.updateRouteTable(ctx, "del", req)
}

func (s *AgentServiceServer) updateRouteTable(ctx context.Context, action string, req *pb.RouteRequest) (*pb.RouteInfo, error) {
	link, err := netlink.LinkByName(req.LinkName)
	if err != nil {
		return nil, os.NewSyscallError("rtnetlink", err)
	}

	if err := updateRouteTable(action, link, req.Dst, req.Src, req.Gw, netlink.Scope(req.Scope), int(req.Table)); err != nil {
		return nil, err
	}

	return &pb.RouteInfo{
		LinkIndex: int32(link.Attrs().Index),
		LinkName:  req.LinkName,
		Src:       req.Src,
		Dst:       req.Dst,
		Gw:        req.Gw,
		Table:     req.Table,
		Scope:     req.Scope,
	}, nil
}

func updateRouteTable(action string, link netlink.Link, dst, src, gw string, scope netlink.Scope, table int) error {
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

func (s *AgentServiceServer) GetInterfaces(ctx context.Context, req *empty.Empty) (*pb.InterfaceList, error) {
	tmp, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ifs := make([]*pb.InterfaceInfo, 0, len(tmp))

	for _, x := range tmp {
		addrs, err := x.Addrs()
		if err != nil {
			return nil, err
		}

		strAddrs := make([]string, 0, len(addrs))
		for _, v := range addrs {
			strAddrs = append(strAddrs, v.(*net.IPNet).String())
		}

		ifs = append(ifs, &pb.InterfaceInfo{
			Index:  int32(x.Index),
			Name:   x.Name,
			Flags:  uint32(x.Flags),
			MTU:    int32(x.MTU),
			HwAddr: x.HardwareAddr.String(),
			Addrs:  strAddrs,
		})
	}

	return &pb.InterfaceList{Interfaces: ifs}, nil
}

func (s *AgentServiceServer) SetInterfaceLinkUp(ctx context.Context, req *pb.LinkNameRequest) (*empty.Empty, error) {
	if err := setInterfaceLinkUp(req.Name); err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func setInterfaceLinkUp(name string) error {
	iface := &netlink.Device{netlink.LinkAttrs{Name: name}}

	if err := netlink.LinkSetUp(iface); err != nil {
		return os.NewSyscallError("rtnetlink", err)
	}

	return nil
}

func (s *AgentServiceServer) SetInterfaceLinkDown(ctx context.Context, req *pb.LinkNameRequest) (*empty.Empty, error) {
	if err := setInterfaceLinkDown(req.Name); err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func setInterfaceLinkDown(name string) error {
	iface := &netlink.Device{netlink.LinkAttrs{Name: name}}

	if err := netlink.LinkSetDown(iface); err != nil {
		return os.NewSyscallError("rtnetlink", err)
	}

	return nil
}

func (s *AgentServiceServer) AddIPAddr(ctx context.Context, req *pb.IPAddrRequest) (*empty.Empty, error) {
	if err := updateAddrList("add", req.LinkName, req.Addr); err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func (s *AgentServiceServer) DelIPAddr(ctx context.Context, req *pb.IPAddrRequest) (*empty.Empty, error) {
	if err := updateAddrList("del", req.LinkName, req.Addr); err != nil {
		return nil, err
	}

	return new(empty.Empty), nil
}

func updateAddrList(action string, ifname, ipaddr string) error {
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
