package client

import (
	"crypto/x509"
)

// tlsPoolFromPEM создаёт CertPool из одного или нескольких сертификатов в PEM.
// Возвращает nil если декодирование не удалось.
func tlsPoolFromPEM(pem []byte) *x509.CertPool {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil
	}
	return pool
}
