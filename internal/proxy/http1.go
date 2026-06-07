package proxy

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
)

// serveHTTP1Tunnel обрабатывает CONNECT для HTTP/1.1 через hijack.
// При HTTP/2 ResponseWriter не поддерживает Hijack — используется copyFlush.
// При HTTP/1.1 нужен hijack для полного bidirectional streaming.
func serveHTTP1Tunnel(w http.ResponseWriter, r *http.Request, target, username string, dialer *net.Dialer) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		// не должно случиться для HTTP/1.1, но на всякий случай
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return
	}

	conn, err := dialer.DialContext(r.Context(), "tcp", target)
	if err != nil {
		slog.Warn("h1 dial failed", "target", target, "user", username, "err", err)
		http.Error(w, fmt.Sprintf("cannot connect to %s", target), http.StatusBadGateway)
		return
	}
	defer conn.Close()

	clientConn, brw, err := hj.Hijack()
	if err != nil {
		slog.Warn("hijack failed", "err", err)
		conn.Close()
		return
	}
	defer clientConn.Close()

	// Шлём 200 вручную — после Hijack писать в w нельзя
	_, _ = fmt.Fprint(clientConn, "HTTP/1.1 200 Connection established\r\n\r\n")

	slog.Debug("h1 tunnel open", "target", target, "user", username)

	// Флушим буферизованные данные которые клиент уже успел прислать
	var clientReader io.Reader = clientConn
	if brw.Reader.Buffered() > 0 {
		clientReader = io.MultiReader(brw.Reader, clientConn)
	}

	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(conn, clientReader)
		errCh <- err
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()
	go func() {
		_, err := io.Copy(clientConn, conn)
		errCh <- err
		if tc, ok := clientConn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()

	<-errCh
	slog.Debug("h1 tunnel closed", "target", target, "user", username)
}

// isHTTP1 возвращает true если запрос пришёл по HTTP/1.x.
func isHTTP1(r *http.Request) bool {
	return r.ProtoMajor == 1
}
