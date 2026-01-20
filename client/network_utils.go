package client

import (
	"net"
	"strings"

	"github.com/vishvananda/netlink"
)

func ParseIPNet(s string) (*net.IPNet, error) {
	if !strings.Contains(s, "/") {
		if net.ParseIP(s).To4() != nil {
			s += "/32"
		} else {
			s += "/128"
		}
	}

	return netlink.ParseIPNet(s)
}
