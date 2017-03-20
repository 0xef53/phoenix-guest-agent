package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type SysInfo struct {
	Uname   *Utsname      `json:"uname"`    // utsname struct
	LongBit int           `json:"long_bit"` // getconf LONG_BIT
	Uptime  time.Duration `json:"uptime"`   // time since boot
	Loadavg LoadAverage   `json:"loadavg"`  // 1, 5, and 15 minute load averages
	Mem     MemStat       `json:"ram"`      // memory stat (total/free/buffers/cached) in kB
	Swap    SwapStat      `json:"swap"`     // swap stat (total/free) in kB
	Users   []LoggedUser  `json:"users"`    // logged-in users from /var/run/utmp
	Disks   []BlockDev    `json:"disks"`    // some disks stats
}

type Utsname struct {
	Sysname    string `json:"sysname"`
	Nodename   string `json:"nodename"`
	Release    string `json:"release"`
	Version    string `json:"version"`
	Machine    string `json:"machine"`
	Domainname string `json:"domain"`
}

type LoadAverage struct {
	One     float64 `json:"1m"`
	Five    float64 `json:"5m"`
	Fifteen float64 `json:"15m"`
}

type MemStat struct {
	Total     uint64 `json:"total"`
	Free      uint64 `json:"free"`
	Buffers   uint64 `json:"buffers"`
	Cached    uint64 `json:"cached"`
	FreeTotal uint64 `json:"free_total"`
}

type SwapStat struct {
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
}

type LoggedUser struct {
	Name      string `json:"name"`
	Device    string `json:"device"`
	Host      string `json:"host"`
	LoginTime int64  `json:"login_time"`
}

type BlockDev struct {
	Path       string `json:"name"`
	IsMounted  bool   `json:"is_mounted"`
	Mountpoint string `json:"mountpoint"`
	SizeTotal  int64  `json:"size_total"` // in kB
	SizeUsed   int64  `json:"size_used"`  // in kB
	SizeAvail  int64  `json:"size_avail"` // in kB
}

// Values for Utmp.Type field
type Utype int16

// Type for ut_exit, below
const (
	Empty        Utype = iota // record does not contain valid info (formerly known as UT_UNKNOWN on Linux)
	RunLevel           = iota // change in system run-level (see init(8))
	BootTime           = iota // time of system boot (in ut_tv)
	NewTime            = iota // time after system clock change (in ut_tv)
	OldTime            = iota // time before system clock change (in ut_tv)
	InitProcess        = iota // process spawned by init(8)
	LoginProcess       = iota // session leader process for user login
	UserProcess        = iota // normal process
	DeadProcess        = iota // terminated process
	Accounting         = iota // not implemented

	LineSize = 32
	NameSize = 32
	HostSize = 256
)

type ExitStatus struct {
	Termination int16 `json:"termination"` // process termination status
	Exit        int16 `json:"exit"`        // process exit status
}

type TimeVal struct {
	Sec  int32 `json:"seconds"`
	Usec int32 `json:"microseconds"`
}

// http://man7.org/linux/man-pages/man5/utmp.5.html
type Utmp struct {
	Type    Utype          // type of record
	_       int16          // padding because Go doesn't 4-byte align
	Pid     int32          // PID of login process
	Device  [LineSize]byte // device name of tty - "/dev/"
	Id      [4]byte        // terminal name suffix or inittab(5) ID
	User    [NameSize]byte // username
	Host    [HostSize]byte // hostname for remote login or kernel version for run-level messages
	Exit    ExitStatus     // exit status of a process marked as DeadProcess; not used by Linux init(1)
	Session int32          // session ID (getsid(2)), used for windowing
	Time    TimeVal        // time entry was made
	Addr    [4]int32       // internet address of remote host; IPv4 address uses just Addr[0]
	Unused  [20]byte       // reserved for future use
}

