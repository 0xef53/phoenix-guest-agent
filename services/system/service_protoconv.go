package system

import (
	"github.com/0xef53/phoenix-guest-agent/core"

	pb_types "github.com/0xef53/phoenix-guest-agent/api/types/v2"
)

func agentInfoToProto(info *core.AgentInfo) *pb_types.AgentInfo {
	proto := pb_types.AgentInfo{
		Version:  info.Version.String(),
		IsLocked: info.IsLocked,
	}

	if info.Features != nil {
		proto.Features = &pb_types.AgentInfo_Features{
			LegacyMode: info.Features.LegacyMode,
			SerialPort: info.Features.SerialPort,
			WithoutSSH: info.Features.WithoutSSH,
			WithoutTCP: info.Features.WithoutTCP,
		}
	}

	return &proto
}
