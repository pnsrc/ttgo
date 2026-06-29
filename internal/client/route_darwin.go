//go:build darwin

package client

import (
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"strings"
)

// RouteManager управляет macOS routing table: ставит default через TUN
// и оставляет explicit /32 route к endpoint через старый шлюз, чтобы
// CONNECT-трафик к серверу не уходил в петлю.
type RouteManager struct {
	tunName     string
	endpointIP  string // IPv4 endpoint адрес (резолвится из hostname:port)
	origGateway string // дефолтный шлюз до подключения
	origIface   string // дефолтный интерфейс до подключения
	tunIP       string // адрес который мы присваиваем TUN-интерфейсу

	installed bool
}

// NewRouteManager — нужно знать endpoint IP заранее.
func NewRouteManager(tunName, endpointHostPort string) (*RouteManager, error) {
	host, _, err := net.SplitHostPort(endpointHostPort)
	if err != nil {
		return nil, err
	}
	// Резолвим в IPv4
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}
	var v4 net.IP
	for _, ip := range ips {
		if v4_ := ip.To4(); v4_ != nil {
			v4 = v4_
			break
		}
	}
	if v4 == nil {
		return nil, fmt.Errorf("no IPv4 for %s", host)
	}
	return &RouteManager{tunName: tunName, endpointIP: v4.String(), tunIP: tunIPv4}, nil
}

// Install ставит TUN адрес, default через TUN, и host-route для endpoint
// через original gateway.
func (r *RouteManager) Install() error {
	// 1. Получаем оригинальный default route
	gw, iface, err := defaultRoute()
	if err != nil {
		return fmt.Errorf("get default route: %w", err)
	}
	r.origGateway, r.origIface = gw, iface
	slog.Info("original default route", "gateway", gw, "iface", iface)

	// 2. Присваиваем адрес TUN-интерфейсу.
	// ifconfig utunX 10.99.0.2 10.99.0.1 up
	peer := "10.99.0.1"
	if err := run("ifconfig", r.tunName, r.tunIP, peer, "up"); err != nil {
		return fmt.Errorf("ifconfig: %w", err)
	}

	// 3. Host route к endpoint через старый шлюз
	if err := run("route", "-n", "add", "-host", r.endpointIP, gw); err != nil {
		return fmt.Errorf("route add endpoint: %w", err)
	}

	// 4. Удаляем дефолт и ставим через TUN.
	// Используем split-default трюк: 0.0.0.0/1 + 128.0.0.0/1 покрывает всё,
	// не трогая оригинальный default — тогда роллбэк проще.
	for _, half := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if err := run("route", "-n", "add", "-net", half, "-interface", r.tunName); err != nil {
			return fmt.Errorf("route add %s: %w", half, err)
		}
	}

	r.installed = true
	return nil
}

// Restore чистит за собой.
func (r *RouteManager) Restore() error {
	if !r.installed {
		return nil
	}
	r.installed = false

	var errs []string
	for _, half := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		if err := run("route", "-n", "delete", "-net", half, "-interface", r.tunName); err != nil {
			errs = append(errs, fmt.Sprintf("delete %s: %v", half, err))
		}
	}
	if err := run("route", "-n", "delete", "-host", r.endpointIP); err != nil {
		errs = append(errs, fmt.Sprintf("delete endpoint route: %v", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("restore routes: %s", strings.Join(errs, "; "))
	}
	return nil
}

// defaultRoute parses `route -n get default` for gateway and interface.
func defaultRoute() (gateway, iface string, err error) {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return "", "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "gateway:"):
			gateway = strings.TrimSpace(strings.TrimPrefix(line, "gateway:"))
		case strings.HasPrefix(line, "interface:"):
			iface = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
		}
	}
	if gateway == "" || iface == "" {
		return "", "", fmt.Errorf("could not parse default route")
	}
	return gateway, iface, nil
}

func run(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s: %w", cmd, strings.TrimSpace(string(out)), err)
	}
	return nil
}
