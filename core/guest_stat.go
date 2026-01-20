package core

import (
	"context"
	"errors"
)

var (
	ErrNotReadyNow = errors.New("not ready yet")
)

func (s *Server) GetGuestInfo(ctx context.Context) (*GuestInfo, error) {
	st := s.stat()

	if st == nil || st.Uptime == 0 {
		return nil, ErrNotReadyNow
	}

	return st, nil
}
