//go:build windows

package client

import (
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"strings"
	"sync"
)

type RouteManager struct {
	tunName        string
	endpointIP     string
	origGateway    string
	origIface      string
	origDNSAdapter string
	origDNSServers []string
	tunIP          string
	tunIndex       string
	dnsBypass      []string
	exclusions     []string

	mu         sync.Mutex
	hostBypass []string
	netBypass  []string

	installed bool
}

func NewRouteManager(tunName, endpointHostPort string, exclusions []string) (*RouteManager, error) {
	host, _, err := net.SplitHostPort(endpointHostPort)
	if err != nil {
		return nil, err
	}
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
	return &RouteManager{
		tunName:    tunName,
		endpointIP: v4.String(),
		tunIP:      tunIPv4,
		exclusions: exclusions,
	}, nil
}

func (r *RouteManager) Install() error {
	gw, iface, err := defaultRoute()
	if err != nil {
		return fmt.Errorf("get default route: %w", err)
	}
	r.origGateway, r.origIface = gw, iface
	slog.Info("original default route", "gateway", gw, "iface", iface)

	idx, err := getTUNIndex(r.tunName)
	if err != nil {
		return fmt.Errorf("get tun index: %w", err)
	}
	r.tunIndex = idx

	peer := "10.99.0.1"
	if err := run("netsh", "interface", "ipv4", "set", "address",
		"name="+r.tunName, "static", r.tunIP, "255.255.255.0", peer); err != nil {
		return fmt.Errorf("set tun address: %w", err)
	}

	if err := run("route", "add", r.endpointIP, "mask", "255.255.255.255", gw, "metric", "1"); err != nil {
		return fmt.Errorf("route add endpoint: %w", err)
	}

	r.origDNSAdapter = getDefaultNetAdapter()
	if r.origDNSAdapter != "" {
		r.origDNSServers = getDNSForAdapter(r.origDNSAdapter)
		slog.Info("overriding system DNS", "adapter", r.origDNSAdapter, "orig_dns", r.origDNSServers, "new_dns", peer)
		run("netsh", "interface", "ipv4", "set", "dnsservers",
			"name="+r.origDNSAdapter, "static", peer, "primary", "validate=no")
	}

	for _, dnsIP := range []string{"1.1.1.1", "8.8.8.8"} {
		if err := run("route", "add", dnsIP, "mask", "255.255.255.255", gw, "metric", "1"); err != nil {
			slog.Debug("public dns bypass route add failed", "ip", dnsIP, "err", err)
			continue
		}
		r.dnsBypass = append(r.dnsBypass, dnsIP)
	}

	for _, exc := range r.exclusions {
		exc = strings.TrimSpace(exc)
		if exc == "" {
			continue
		}
		if _, ipnet, err := net.ParseCIDR(exc); err == nil {
			mask := net.IP(ipnet.Mask).String()
			if err := run("route", "add", ipnet.IP.String(), "mask", mask, gw, "metric", "1"); err == nil {
				r.netBypass = append(r.netBypass, ipnet.String())
			}
			continue
		}
		if ip := net.ParseIP(exc); ip != nil && ip.To4() != nil {
			if err := run("route", "add", ip.String(), "mask", "255.255.255.255", gw, "metric", "1"); err == nil {
				r.hostBypass = append(r.hostBypass, ip.String())
			}
			continue
		}
		ips, err := net.LookupIP(exc)
		if err != nil {
			continue
		}
		for _, ip := range ips {
			if ip.To4() != nil {
				if err := run("route", "add", ip.String(), "mask", "255.255.255.255", gw, "metric", "1"); err == nil {
					r.hostBypass = append(r.hostBypass, ip.String())
				}
			}
		}
	}

	// Split-default: 0.0.0.0/1 + 128.0.0.0/1 через TUN
	for _, half := range []string{"0.0.0.0", "128.0.0.0"} {
		if err := run("route", "add", half, "mask", "128.0.0.0", "0.0.0.0", "if", r.tunIndex, "metric", "1"); err != nil {
			return fmt.Errorf("route add %s/1: %w", half, err)
		}
	}

	r.installed = true
	return nil
}

