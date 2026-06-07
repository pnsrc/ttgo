package admin

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// vpnConfigAdmin — упрощённая версия config.Config для чтения/записи в ttadmin.
// Не импортируем internal/config чтобы не тянуть лишние зависимости.
type vpnConfigAdmin struct {
	ListenAddress   string `toml:"listen_address"`
	IPv6Available   bool   `toml:"ipv6_available"`
	AllowPrivateNet bool   `toml:"allow_private_network_connections"`

	TLSHandshakeTimeoutSecs        int `toml:"tls_handshake_timeout_secs"`
	ClientListenerTimeoutSecs      int `toml:"client_listener_timeout_secs"`
	ConnectionEstablishTimeoutSecs int `toml:"connection_establishment_timeout_secs"`
	TCPConnectionsTimeoutSecs      int `toml:"tcp_connections_timeout_secs"`
	UDPConnectionsTimeoutSecs      int `toml:"udp_connections_timeout_secs"`

	CredentialsFile string `toml:"credentials_file"`
	RulesFile       string `toml:"rules_file"`

	AuthFailureStatusCode int    `toml:"auth_failure_status_code"`
	StoreType             string `toml:"store_type"`
	StoreDSN              string `toml:"store_dsn"`
	CacheTTLSecs          int    `toml:"cache_ttl_secs"`

	// Вложенные секции сохраняем как raw для round-trip без потери данных
	HTTP2 *http2ConfigAdmin `toml:"listen_protocols"`
	Admin *adminConfigAdmin `toml:"admin"`
}

type http2ConfigAdmin struct {
	HTTP2 *struct {
		MaxConcurrentStreams    int `toml:"max_concurrent_streams"`
		InitialStreamWindowSize int `toml:"initial_stream_window_size"`
		MaxFrameSize            int `toml:"max_frame_size"`
	} `toml:"http2"`
}

type adminConfigAdmin struct {
	Address string `toml:"address"`
	Token   string `toml:"token"`
}

func loadVPNConfig(path string) (*vpnConfigAdmin, error) {
	var cfg vpnConfigAdmin
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

// patchVPNStoreType записывает store_type и store_dsn в vpn.toml.
// Делаем простую построчную замену чтобы не сломать комментарии.
func patchVPNStoreType(path, storeType, storeDSN, credFile string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	replaced := map[string]bool{}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "store_type"):
			lines[i] = fmt.Sprintf("store_type = %q", storeType)
			replaced["store_type"] = true
		case strings.HasPrefix(trimmed, "store_dsn"):
			lines[i] = fmt.Sprintf("store_dsn = %q", storeDSN)
			replaced["store_dsn"] = true
		case strings.HasPrefix(trimmed, "credentials_file") && credFile != "":
			lines[i] = fmt.Sprintf("credentials_file = %q", credFile)
			replaced["credentials_file"] = true
		}
	}

	// Добавляем поля если их не было
	insertAfter := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "auth_failure_status_code") {
			insertAfter = i
		}
	}

	var extras []string
	if !replaced["store_type"] {
		extras = append(extras, fmt.Sprintf("store_type = %q", storeType))
	}
	if !replaced["store_dsn"] {
		extras = append(extras, fmt.Sprintf("store_dsn = %q", storeDSN))
	}

	if len(extras) > 0 && insertAfter >= 0 {
		tail := append([]string{}, lines[insertAfter+1:]...)
		lines = append(lines[:insertAfter+1], extras...)
		lines = append(lines, tail...)
	} else if len(extras) > 0 {
		lines = append(lines, extras...)
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}
