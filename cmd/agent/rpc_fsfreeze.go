package main

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	empty "github.com/golang/protobuf/ptypes/empty"
	log "github.com/sirupsen/logrus"
)

// from linux/fs.h
const FIFREEZE = 0xC0045877
const FITHAW = 0xC0045878

type MPEntry struct {
	FSSpec string
	FSFile string
	FSType string
}

func getMountPoints() ([]MPEntry, error) {
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
		// Skip invalid, already added and virtual records
		if len(parts) < 2 || present(parts[0]) || parts[0][0] != '/' ||
			parts[2] == "smbfs" || parts[2] == "cifs" {
			continue
		}

		// Ignore loop devices
		if strings.HasPrefix(parts[0], "/dev/loop") {
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

func (s *AgentServiceServer) FreezeFileSystems(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	mm, err := getMountPoints()
	if err != nil {
		return nil, err
	}

	s.lock()

	freeze := func(s string) error {
		fs, err := os.Open(s)
		if err != nil {
			return err
		}
		defer fs.Close()

		if err := ioctl(fs.Fd(), FIFREEZE, 0); err != nil {
			errno := err.(*os.SyscallError).Err.(syscall.Errno)
			if errno != syscall.EOPNOTSUPP && errno != syscall.EBUSY {
				return err
			}
		}

		return nil
	}

	for _, m := range mm {
		log.Debugf("Freezing: %s", m)
		if err := freeze(m.FSFile); err != nil {
			return nil, err
		}
	}

	log.Debug("All filesystems are frozen now")

	return new(empty.Empty), nil
}

func (s *AgentServiceServer) UnfreezeFileSystems(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	mm, err := getMountPoints()
	if err != nil {
		return nil, err
	}

	unfreeze := func(s string) error {
		fs, err := os.Open(s)
		if err != nil {
			return err
		}
		defer fs.Close()

		if err := ioctl(fs.Fd(), FITHAW, 0); err != nil {
			errno := err.(*os.SyscallError).Err.(syscall.Errno)
			if errno != syscall.EINVAL {
				return err
			}
		}

		return nil
	}

	for _, m := range mm {
		log.Debugf("Unfreezing: %s", m)
		if err := unfreeze(m.FSFile); err != nil {
			return nil, err
		}
	}

	s.unlock()

	log.Debug("All filesystems are thawed now")

	return new(empty.Empty), nil
}
