package proxy

import (
	"encoding/binary"
	"net"
)

// TrustTunnel кастомный HTTP/2 error code — auth required.
const H2ErrCodeAuthRequired = uint32(0x1F)

// buildGoAwayFrame собирает бинарный HTTP/2 GOAWAY frame.
//
// HTTP/2 GOAWAY (RFC 9113 §6.8):
//   frame header [9 bytes]: length(3) | type(1) | flags(1) | stream_id(4)
//   payload      [8 bytes]: last_stream_id(4) | error_code(4)
func buildGoAwayFrame(lastStreamID uint32, errorCode uint32) []byte {
	frame := make([]byte, 9+8)
	// length = 8
	frame[0], frame[1], frame[2] = 0, 0, 8
	// type = 0x7 (GOAWAY)
	frame[3] = 0x7
	// flags = 0
	frame[4] = 0
	// stream id = 0 (connection-level frame)
	binary.BigEndian.PutUint32(frame[5:9], 0)
	// payload
	binary.BigEndian.PutUint32(frame[9:13], lastStreamID&0x7FFFFFFF)
	binary.BigEndian.PutUint32(frame[13:17], errorCode)
	return frame
}

// SendGoAway пишет HTTP/2 GOAWAY с кодом 0x1F напрямую в net.Conn.
// Используется для принудительного выкидывания клиента с сервера
// (например при инвалидации credentials посреди сессии).
//
// lastStreamID — последний успешно обработанный stream ID,
// клиент может retry стримы с ID > lastStreamID.
func SendGoAway(conn net.Conn, lastStreamID uint32) error {
	_, err := conn.Write(buildGoAwayFrame(lastStreamID, H2ErrCodeAuthRequired))
	return err
}
