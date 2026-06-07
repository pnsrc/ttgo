package server

// tlspeek.go — извлекает TLS ClientHello random из первого TLS record
// без полного парсинга хендшейка. Работает до tls.Server.
//
// TLS ClientHello record layout (упрощённо):
//   [0]    Content-Type = 0x16 (Handshake)
//   [1-2]  Legacy version (0x0301)
//   [3-4]  Length of record payload
//   [5]    Handshake type = 0x01 (ClientHello)
//   [6-8]  Handshake length (3 bytes)
//   [9-10] Client version
//   [11-42] Random (32 bytes)  <-- нам нужно это

import (
	"net"
)

const (
	tlsRecordHeaderLen  = 5
	tlsHandshakeHeader  = 4  // type(1) + length(3)
	tlsVersionLen       = 2
	tlsRandomLen        = 32
	tlsRandomOffset     = tlsRecordHeaderLen + tlsHandshakeHeader + tlsVersionLen // = 11
	tlsMinClientHelloLen = tlsRandomOffset + tlsRandomLen                          // = 43
)

// peekConn — net.Conn который буферизует первые N байт для peek,
// остальные читает как обычно.
type peekConn struct {
	net.Conn
	buf    []byte
	offset int
}

func newPeekConn(c net.Conn, peeked []byte) *peekConn {
	return &peekConn{Conn: c, buf: peeked}
}

func (c *peekConn) Read(b []byte) (int, error) {
	if c.offset < len(c.buf) {
		n := copy(b, c.buf[c.offset:])
		c.offset += n
		return n, nil
	}
	return c.Conn.Read(b)
}

// extractClientRandom читает начало TLS record и возвращает 32-байтный random.
// Возвращает nil если не смогло (не TLS, слишком короткий пакет и т.д.).
// buf должен содержать первые >= 43 байта соединения.
func extractClientRandom(buf []byte) []byte {
	if len(buf) < tlsMinClientHelloLen {
		return nil
	}
	// Content-Type должен быть Handshake (0x16)
	if buf[0] != 0x16 {
		return nil
	}
	// Handshake type должен быть ClientHello (0x01)
	if buf[tlsRecordHeaderLen] != 0x01 {
		return nil
	}
	random := make([]byte, tlsRandomLen)
	copy(random, buf[tlsRandomOffset:tlsRandomOffset+tlsRandomLen])
	return random
}
