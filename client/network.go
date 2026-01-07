package client

import (
	"context"
	"fmt"
	"syscall"

	grpc_interfaces "github.com/0xef53/phoenix-guest-agent/internal/grpc/interfaces"

	pb_agent "github.com/0xef53/phoenix-guest-agent/api/services/agent/v2"
	pb_types "github.com/0xef53/phoenix-guest-agent/api/types/v2"

	empty "github.com/golang/protobuf/ptypes/empty"

	"github.com/vishvananda/netlink"
)

func (c *client) ShowRouteList(ctx context.Context, family string) error {
	req := pb_agent.GetRouteListRequest{}

	switch family {
	case "6":
		req.Family = syscall.AF_INET6
	default:
		req.Family = syscall.AF_INET
	}

	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		resp, err := grpcClient.Network().GetRouteList(ctx, &req)
		if err != nil {
			return err
		}

		return PrintJSON(resp)
	})
}

func (c *client) UpdateRouteTable(ctx context.Context, action, dst, gw, ifname string) error {
	req := pb_agent.RouteRequest{
		LinkName: ifname,
	}

	if len(gw) == 0 {
		// link route
		req.Scope = pb_types.RouteScope(netlink.SCOPE_LINK)

		dstAddr, err := ParseIPNet(dst)
		if err != nil {
			return err
		}

		req.Dst = dstAddr.String()
	} else {
		// route via gateway
		gwAddr, err := ParseIPNet(gw)
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
			dstAddr, err := ParseIPNet(dst)
			if err != nil {
				return err
			}

			req.Dst = dstAddr.String()
		}
	}

	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) (err error) {
		switch action {
		case "add":
			_, err = grpcClient.Network().AddRoute(ctx, &req)
		case "del":
			_, err = grpcClient.Network().DelRoute(ctx, &req)
		default:
			return fmt.Errorf("invalid action: %s", action)
		}

		return err
	})
}

func (c *client) ShowInterfaces(ctx context.Context) error {
	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) error {
		resp, err := grpcClient.Network().GetInterfaces(ctx, new(empty.Empty))
		if err != nil {
			return err
		}

		return PrintJSON(resp)
	})
}

func (c *client) UpdateInterfaceLinkState(ctx context.Context, action, ifname string) error {
	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) (err error) {
		req := pb_agent.SetInterfaceLinkStateRequest{
			LinkName: ifname,
		}

		switch action {
		case "up":
			_, err = grpcClient.Network().SetInterfaceLinkUp(ctx, &req)
		case "down":
			_, err = grpcClient.Network().SetInterfaceLinkDown(ctx, &req)
		default:
			return fmt.Errorf("invalid action: %s", action)
		}

		return err
	})
}

func (c *client) UpdateInterfaceAddrList(ctx context.Context, action, ipcidr, ifname string) error {
	return c.executeGRPC(ctx, func(grpcClient *grpc_interfaces.Agent) (err error) {
		req := pb_agent.IPAddrRequest{
			LinkName: ifname,
			Addr:     ipcidr,
		}

		switch action {
		case "add":
			_, err = grpcClient.Network().AddIPAddr(ctx, &req)
		case "del":
			_, err = grpcClient.Network().DelIPAddr(ctx, &req)
		default:
			return fmt.Errorf("invalid action: %s", action)
		}

		return err
	})
}
