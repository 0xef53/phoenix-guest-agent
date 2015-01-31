package commands

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
)

type FD struct {
	sync.RWMutex
	next int
	h    map[int]*os.File
}

func NewFD() *FD {
	return &FD{next: 1, h: make(map[int]*os.File)}
}

func (fd *FD) Add(f *os.File) {
	fd.Lock()
	defer fd.Unlock()
	fd.h[fd.next] = f
	fd.next += 1
}

func (fd *FD) Get(id int) (f *os.File, err error) {
	fd.RLock()
	defer fd.RUnlock()
	f, ok := fd.h[id]
	if !ok {
		return nil, fmt.Errorf("Incorrect handle id")
	}
	return f, nil
}

func (fd *FD) Del(id int) {
	fd.Lock()
	defer fd.Unlock()
	if fd.h[id] != nil {
		fd.h[id].Close()
		delete(fd.h, id)
	}
}

var FDStore = NewFD()

func FileOpen(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	var f *os.File
	var err error

	args := &struct {
		Path  string      `json:"path"`
		Mode  string      `json:"mode"`
		Perm  os.FileMode `json:"perm"`
		Force bool        `json:"force"`
	}{Perm: 0644, Force: false}
	json.Unmarshal(*rawArgs, &args)

	switch args.Mode {
	case "w":
		f, err = os.OpenFile(args.Path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, args.Perm)
		if err != nil && args.Force && err.(*os.PathError).Err.(syscall.Errno) == syscall.ETXTBSY {
			if err = os.Remove(args.Path); err == nil {
				f, err = os.OpenFile(args.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, args.Perm)
			}
		}
	default:
		f, err = os.OpenFile(args.Path, os.O_RDONLY, 0)
	}
	if err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	FDStore.Add(f)
	cResp <- &Response{(FDStore.next - 1), tag, nil}
}

func FileRead(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Id int `json:"handle_id"`
	}{}
	json.Unmarshal(*rawArgs, &args)

	f, err := FDStore.Get(args.Id)
	if err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	buf := make([]byte, 4096)
	eof := false
	n, err := f.Read(buf)
	if err == io.EOF {
		f.Close()
		FDStore.Del(args.Id)
		eof = true
	} else if err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	res := &struct {
		Buf []byte `json:"bufb64"`
		Eof bool   `json:"eof"`
	}{
		[]byte(buf[:n]),
		eof,
	}
	cResp <- &Response{res, tag, nil}
}

func FileWrite(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Id  int    `json:"handle_id"`
		Buf []byte `json:"bufb64"`
		Eof bool   `json:"eof"`
	}{}
	json.Unmarshal(*rawArgs, &args)
	f, err := FDStore.Get(args.Id)
	if err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	if args.Eof {
		FDStore.Del(args.Id)
		cResp <- &Response{true, tag, nil}
		return
	}
	_, err = f.Write(args.Buf)
	if err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	cResp <- &Response{true, tag, nil}
}

func FileClose(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Id int `json:"handle_id"`
	}{}
	json.Unmarshal(*rawArgs, &args)

	FDStore.Del(args.Id)
	cResp <- &Response{true, tag, nil}
}

func DirectoryCreate(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Path string      `json:"path"`
		Perm os.FileMode `json:"perm"`
	}{Perm: 0755}
	json.Unmarshal(*rawArgs, &args)

	if err := os.MkdirAll(args.Path, args.Perm); err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	cResp <- &Response{true, tag, nil}
}

func FileChmod(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Path string      `json:"path"`
		Perm os.FileMode `json:"perm"`
	}{}
	json.Unmarshal(*rawArgs, &args)

	if err := os.Chmod(args.Path, args.Perm); err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	cResp <- &Response{true, tag, nil}
}

func FileChown(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Path string `json:"path"`
		Uid  int    `json:"uid"`
		Gid  int    `json:"gid"`
	}{}
	json.Unmarshal(*rawArgs, &args)

	if err := os.Chown(args.Path, args.Uid, args.Gid); err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	cResp <- &Response{true, tag, nil}
}

type FStat struct {
	Name  string      `json:"name"`
	IsDir bool        `json:"isdir"`
	Stat  interface{} `json:"stat"`
}

func FileStat(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Path string `json:"path"`
	}{}
	json.Unmarshal(*rawArgs, &args)

	file, err := os.Lstat(args.Path)
	if err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	cResp <- &Response{&FStat{file.Name(), file.IsDir(), file.Sys()}, tag, nil}
}

func DirectoryList(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Path string `json:"path"`
		N    int    `json:"n"`
	}{N: -1}
	json.Unmarshal(*rawArgs, &args)

	dir, err := os.Open(args.Path)
	if err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	files, err := dir.Readdir(args.N)
	if err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	flist := make([]*FStat, 0, len(files))
	for _, file := range files {
		flist = append(flist, &FStat{file.Name(), file.IsDir(), file.Sys()})
	}
	cResp <- &Response{&flist, tag, nil}
}

func GetFileMd5sum(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
	args := &struct {
		Path string `json:"path"`
	}{}
	json.Unmarshal(*rawArgs, &args)

	f, err := os.Open(args.Path)
	if err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	defer f.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, f); err != nil {
		cResp <- &Response{nil, tag, err}
		return
	}
	cResp <- &Response{hex.EncodeToString(hash.Sum(nil)), tag, nil}
}
