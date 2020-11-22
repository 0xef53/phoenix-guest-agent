package main

import (
	"fmt"
	"net"
	"os"

	"github.com/0xef53/phoenix-guest-agent/pkg/cloudinit"

	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

func netinitConfigureInterface(ifname string) error {
	if found, err := cloudinit.IsNoCloudMarkerPresent(); err == nil {
		if !found {
			return fmt.Errorf("unable to find the cloud-init NoCloud datasource marker")
		}
	} else {
		return err
	}

	link, err := netlink.LinkByName(ifname)
	if err != nil {
		return os.NewSyscallError("rtnetlink: not found", err)
	}

	if link.Type() != "device" {
		return fmt.Errorf("not a physical device: %s", ifname)
	}

	assigned, err := func() (map[string]struct{}, error) {
		m := make(map[string]struct{})
		addrs, err := netlink.AddrList(link, 0)
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			m[addr.IPNet.String()] = struct{}{}
		}
		return m, nil
	}()
	if err != nil {
		return fmt.Errorf("unable to get a list of assigned IP addresses: %s", ifname)
	}

	// Cloud-Init conf
	log.Debugf("looking for cloud-init configuration for %s (mac = %s)", ifname, link.Attrs().HardwareAddr)

	iconf, err := func() (*cloudinit.EthernetConfig, error) {
		data, err := cloudinit.ReadData()
		if err != nil {
			return nil, err
		}
		if data.Network == nil {
			return nil, fmt.Errorf("cloud-init network-config is empty")
		}

		for _, v := range data.Network.Ethernets {
			if v.Match.MacAddress == link.Attrs().HardwareAddr.String() {
				return &v, nil
			}
		}

		return nil, fmt.Errorf("unable to find configuration for %s by mac-address", ifname)
	}()
	if err != nil {
		return err
	}

	log.Debugf("bringing interface %s up", ifname)
	if err := setInterfaceLinkUp(ifname); err != nil {
		return fmt.Errorf("failed to change the link state: %s", err)
	}

	var ip4addrs, ip6addrs []*net.IPNet

	// IP addresses
	for _, ipstr := range iconf.Addresses {
		ip, ipnet, err := ParseCIDR(ipstr)
		if err != nil {
			return err
		}
		ipnet.IP = ip

		if ipnet.IP.To4() != nil {
			ip4addrs = append(ip4addrs, ipnet)
		} else {
			ip6addrs = append(ip6addrs, ipnet)
		}

		if _, ok := assigned[ipstr]; ok {
			log.Debugf("address already assigned: %s", ipstr)
		} else {
			log.Debugf("assigning an IP address: %s", ipstr)
			if err := updateAddrList("add", ifname, ipstr); err != nil {
				return fmt.Errorf("failed to assign: %s", err)
			}
		}
	}

	// IPv4 gateway
	if len(ip4addrs) > 0 && len(iconf.Gateway4) > 0 {
		log.Debugf("configuring an IPv4 gateway via %s", iconf.Gateway4)
		if ones, _ := ip4addrs[0].Mask.Size(); ones == 32 {
			// link route
			if err := updateRouteTable("replace", link, iconf.Gateway4+"/32", "", "", netlink.SCOPE_LINK, 0); err != nil {
				return fmt.Errorf("failed to add a link route (%s via device %s): %s", iconf.Gateway4, ifname, err)
			}
		}
		if err := updateRouteTable("replace", link, "0.0.0.0/0", "", iconf.Gateway4, 0, 0); err != nil {
			return fmt.Errorf("failed to add default route: %s", err)
		}
	}

	// IPv6 gateway
	if len(ip6addrs) > 0 && len(iconf.Gateway6) > 0 {
		log.Debugf("configuring an IPv6 gateway via %s", iconf.Gateway6)
		if err := updateRouteTable("replace", link, "::/0", "", iconf.Gateway6, 0, 0); err != nil {
			return fmt.Errorf("failed to add default route: %s", err)
		}
	}

	// Routes
	for _, r := range iconf.Routes {
		log.Debugf("adding a route: %s via %s", r.To, r.Via)
		if err := updateRouteTable("add", link, r.To, "", r.Via, 0, 0); err != nil {
			log.Errorf("failed to add route: %s", err)
		}
	}

	return nil
}

func netinitDeconfigureInterface(ifname string) error {
	link, err := netlink.LinkByName(ifname)
	if err != nil {
		return os.NewSyscallError("rtnetlink: not found", err)
	}

	if link.Type() != "device" {
		return fmt.Errorf("not a physical device: %s", ifname)
	}

	log.Debugf("bringing interface %s down", ifname)
	if err := setInterfaceLinkDown(ifname); err != nil {
		return fmt.Errorf("failed to change the link state: %s", err)
	}

	assigned, err := netlink.AddrList(link, 0)
	if err != nil {
		return err
	}

	for _, addr := range assigned {
		log.Debugf("removing an IP address: %s", addr.IPNet)
		if err := updateAddrList("del", ifname, addr.IPNet.String()); err != nil {
			return fmt.Errorf("failed to remove: %s", err)
		}
	}

	return nil
}
