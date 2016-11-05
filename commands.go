package main

import (
	"encoding/json"
)

var Commands = map[string]func(chan<- *Response, *json.RawMessage, string){
	"sysinfo":           GetSystemInfo,
	"get-netifaces":     GetNetIfaces,
	"get-route-list":    GetRouteList,
	"route-add":         RouteAdd,
	"route-del":         RouteDel,
	"ipaddr-add":        IpAddrAdd,
	"ipaddr-del":        IpAddrDel,
	"linux-ipaddr-add":  IpAddrAdd, // Deprecated since ver. 0.4
	"linux-ipaddr-del":  IpAddrDel, // Deprecated since ver. 0.4
	"net-iface-up":      NetIfaceUp,
	"net-iface-down":    NetIfaceDown,
	"file-open":         FileOpen,
	"file-close":        FileClose,
	"file-read":         FileRead,
	"file-write":        FileWrite,
	"get-file-md5sum":   GetFileMd5sum,
	"directory-create":  DirectoryCreate,
	"directory-list":    DirectoryList,
	"file-chmod":        FileChmod,
	"file-chown":        FileChown,
	"file-stat":         FileStat,
	"fs-freeze":         FsFreeze,
	"fs-unfreeze":       FsUnFreeze,
	"get-freeze-status": GetFreezeStatus,
}

type Request struct {
	Command string           `json:"execute"`
	RawArgs *json.RawMessage `json:"arguments"`
	Tag     string           `json:"tag"`
}

type Response struct {
	Value interface{}
	Tag   string
	Err   error
}

func GetCommandList(cResp chan<- *Response, tag string) {
	CmdList := make([]string, 0, 3+len(Commands))

	for _, item := range []string{"get-commands", "agent-shutdown", "ping"} {
		CmdList = append(CmdList, item)
	}

	for cmdName, _ := range Commands {
		CmdList = append(CmdList, cmdName)
	}

	cResp <- &Response{&CmdList, tag, nil}
}
