package commands

import (
	"encoding/json"
)

var Commands = map[string]func(chan<- *Response, *json.RawMessage, string){
	"get-netifaces":        GetNetIfaces,
	"get-default-gateways": GetDefaultGateways,
	"fs-freeze":            FsFreeze,
	"fs-unfreeze":          FsUnFreeze,
	"get-freeze-status":    GetFreezeStatus,
	"linux-ipaddr-add":     LinuxIpAddrAdd,
	"linux-ipaddr-del":     LinuxIpAddrDel,
	"file-open":            FileOpen,
	"file-close":           FileClose,
	"file-read":            FileRead,
	"file-write":           FileWrite,
	"get-file-md5sum":      GetFileMd5sum,
	"directory-create":     DirectoryCreate,
	"directory-list":       DirectoryList,
	"file-chmod":           FileChmod,
	"file-chown":           FileChown,
	"file-stat":            FileStat,
}

type Response struct {
	Value interface{}
	Tag   string
	Err   error
}

// An extended exec.ExitError
type ExtExitError struct {
	Err  error
	Desc string
}

func NewExtExitError(err error, desc string) error {
	return &ExtExitError{err, desc}
}

func (e *ExtExitError) Error() string {
	return e.Err.Error() + ": " + e.Desc
}

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
