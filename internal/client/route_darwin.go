//go:build darwin

package client

import (
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"strings"
	"sync"
)

// RouteManager управляет macOS routing table: ставит default через TUN
// и оставляет explicit /32 route к endpoint через старый шлюз, чтобы
// CONNECT-трафик к серверу не уходил в петлю.
type RouteManager struct {
	tunName     string
	endpointIP  string // IPv4 endpoint адрес (резолвится из hostname:port)
	origGateway string // дефолтный шлюз до подключения
	origIface      string // дефолтный интерфейс до подключения
	origDNSService string   // имя сервиса для networksetup (например, "Wi-Fi")
	origDNSServers []string // оригинальные DNS серверы до подключения
	tunIP          string   // адрес который мы присваиваем TUN-интерфейсу
	dnsBypass      []string
	exclusions     []string
	
	mu         sync.Mutex
	hostBypass []string
	netBypass  []string

	installed bool
}

// NewRouteManager — нужно знать endpoint IP заранее.
func NewRouteManager(tunName, endpointHostPort string, exclusions []string) (*RouteManager, error) {
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
	return &RouteManager{tunName: tunName, endpointIP: v4.String(), tunIP: tunIPv4, exclusions: exclusions}, nil
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

	// 3.1. DNS Override: захватываем DNS через TUN.
	r.origDNSService = getNetworkServiceForIface(r.origIface)
	if r.origDNSService != "" {
		r.origDNSServers = getDNSForService(r.origDNSService)
		slog.Info("overriding system DNS", "service", r.origDNSService, "orig_dns", r.origDNSServers, "new_dns", peer)
		if err := run("networksetup", "-setdnsservers", r.origDNSService, peer); err != nil {
			slog.Warn("failed to override dns", "err", err)
		}
	} else {
		slog.Warn("could not determine network service for DNS override", "iface", r.origIface)
	}

	// 3.2. Public DNS Bypass: исключаем публичные DNS, которые мы используем для резолва в relayDNSUDP
	for _, dnsIP := range []string{"1.1.1.1", "8.8.8.8"} {
		if err := run("route", "-n", "add", "-host", dnsIP, gw); err != nil {
			slog.Debug("public dns bypass route add failed", "ip", dnsIP, "err", err)
			continue
		}
		r.dnsBypass = append(r.dnsBypass, dnsIP)
		slog.Info("public dns bypass route added", "ip", dnsIP, "via", gw)
	}
	slog.Info("dns bypass configured", "count", len(r.dnsBypass))

	// 3.2. Exclusions bypass
	for _, exc := range r.exclusions {
		exc = strings.TrimSpace(exc)
		if exc == "" {
			continue
		}

		if _, ipnet, err := net.ParseCIDR(exc); err == nil {
			if err := run("route", "-n", "add", "-net", ipnet.String(), gw); err == nil {
				r.netBypass = append(r.netBypass, ipnet.String())
				slog.Info("exclusion bypass route added", "net", ipnet.String(), "via", gw)
			} else {
				slog.Warn("failed to add bypass route", "net", ipnet.String(), "err", err)
			}
			continue
		}

		if ip := net.ParseIP(exc); ip != nil {
			if ip.To4() != nil {
				if err := run("route", "-n", "add", "-host", ip.String(), gw); err == nil {
					r.hostBypass = append(r.hostBypass, ip.String())
					slog.Info("exclusion bypass route added", "host", ip.String(), "via", gw)
				} else {
					slog.Warn("failed to add bypass route", "host", ip.String(), "err", err)
				}
			}
			continue
		}

		ips, err := net.LookupIP(exc)
		if err != nil {
			slog.Warn("failed to resolve exclusion domain", "domain", exc, "err", err)
			continue
		}
		for _, ip := range ips {
			if ip.To4() != nil {
				if err := run("route", "-n", "add", "-host", ip.String(), gw); err == nil {
					r.hostBypass = append(r.hostBypass, ip.String())
					slog.Info("exclusion bypass route added", "domain", exc, "host", ip.String(), "via", gw)
				} else {
					slog.Warn("failed to add bypass route", "domain", exc, "host", ip.String(), "err", err)
				}
			}
		}
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
	
	if r.origDNSService != "" {
		args := []string{"-setdnsservers", r.origDNSService}
		if len(r.origDNSServers) > 0 {
			args = append(args, r.origDNSServers...)
		} else {
			args = append(args, "empty")
		}
		slog.Info("restoring system DNS", "service", r.origDNSService, "dns", r.origDNSServers)
		if err := run("networksetup", args...); err != nil {
			errs = append(errs, fmt.Sprintf("restore dns %s: %v", r.origDNSService, err))
		}
	}

	for _, dnsIP := range r.dnsBypass {
		if err := run("route", "-n", "delete", "-host", dnsIP); err != nil {
			errs = append(errs, fmt.Sprintf("delete dns route %s: %v", dnsIP, err))
		}
	}
	for _, ip := range r.hostBypass {
		if err := run("route", "-n", "delete", "-host", ip); err != nil {
			errs = append(errs, fmt.Sprintf("delete host bypass %s: %v", ip, err))
		}
	}
	for _, netAddr := range r.netBypass {
		if err := run("route", "-n", "delete", "-net", netAddr); err != nil {
			errs = append(errs, fmt.Sprintf("delete net bypass %s: %v", netAddr, err))
		}
	}
	r.dnsBypass = nil
	r.hostBypass = nil
	r.netBypass = nil
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

func systemDNSServers() []string {
	out, err := exec.Command("scutil", "--dns").Output()
	if err != nil {
		slog.Debug("read system dns failed", "err", err)
		return nil
	}

	lines := strings.Split(string(out), "\n")
	seen := make(map[string]struct{})
	res := make([]string, 0, 2)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "nameserver[") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		ip := strings.TrimSpace(parts[1])
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.To4() == nil {
			continue
		}
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		res = append(res, ip)
	}
	slog.Debug("system dns discovered", "servers", strings.Join(res, ","))
	return res
}

