package core

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// from linux/fs.h
const FIFREEZE = 0xC0045877
const FITHAW = 0xC0045878

type MPEntry struct {
	FSSpec string
	FSFile string
	FSType string
}

func GetMountPoints() ([]MPEntry, error) {
	f, err := os.Open("/proc/self/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	mpoints := make([]MPEntry, 0, 10)

	present := func(fsspec string) bool {
		for _, v := range mpoints {
			if v.FSSpec == fsspec {
				return true
			}
		}

		return false
	}

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		// See: man 5 fstab
		// /dev/vda / ext4 rw,noatime,errors=remount-ro,data=ordered 0 0
		parts := strings.Fields(scanner.Text())

		// Skip invalid, already added, virtual records and loop devices
		switch {
		case len(parts) < 2:
			continue
		case present(parts[0]):
			continue
		case parts[0][0] != '/':
			continue
		case parts[2] == "smbfs", parts[2] == "cifs":
			continue
		case strings.HasPrefix(parts[0], "/dev/loop"):
			continue
		}

		// Ignore dm- devices
		st, err := os.Lstat(parts[0])
		if err != nil {
			return nil, err
		}

		if st.Mode()&os.ModeSymlink != 0 {
			if s, err := os.Readlink(parts[0]); err != nil {
				return nil, err
			} else {
				parts[0] = filepath.Base(s)
			}
		}

		if strings.HasPrefix(parts[0], "dm-") {
			continue
		}

		mpoints = append(mpoints, MPEntry{parts[0], parts[1], parts[2]})
	}
	if err = scanner.Err(); err != nil {
		return nil, err
	}

	return mpoints, nil
}

func ioctl(fd uintptr, request, argp uintptr) (err error) {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, argp)
	if errno != 0 {
		err = errno
	}

	return os.NewSyscallError("ioctl", err)
}

func GetOSUsers() (map[string]uint32, map[uint32]string, error) {
	fd, err := os.Open("/etc/passwd")
	if err != nil {
		return nil, nil, err
	}
	defer fd.Close()

	names := make(map[string]uint32)
	uids := make(map[uint32]string)

	scanner := bufio.NewScanner(fd)

	for scanner.Scan() {
		// alice:x:1005:1006::/home/alice:/usr/bin/bash
		parts := strings.SplitN(scanner.Text(), ":", 7)

		if len(parts) < 6 || parts[0] == "" || parts[0][0] == '+' || parts[0][0] == '-' {
			continue
		}

		uid, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, nil, err
		}

		names[parts[0]] = uint32(uid)
		uids[uint32(uid)] = parts[0]
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return names, uids, nil
}

func GetOSGroups() (map[string]uint32, map[uint32]string, error) {
	fd, err := os.Open("/etc/group")
	if err != nil {
		return nil, nil, err
	}
	defer fd.Close()

	names := make(map[string]uint32)
	gids := make(map[uint32]string)

	scanner := bufio.NewScanner(fd)

	for scanner.Scan() {
		// wheel:*:0:root
		parts := strings.SplitN(scanner.Text(), ":", 4)

		if len(parts) < 4 || parts[0] == "" || parts[0][0] == '+' || parts[0][0] == '-' {
			// If the file contains +foo and you search for "foo", glibc
			// returns an "invalid argument" error. Similarly, if you search
			// for a gid for a row where the group name starts with "+" or "-",
			// glibc fails to find the record.
			continue
		}

		gid, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, nil, err
		}

		names[parts[0]] = uint32(gid)
		gids[uint32(gid)] = parts[0]
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return names, gids, nil
}
