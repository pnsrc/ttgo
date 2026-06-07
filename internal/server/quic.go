package server

import (
	"time"

	"github.com/quic-go/quic-go"

	"github.com/pnsrc/ttgo/internal/config"
)

func buildQUICConfig(q *config.QUICConfig) *quic.Config {
	cfg := &quic.Config{
		MaxIdleTimeout:                 2 * (30 + 7) * time.Second, // 2*(conn_timeout+health_check)
		InitialStreamReceiveWindow:     1048576,
		MaxStreamReceiveWindow:         16777216,
		InitialConnectionReceiveWindow: 104857600,
		MaxConnectionReceiveWindow:     25165824,
		MaxIncomingStreams:             4096,
		MaxIncomingUniStreams:          4096,
		Allow0RTT:                      true,
		DisablePathMTUDiscovery:        false,
	}

	if q == nil {
		return cfg
	}

	if q.InitialMaxStreamDataBidiRemote > 0 {
		cfg.InitialStreamReceiveWindow = uint64(q.InitialMaxStreamDataBidiRemote)
	}
	if q.MaxStreamWindow > 0 {
		cfg.MaxStreamReceiveWindow = uint64(q.MaxStreamWindow)
	}
	if q.InitialMaxData > 0 {
		cfg.InitialConnectionReceiveWindow = uint64(q.InitialMaxData)
	}
	if q.MaxConnectionWindow > 0 {
		cfg.MaxConnectionReceiveWindow = uint64(q.MaxConnectionWindow)
	}
	if q.InitialMaxStreamsBidi > 0 {
		cfg.MaxIncomingStreams = int64(q.InitialMaxStreamsBidi)
	}
	if q.InitialMaxStreamsUni > 0 {
		cfg.MaxIncomingUniStreams = int64(q.InitialMaxStreamsUni)
	}
	cfg.Allow0RTT = q.EnableEarlyData

	return cfg
}
