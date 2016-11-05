package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// from linux/fs.h
const FIFREEZE = 0xC0045877
const FITHAW = 0xC0045878

var FROZEN bool = false

type MEntry struct {
	FSSpec string
	FSFile string
	FSType string
}

func GetMountPoints() ([]MEntry, error) {
	f, err := os.Open("/proc/self/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m := make([]MEntry, 0, 10)

	isExist := func(fsspec string) bool {
		for _, v := range m {
			if v.FSSpec == fsspec {
				return true
			}
		}
		return false
	}

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || isExist(fields[0]) || fields[0][0] != '/' ||
			fields[2] == "smbfs" || fields[2] == "cifs" {
			continue
		}

		// Ignoring the loop devices
		if strings.HasPrefix(fields[0], "/dev/loop") {
			continue
		}
		// Ignoring the dm- devices
		st, err := os.Lstat(fields[0])
		if err != nil {
			return nil, err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			if s, err := os.Readlink(fields[0]); err != nil {
				return nil, err
			} else {
				fields[0] = filepath.Base(s)
			}
		}
		if strings.HasPrefix(fields[0], "dm-") {
			continue
		}

		m = append(m, MEntry{fields[0], fields[1], fields[2]})
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}

	return m, nil
}

func ioctl(fd uintptr, request, argp uintptr) (err error) {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, argp)
	if errno != 0 {
		err = errno
	}

	return os.NewSyscallError("ioctl", err)
}

func GetFreezeStatus(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	cResp <- &Response{FROZEN, tag, nil}
}

func FsFreeze(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	m, err := GetMountPoints()
	if err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}

	FROZEN = true

	for _, mp := range m {
		fs, err := os.Open(mp.FSFile)
		if err != nil {
			cResp <- &Response{nil, tag, err}
			return
		}

		if err := ioctl(fs.Fd(), FIFREEZE, 0); err != nil &&
			err.(*os.SyscallError).Err.(syscall.Errno) != syscall.EOPNOTSUPP &&
			err.(*os.SyscallError).Err.(syscall.Errno) != syscall.EBUSY {
			cResp <- &Response{nil, tag, err}
			return
		}

		fs.Close()
	}

	cResp <- &Response{true, tag, nil}
}

func FsUnFreeze(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	m, err := GetMountPoints()
	if err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}

	for _, mp := range m {
		fs, err := os.Open(mp.FSFile)
		if err != nil {
			cResp <- &Response{nil, tag, err}
			return
		}

		if err := ioctl(fs.Fd(), FITHAW, 0); err != nil && err.(*os.SyscallError).Err.(syscall.Errno) != syscall.EINVAL {
			cResp <- &Response{nil, tag, err}
			return
		}

		fs.Close()
	}

	FROZEN = false

	cResp <- &Response{true, tag, nil}
}