func GetSystemInfo(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	st := &syscall.Sysinfo_t{}

	if err := syscall.Sysinfo(st); err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}

	sinfo := &SysInfo{}
	sinfo.Uptime = time.Duration(st.Uptime)

	// float64(1<<SI_LOAD_SHIFT) == 65536.0
	scale := 65536.0
	sinfo.Loadavg.One = float64(st.Loads[0]) / scale
	sinfo.Loadavg.Five = float64(st.Loads[1]) / scale
	sinfo.Loadavg.Fifteen = float64(st.Loads[2]) / scale

	if err := getMemInfo(&sinfo.Mem); err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}

	unit := uint64(st.Unit) * 1024 // kB

	sinfo.Swap.Total = uint64(st.Totalswap) / unit
	sinfo.Swap.Free = uint64(st.Freeswap) / unit

	sinfo.LongBit = getLongBit()

	switch u, err := getUname(); {
	case err == nil:
		sinfo.Uname = u
	default:
		cResp <- &Response{nil, tag, err}
		return
	}

	switch u, err := getLoggedUsers(); {
	case err == nil:
		sinfo.Users = u
	default:
		cResp <- &Response{nil, tag, err}
		return
	}

	switch d, err := getDisksInfo(); {
	case err == nil:
		sinfo.Disks = d
	default:
		cResp <- &Response{nil, tag, err}
		return
	}

	cResp <- &Response{sinfo, tag, nil}
}

func getLongBit() int {
	out, err := exec.Command("/usr/bin/getconf", "LONG_BIT").Output()
	if err != nil {
		fmt.Println(err)
		return 0
	}

	i, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}

	return i
}

func getUname() (*Utsname, error) {
	u := syscall.Utsname{}
	if err := syscall.Uname(&u); err != nil {
		return nil, err
	}

	toString := func(f [65]int8) string {
		out := make([]byte, 0, 64)
		for _, v := range f[:] {
			if v == 0 {
				break
			}
			out = append(out, uint8(v))
		}
		return string(out)
	}

	uname := Utsname{
		Sysname:    toString(u.Sysname),
		Nodename:   toString(u.Nodename),
		Release:    toString(u.Release),
		Version:    toString(u.Version),
		Machine:    toString(u.Machine),
		Domainname: toString(u.Domainname),
	}

	return &uname, nil

}

func getLoggedUsers() ([]LoggedUser, error) {
	f, err := os.Open("/var/run/utmp")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	users := []LoggedUser{}

	for {
		entry := new(Utmp)

		err := binary.Read(f, binary.LittleEndian, entry)
		if err != nil && err != io.EOF {
			continue
		}
		if err == io.EOF {
			break
		}

		if entry.Type != UserProcess {
			continue
		}

		u := LoggedUser{
			Name:      string(bytes.Trim(entry.User[:], "\u0000")),
			Device:    string(bytes.Trim(entry.Device[:], "\u0000")),
			Host:      string(bytes.Trim(entry.Host[:], "\u0000")),
			LoginTime: time.Unix(int64(entry.Time.Sec), int64(entry.Time.Usec)).Unix(),
		}

		if u.Name != "" && u.Host != "" {
			users = append(users, u)
		}
	}

	return users, nil

}

func getDisksInfo() ([]BlockDev, error) {
	dir, err := ioutil.ReadDir("/sys/block")
	if err != nil {
		return nil, err
	}

	mounts, err := parseMounts()
	if err != nil {
		return nil, err
	}

	disks := []BlockDev{}

	for _, file := range dir {
		// Checking whether device is physical device
		if _, err := os.Stat(filepath.Join("/sys/block", file.Name(), "device")); err != nil {
			continue
		}

		d := BlockDev{
			Path: filepath.Join("/dev", file.Name()),
		}

		if _, ok := mounts[d.Path]; ok {
			d.IsMounted = true
			d.Mountpoint = mounts[d.Path]
			st := syscall.Statfs_t{}
			if err := syscall.Statfs(d.Mountpoint, &st); err != nil {
				return nil, err
			}
			d.SizeTotal = (int64(st.Bsize) * int64(st.Blocks)) / 1024
			d.SizeUsed = d.SizeTotal - (int64(st.Bsize)*int64(st.Bfree))/1024
			d.SizeAvail = (int64(st.Bsize) * int64(st.Bavail)) / 1024
		}

		disks = append(disks, d)
	}

	return disks, nil
}

func getMemInfo(mi *MemStat) error {
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

func parseMounts() (map[string]string, error) {
	f, err := os.Open("/proc/self/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	mounts := make(map[string]string)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		mounts[fields[0]] = fields[1]
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}

	return mounts, nil
}
