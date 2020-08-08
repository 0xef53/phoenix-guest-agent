package main

import (
	empty "github.com/golang/protobuf/ptypes/empty"
)

func (c *Command) FreezeAll() error {
	_, err := c.client.FreezeFileSystems(c.ctx, new(empty.Empty))

	return err
}

func (c *Command) UnfreezeAll() error {
	_, err := c.client.UnfreezeFileSystems(c.ctx, new(empty.Empty))

	return err
}
