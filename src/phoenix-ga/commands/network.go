package commands

import (
	"encoding/json"
	"net"
	"os"

	"github.com/vishvananda/netlink"
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

func NewRTNetlinkError(err error) error {
	return os.NewSyscallError("rtnetlink", err)
}

func IpAddrAdd(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Ifname string `json:"ifname"`
		IpCidr string `json:"ip"`
	}{}
	json.Unmarshal(*rawArgs, &args)
	iface, err := netlink.LinkByName(args.Ifname)
	if err != nil {
		cResp <- &Response{nil, tag, NewRTNetlinkError(err)}
		return
	}
	ip, err := netlink.ParseAddr(args.IpCidr)
	if err != nil {
		cResp <- &Response{nil, tag, NewRTNetlinkError(err)}
		return
	}
	if err := netlink.AddrAdd(iface, ip); err != nil {
		cResp <- &Response{nil, tag, NewRTNetlinkError(err)}
		return
	}
	cResp <- &Response{true, tag, nil}
}

func IpAddrDel(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Ifname string `json:"ifname"`
		IpCidr string `json:"ip"`
	}{}
	json.Unmarshal(*rawArgs, &args)
	iface, err := netlink.LinkByName(args.Ifname)
	if err != nil {
		cResp <- &Response{nil, tag, NewRTNetlinkError(err)}
		return
	}
	ip, err := netlink.ParseAddr(args.IpCidr)
	if err != nil {
		cResp <- &Response{nil, tag, NewRTNetlinkError(err)}
		return
	}
	if err := netlink.AddrDel(iface, ip); err != nil {
		cResp <- &Response{nil, tag, NewRTNetlinkError(err)}
		return
	}
	cResp <- &Response{true, tag, nil}
}

type IPNet net.IPNet

func (n IPNet) MarshalJSON() ([]byte, error) {
	t := struct {
		IP   net.IP `json:"ip"`
		Mask net.IP `json:"mask"`
	}{
		IP:   n.IP,
		Mask: net.IP(n.Mask),
	}
	return json.Marshal(t)
}

type Route struct {
	Ifname string        `json:"ifname"`
	Scope  netlink.Scope `json:"scope"`
	Dst    *IPNet        `json:"dst"`
	Src    net.IP        `json:"src"`
	Gw     net.IP        `json:"gateway"`
}

func GetRouteList(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Ifname string `json:"ifname"`
		Family string `json:"family"`
	}{}
	json.Unmarshal(*rawArgs, &args)
	rlist, err := netlink.RouteList(nil, 4)
	if err != nil {
		cResp <- &Response{nil, tag, NewRTNetlinkError(err)}
		return
	}
	rlist2 := make([]Route, 0, len(rlist))
	for _, r := range rlist {
		link, _ := net.InterfaceByIndex(r.LinkIndex)
		var n IPNet
		if r.Dst != nil {
			n = IPNet(*r.Dst)
		}
		r2 := Route{
			Ifname: link.Name,
			Scope:  r.Scope,
			Dst:    &n,
			Src:    r.Src,
			Gw:     r.Gw,
		}
		rlist2 = append(rlist2, r2)
	}
	cResp <- &Response{rlist2, tag, nil}
}

func NetIfaceUp(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Ifname string `json:"ifname"`
	}{}
	json.Unmarshal(*rawArgs, &args)
	iface := &netlink.Device{netlink.LinkAttrs{Name: args.Ifname}}
	if err := netlink.LinkSetUp(iface); err != nil {
		cResp <- &Response{nil, tag, NewRTNetlinkError(err)}
		return
	}
	cResp <- &Response{true, tag, nil}
}

func NetIfaceDown(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Ifname string `json:"ifname"`
	}{}
	json.Unmarshal(*rawArgs, &args)
	iface := &netlink.Device{netlink.LinkAttrs{Name: args.Ifname}}
	if err := netlink.LinkSetDown(iface); err != nil {
		cResp <- &Response{nil, tag, NewRTNetlinkError(err)}
		return
	}
	cResp <- &Response{true, tag, nil}
}
