package server

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"sync"

	"github.com/pnsrc/ttgo/internal/config"
)

// TLSManager держит сертификаты по hostname, поддерживает hot reload.
type TLSManager struct {
	mu    sync.RWMutex
	certs map[string]*tls.Certificate
}

func NewTLSManager() *TLSManager {
	return &TLSManager{certs: make(map[string]*tls.Certificate)}
}

func (m *TLSManager) Load(hosts []config.TLSHost) error {
	certs := make(map[string]*tls.Certificate, len(hosts))
	for _, h := range hosts {
		cert, err := tls.LoadX509KeyPair(h.CertChainPath, h.PrivateKeyPath)
		if err != nil {
			return fmt.Errorf("load cert for %s: %w", h.Hostname, err)
		}
		c := cert
		certs[h.Hostname] = &c
		slog.Info("loaded TLS cert", "hostname", h.Hostname)
	}
	m.mu.Lock()
	m.certs = certs
	m.mu.Unlock()
	return nil
}

func (m *TLSManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	m.mu.RLock()
	cert, ok := m.certs[hello.ServerName]
	if !ok {
		for _, c := range m.certs {
			cert = c
			break
		}
	}
	m.mu.RUnlock()
	if cert == nil {
		return nil, fmt.Errorf("no certificate for %q", hello.ServerName)
	}
	return cert, nil
}
