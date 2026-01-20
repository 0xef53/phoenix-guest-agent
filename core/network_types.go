package core

import (
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
)

type InetFamily uint16

const (
	AF_UNSPEC InetFamily = syscall.AF_UNSPEC
	AF_INET   InetFamily = syscall.AF_INET
	AF_INET6  InetFamily = syscall.AF_INET6
)

type RouteInfo struct {
	LinkIndex int
	LinkName  string
	Scope     netlink.Scope
	Dst       string
	Src       string
	Gw        string
	Table     int
}

type RouteAttrs struct {
	LinkName string
	Scope    netlink.Scope
	Dst      string
	Src      string
	Gw       string
	Table    int
}

type InterfaceInfo struct {
	Index  int
	Name   string
	HwAddr string
	Flags  net.Flags
	MTU    int
	Addrs  []string
}
