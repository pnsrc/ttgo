package server

import (
	"context"
	"io"
	"net"
	"net/http"
)

type ctxRandomKey struct{}
type ctxSessionKey struct{}
type ctxRawConnKey struct{}

// TLSRandomFromContext возвращает TLS ClientHello random (32 байта) или nil.
func TLSRandomFromContext(ctx context.Context) []byte {
	v, _ := ctx.Value(ctxRandomKey{}).([]byte)
	return v
}

// SessionFromContext возвращает per-session объект.
func SessionFromContext(ctx context.Context) *Session {
	v, _ := ctx.Value(ctxSessionKey{}).(*Session)
	return v
}

func contextWithRandom(ctx context.Context, random []byte) context.Context {
	return context.WithValue(ctx, ctxRandomKey{}, random)
}

func contextWithSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, ctxSessionKey{}, s)
}

// RawConnFromContext возвращает исходный net.Conn для данного соединения.
func RawConnFromContext(ctx context.Context) net.Conn {
	v, _ := ctx.Value(ctxRawConnKey{}).(net.Conn)
	return v
}

func contextWithRawConn(ctx context.Context, c net.Conn) context.Context {
	return context.WithValue(ctx, ctxRawConnKey{}, c)
}

// randomConn — net.Conn с буферизованными первыми байтами и client random.
type randomConn struct {
	net.Conn
	buf    []byte
	pos    int
	random []byte
}

func (c *randomConn) Read(b []byte) (int, error) {
	if c.pos < len(c.buf) {
		n := copy(b, c.buf[c.pos:])
		c.pos += n
		return n, nil
	}
	return c.Conn.Read(b)
}

// randomListener оборачивает net.Listener — пикает ClientHello random.
type randomListener struct {
	net.Listener
}

func NewRandomListener(l net.Listener) *randomListener {
	return &randomListener{l}
}

func (l *randomListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		buf := make([]byte, tlsMinClientHelloLen)
		n, err := io.ReadAtLeast(conn, buf, tlsMinClientHelloLen)
		if err != nil {
			conn.Close()
			continue
		}
		random := extractClientRandom(buf[:n])
		return &randomConn{
			Conn:   conn,
			buf:    buf[:n],
			random: random,
		}, nil
	}
}

// ConnContextFunc — передаётся в http.Server.ConnContext.
//
// Цепочка для HTTP/2:
//   net.Listen → randomListener → *randomConn
//   tls.NewListener → *tls.Conn{ NetConn: *randomConn }
//   ConnContext(ctx, *tls.Conn)
//     → tc.NetConn().(*randomConn).random ✓
//
// ctx здесь уже отменяемый на время жизни соединения (http.Server гарантирует).
// Вешаем на него cleanup сессии.
func ConnContextFunc() func(context.Context, net.Conn) context.Context {
	return func(ctx context.Context, c net.Conn) context.Context {
		var random []byte
		var rawConn net.Conn
		if tc, ok := c.(interface{ NetConn() net.Conn }); ok {
			inner := tc.NetConn()
			rawConn = inner
			if rc, ok := inner.(*randomConn); ok {
				random = rc.random
				rawConn = rc.Conn // реальный TCP conn под буфером
			}
		}
		if random != nil {
			ctx = contextWithRandom(ctx, random)
		}
		if rawConn != nil {
			ctx = contextWithRawConn(ctx, rawConn)
		}

		// newSession вешает cleanup UDP на ctx.Done()
		sess := newSession(ctx)
		ctx = contextWithSession(ctx, sess)
		return ctx
	}
}

// WrapHandler добавляет Alt-Svc header если HTTP/3 включён.
func WrapHandler(h http.Handler, port string) http.Handler {
	if port == "" {
		return h
	}
	altSvc := `h3="` + port + `"; ma=86400`
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Alt-Svc", altSvc)
		h.ServeHTTP(w, r)
	})
}
