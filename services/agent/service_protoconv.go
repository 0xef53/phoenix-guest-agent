package agent

import (
	"github.com/0xef53/phoenix-guest-agent/core"

	pb_types "github.com/0xef53/phoenix-guest-agent/api/types/v2"
)

func guestInfoToProto(info *core.GuestInfo) *pb_types.GuestInfo {
	proto := pb_types.GuestInfo{
		Uptime:       info.Uptime,
		Users:        make([]*pb_types.GuestInfo_LoggedUser, 0, len(info.Users)),
		BlockDevices: make([]*pb_types.GuestInfo_BlockDevice, 0, len(info.Blockdevs)),
	}

	if info.Uname != nil {
		proto.Uname = &pb_types.GuestInfo_Utsname{
			Sysname:    info.Uname.Sysname,
			Nodename:   info.Uname.Nodename,
			Release:    info.Uname.Release,
			Version:    info.Uname.Version,
			Machine:    info.Uname.Machine,
			Domainname: info.Uname.Domainname,
		}
	}

	if info.Loadavg != nil {
		proto.Loadavg = &pb_types.GuestInfo_LoadAverage{
			One:     info.Loadavg.One,
			Five:    info.Loadavg.Five,
			Fifteen: info.Loadavg.Fifteen,
		}
	}

	if info.Memory != nil {
		proto.Mem = &pb_types.GuestInfo_MemStat{
			Total:     info.Memory.Total,
			Free:      info.Memory.Free,
			Buffers:   info.Memory.Buffers,
			Cached:    info.Memory.Cached,
			FreeTotal: info.Memory.FreeTotal,
		}
	}

	if info.Swap != nil {
		proto.Swap = &pb_types.GuestInfo_SwapStat{
			Total: info.Swap.Total,
			Free:  info.Swap.Free,
		}
	}

	for _, u := range info.Users {
		proto.Users = append(proto.Users, &pb_types.GuestInfo_LoggedUser{
			Name:      u.Name,
			Device:    u.Device,
			Host:      u.Host,
			LoginTime: u.LoginTime,
		})
	}

	for _, d := range info.Blockdevs {
		proto.BlockDevices = append(proto.BlockDevices, &pb_types.GuestInfo_BlockDevice{
			Path:        d.Path,
			IsMounted:   d.IsMounted,
			MountPoint:  d.MountPoint,
			SizeTotal:   int64(d.SizeTotal),
			SizeUsed:    int64(d.SizeUsed),
			SizeAvail:   int64(d.SizeAvail),
			InodesTotal: int64(d.InodesTotal),
			InodesUsed:  int64(d.InodesUsed),
			InodesAvail: int64(d.InodesAvail),
		})
	}

	return &proto
}
