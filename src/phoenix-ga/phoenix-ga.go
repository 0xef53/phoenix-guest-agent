package main

import (
    "bufio"
    "commands"
    "encoding/base64"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "os"
    "os/exec"
    "sync"
    "syscall"
    "time"
)


// Defines & defaults
const VERSION string   = "0.1"
const LOGFILE string   = "/var/log/phoenix.log"
var PORTPATH string    = "/dev/virtio-ports/org.guest-agent.0"
var DEBUG bool         = false
// end of defines


func debug(msg string) {
    if DEBUG {
        log.Print("DEBUG: ", msg)
    }
}


type Request struct {
    Command string              `json:"execute"`
    RawArgs *json.RawMessage    `json:"arguments"`
    Tag string                  `json:"tag"`
}


type Port struct {
    sync.Mutex
    f *os.File
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
    return &Port{f:f, fd:fd}, nil
}


func (p *Port) SendError(err error, tag string) (int, error) {
    var code int = -1
    switch err.(type) {
        case *os.PathError:
            code = int(err.(*os.PathError).Err.(syscall.Errno))
        case *commands.ExtExitError:
            status := err.(*commands.ExtExitError).Err.(*exec.ExitError).Sys().(syscall.WaitStatus)
            switch {
                case status.Exited():
                    code = status.ExitStatus()
                case status.Signaled():
                    code = 128 + int(status.Signal())
            }
    }
    errJStr := fmt.Sprintf(`{"error": {"bufb64": "%s", "code": %d}, "tag": "%s"}`+"\n",
                          base64.StdEncoding.EncodeToString([]byte(err.Error())),
                          code,
                          tag)
    p.Lock()
    defer p.Unlock()
    return p.f.Write([]byte(errJStr))
}


func (p *Port) SendResponse(resp interface{}, tag string) (int, error) {
    jsonResp, err := json.Marshal(struct{
                                      Return interface{} `json:"return"`
                                      Tag string `json:"tag"`
                                  }{resp, tag})
    if err != nil {
        log.Fatal("Failed to marshal response object: ", err)
    }
    p.Lock()
    defer p.Unlock()
    return p.f.Write(append(jsonResp, '\x0a'))
}


func isKnownCommand(cmd string) bool {
    for k, _ := range commands.Commands {
        if k == cmd {
            return true
        }
    }
    return false
}


func readPort(cReq chan<- []byte, epollFd int, reader *bufio.Reader) {
    events := make([]syscall.EpollEvent, 32)
    var buf []byte
    var err error
    for {
        if _, err := syscall.EpollWait(epollFd, events, -1) ; err != nil {
            log.Fatal("Error receiving events from epoll: ", err)
        }
        buf, err = reader.ReadBytes('\x0a')
        if err != nil {
            if err == io.EOF {
                time.Sleep(time.Second*1)
                continue
            }
            log.Fatal(err)
        }
        break
    }
    cReq <- buf
}


func init() {
    flag.StringVar(&PORTPATH, "p", PORTPATH, "device path")
    flag.BoolVar(&DEBUG, "debug", DEBUG, "show debug output")
}


func main() {
    flag.Parse()

    log_f, err := os.OpenFile(LOGFILE, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
    if err == nil {
        log.SetOutput(log_f)
    }
    log.Printf("Phoenix Guest Agent started [pid=%d]", os.Getpid())

    port, err := OpenPort(PORTPATH)
    if err != nil {
        log.Fatal("Failed to open character device: ", err)
    }
    defer port.f.Close()

    epollFd, err := syscall.EpollCreate1(0)
    if err != nil {
        log.Fatal("Epoll create error: ", err)
    }
    defer syscall.Close(epollFd)

    ctlEvent := syscall.EpollEvent{Events: syscall.EPOLLIN, Fd: int32(port.fd)}
    if err := syscall.EpollCtl(epollFd, syscall.EPOLL_CTL_ADD, int(port.fd), &ctlEvent); err != nil {
        log.Fatal("Error registering event: ", err)
    }

    reader := bufio.NewReader(port.f)
    cReq := make(chan []byte)
    cResp := make(chan *commands.Response, 1)
    lock := false

    for {
        if !lock {
            lock = true
            debug("Starting goroutine: readPort()")
            go readPort(cReq, epollFd, reader)
        }
        select {
            case jsonReq := <-cReq:
                debug("New request from readPort()")
                lock = false
                req := &Request{}
                if err := json.Unmarshal(jsonReq, &req); err != nil {
                     port.SendError(fmt.Errorf("JSON parse error: %s", err), "")
                     log.Printf("JSON parse error: %s", err)
                     continue
                }
                switch req.Command {
                    case "ping":
                        port.SendResponse(VERSION, req.Tag)
                        continue
                    case "agent-shutdown":
                        log.Print("Shutdown command received from client")
                        return
                    case "get-commands":
                        go commands.GetCommands(cResp, req.Tag)
                        continue
                }
                if isKnownCommand(req.Command) {
                    log.Printf("Processing command: %s, tag = %s", req.Command, req.Tag)
                    go commands.Commands[req.Command](cResp, req.RawArgs, req.Tag)
                } else {
                    port.SendError(fmt.Errorf("The command not found: %s", req.Command), req.Tag)
                    log.Printf("The command not found: %s", req.Command)
                }
            case resp := <-cResp:
                if resp.Err != nil {
                    port.SendError(resp.Err, resp.Tag)
                } else {
                    port.SendResponse(resp.Value, resp.Tag)
                }
        } // end of select
    }
}
