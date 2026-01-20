package secure_shell

import (
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"
	"unsafe"
)

func getUserShell(u *user.User) string {
	if u == nil {
		return "/bin/sh"
	}

	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return "/bin/sh"
	}

	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, u.Username+":") {
			parts := strings.Split(line, ":")

			if len(parts) >= 7 {
				shell := parts[6]

				if shell != "" {
					return shell
				}
			}
		}
	}

	return "/bin/sh"
}

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})),
	)
}

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}

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

	return exitCode
}
