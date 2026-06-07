package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
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

	AuthFailureStatusCode int `toml:"auth_failure_status_code"`

	// store_type: "file" (default) | "sqlite" | "postgres"
	StoreType    string `toml:"store_type"`
	// store_dsn: путь к sqlite / postgres DSN
	// для "file" — не нужен, используется credentials_file
	StoreDSN     string `toml:"store_dsn"`
	// cache_ttl_secs: TTL кэша в памяти (default 30)
	CacheTTLSecs int    `toml:"cache_ttl_secs"`

	ListenProtocols ListenProtocols `toml:"listen_protocols"`
	ForwardProtocol ForwardProtocol `toml:"forward_protocol"`
	ICMP            *ICMPConfig     `toml:"icmp"`
	Metrics         *MetricsConfig  `toml:"metrics"`
}

type ListenProtocols struct {
	HTTP1 *HTTP1Config `toml:"http1"`
	HTTP2 *HTTP2Config `toml:"http2"`
	QUIC  *QUICConfig  `toml:"quic"`
}

type HTTP1Config struct {
	UploadBufferSize int `toml:"upload_buffer_size"`
}

type HTTP2Config struct {
	InitialConnectionWindowSize int `toml:"initial_connection_window_size"`
	InitialStreamWindowSize     int `toml:"initial_stream_window_size"`
	MaxConcurrentStreams         int `toml:"max_concurrent_streams"`
	MaxFrameSize                int `toml:"max_frame_size"`
	HeaderTableSize             int `toml:"header_table_size"`
}

type QUICConfig struct {
	RecvUDPPayloadSize             int  `toml:"recv_udp_payload_size"`
	SendUDPPayloadSize             int  `toml:"send_udp_payload_size"`
	InitialMaxData                 int  `toml:"initial_max_data"`
	InitialMaxStreamDataBidiLocal  int  `toml:"initial_max_stream_data_bidi_local"`
	InitialMaxStreamDataBidiRemote int  `toml:"initial_max_stream_data_bidi_remote"`
	InitialMaxStreamDataUni        int  `toml:"initial_max_stream_data_uni"`
	InitialMaxStreamsBidi          int  `toml:"initial_max_streams_bidi"`
	InitialMaxStreamsUni            int  `toml:"initial_max_streams_uni"`
	MaxConnectionWindow            int  `toml:"max_connection_window"`
	MaxStreamWindow                int  `toml:"max_stream_window"`
	DisableActiveMigration         bool `toml:"disable_active_migration"`
	EnableEarlyData                bool `toml:"enable_early_data"`
	MessageQueueCapacity           int  `toml:"message_queue_capacity"`
}

type ForwardProtocol struct {
	Direct *struct{}     `toml:"direct"`
	SOCKS5 *SOCKS5Config `toml:"socks5"`
}

type SOCKS5Config struct {
	Address      string `toml:"address"`
	ExtendedAuth bool   `toml:"extended_auth"`
}

type ICMPConfig struct {
	InterfaceName       string `toml:"interface_name"`
	RequestTimeoutSecs  int    `toml:"request_timeout_secs"`
	RecvMessageQueueCap int    `toml:"recv_message_queue_capacity"`
}

type MetricsConfig struct {
	Address            string `toml:"address"`
	RequestTimeoutSecs int    `toml:"request_timeout_secs"`
}

// HostsConfig — hosts.toml
type HostsConfig struct {
	MainHosts         []TLSHost `toml:"main_hosts"`
	PingHosts         []TLSHost `toml:"ping_hosts"`
	SpeedtestHosts    []TLSHost `toml:"speedtest_hosts"`
	ReverseProxyHosts []TLSHost `toml:"reverse_proxy_hosts"`
}

type TLSHost struct {
	Hostname       string `toml:"hostname"`
	CertChainPath  string `toml:"cert_chain_path"`
	PrivateKeyPath string `toml:"private_key_path"`
}

func defaults(c *Config) {
	if c.ListenAddress == "" {
		c.ListenAddress = "0.0.0.0:443"
	}
	if c.TLSHandshakeTimeoutSecs == 0 {
		c.TLSHandshakeTimeoutSecs = 10
	}
	if c.ClientListenerTimeoutSecs == 0 {
		c.ClientListenerTimeoutSecs = 600
	}
	if c.ConnectionEstablishTimeoutSecs == 0 {
		c.ConnectionEstablishTimeoutSecs = 30
	}
	if c.TCPConnectionsTimeoutSecs == 0 {
		c.TCPConnectionsTimeoutSecs = 604800
	}
	if c.UDPConnectionsTimeoutSecs == 0 {
		c.UDPConnectionsTimeoutSecs = 300
	}
	if c.AuthFailureStatusCode == 0 {
		c.AuthFailureStatusCode = 407
	}
	if c.StoreType == "" {
		c.StoreType = "file"
	}
	if c.CacheTTLSecs == 0 {
		c.CacheTTLSecs = 30
	}
	if c.ListenProtocols.HTTP2 == nil {
		c.ListenProtocols.HTTP2 = &HTTP2Config{
			InitialConnectionWindowSize: 8388608,
			InitialStreamWindowSize:     131072,
			MaxConcurrentStreams:        1000,
			MaxFrameSize:               16384,
			HeaderTableSize:            65536,
		}
	}
}

func Load(vpnPath string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(vpnPath, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", vpnPath, err)
	}
	defaults(&cfg)
	return &cfg, nil
}

func LoadHosts(hostsPath string) (*HostsConfig, error) {
	var hc HostsConfig
	if _, err := toml.DecodeFile(hostsPath, &hc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", hostsPath, err)
	}
	return &hc, nil
}
