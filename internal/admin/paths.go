package admin

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// Paths holds locations of the config files.
type Paths struct {
	VPN   string
	Hosts string
	Creds string
}

// serverPID finds the running trusttunnel_endpoint process PID.
func serverPID() (int, error) {
	out, err := exec.Command("pgrep", "-f", "trusttunnel_endpoint").Output()
	if err != nil {
		return 0, fmt.Errorf("server not running")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

// reloadTLS sends SIGHUP to the server process (hot-reload TLS certs + hosts).
func reloadTLS() error {
	pid, err := serverPID()
	if err != nil {
		return err
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGHUP)
}

// serverRunning returns true if the server process is alive.
func serverRunning() bool {
	_, err := serverPID()
	return err == nil
}
