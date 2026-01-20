package core

import "context"

func (s *Server) GetUserPrivateKey(_ context.Context) []byte {
	if s.sshUserKey != nil {
		return s.sshUserKey()
	}

	return nil
}
