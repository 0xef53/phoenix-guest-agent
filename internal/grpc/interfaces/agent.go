package interfaces

import (
	pb_agent "github.com/0xef53/phoenix-guest-agent/api/services/agent/v2"
	pb_secure_shell "github.com/0xef53/phoenix-guest-agent/api/services/secure_shell/v2"
	pb_system "github.com/0xef53/phoenix-guest-agent/api/services/system/v2"

	grpc "google.golang.org/grpc"
)

type Agent struct {
	Client_System pb_system.AgentSystemServiceClient

	Client_Agent      pb_agent.AgentServiceClient
	Client_Network    pb_agent.AgentNetworkServiceClient
	Client_FileSystem pb_agent.AgentFileSystemServiceClient

	Client_SecureShell pb_secure_shell.AgentSecureShellServiceClient
}

func NewAgentInterface(conn *grpc.ClientConn) *Agent {
	return &Agent{
		Client_System:      pb_system.NewAgentSystemServiceClient(conn),
		Client_Agent:       pb_agent.NewAgentServiceClient(conn),
		Client_Network:     pb_agent.NewAgentNetworkServiceClient(conn),
		Client_FileSystem:  pb_agent.NewAgentFileSystemServiceClient(conn),
		Client_SecureShell: pb_secure_shell.NewAgentSecureShellServiceClient(conn),
	}
}

func (k *Agent) System() pb_system.AgentSystemServiceClient {
	return k.Client_System
}

func (k *Agent) Agent() pb_agent.AgentServiceClient {
	return k.Client_Agent
}

func (k *Agent) Network() pb_agent.AgentNetworkServiceClient {
	return k.Client_Network
}

func (k *Agent) FileSystem() pb_agent.AgentFileSystemServiceClient {
	return k.Client_FileSystem
}

func (k *Agent) SecureShell() pb_secure_shell.AgentSecureShellServiceClient {
	return k.Client_SecureShell
}
