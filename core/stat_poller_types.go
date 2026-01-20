package core

type GuestInfo struct {
	Uname     *GuestInfo_Utsname
	Uptime    int64
	Loadavg   *GuestInfo_LoadAverage
	Memory    *GuestInfo_MemStat
	Swap      *GuestInfo_SwapStat
	Users     []*GuestInfo_LoggedUser
	Blockdevs []*GuestInfo_BlockDevice
}

type GuestInfo_Utsname struct {
	Sysname    string
	Nodename   string
	Release    string
	Version    string
	Machine    string
	Domainname string
}

type GuestInfo_LoadAverage struct {
	One     float64
	Five    float64
	Fifteen float64
}

type GuestInfo_MemStat struct {
	Total     uint64
	Free      uint64
	Buffers   uint64
	Cached    uint64
	FreeTotal uint64
}

type GuestInfo_SwapStat struct {
	Total uint64
	Free  uint64
}

type GuestInfo_LoggedUser struct {
	Name      string
	Device    string
	Host      string
	LoginTime int64
}

type GuestInfo_BlockDevice struct {
	Path        string
	IsMounted   bool
	MountPoint  string
	SizeTotal   uint64
	SizeUsed    uint64
	SizeAvail   uint64
	InodesTotal uint64
	InodesUsed  uint64
	InodesAvail uint64
}
