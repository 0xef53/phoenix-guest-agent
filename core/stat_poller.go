package core

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/0xef53/phoenix-guest-agent/internal/utmp"

	systemd_login1 "github.com/coreos/go-systemd/v22/login1"
)

type StatPoller struct {
	mu   sync.Mutex
	stat *GuestInfo
}

func (p *StatPoller) Stat() *GuestInfo {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.stat
}

func (p *StatPoller) Run(ctx context.Context, interval time.Duration) error {
	p.stat = &GuestInfo{
		Uname:   new(GuestInfo_Utsname),
		Loadavg: new(GuestInfo_LoadAverage),
		Memory:  new(GuestInfo_MemStat),
		Swap:    new(GuestInfo_SwapStat),
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

		if err := p.updateMemStatFields(p.stat.Memory); err != nil {
			return err
		}

		unit := uint64(st.Unit) * 1024 // kB

		p.stat.Swap.Total = uint64(st.Totalswap) / unit
		p.stat.Swap.Free = uint64(st.Freeswap) / unit

		if err := p.updateUtsnameFields(p.stat.Uname); err != nil {
			return err
		}

		if dd, err := p.getBlockdevStat(); err == nil {
			p.stat.Blockdevs = dd
		} else {
			return err
		}

		if uu, err := p.getLoggedUsers(); err == nil {
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

func (p *StatPoller) updateMemStatFields(st *GuestInfo_MemStat) error {
	fd, err := os.Open("/proc/meminfo")
	if err != nil {
		return err
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)

	// number of fields we're interested in
	n := 4

	for scanner.Scan() && n > 0 {
		switch {
		case bytes.HasPrefix(scanner.Bytes(), []byte(`MemTotal:`)):
			_, err = fmt.Sscanf(scanner.Text(), "MemTotal:%d", &st.Total)
		case bytes.HasPrefix(scanner.Bytes(), []byte(`MemFree:`)):
			_, err = fmt.Sscanf(scanner.Text(), "MemFree:%d", &st.Free)
		case bytes.HasPrefix(scanner.Bytes(), []byte(`Buffers:`)):
			_, err = fmt.Sscanf(scanner.Text(), "Buffers:%d", &st.Buffers)
		case bytes.HasPrefix(scanner.Bytes(), []byte(`Cached:`)):
			_, err = fmt.Sscanf(scanner.Text(), "Cached:%d", &st.Cached)
		default:
			continue
		}

		if err != nil {
			return err
		}

		n--
	}
	if err = scanner.Err(); err != nil {
		return err
	}

	st.FreeTotal = st.Free + st.Buffers + st.Cached

	return nil
}

func (p *StatPoller) updateUtsnameFields(st *GuestInfo_Utsname) error {
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

	st.Sysname = str(tmp.Sysname)
	st.Nodename = str(tmp.Nodename)
	st.Release = str(tmp.Release)
	st.Version = str(tmp.Version)
	st.Machine = str(tmp.Machine)
	st.Domainname = str(tmp.Domainname)

	return nil

}

func (p *StatPoller) getBlockdevStat() ([]*GuestInfo_BlockDevice, error) {
	dir, err := os.ReadDir("/sys/class/block")
	if err != nil {
		return nil, err
	}

	mounts, err := func() (map[string]string, error) {
		fd, err := os.Open("/proc/self/mounts")
		if err != nil {
			return nil, err
		}
		defer fd.Close()

		mm := make(map[string]string)

		scanner := bufio.NewScanner(fd)

		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())

			if len(fields) < 2 {
				continue
			}

			var realname string

			switch s, err := filepath.EvalSymlinks(fields[0]); {
			case err == nil:
				realname = s
			case os.IsNotExist(err):
				realname = fields[0]
			default:
				return nil, err
			}

			if _, ok := mm[realname]; !ok {
				mm[realname] = fields[1]
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

	devices := make([]*GuestInfo_BlockDevice, 0, 1)

	for _, file := range dir {
		if _, err := os.Stat(filepath.Join("/sys/class/block", file.Name(), "dev")); err != nil {
			continue
		}

		d := GuestInfo_BlockDevice{
			Path: filepath.Join("/dev", file.Name()),
		}

		if _, ok := mounts[d.Path]; ok {
			d.IsMounted = true

			d.MountPoint = mounts[d.Path]

			st := syscall.Statfs_t{}

			if err := syscall.Statfs(d.MountPoint, &st); err != nil {
				return nil, err
			}

			d.SizeTotal = (uint64(st.Bsize) * st.Blocks) / 1024
			d.SizeUsed = d.SizeTotal - (uint64(st.Bsize)*st.Bfree)/1024
			d.SizeAvail = (uint64(st.Bsize) * st.Bavail) / 1024
			d.InodesTotal = st.Files
			d.InodesUsed = st.Files - st.Ffree
			d.InodesAvail = st.Ffree
		}

		devices = append(devices, &d)
	}

	return devices, nil
}

func (p *StatPoller) getLoggedUsers() ([]*GuestInfo_LoggedUser, error) {
	// New method, but for now, we only use it
	// if the file "/var/run/utmp" option doesn't work
	viaSystemd := func() ([]*GuestInfo_LoggedUser, error) {
		ctx := context.Background()

		conn, err := systemd_login1.New()
		if err != nil {
			return nil, err
		}
		defer conn.Close()

		sessions, err := conn.ListSessionsContext(ctx)
		if err != nil {
			return nil, err
		}

		users := make([]*GuestInfo_LoggedUser, 0, len(sessions))

		for _, session := range sessions {
			u := GuestInfo_LoggedUser{
				Name: session.User,
			}

			props, err := conn.GetSessionPropertiesContext(ctx, session.Path)
			if err != nil {
				return nil, err
			}

			if x, ok := props["TTY"]; ok {
				u.Device = strings.Trim(x.String(), "\"")
			}

			if len(u.Device) == 0 {
				var service, scope string

				if x, ok := props["Service"]; ok {
					service = strings.Trim(x.String(), "\"")
				}

				if x, ok := props["Scope"]; ok {
					scope = strings.Trim(x.String(), "\"")
				}

				u.Device = fmt.Sprintf("%s:%s", service, scope)
			}

			if x, ok := props["RemoteHost"]; ok {
				u.Host = strings.Trim(x.String(), "\"")
			}

			if x, ok := props["Timestamp"]; ok {
				if tsMicro, ok := x.Value().(uint64); ok {
					sec := int64(tsMicro / 1_000_000)
					nsec := int64((tsMicro % 1_000_000) * 1000)

					u.LoginTime = time.Unix(sec, nsec).Unix()
				}
			}

			if u.Name != "" && u.Host != "" && u.Device != "hvc0" {
				users = append(users, &u)
			}
		}

		return users, nil
	}

	// A legacy method, but we'll try it first
	viaUtmpFile := func() ([]*GuestInfo_LoggedUser, error) {
		entries, err := utmp.ReadFile("/var/run/utmp")
		if err != nil {
			return nil, err
		}

		users := make([]*GuestInfo_LoggedUser, 0, len(entries))

		for _, entry := range entries {
			if entry.Type != utmp.UserProcess {
				continue
			}

			u := GuestInfo_LoggedUser{
				Name:      string(bytes.Trim(entry.User[:], "\u0000")),
				Device:    string(bytes.Trim(entry.Device[:], "\u0000")),
				Host:      string(bytes.Trim(entry.Host[:], "\u0000")),
				LoginTime: time.Unix(int64(entry.Time.Sec), int64(entry.Time.Usec)).Unix(),
			}

			if u.Name != "" && u.Host != "" && u.Device != "hvc0" {
				users = append(users, &u)
			}
		}

		return users, nil
	}

	users, err := viaUtmpFile()
	if err != nil {
		if os.IsNotExist(err) {
			return viaSystemd()
		}
	}

	return users, nil
}
