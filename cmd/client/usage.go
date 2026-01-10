package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var usageContent = [][]string{
	{
		"version",
		"print version",
	},
	{
		"agent-info",
		"print information about the Agent running inside a guest",
	},
	{
		"guest-info",
		"print system information and statistics of a guest OS",
		"(uptime, la, mem/swap/disks statistics, logged-in users)",
	},
	{
		"secure-shell [-u username] [--shell SHELL] -- [command [argument ...]]",
		"start SSH connection to the built-in PGA Secure Shell Server",
	},
	{
		"ip addr show",
		"print summary info about network interfaces",
	},
	{
		"ip addr add|del ADDR dev IFNAME",
		"add or remove IPv4/IPv6 address",
	},
	{
		"ip link set up|down dev IFNAME",
		"bring interface link up or down",
	},
	{
		"ip [-4|-6] route list",
		"print the routing table entries",
	},
	{
		"ip route add|del PREFIX via GWADDR dev IFNAME",
		"add or remove route",
	},
	{
		"ls [-l] [-d] FILE|DIRECTORY",
		"print file stat or directory content",
	},
	{
		"cat FILE",
		"print file content",
	},
	{
		"mkdir [-m OCTAL-MODE] DIRECTORY",
		"create new directory",
	},
	{
		"chmod OCTAL-MODE FILE|DIRECTORY",
		"change file/directory mode bits",
	},
	{
		"chown OWNER:GROUP FILE|DIRECTORY",
		"change file/directory owner and group",
	},
	{
		"md5sum FILE",
		"print file md5 checksum",
	},
	{
		"rcp guest:SRCFILE DSTFILE",
		"copy file from a guest system",
	},
	{
		"rcp SRCFILE|- guest:DSTFILE",
		"copy local file to a guest system",
	},
	{
		"fs-freeze",
		"sync and freeze all freezable guest filesystems",
	},
	{
		"fs-unfreeze",
		"unfreeze all frozen guest filesystems",
	},
}

func usage() {
	printSectionUsage("")
}

func printSectionUsage(prefix string) {
	s := fmt.Sprintf("Usage:\n  %s [options] ENDPOINT command [args]\n\n", filepath.Base(os.Args[0]))
	s += "Commands:\n"

	var c [][]string

	for _, v := range usageContent {
		if len(v) >= 2 {
			if strings.HasPrefix(v[0], prefix) {
				c = append(c, v)
			}
		}
	}

	if len(c) == 0 {
		c = usageContent
	}

	for _, v := range c {
		s += "  " + v[0] + "\n"
		for _, h := range v[1:] {
			s += "      " + h + "\n"
		}
		s += "\n"
	}

	fmt.Fprintf(os.Stderr, "%s", s)

	os.Exit(2)
}
