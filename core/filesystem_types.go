package core

import "io/fs"

type FileStat struct {
	Name      string
	Mode      fs.FileMode
	IsDir     bool
	Owner     *FileStat_Owner
	Group     *FileStat_Group
	SizeBytes int64
}

type FileStat_Owner struct {
	UID  uint32
	Name string
}

type FileStat_Group struct {
	GID  uint32
	Name string
}
