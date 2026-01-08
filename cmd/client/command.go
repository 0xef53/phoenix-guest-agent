package main

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/0xef53/phoenix-guest-agent/cert"
	"github.com/0xef53/phoenix-guest-agent/client"
)

func ExecuteCommand(args []string) error {
	var crtStore cert.Store

	if v := os.Getenv("CERTDIR"); len(v) != 0 {
		crtStore = cert.Dir(v)
	} else {
		crtStore = cert.EmbedStore
	}

	endpoint := args[0]

	client, err := client.NewClient(endpoint, crtStore)
	if err != nil {
		return err
	}

	ctx := context.Background()

	switch strings.Join(args, " ") {
	case "version":
		return client.ShowVersion(ctx)
	}

	// Shift first argument which is endpoint
	args = args[1:]

	if len(args) == 0 {
		flag.Usage()

		return nil
	}

	// Commands WITHOUT variable arguments
	switch strings.Join(args, " ") {
	case "agent-shutdown":
		return client.ShutdownAgent(ctx)
	case "agent-info":
		return client.ShowAgentInfo(ctx)
	case "guest-info":
		return client.ShowGuestInfo(ctx)
	case "ssh":
		return client.ExecSecureShellClient(ctx)
	}

	// Commands WITH variable arguments
	switch {
	// network
	case argsMatch("ip -4 r|ro|route l|list", args), argsMatch("ip r|ro|route l|list", args):
		return client.ShowRouteList(ctx, "4")
	case argsMatch("ip -6 r|ro|route l|list", args):
		return client.ShowRouteList(ctx, "6")
	case argsMatch("ip a|addr s|show", args):
		return client.ShowInterfaces(ctx)
	case argsMatch("ip addr add|del ADDR dev IFNAME", args, 3, 5):
		return client.UpdateInterfaceAddrList(ctx, args[2], args[3], args[5])
	case argsMatch("ip l|link set up|down dev IFNAME", args, 5):
		return client.UpdateInterfaceLinkState(ctx, args[3], args[5])
	case argsMatch("ip r|ro|route add|del PREFIX via ADDR dev IFNAME", args, 3, 5, 7):
		return client.UpdateRouteTable(ctx, args[2], args[3], args[5], args[7])
	case argsMatch("ip r|ro|route add|del PREFIX dev IFNAME", args, 3, 5):
		return client.UpdateRouteTable(ctx, args[2], args[3], "", args[5])

	// file system
	case argsMatch("fs-freeze", args):
		return client.FreezeAll(ctx)
	case argsMatch("fs-unfreeze", args):
		return client.UnfreezeAll(ctx)

	case argsMatch("md5sum FILE", args, 1):
		return client.ShowFileMD5Hash(ctx, args[1])
	case argsMatch("mkdir DIRECTORY", args, 1):
		return client.CreateDir(ctx, args[1], "0755")
	case argsMatch("mkdir -m MODE DIRECTORY", args, 2, 3):
		return client.CreateDir(ctx, args[3], args[2])
	case argsMatch("chmod MODE FILE", args, 1, 2):
		return client.SetFileMode(ctx, args[2], args[1])
	case argsMatch("chown OWNER_GROUP FILE", args, 1, 2):
		return client.SetFileOwner(ctx, args[2], args[1])
	case argsMatch("rcp SRCFILE DSTFILE", args, 1, 2):
		return client.CopyFile(ctx, args[1], args[2])
	case argsMatch("cat FILE", args, 1):
		return client.ShowFileContent(ctx, args[1])
	case args[0] == "ls":
		var useLongFormat, withoutContent bool

		lscmd := flag.NewFlagSet("", flag.ExitOnError)
		lscmd.BoolVar(&useLongFormat, "l", useLongFormat, "use a long listing format")
		lscmd.BoolVar(&withoutContent, "d", withoutContent, "list directory entries instead of contents")
		lscmd.Parse(args[1:])

		return client.ShowFileStat(ctx, lscmd.Arg(0), useLongFormat, withoutContent)
	}

	printSectionUsage(getFirstN(strings.Join(args, " "), 3))

	return nil
}

// argsMatch checks whether s1 and s2 length and values match.
// Values at positions in ignore are skipped from comparison.
func argsMatch(s1 string, s2 []string, ignore ...int) bool {
	f1 := strings.Fields(s1)
	if len(f1) != len(s2) {
		return false
	}

	contains := func(s []string, e string) bool {
		for _, i := range s {
			if i == e {
				return true
			}
		}
		return false
	}

cmpLoop:
	for i := range f1 {
		for _, i2 := range ignore {
			if i == i2 {
				continue cmpLoop
			}
		}

		vars := strings.Split(f1[i], "|")
		if !contains(vars, s2[i]) {
			return false
		}
	}
	return true
}

func getFirstN(s string, n int) string {
	if len(s) < n {
		n = len(s)
	}
	return s[:n]
}
