package cloudinit

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/digitalocean/go-smbios/smbios"
	"gopkg.in/yaml.v2"
)

type Data struct {
	Network *NetworkConfig
}

type NetworkConfig struct {
	Version   int                       `json:"version" yaml:"version"`
	Ethernets map[string]EthernetConfig `json:"ethernets" yaml:"ethernets"`
}

type EthernetConfig struct {
	Match struct {
		MacAddress string `json:"mac_address" yaml:"mac_address"`
	} `json:"match" yaml:"match"`
	Addresses []string `json:"addresses" yaml:"addresses"`
	Gateway4  string   `json:"gateway4" yaml:"gateway4"`
	Gateway6  string   `json:"gateway6" yaml:"gateway6"`
	Routes    []struct {
		To  string `json:"to" yaml:"to"`
		Via string `json:"via" yaml:"via"`
	} `json:"routes" yaml:"routes"`
}

func ReadData() (*Data, error) {
	dir, err := ioutil.ReadDir("/sys/block")
	if err != nil {
		return nil, err
	}

	var devname string

	for _, file := range dir {
		if _, err := os.Stat(filepath.Join("/sys/class/block", file.Name(), "dev")); err != nil {
			continue
		}
		fslabel, err := GetDeviceAttr(filepath.Join("/dev", file.Name()), "LABEL")
		if err != nil {
			return nil, err
		}
		if strings.ToUpper(fslabel) == "CIDATA" {
			devname = filepath.Join("/dev", file.Name())
			break
		}
	}

	if len(devname) == 0 {
		return nil, fmt.Errorf("CIDATA device not found")
	}

	if err := os.MkdirAll("/run/phoenix-ga", 0750); err != nil {
		return nil, err
	}
	tmpdir, err := ioutil.TempDir("/run/phoenix-ga", "netinit_*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpdir)

	var flags uintptr = syscall.MS_NOATIME | syscall.MS_SILENT | syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_RDONLY

	if err := syscall.Mount(devname, tmpdir, "iso9660", flags, ""); err != nil {
		return nil, fmt.Errorf("mount error: %s", err)
	}
	defer syscall.Unmount(tmpdir, 0)

	b, err := ioutil.ReadFile(filepath.Join(tmpdir, "network-config"))
	if err != nil {
		return nil, err
	}

	nnn := NetworkConfig{}

	if err := yaml.Unmarshal(b, &nnn); err != nil {
		return nil, err
	}

	return &Data{Network: &nnn}, nil
}

func GetDeviceAttr(device, attr string) (string, error) {
	// Check device availability
	if fd, err := os.Open(device); err == nil {
		fd.Close()
	} else {
		return "", err
	}

	out, err := exec.Command("blkid", "-c", "/dev/null", "-ovalue", "-t", "TYPE=iso9660", "-s", strings.ToUpper(attr), device).CombinedOutput()
	if err != nil {
		var exitCode int
		if exiterr, ok := err.(*exec.ExitError); ok {
			status := exiterr.Sys().(syscall.WaitStatus)
			switch {
			case status.Exited():
				exitCode = status.ExitStatus()
			case status.Signaled():
				exitCode = 128 + int(status.Signal())
			}
		}
		if exitCode != 2 {
			return "", fmt.Errorf("blkid probe failed (%s): %s", err, out)
		}
	}
	return strings.TrimSpace(string(out)), nil
}

func IsNoCloudMarkerPresent() (bool, error) {
	// Find SMBIOS data in operating system-specific location
	rc, _, err := smbios.Stream()
	if err != nil {
		return false, fmt.Errorf("failed to open stream: %s", err)
	}
	defer rc.Close()

	// Decode SMBIOS structures from the stream
	d := smbios.NewDecoder(rc)
	ss, err := d.Decode()
	if err != nil {
		return false, fmt.Errorf("failed to decode smbios structures: %s", err)
	}

	for _, s := range ss {
		// Only look at System Information
		if s.Header.Type != 1 {
			continue
		}

		for idx, b := range s.Formatted {
			if b == 4 { // see Table 10 of the spec — System Information (Type 1) structure
				if len(s.Strings) > idx {
					if strings.Contains(s.Strings[idx], "ds=nocloud") {
						return true, nil
					}
					break
				}
			}
		}

		break
	}

	return false, nil
}
