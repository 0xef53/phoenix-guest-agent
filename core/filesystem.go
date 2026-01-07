package core

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"

	log "github.com/sirupsen/logrus"
)

func (s *Server) GetFileMD5Hash(ctx context.Context, fpath string) (string, error) {
	fd, err := os.Open(fpath)
	if err != nil {
		return "", err
	}
	defer fd.Close()

	hash := md5.New()

	if _, err := io.Copy(hash, fd); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *Server) GetFileStat(ctx context.Context, fpath string, withContent bool) ([]*FileStat, error) {
	info, err := os.Lstat(fpath)
	if err != nil {
		return nil, err
	}

	_, uids, err := GetOSUsers()
	if err != nil {
		return nil, err
	}

	_, gids, err := GetOSGroups()
	if err != nil {
		return nil, err
	}

	getStat := func(fi os.FileInfo) *FileStat {
		st := FileStat{
			Name:      fi.Name(),
			Mode:      fi.Mode(),
			SizeBytes: fi.Size(),
			IsDir:     fi.IsDir(),
		}

		if sys, ok := fi.Sys().(*syscall.Stat_t); ok {
			st.Owner = &FileStat_Owner{UID: sys.Uid}

			if v, ok := uids[sys.Uid]; ok {
				st.Owner.Name = v
			}

			st.Group = &FileStat_Group{GID: sys.Gid}

			if v, ok := gids[sys.Gid]; ok {
				st.Group.Name = v
			}
		}

		return &st
	}

	files := make([]*FileStat, 0)

	if info.IsDir() && withContent {
		dir, err := os.Open(fpath)
		if err != nil {
			return nil, err
		}

		ffi, err := dir.Readdir(-1)
		if err != nil {
			return nil, err
		}

		for _, fi := range ffi {
			files = append(files, getStat(fi))
		}
	} else {
		files = append(files, getStat(info))
	}

	return files, nil
}

func (s *Server) SetFileOwner(ctx context.Context, fpath, owner, group string) error {
	users, _, err := GetOSUsers()
	if err != nil {
		return err
	}

	groups, _, err := GetOSGroups()
	if err != nil {
		return err
	}

	var uid, gid int

	if v, ok := users[owner]; ok {
		uid = int(v)
	} else {
		v, err := strconv.Atoi(owner)
		if err != nil {
			return fmt.Errorf("invalid user name/uid: %s", owner)
		}
		uid = v
	}

	if v, ok := groups[group]; ok {
		gid = int(v)
	} else {
		v, err := strconv.Atoi(group)
		if err != nil {
			return fmt.Errorf("invalid group name/gid: %s", group)
		}
		gid = v
	}

	return os.Chown(fpath, uid, gid)
}

func (s *Server) SetFileMode(ctx context.Context, fpath string, mode os.FileMode) error {
	oldmask := syscall.Umask(0000)
	defer syscall.Umask(oldmask)

	return os.Chmod(fpath, os.FileMode(mode))
}

func (s *Server) CreateDir(ctx context.Context, fpath string, mode os.FileMode) error {
	oldmask := syscall.Umask(0000)
	defer syscall.Umask(oldmask)

	return os.MkdirAll(fpath, mode)
}

func (s *Server) FreezeFileSystems(ctx context.Context) error {
	mm, err := GetMountPoints()
	if err != nil {
		return err
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
		log.Infof("Freezing: %s", m)

		if err := freeze(m.FSFile); err != nil {
			return err
		}
	}

	log.Info("All filesystems are frozen now")

	return nil
}

func (s *Server) UnfreezeFileSystems(ctx context.Context) error {
	mm, err := GetMountPoints()
	if err != nil {
		return err
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
		log.Infof("Unfreezing: %s", m)

		if err := unfreeze(m.FSFile); err != nil {
			return err
		}
	}

	s.unlock()

	log.Info("All filesystems are thawed now")

	return nil
}
