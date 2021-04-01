package main

import (
	empty "github.com/golang/protobuf/ptypes/empty"
)

func (c *Command) ShutdownAgent() error {
	_, err := c.client.ShutdownAgent(c.ctx, new(empty.Empty))

	return err
}

func (c *Command) ShowAgentInfo() error {
	resp, err := c.client.GetAgentInfo(c.ctx, new(empty.Empty))
	if err != nil {
		return err
	}

	return printJSON(resp)
}

func (c *Command) ShowGuestInfo() error {
	resp, err := c.client.GetGuestInfo(c.ctx, new(empty.Empty))
	if err != nil {
		return err
	}

	return printJSON(resp)
}
