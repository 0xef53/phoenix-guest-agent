package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/0xef53/phoenix-guest-agent/cert"
	"github.com/0xef53/phoenix-guest-agent/core"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: true,
	})
}

func main() {
	app := new(cli.Command)

	app.Usage = "A guest-side agent for QEMU virtual machines"
	app.HideHelpCommand = true
	app.EnableShellCompletion = true

	// If no arguments provided
	app.Action = runAgent

	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "enable debug/verbose mode",
		},
	}

	app.Commands = []*cli.Command{
		// AGENT
		&cli.Command{
			Name:  "serve",
			Usage: "run Guest Agent application",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "without-ssh", Usage: "do not run the internal SSH server"},
				&cli.BoolFlag{Name: "without-tcp", Usage: "do not bind to TCP socket"},
				&cli.BoolFlag{Name: "legacy", Usage: "use legacy mode instead of VM sockets"},
				&cli.StringFlag{Name: "path", Aliases: []string{"p"}, Value: core.DefaultGuestSerialPort, Usage: "path to the virtio serial port"},
				&cli.StringFlag{Name: "cert-dir", Sources: cli.EnvVars("CERTDIR"), Usage: "directory with agent certificates"},
			},
			Action: runAgent,
		},
		// NETINIT
		&cli.Command{
			Name:  "netinit",
			Usage: "configure/deconfigure network interfaces",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "configure-iface", Usage: "bring the interface up and add IP addresses"},
				&cli.StringFlag{Name: "deconfigure-iface", Usage: "flush all IP addresses and bring the interface down"},
			},
			Action: runNetInit,
		},
		// VERSION
		&cli.Command{
			Name:  "version",
			Usage: "print the version information",
			Action: func(ctx context.Context, c *cli.Command) error {
				fmt.Printf("v%s, (built %s)\n", core.AgentVersion, runtime.Version())
				return nil
			},
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatalln(err)
	}
}

func runAgent(ctx context.Context, c *cli.Command) error {
	if c.Bool("verbose") {
		log.SetLevel(log.DebugLevel)
	}

	agent := Agent{
		AgentFeatures: core.AgentFeatures{
			WithoutSSH: c.Bool("without-ssh"),
			WithoutTCP: c.Bool("without-tcp"),
			LegacyMode: c.Bool("legacy"),
			SerialPort: c.String("path"),
		},
	}

	if v := c.String("cert-dir"); len(v) != 0 {
		agent.crtStore = cert.Dir(v)
	} else {
		agent.crtStore = cert.EmbedStore
	}

	// This global cancel context is used by the graceful shutdown function
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Register signal handler
	go func() {
		sigc := make(chan os.Signal, 1)

		signal.Notify(sigc, syscall.SIGTERM, syscall.SIGINT)
		defer signal.Stop(sigc)

		sig := <-sigc

		log.WithField("signal", sig).Info("Graceful shutdown initiated ...")

		cancel()
	}()

	return agent.ListenAndServe(ctx)
}

func runNetInit(_ context.Context, c *cli.Command) error {
	if c.IsSet("verbose") {
		log.SetLevel(log.DebugLevel)
	}

	switch {
	case c.IsSet("configure-iface"):
		return netinitConfigureInterface(c.String("configure-iface"))
	case c.IsSet("deconfigure-iface"):
		return netinitDeconfigureInterface(c.String("deconfigure-iface"))
	}

	return nil
}
