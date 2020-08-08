package main

import (
	"fmt"
	"net"
	"strings"
	"syscall"

	pb "github.com/0xef53/phoenix-guest-agent/protobufs/agent"

	empty "github.com/golang/protobuf/ptypes/empty"
	"github.com/vishvananda/netlink"
)

func (c *Command) ShowRouteList(family string) error {
	req := pb.RouteListRequest{}

	switch family {
	case "6":
		req.Family = syscall.AF_INET6
	default:
		req.Family = syscall.AF_INET
	}

	resp, err := c.client.GetRouteList(c.ctx, &req)
	if err != nil {
		return err
	}

	return printJSON(resp)
}

func (c *Command) UpdateRouteTable(action, dst, gw, ifname string) error {
	req := pb.RouteRequest{
		LinkName: ifname,
	}

	if len(gw) == 0 {
		// link route
		req.Scope = pb.RouteScope(netlink.SCOPE_LINK)
		dstAddr, err := parseIPNet(dst)
		if err != nil {
			return err
		}
		req.Dst = dstAddr.String()
	} else {
		// route via gateway
		gwAddr, err := parseIPNet(gw)
		if err != nil {
			return err
		}
		req.Gw = gwAddr.IP.String()
		if dst == "default" {
			if gwAddr.IP.To4() != nil {
				req.Dst = "0.0.0.0/0"
			} else {
				req.Dst = "::/0"
			}
		} else {
			dstAddr, err := parseIPNet(dst)
			if err != nil {
				return err
			}
			req.Dst = dstAddr.String()
		}
	}

	switch action {
	case "add":
		if _, err := c.client.AddRoute(c.ctx, &req); err != nil {
			return err
		}
	case "del":
		if _, err := c.client.DelRoute(c.ctx, &req); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid action: %s", action)
	}

	return nil
}

func parseIPNet(s string) (*net.IPNet, error) {
	if !strings.Contains(s, "/") {
		if net.ParseIP(s).To4() != nil {
			s += "/32"
		} else {
			s += "/128"
		}
	}
	return netlink.ParseIPNet(s)
}

func (c *Command) ShowInterfaces() error {
	resp, err := c.client.GetInterfaces(c.ctx, new(empty.Empty))
	if err != nil {
		return err
	}

	return printJSON(resp)
}

func (c *Command) UpdateInterfaceLinkState(action, ifname string) error {
	req := pb.LinkNameRequest{
		Name: ifname,
	}

	switch action {
	case "up":
		if _, err := c.client.SetInterfaceLinkUp(c.ctx, &req); err != nil {
			return err
		}
	case "down":
		if _, err := c.client.SetInterfaceLinkDown(c.ctx, &req); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid action: %s", action)
	}

	return nil
}

func (c *Command) UpdateInterfaceAddrList(action, ipcidr, ifname string) error {
	req := pb.IPAddrRequest{
		LinkName: ifname,
		Addr:     ipcidr,
	}

	switch action {
	case "add":
		if _, err := c.client.AddIPAddr(c.ctx, &req); err != nil {
			return err
		}
	case "del":
		if _, err := c.client.DelIPAddr(c.ctx, &req); err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid action: %s", action)
	}

	return nil
}
