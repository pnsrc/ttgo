package admin

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/acme"
)

const letsEncryptURL = "https://acme-v02.api.letsencrypt.org/directory"

// ObtainCert gets a Let's Encrypt certificate for domain.
// Requires port 80 to be free (HTTP-01 challenge).
// Saves cert + key to certPath / keyPath.
func ObtainCert(domain, email, certPath, keyPath string, progress func(string)) error {
	accountKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate account key: %w", err)
	}

	client := &acme.Client{
		Key:          accountKey,
		DirectoryURL: letsEncryptURL,
	}

	ctx := context.Background()

	progress("Registering ACME account...")
	acc := &acme.Account{Contact: []string{"mailto:" + email}}
	if _, err := client.Register(ctx, acc, acme.AcceptTOS); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	progress("Requesting authorization...")
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(domain))
	if err != nil {
		return fmt.Errorf("authorize order: %w", err)
	}

	var chalURI, token string
	for _, authURL := range order.AuthzURLs {
		auth, err := client.GetAuthorization(ctx, authURL)
		if err != nil {
			return fmt.Errorf("get auth: %w", err)
		}
		if auth.Status == "valid" {
			continue
		}
		for _, chal := range auth.Challenges {
			if chal.Type == "http-01" {
				chalURI = chal.URI
				token = chal.Token
				break
			}
		}
	}
	if token == "" {
		return fmt.Errorf("no http-01 challenge available")
	}

	keyAuth, err := client.HTTP01ChallengeResponse(token)
	if err != nil {
		return fmt.Errorf("key auth: %w", err)
	}

	progress("Starting HTTP-01 challenge server on :80...")
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/acme-challenge/"+token, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, keyAuth)
	})
	ln, err := net.Listen("tcp", ":80")
	if err != nil {
		return fmt.Errorf("listen :80: %w (make sure port 80 is free)", err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Close()

	progress("Accepting challenge...")
	if _, err := client.Accept(ctx, &acme.Challenge{URI: chalURI, Token: token, Type: "http-01"}); err != nil {
		return fmt.Errorf("accept challenge: %w", err)
	}

	order, err = client.WaitOrder(ctx, order.URI)
	if err != nil {
		return fmt.Errorf("wait order: %w", err)
	}

	progress("Generating certificate key...")
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate cert key: %w", err)
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		DNSNames: []string{domain},
	}, certKey)
	if err != nil {
		return fmt.Errorf("create csr: %w", err)
	}

	progress("Finalizing order...")
	der, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return fmt.Errorf("finalize order: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return err
	}
	certFile, err := os.OpenFile(certPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	for _, d := range der {
		pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: d})
	}
	certFile.Close()

	keyFile, err := os.OpenFile(keyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	keyDer, _ := x509.MarshalECPrivateKey(certKey)
	pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer})
	keyFile.Close()

	progress(fmt.Sprintf("Certificate saved → %s", certPath))
	return nil
}

// CertInfo returns domains and expiry from a PEM cert file.
func CertInfo(certPath string) (domains []string, expiry time.Time, err error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return nil, time.Time{}, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, time.Time{}, fmt.Errorf("no PEM block in %s", certPath)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, time.Time{}, err
	}
	names := cert.DNSNames
	for _, ip := range cert.IPAddresses {
		names = append(names, ip.String())
	}
	return names, cert.NotAfter, nil
}

// GenerateSelfSigned creates a self-signed cert for hostname (domain or IP).
func GenerateSelfSigned(hostname, certPath, keyPath string) error {
	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		SerialNumber: serial,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
	}
	template.Subject.CommonName = hostname

	if ip := net.ParseIP(hostname); ip != nil {
		template.IPAddresses = []net.IP{ip}
	} else {
		template.DNSNames = []string{hostname}
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return err
	}

	cf, err := os.OpenFile(certPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: der})
	cf.Close()

	kf, err := os.OpenFile(keyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	keyDer, _ := x509.MarshalECPrivateKey(key)
	pem.Encode(kf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer})
	kf.Close()

	return nil
}

// VerifyCert checks if cert+key are a valid pair.
func VerifyCert(certPath, keyPath string) error {
	_, err := tls.LoadX509KeyPair(certPath, keyPath)
	return err
}
