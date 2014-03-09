package commands

import (
    "crypto/md5"
    "encoding/base64"
    "encoding/json"
    "encoding/hex"
    "fmt"
    "io"
    "net"
    "os"
    "os/exec"
    "sync"
    "syscall"
    "unsafe"
)


var Commands = map[string]func(chan<- *Response, *json.RawMessage, string) {
    "get-netifaces": GetNetIfaces,
    "linux-ipaddr-add": LinuxIpAddrAdd,
    "linux-ipaddr-del": LinuxIpAddrDel,
    "file-open": FileOpen,
    "file-close": FileClose,
    "file-read": FileRead,
    "file-write": FileWrite,
    "get-file-md5sum": GetFileMd5sum,
    "directory-create": DirectoryCreate,
    "directory-list": DirectoryList,
    "file-chmod": FileChmod,
    "file-chown": FileChown,
    "file-stat": FileStat,
}


type Response struct {
    Value interface{}
    Tag string
    Err error
}


// An extended exec.ExitError
type ExtExitError struct {
    Err error
    Desc string
}


func NewExtExitError(err error, desc string) error {
    return &ExtExitError{err, desc}
}


func (e *ExtExitError) Error() string {
    return e.Err.Error() + ": " + e.Desc
}


type FD struct {
    sync.RWMutex
    next int
    h map[int]*os.File
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


func GetCommands(cResp chan<- *Response, tag string) {
    CmdList := make([]string, 0, 3+len(Commands))
    for _, item := range []string{"get-commands", "agent-shutdown", "ping"} {
        CmdList = append(CmdList, item)
    }
    for cmdName, _ := range Commands {
        CmdList = append(CmdList, cmdName)
    }
    cResp <- &Response{&CmdList, tag, nil}
}


func FileOpen(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
    var f *os.File
    var err error

    args := &struct{
        Path string      `json:"path"`
        Mode string      `json:"mode"`
        Perm os.FileMode `json:"perm"`
    }{Perm: 0644}
    json.Unmarshal(*rawArgs, &args)

    switch args.Mode {
        case "w":
            f, err = os.OpenFile(args.Path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, args.Perm)
        default:
            f, err = os.OpenFile(args.Path, os.O_RDONLY, 0)
    }
    if err != nil {
        cResp <- &Response{nil, tag, err}
        return
    }
    FDStore.Add(f)
    cResp <- &Response{(FDStore.next-1), tag, nil}
}


func FileRead(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
    args := &struct{
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
    cResp <- &Response{
                 &struct{
                     Buf_b64 string `json:"bufb64"`
                     Eof bool       `json:"eof"`
                 }{base64.StdEncoding.EncodeToString(buf[:n]), eof},
                 tag, nil,
             }
}


func FileWrite(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
    args := &struct{
        Id int           `json:"handle_id"`
        Bufb64 string    `json:"bufb64"`
        Eof bool         `json:"eof"`
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

    dbuf, err := base64.StdEncoding.DecodeString(args.Bufb64)
    if err != nil {
        cResp <- &Response{nil, tag, err}
        return
    }
    _, err = f.Write(dbuf)
    if err != nil {
        cResp <- &Response{nil, tag, err}
        return
    }
    cResp <- &Response{true, tag, nil}
}


func FileClose(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
    args := &struct{
        Id int `json:"handle_id"`
    }{}
    json.Unmarshal(*rawArgs, &args)

    FDStore.Del(args.Id)
    cResp <- &Response{true, tag, nil}
}


func DirectoryCreate(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
    args := &struct{
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
    args := &struct{
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
    args := &struct{
        Path string `json:"path"`
        Uid int     `json:"uid"`
        Gid int     `json:"gid"`
    }{}
    json.Unmarshal(*rawArgs, &args)

    if err := os.Chown(args.Path, args.Uid, args.Gid); err != nil {
        cResp <- &Response{nil, tag, err}
        return
    }
    cResp <- &Response{true, tag, nil}
}


type FStat struct {
    Name string      `json:"name"`
    IsDir bool       `json:"isdir"`
    Stat interface{} `json:"stat"`
}


func FileStat(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
    args := &struct{
        Path string `json:"path"`
    }{}
    json.Unmarshal(*rawArgs, &args)

    file, err := os.Stat(args.Path)
    if err != nil {
        cResp <- &Response{nil, tag, err}
        return
    }
    cResp <- &Response{&FStat{file.Name(), file.IsDir(), file.Sys()}, tag, nil}
}


func DirectoryList(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
    args := &struct{
        Path string `json:"path"`
        N int       `json:"n"`
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


type NetIf struct {
    Index int       `json:"index"`
    Name string     `json:"name"`
    Hwaddr string   `json:"hwaddr"`
    Flags string    `json:"flags"`
    Ips []string    `json:"ips"`
}


func GetNetIfaces(cResp chan<- *Response, args *json.RawMessage, tag string) {
    ifaces, err := net.Interfaces()
    if err != nil {
        cResp <- &Response{nil, tag, err}
        return
    }
    iflist := make([]*NetIf, 0, len(ifaces))
    for _, netif := range ifaces {
        addrs, err := netif.Addrs()
        if err != nil {
            cResp <- &Response{nil, tag, err}
            return
        }
        str_addrs := make([]string, 0, len(addrs))
        for _, addr := range addrs {
            str_addrs = append(str_addrs, addr.String())
        }
        iflist = append(iflist, &NetIf{
                                    netif.Index,
                                    netif.Name,
                                    netif.HardwareAddr.String(),
                                    netif.Flags.String(),
                                    str_addrs,
                                })
    }
    cResp <- &Response{&iflist, tag, nil}
}


func LinuxIpAddr(action string, ip, dev string) (error) {
    out, err := exec.Command("/bin/ip", "addr", action, ip, "dev", dev).CombinedOutput()
    if err != nil {
        return NewExtExitError(err, string(out))
    }
    return nil
}


func LinuxIpAddrAdd(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
    args := &struct{
        Dev string       `json:"dev"`
        IpCidr string    `json:"ip"`
    }{}
    json.Unmarshal(*rawArgs, &args)

    if err := LinuxIpAddr("add", args.IpCidr, args.Dev); err != nil {
        cResp <- &Response{nil, tag, err}
        return
    }
    cResp <- &Response{true, tag, nil}
}


func LinuxIpAddrDel(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
    args := &struct{
        Dev string       `json:"dev"`
        IpCidr string    `json:"ip"`
    }{}
    json.Unmarshal(*rawArgs, &args)

    if err := LinuxIpAddr("del", args.IpCidr, args.Dev); err != nil {
        cResp <- &Response{nil, tag, err}
        return
    }
    cResp <- &Response{true, tag, nil}
}


func GetFileMd5sum(cResp chan<- *Response, rawArgs *json.RawMessage, tag string) {
    args := &struct{
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