func run(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s: %w", cmd, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func getNetworkServiceForIface(iface string) string {
	out, err := exec.Command("networksetup", "-listnetworkserviceorder").Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	var currentService string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "(Hardware Port:") {
			if strings.Contains(line, "Device: "+iface+")") {
				return currentService
			}
		} else if strings.HasPrefix(line, "(") {
			idx := strings.Index(line, ") ")
			if idx != -1 {
				currentService = line[idx+2:]
			}
		}
	}
	return ""
}

func getDNSForService(service string) []string {
	out, err := exec.Command("networksetup", "-getdnsservers", service).Output()
	if err != nil {
		return nil
	}
	text := strings.TrimSpace(string(out))
	if strings.Contains(text, "aren't any") {
		return nil
	}
	var servers []string
	for _, line := range strings.Split(text, "\n") {
		s := strings.TrimSpace(line)
		if net.ParseIP(s) != nil {
			servers = append(servers, s)
		}
	}
	return servers
}

// RecoverDNS checks if system DNS is still pointing to a dead TUN peer
// (e.g. after crash/force-quit) and restores it. Call on app startup.
func RecoverDNS() {
	servers := systemDNSServers()
	for _, s := range servers {
		if strings.HasPrefix(s, "10.99.0.") {
			slog.Warn("stale VPN DNS detected, restoring", "dns", s)
			svc := findActiveDNSService()
			if svc != "" {
				if err := run("networksetup", "-setdnsservers", svc, "empty"); err != nil {
					slog.Error("failed to restore stale DNS", "service", svc, "err", err)
				} else {
					slog.Info("stale DNS restored", "service", svc)
				}
			}
			cleanupStaleRoutes()
			return
		}
	}
}

func findActiveDNSService() string {
	gw, iface, err := defaultRoute()
	if err != nil {
		return ""
	}
	_ = gw
	return getNetworkServiceForIface(iface)
}

func cleanupStaleRoutes() {
	for _, half := range []string{"0.0.0.0/1", "128.0.0.0/1"} {
		run("route", "-n", "delete", "-net", half)
	}
	for _, dns := range []string{"1.1.1.1", "8.8.8.8"} {
		run("route", "-n", "delete", "-host", dns)
	}
}

// AddDynamicHostBypassBatch добавляет роуты динамически (для перехваченного DNS).
func (r *RouteManager) AddDynamicHostBypassBatch(ips []net.IP) {
	if len(ips) == 0 || r.origGateway == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ip := range ips {
		if ip.To4() == nil {
			continue // пока поддерживаем только IPv4 bypass
		}
		ipStr := ip.String()
		// Проверяем, нет ли его уже
		exists := false
		for _, ex := range r.hostBypass {
			if ex == ipStr {
				exists = true
				break
			}
		}
		if exists {
			continue
		}

		if err := run("route", "-n", "add", "-host", ipStr, r.origGateway); err == nil {
			r.hostBypass = append(r.hostBypass, ipStr)
			slog.Debug("dynamic dns bypass route added", "host", ipStr, "via", r.origGateway)
		} else {
			slog.Warn("failed to add dynamic bypass route", "host", ipStr, "err", err)
		}
	}
}
