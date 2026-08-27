//go:build !darwin && !windows

package client

import (
	"fmt"
	"net"
)

// RouteManager — stub для не-macOS платформ.
// TODO: implement linux (ip route) и windows (netsh / IP Helper API).
type RouteManager struct{}

func NewRouteManager(tunName, endpointHostPort string, exclusions []string) (*RouteManager, error) {
	return nil, fmt.Errorf("RouteManager: platform not yet supported")
}

func (r *RouteManager) Install() error                        { return nil }
func (r *RouteManager) Restore() error                        { return nil }
func (r *RouteManager) AddDynamicHostBypassBatch(ips []net.IP) {}
func RecoverDNS()                                              {}
