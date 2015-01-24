package commands

import (
	"encoding/json"
	"net"
	"os"
	"syscall"
	"unsafe"

	"github.com/docker/libcontainer/network"
)

type NetIf struct {
	Index  int      `json:"index"`
	Name   string   `json:"name"`
	Hwaddr string   `json:"hwaddr"`
	Flags  string   `json:"flags"`
	Ips    []string `json:"ips"`
}

func GetNetIfaces(cResp chan<- *Response, args *json.RawMessage, tag string) {
	ifaces, err := net.Interfaces()
	if err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	iflist := make([]*NetIf, 0, len(ifaces))
	for _, netif := range ifaces {
		addrs, err := netif.Addrs()
		if err != nil {
			cResp <- &Response{nil, tag, err}
			return
		}
		str_addrs := make([]string, 0, len(addrs))
		for _, addr := range addrs {
			str_addrs = append(str_addrs, addr.String())
		}
		iflist = append(iflist, &NetIf{
			netif.Index,
			netif.Name,
			netif.HardwareAddr.String(),
			netif.Flags.String(),
			str_addrs,
		})
	}
	cResp <- &Response{&iflist, tag, nil}
}

func IpAddrAdd(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Dev    string `json:"dev"`
		IpCidr string `json:"ip"`
	}{}
	json.Unmarshal(*rawArgs, &args)
	if err := network.SetInterfaceIp(args.Dev, args.IpCidr); err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	cResp <- &Response{true, tag, nil}
}

func IpAddrDel(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Dev    string `json:"dev"`
		IpCidr string `json:"ip"`
	}{}
	json.Unmarshal(*rawArgs, &args)
	if err := network.DeleteInterfaceIp(args.Dev, args.IpCidr); err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	cResp <- &Response{true, tag, nil}
}

func GetDefaultGateways(cResp chan<- *Response, args *json.RawMessage, tag string) {
	tab, err := syscall.NetlinkRIB(syscall.RTM_GETROUTE, syscall.AF_UNSPEC)
	if err != nil {
		cResp <- &Response{nil, tag, os.NewSyscallError("netlink rib", err)}
		return
	}
	msgs, err := syscall.ParseNetlinkMessage(tab)
	if err != nil {
		cResp <- &Response{nil, tag, os.NewSyscallError("netlink message", err)}
		return
	}
	var gateways []string
	var ip net.IP
loop:
	for _, m := range msgs {
		switch m.Header.Type {
		case syscall.NLMSG_DONE:
			break loop
		case syscall.RTM_NEWROUTE:
			msg := (*syscall.RtMsg)(unsafe.Pointer(&m.Data[0]))
			// Leave only default routes from main table
			if msg.Table != syscall.RT_TABLE_MAIN || msg.Dst_len != 0 {
				continue
			}
			// Leave only ipv4/ipv6 routes
			if msg.Family != syscall.AF_INET && msg.Family != syscall.AF_INET6 {
				continue
			}
			attrs, err := syscall.ParseNetlinkRouteAttr(&m)
			if err != nil {
				cResp <- &Response{nil, tag, os.NewSyscallError("netlink message payload", err)}
				return
			}
			for _, attr := range attrs {
				if attr.Attr.Type == syscall.RTA_GATEWAY {
					ip = net.IP(attr.Value)
					gateways = append(gateways, ip.String())
				}
			}
		}
	}
	cResp <- &Response{&gateways, tag, nil}
}

func NetIfaceUp(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Dev string `json:"dev"`
	}{}
	json.Unmarshal(*rawArgs, &args)
	if err := network.InterfaceUp(args.Dev); err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	cResp <- &Response{true, tag, nil}
}

func NetIfaceDown(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Dev string `json:"dev"`
	}{}
	json.Unmarshal(*rawArgs, &args)
	if err := network.InterfaceDown(args.Dev); err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	cResp <- &Response{true, tag, nil}
}
