package network

import (
	"github.com/0xef53/phoenix-guest-agent/core"

	pb_types "github.com/0xef53/phoenix-guest-agent/api/types/v2"
)

func routeToProto(route *core.RouteInfo) *pb_types.RouteInfo {
	proto := pb_types.RouteInfo{
		LinkIndex: int32(route.LinkIndex),
		LinkName:  route.LinkName,
		Scope:     pb_types.RouteScope(route.Scope),
		Dst:       route.Dst,
		Src:       route.Src,
		Gw:        route.Gw,
		Table:     int32(route.Table),
	}

	return &proto
}

func routesToProto(routes []*core.RouteInfo) []*pb_types.RouteInfo {
	protos := make([]*pb_types.RouteInfo, 0, len(routes))

	for _, r := range routes {
		protos = append(protos, routeToProto(r))
	}

	return protos
}

func ifaceToProto(iface *core.InterfaceInfo) *pb_types.InterfaceInfo {
	proto := pb_types.InterfaceInfo{
		Index:  int32(iface.Index),
		Name:   iface.Name,
		HwAddr: iface.HwAddr,
		Flags:  uint32(iface.Flags),
		MTU:    int32(iface.MTU),
		Addrs:  iface.Addrs,
	}

	return &proto
}

func ifaceListToProto(ifaces []*core.InterfaceInfo) []*pb_types.InterfaceInfo {
	protos := make([]*pb_types.InterfaceInfo, 0, len(ifaces))

	for _, iface := range ifaces {
		protos = append(protos, ifaceToProto(iface))
	}

	return protos
}
