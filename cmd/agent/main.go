package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

const VERSION = "1.0.1"

func main() {
	var portPath string = "/dev/virtio-ports/org.guest-agent.0"
	var legacyMode bool
	var withoutSSH bool
	var verboseMode bool
	var printVer bool

	flag.StringVar(&portPath, "p", portPath, "`path` to the virtio serial port")
	flag.BoolVar(&withoutSSH, "without-ssh", withoutSSH, "do not run the internal SSH server")
	flag.BoolVar(&legacyMode, "legacy", legacyMode, "use legacy mode instead of VM sockets")
	flag.BoolVar(&verboseMode, "v", verboseMode, "enable debug/verbose mode")
	flag.BoolVar(&printVer, "version", printVer, "print version information and quit")

	flag.Parse()

	if printVer {
		fmt.Println("Version:", VERSION)
		os.Exit(2)
	}

	log.SetFormatter(&log.TextFormatter{
		DisableColors:    true,
		DisableTimestamp: true,
	})

	if verboseMode {
		log.SetLevel(log.DebugLevel)
	}

	agent := Agent{
		WithoutSSH: withoutSSH,
		LegacyMode: legacyMode,
		SerialPort: portPath,
	}

	if err := agent.Serve(); err != nil {
		log.Fatalln(err)
	}
}
