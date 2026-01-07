package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Usage = usage

	flag.Parse()

	if flag.NArg() == 0 {
		flag.Usage()
	}

	if err := ExecuteCommand(flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)

		os.Exit(1)
	}
}
