package main

import (
	"context"
	"flag"
	"fmt"
	"runtime"
	"strings"

	pb "github.com/0xef53/phoenix-guest-agent/protobufs/agent"
)

type Command struct {
	ctx    context.Context
	client pb.AgentServiceClient
}

func ExecuteCommand(args []string) error {
	switch strings.Join(args, " ") {
	case "version":
		fmt.Printf("v%s (built w/%s)\n", VERSION, runtime.Version())
		return nil
	}

	client, err := newClient(args[0])
	if err != nil {
		return err
	}
	args = args[1:]

	if len(args) == 0 {
		flag.Usage()
		return nil
	}

	cmd := Command{
		ctx:    context.Background(),
		client: client,
	}

	// Commands WITHOUT variable arguments
	switch strings.Join(args, " ") {
	case "agent-shutdown":
		return cmd.ShutdownAgent()
	case "agent-info":
		return cmd.ShowAgentInfo()
	case "guest-info":
		return cmd.ShowGuestInfo()
	}

	// Commands WITH variable arguments
	switch {
	// network
	case argsMatch("ip -4 r|ro|route l|list", args), argsMatch("ip r|ro|route l|list", args):
		return cmd.ShowRouteList("4")
	case argsMatch("ip -6 r|ro|route l|list", args):
		return cmd.ShowRouteList("6")
	case argsMatch("ip a|addr s|show", args):
		return cmd.ShowInterfaces()
	case argsMatch("ip addr add|del ADDR dev IFNAME", args, 3, 5):
		return cmd.UpdateInterfaceAddrList(args[2], args[3], args[5])
	case argsMatch("ip l|link set up|down dev IFNAME", args, 5):
		return cmd.UpdateInterfaceLinkState(args[3], args[5])
	case argsMatch("ip r|ro|route add|del PREFIX via ADDR dev IFNAME", args, 3, 5, 7):
		return cmd.UpdateRouteTable(args[2], args[3], args[5], args[7])
	case argsMatch("ip r|ro|route add|del PREFIX dev IFNAME", args, 3, 5):
		return cmd.UpdateRouteTable(args[2], args[3], "", args[5])

	// file system
	case argsMatch("fs-freeze", args):
		return cmd.FreezeAll()
	case argsMatch("fs-unfreeze", args):
		return cmd.UnfreezeAll()

	case argsMatch("md5sum FILE", args, 1):
		return cmd.ShowFileMD5Hash(args[1])
	case argsMatch("mkdir DIRECTORY", args, 1):
		return cmd.CreateDir(args[1], "0755")
	case argsMatch("mkdir -m MODE DIRECTORY", args, 2, 3):
		return cmd.CreateDir(args[3], args[2])
	case argsMatch("chmod MODE FILE", args, 1, 2):
		return cmd.SetFileMode(args[2], args[1])
	case argsMatch("chown OWNER_GROUP FILE", args, 1, 2):
		return cmd.SetFileOwner(args[2], args[1])
	case argsMatch("rcp SRCFILE DSTFILE", args, 1, 2):
		return cmd.CopyFile(args[1], args[2])
	case argsMatch("cat FILE", args, 1):
		return cmd.ShowFileContent(args[1])
	case args[0] == "ls":
		var useLongFormat, withoutContent bool
		lscmd := flag.NewFlagSet("", flag.ExitOnError)
		lscmd.BoolVar(&useLongFormat, "l", useLongFormat, "use a long listing format")
		lscmd.BoolVar(&withoutContent, "d", withoutContent, "list directory entries instead of contents")
		lscmd.Parse(args[1:])
		return cmd.ShowFileStat(lscmd.Arg(0), useLongFormat, withoutContent)
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
