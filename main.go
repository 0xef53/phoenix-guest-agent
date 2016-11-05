package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"syscall"
	"time"
)

const (
	VERSION = "0.6"
	LOGFILE = "/var/log/phoenix.log"
)

var (
	PORTPATH string = "/dev/virtio-ports/org.guest-agent.0"
	DEBUG    bool   = false
	SHOWVER  bool   = false
)

func debug(v ...interface{}) {
	if DEBUG {
		log.Println(v...)
	}
}

type Port struct {
	sync.Mutex
	f  *os.File
	fd uintptr
}

func OpenPort(dev string) (*Port, error) {
	f, err := os.OpenFile(dev, syscall.O_RDWR|syscall.O_ASYNC|syscall.O_NDELAY, 0666)
	if err != nil {
		return nil, err
	}

	fd := f.Fd()

	if err := syscall.SetNonblock(int(fd), false); err != nil {
		return nil, err
	}

	return &Port{f: f, fd: fd}, nil
}

func (p *Port) SendError(err error, tag string) (int, error) {
	code := -1

	switch err.(type) {
	case *os.PathError:
		code = int(err.(*os.PathError).Err.(syscall.Errno))
	}

	res := fmt.Sprintf(
		`{"error": {"bufb64": "%s", "code": %d}, "tag": "%s"}`+"\n",
		base64.StdEncoding.EncodeToString([]byte(err.Error())),
		code,
		tag)

	p.Lock()
	defer p.Unlock()

	return p.f.Write([]byte(res))
}

func (p *Port) SendResponse(resp interface{}, tag string) (int, error) {
	res := struct {
		Return interface{} `json:"return"`
		Tag    string      `json:"tag"`
	}{
		resp,
		tag,
	}

	resB, err := json.Marshal(&res)
	if err != nil {
		log.Fatalln("Failed to marshal response object:", err)
	}

	p.Lock()
	defer p.Unlock()

	return p.f.Write(append(resB, '\x0a'))
}

func (p *Port) Close() error {
	return p.f.Close()
}

func listenPort(cReq chan<- []byte, epollFd int, reader *bufio.Reader) {
	events := make([]syscall.EpollEvent, 32)

	var buf []byte
	var err error

	for {
		if _, err := syscall.EpollWait(epollFd, events, -1); err != nil {
			log.Fatalln("Error receiving epoll events:", err)
		}

		buf, err = reader.ReadBytes('\x0a')
		switch err {
		case nil:
		case io.EOF:
			time.Sleep(time.Second * 1)
			continue
		default:
			log.Fatalln(err)
		}

		break
	}

	cReq <- buf
}

func init() {
	flag.StringVar(&PORTPATH, "p", PORTPATH, "device path")
	flag.BoolVar(&DEBUG, "debug", DEBUG, "enable debug mode")
	flag.BoolVar(&SHOWVER, "v", SHOWVER, "print version information and quit")
}

func main() {
	flag.Parse()

	if SHOWVER {
		fmt.Println("Version:", VERSION)
		return
	}

	debug("Phoenix Guest Agent started [pid=" + fmt.Sprintf("%d", os.Getpid()) + "]")

	port, err := OpenPort(PORTPATH)
	if err != nil {
		log.Fatalln("Failed to open character device:", err)
	}
	defer port.Close()

	epollFd, err := syscall.EpollCreate1(0)
	if err != nil {
		log.Fatalln("Error creating epoll:", err)
	}
	defer syscall.Close(epollFd)

	ctlEvent := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(port.fd)}
	if err := syscall.EpollCtl(epollFd, syscall.EPOLL_CTL_ADD, int(port.fd), &ctlEvent); err != nil {
		log.Fatalln("Error registering epoll event:", err)
	}

	reader := bufio.NewReader(port.f)

	cReq := make(chan []byte)
	cResp := make(chan *Response, 1)

	lock := false

	for {
		if !lock {
			lock = true
			go listenPort(cReq, epollFd, reader)
		}

		select {
		case jsonReq := <-cReq:
			lock = false

			req := &Request{}

			if err := json.Unmarshal(jsonReq, &req); err != nil {
				debug("JSON parse error:", err)
				port.SendError(fmt.Errorf("JSON parse error: %s", err), "")
				continue
			}

			switch req.Command {
			case "ping":
				port.SendResponse(VERSION, req.Tag)
				continue
			case "agent-shutdown":
				debug("Shutdown command received from client")
				return
			case "get-commands":
				go GetCommandList(cResp, req.Tag)
				continue
			}

			if FROZEN && req.Command != "get-freeze-status" && req.Command != "fs-unfreeze" {
				debug("All filesystems are frozen. Cannot execute:", req.Command)
				port.SendError(fmt.Errorf("All filesystems are frozen. Cannot execute: %s", req.Command), req.Tag)
				continue
			}

			if _, ok := Commands[req.Command]; !ok {
				debug("Unknown command:", req.Command)
				port.SendError(fmt.Errorf("Unknown command: %s", req.Command), req.Tag)
				continue
			}

			debug("Processing command:", req.Command+", tag =", req.Tag)
			go Commands[req.Command](cResp, req.RawArgs, req.Tag)
		case resp := <-cResp:
			if resp.Err != nil {
				port.SendError(resp.Err, resp.Tag)
			} else {
				port.SendResponse(resp.Value, resp.Tag)
			}
		} // end of select
	}
}
