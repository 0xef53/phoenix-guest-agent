package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/0xef53/phoenix-guest-agent/cert"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: true,
	})

	app := cli.NewApp()
	app.Usage = "A guest-side agent for qemu-kvm virtual machines"
	app.HideHelpCommand = true

	// If no arguments provided
	app.Action = runAgent

	app.Flags = []cli.Flag{
		&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "enable debug/verbose mode"},
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
				&cli.StringFlag{Name: "path", Aliases: []string{"p"}, Value: DefaultSerialPort, Usage: "path to the virtio serial port"},
				&cli.StringFlag{Name: "cert-dir", EnvVars: []string{"CERTDIR"}, Usage: "directory with agent certificates"},
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
			Action: func(c *cli.Context) error {
				fmt.Printf("v%s, (built %s)\n", AgentVersion, runtime.Version())
				return nil
			},
		},
	}

	// Try to load vsock module
	if err := LoadVSockModule(); err != nil {
		log.Warnf("Non-fatal error: %s", err)
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatalln(err)
	}
}

func runAgent(c *cli.Context) error {
	if c.IsSet("verbose") {
		log.SetLevel(log.DebugLevel)
	}

	agent := Agent{
		withoutSSH: c.Bool("without-ssh"),
		withoutTCP: c.Bool("without-tcp"),
		legacyMode: c.Bool("legacy"),
		serialPort: c.String("path"),
	}

	if len(c.Command.Name) == 0 {
		agent.serialPort = DefaultSerialPort
	}

	if v := c.String("cert-dir"); len(v) != 0 {
		agent.crtStore = cert.Dir(v)
	} else {
		agent.crtStore = cert.EmbedStore
	}

	// Global context
	ctx, cancel := context.WithCancel(context.Background())

	// Signal handler
	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		log.WithField("signal", <-sigc).Info("Graceful shutdown initiated ...")
		cancel()
	}()

	return agent.ListenAndServe(ctx)
}

func runNetInit(c *cli.Context) error {
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
