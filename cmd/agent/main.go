package main

import (
	"fmt"
	"os"
	"runtime"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

const VERSION = "1.0.2"
const DEFAULT_SERIAL_PORT = "/dev/virtio-ports/org.guest-agent.0"

func main() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: true,
	})

	app := cli.NewApp()
	app.Usage = "A guest-side agent for qemu-kvm virtual machines"
	app.HideHelpCommand = true

	// If no arguments provided
	app.Action = func(c *cli.Context) error {
		if c.IsSet("verbose") {
			log.SetLevel(log.DebugLevel)
		}
		agent := Agent{
			SerialPort: DEFAULT_SERIAL_PORT,
		}
		return agent.Serve()
	}

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
				&cli.BoolFlag{Name: "legacy", Usage: "use legacy mode instead of VM sockets"},
				&cli.StringFlag{Name: "path", Aliases: []string{"p"}, Value: DEFAULT_SERIAL_PORT, Usage: "path to the virtio serial port"},
			},
			Action: func(c *cli.Context) error {
				if c.IsSet("verbose") {
					log.SetLevel(log.DebugLevel)
				}
				agent := Agent{
					WithoutSSH: c.Bool("without-ssh"),
					LegacyMode: c.Bool("legacy"),
					SerialPort: c.String("path"),
				}
				return agent.Serve()
			},
		},
		// NETINIT
		&cli.Command{
			Name:  "netinit",
			Usage: "configure/deconfigure network interfaces",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "configure-iface", Usage: "bring the interface up and add IP addresses"},
				&cli.StringFlag{Name: "deconfigure-iface", Usage: "flush all IP addresses and bring the interface down"},
			},
			Action: func(c *cli.Context) error {
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
			},
		},
		// VERSION
		&cli.Command{
			Name:  "version",
			Usage: "print the version information",
			Action: func(c *cli.Context) error {
				fmt.Printf("v%s, (built %s)\n", VERSION, runtime.Version())
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
