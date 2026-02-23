package core

import (
	"fmt"
	"os/exec"
	"time"
)

func LoadVSockModule() error {
	_, err := exec.Command("modprobe", "vmw_vsock_virtio_transport").CombinedOutput()
	if err != nil {
		return fmt.Errorf("could not load vsock module: modprobe failed with %s", err)
	}

	time.Sleep(time.Second)

	return nil
}
