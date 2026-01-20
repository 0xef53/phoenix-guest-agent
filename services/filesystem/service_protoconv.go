package filesystem

import (
	"github.com/0xef53/phoenix-guest-agent/core"

	pb_types "github.com/0xef53/phoenix-guest-agent/api/types/v2"
)

func fileStatsToProto(fstats []*core.FileStat) []*pb_types.FileStat {
	files := make([]*pb_types.FileStat, 0, len(fstats))

	for _, v := range fstats {
		file := pb_types.FileStat{
			Name:      v.Name,
			Mode:      uint32(v.Mode),
			IsDir:     v.IsDir,
			SizeBytes: v.SizeBytes,
		}

		if v.Owner != nil {
			file.Owner = &pb_types.FileStat_Owner{
				UID:  v.Owner.UID,
				Name: v.Owner.Name,
			}
		}

		if v.Group != nil {
			file.Group = &pb_types.FileStat_Group{
				GID:  v.Group.GID,
				Name: v.Group.Name,
			}
		}

		files = append(files, &file)
	}

	return files
}
