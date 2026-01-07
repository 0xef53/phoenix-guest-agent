package client

import (
	"context"
	"fmt"
	"runtime"

	"github.com/0xef53/phoenix-guest-agent/core"
)

func (c *client) ShowVersion(_ context.Context) error {
	fmt.Printf("v%s (built w/%s)\n", core.AgentVersion, runtime.Version())

	return nil
}
