package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/0xef53/phoenix-guest-agent/pkg/utmp"
	pb "github.com/0xef53/phoenix-guest-agent/protobufs/agent"
)

type StatPoller struct {
	mu   sync.Mutex
	stat *pb.GuestInfo
}

func (p *StatPoller) Stat() *pb.GuestInfo {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.stat
}

func (p *StatPoller) Run(ctx context.Context, interval time.Duration) error {
	p.stat = &pb.GuestInfo{
		Uname:   new(pb.GuestInfo_Utsname),
		Loadavg: new(pb.GuestInfo_LoadAverage),
		Mem:     new(pb.GuestInfo_MemStat),
		Swap:    new(pb.GuestInfo_SwapStat),
	}

	// float64(1<<SI_LOAD_SHIFT) == 65536.0
	scale := 65536.0

	update := func() error {
		p.mu.Lock()
		defer p.mu.Unlock()

		st := &syscall.Sysinfo_t{}
		if err := syscall.Sysinfo(st); err != nil {
			return err
		}

		p.stat.Uptime = st.Uptime

		p.stat.Loadavg.One = float64(st.Loads[0]) / scale
		p.stat.Loadavg.Five = float64(st.Loads[1]) / scale
		p.stat.Loadavg.Fifteen = float64(st.Loads[2]) / scale

		if err := getMemInfo(p.stat.Mem); err != nil {
			return err
		}

		unit := uint64(st.Unit) * 1024 // kB

		p.stat.Swap.Total = uint64(st.Totalswap) / unit
		p.stat.Swap.Free = uint64(st.Freeswap) / unit

		if err := getUname(p.stat.Uname); err != nil {
			return err
		}

		if dd, err := getBlkdevStat(); err == nil {
			p.stat.BlockDevices = dd
		} else {
			return err
		}

		if uu, err := getLoggedUsers(); err == nil {
			p.stat.Users = uu
		} else {
			return err
		}

		return nil
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

LOOP:
	for {
		select {
		case <-ctx.Done():
			break LOOP
		case <-ticker.C:
		}
		if err := update(); err != nil {
			return err
		}
	}

	return nil
}

func getMemInfo(mi *pb.GuestInfo_MemStat) error {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return err
	}
	defer f.Close()

	s := bufio.NewScanner(f)

	// number of fields we're interested in
	n := 4

	for s.Scan() && n > 0 {
		switch {
		case bytes.HasPrefix(s.Bytes(), []byte(`MemTotal:`)):
			_, err = fmt.Sscanf(s.Text(), "MemTotal:%d", &mi.Total)
		case bytes.HasPrefix(s.Bytes(), []byte(`MemFree:`)):
			_, err = fmt.Sscanf(s.Text(), "MemFree:%d", &mi.Free)
		case bytes.HasPrefix(s.Bytes(), []byte(`Buffers:`)):
			_, err = fmt.Sscanf(s.Text(), "Buffers:%d", &mi.Buffers)
		case bytes.HasPrefix(s.Bytes(), []byte(`Cached:`)):
			_, err = fmt.Sscanf(s.Text(), "Cached:%d", &mi.Cached)
		default:
			continue
		}

		if err != nil {
			return err
		}

		n--
	}
	if err = s.Err(); err != nil {
		return err
	}

	mi.FreeTotal = mi.Free + mi.Buffers + mi.Cached

	return nil
}

func getUname(u *pb.GuestInfo_Utsname) error {
	tmp := syscall.Utsname{}
	if err := syscall.Uname(&tmp); err != nil {
		return err
	}

	str := func(f [65]int8) string {
		out := make([]byte, 0, 64)
		for _, v := range f[:] {
			if v == 0 {
				break
			}
			out = append(out, uint8(v))
		}
		return string(out)
	}

	u.Sysname = str(tmp.Sysname)
	u.Nodename = str(tmp.Nodename)
	u.Release = str(tmp.Release)
	u.Version = str(tmp.Version)
	u.Machine = str(tmp.Machine)
	u.Domainname = str(tmp.Domainname)

	return nil

}

func getBlkdevStat() ([]*pb.GuestInfo_BlockDevice, error) {
	dir, err := ioutil.ReadDir("/sys/class/block")
	if err != nil {
		return nil, err
	}

	mounts, err := func() (map[string]string, error) {
		f, err := os.Open("/proc/self/mounts")
		if err != nil {
			return nil, err
		}
		defer f.Close()

		mm := make(map[string]string)

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) < 2 {
				continue
			}
			if _, ok := mm[fields[0]]; !ok {
				mm[fields[0]] = fields[1]
			}
		}
		if err = scanner.Err(); err != nil {
			return nil, err
		}
		return mm, nil
	}()
	if err != nil {
		return nil, err
	}

	devices := []*pb.GuestInfo_BlockDevice{}

	for _, file := range dir {
		if _, err := os.Stat(filepath.Join("/sys/class/block", file.Name(), "dev")); err != nil {
			continue
		}

		d := pb.GuestInfo_BlockDevice{
			Path: filepath.Join("/dev", file.Name()),
		}

		if _, ok := mounts[d.Path]; ok {
			d.IsMounted = true
			d.MountPoint = mounts[d.Path]
			st := syscall.Statfs_t{}
			if err := syscall.Statfs(d.MountPoint, &st); err != nil {
				return nil, err
			}
			d.SizeTotal = (int64(st.Bsize) * int64(st.Blocks)) / 1024
			d.SizeUsed = d.SizeTotal - (int64(st.Bsize)*int64(st.Bfree))/1024
			d.SizeAvail = (int64(st.Bsize) * int64(st.Bavail)) / 1024
			d.InodesTotal = int64(st.Files)
			d.InodesUsed = int64(st.Files - st.Ffree)
			d.InodesAvail = int64(st.Ffree)
		}

		devices = append(devices, &d)
	}

	return devices, nil
}

func getLoggedUsers() ([]*pb.GuestInfo_LoggedUser, error) {
	entries, err := utmp.ReadFile("/var/run/utmp")
	if err != nil {
		return nil, err
	}

	users := []*pb.GuestInfo_LoggedUser{}

	for _, entry := range entries {
		if entry.Type != utmp.UserProcess {
			continue
		}

		u := pb.GuestInfo_LoggedUser{
			Name:      string(bytes.Trim(entry.User[:], "\u0000")),
			Device:    string(bytes.Trim(entry.Device[:], "\u0000")),
			Host:      string(bytes.Trim(entry.Host[:], "\u0000")),
			LoginTime: time.Unix(int64(entry.Time.Sec), int64(entry.Time.Usec)).Unix(),
		}

		if u.Name != "" && u.Host != "" {
			users = append(users, &u)
		}
	}

	return users, nil
}
