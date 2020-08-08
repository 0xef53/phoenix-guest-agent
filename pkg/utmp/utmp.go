package utmp

import (
	"encoding/binary"
	"io"
	"os"
)

// http://man7.org/linux/man-pages/man5/utmp.5.html
type UtmpEntry struct {
	Type    Utype          // type of record
	_       int16          // padding because Go doesn't 4-byte align
	Pid     int32          // PID of login process
	Device  [LineSize]byte // device name of tty - "/dev/"
	Id      [4]byte        // terminal name suffix or inittab(5) ID
	User    [NameSize]byte // username
	Host    [HostSize]byte // hostname for remote login or kernel version for run-level messages
	Exit    ExitStatus     // exit status of a process marked as DeadProcess; not used by Linux init(1)
	Session int32          // session ID (getsid(2)), used for windowing
	Time    TimeVal        // time entry was made
	Addr    [4]int32       // internet address of remote host; IPv4 address uses just Addr[0]
	Unused  [20]byte       // reserved for future use
}

// Values for utmp.Type field
type Utype int16

// Type for ut_exit, below
const (
	Empty        Utype = iota // record does not contain valid info (formerly known as UT_UNKNOWN on Linux)
	RunLevel           = iota // change in system run-level (see init(8))
	BootTime           = iota // time of system boot (in ut_tv)
	NewTime            = iota // time after system clock change (in ut_tv)
	OldTime            = iota // time before system clock change (in ut_tv)
	InitProcess        = iota // process spawned by init(8)
	LoginProcess       = iota // session leader process for user login
	UserProcess        = iota // normal process
	DeadProcess        = iota // terminated process
	Accounting         = iota // not implemented

	LineSize = 32
	NameSize = 32
	HostSize = 256
)

type ExitStatus struct {
	Termination int16 `json:"termination"` // process termination status
	Exit        int16 `json:"exit"`        // process exit status
}

type TimeVal struct {
	Sec  int32 `json:"seconds"`
	Usec int32 `json:"microseconds"`
}

func ReadFile(filename string) ([]UtmpEntry, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries := make([]UtmpEntry, 0)

	for {
		entry := UtmpEntry{}

		err := binary.Read(f, binary.LittleEndian, &entry)
		if err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}
