package main

import (
	empty "github.com/golang/protobuf/ptypes/empty"
)

func (c *Command) ShowGuestInfo() error {
	resp, err := c.client.GetGuestInfo(c.ctx, new(empty.Empty))
	if err != nil {
		return err
	}

	return printJSON(resp)
}
