package main

import (
	"fmt"
	"os/exec"
)

func LoadVSockModule() error {
	_, err := exec.Command("modprobe", "vmw_vsock_virtio_transport").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not load vsock module: modprobe failed with %s", err)
	}

	return nil
}