func (r *RouteManager) Restore() error {
	if !r.installed {
		return nil
	}
	r.installed = false

	var errs []string
	for _, half := range []string{"0.0.0.0", "128.0.0.0"} {
		if err := run("route", "delete", half, "mask", "128.0.0.0"); err != nil {
			errs = append(errs, fmt.Sprintf("delete %s/1: %v", half, err))
		}
	}
	if err := run("route", "delete", r.endpointIP, "mask", "255.255.255.255"); err != nil {
		errs = append(errs, fmt.Sprintf("delete endpoint route: %v", err))
	}

	if r.origDNSAdapter != "" {
		if len(r.origDNSServers) > 0 {
			run("netsh", "interface", "ipv4", "set", "dnsservers",
				"name="+r.origDNSAdapter, "static", r.origDNSServers[0], "primary", "validate=no")
			for _, s := range r.origDNSServers[1:] {
				run("netsh", "interface", "ipv4", "add", "dnsservers",
					"name="+r.origDNSAdapter, s, "validate=no")
			}
		} else {
			run("netsh", "interface", "ipv4", "set", "dnsservers",
				"name="+r.origDNSAdapter, "dhcp")
		}
	}

	for _, dnsIP := range r.dnsBypass {
		run("route", "delete", dnsIP, "mask", "255.255.255.255")
	}
	r.mu.Lock()
	for _, ip := range r.hostBypass {
		run("route", "delete", ip, "mask", "255.255.255.255")
	}
	for _, netAddr := range r.netBypass {
		_, ipnet, _ := net.ParseCIDR(netAddr)
		if ipnet != nil {
			run("route", "delete", ipnet.IP.String(), "mask", net.IP(ipnet.Mask).String())
		}
	}
	r.dnsBypass = nil
	r.hostBypass = nil
	r.netBypass = nil
	r.mu.Unlock()

	if len(errs) > 0 {
		return fmt.Errorf("restore routes: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (r *RouteManager) AddDynamicHostBypassBatch(ips []net.IP) {
	if len(ips) == 0 || r.origGateway == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ip := range ips {
		if ip.To4() == nil {
			continue
		}
		ipStr := ip.String()
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
		if err := run("route", "add", ipStr, "mask", "255.255.255.255", r.origGateway, "metric", "1"); err == nil {
			r.hostBypass = append(r.hostBypass, ipStr)
		}
	}
}

func defaultRoute() (gateway, iface string, err error) {
	out, err := exec.Command("route", "print", "0.0.0.0").Output()
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "0.0.0.0") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 5 && fields[1] == "0.0.0.0" {
			gateway = fields[2]
			iface = fields[3]
			return gateway, iface, nil
		}
	}
	return "", "", fmt.Errorf("could not parse default route")
}

func getTUNIndex(name string) (string, error) {
	out, err := exec.Command("netsh", "interface", "ipv4", "show", "interfaces").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, name) {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) >= 1 {
				return fields[0], nil
			}
		}
	}
	return "", fmt.Errorf("interface %s not found", name)
}

func getDefaultNetAdapter() string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"(Get-NetRoute -DestinationPrefix '0.0.0.0/0' | Sort-Object RouteMetric | Select-Object -First 1).InterfaceAlias").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func getDNSForAdapter(adapter string) []string {
	out, err := exec.Command("netsh", "interface", "ipv4", "show", "dnsservers", "name="+adapter).Output()
	if err != nil {
		return nil
	}
	var servers []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		ip := net.ParseIP(line)
		if ip != nil && ip.To4() != nil {
			servers = append(servers, ip.String())
		}
	}
	return servers
}

func run(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s: %w", cmd, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func RecoverDNS() {
	adapter := getDefaultNetAdapter()
	if adapter == "" {
		return
	}
	servers := getDNSForAdapter(adapter)
	for _, s := range servers {
		if strings.HasPrefix(s, "10.99.0.") {
			slog.Warn("stale VPN DNS detected, restoring", "dns", s, "adapter", adapter)
			run("netsh", "interface", "ipv4", "set", "dnsservers",
				"name="+adapter, "dhcp")
			for _, half := range []string{"0.0.0.0", "128.0.0.0"} {
				run("route", "delete", half, "mask", "128.0.0.0")
			}
			for _, dns := range []string{"1.1.1.1", "8.8.8.8"} {
				run("route", "delete", dns, "mask", "255.255.255.255")
			}
			return
		}
	}
}
