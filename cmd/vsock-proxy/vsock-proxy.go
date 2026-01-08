package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/mdlayher/vsock"
)

func main() {
	flag.Parse()

	if err := run(flag.Args()); err != nil {
		os.Stderr.WriteString(err.Error() + "\n")
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 2 {
		return errors.New("want 2 sockets: either vsock + unix, or vsock + vsock, see help for details")
	}
	dial, err := dialer(args[1])
	if err != nil {
		return err
	}
	ln, err := listen(args[0])
	if err != nil {
		return err
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go func() {
			if err := serveConn(conn, dial); err != nil {
				log.Print(err)
			}
		}()
	}
}

func listen(addr string) (net.Listener, error) {
	if strings.HasPrefix(addr, "tcp:") {
		fmt.Printf("Listener: TCP (%s)\n", addr[4:])

		return net.Listen("tcp", addr[4:])
	}

	if port, err := strconv.ParseUint(addr, 0, 32); err == nil {
		fmt.Printf("Listener: VSOCK (%s)\n", addr)
		return vsock.Listen(uint32(port), nil)
	}

	return nil, fmt.Errorf("unknown listener: %s", addr)
}

func dialer(addr string) (dialFunc, error) {
	if strings.HasPrefix(addr, "tcp:") {
		fmt.Printf("Dialer: TCP (%s)\n", addr[4:])

		return func() (net.Conn, error) { return net.Dial("tcp", addr[4:]) }, nil
	}

	if ss := strings.SplitN(addr, ":", 2); len(ss) == 2 {
		cid, err := strconv.ParseUint(ss[0], 0, 32)
		if err != nil {
			return nil, fmt.Errorf("vsock address %q CID parse: %w", addr, err)
		}
		port, err := strconv.ParseUint(ss[1], 0, 32)
		if err != nil {
			return nil, fmt.Errorf("vsock address %q PORT parse: %w", addr, err)
		}
		fmt.Println("Dialer: VSOCK")
		return func() (net.Conn, error) {
			return vsock.Dial(uint32(cid), uint32(port), nil)
		}, nil
	}

	return nil, fmt.Errorf("unknown dialer: %s", addr)
}

type dialFunc func() (net.Conn, error)

func serveConn(conn net.Conn, dial dialFunc) error {
	defer conn.Close()
	conn2, err := dial()
	if err != nil {
		return err
	}
	defer conn2.Close()
	go io.Copy(conn2, conn)
	_, err = io.Copy(conn, conn2)
	return err
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s socket-to-listen socket-to-connect\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), socketHelp)
		flag.PrintDefaults()
	}
}

const socketHelp = `
Listening socket can be either in "PORT" format for vsock sockets,
or "/path/to/socket" for unix sockets.

Socket to connect can be either in "CID:PORT" format for vsock sockets,
or "/path/to/socket" for unix sockets.
`
