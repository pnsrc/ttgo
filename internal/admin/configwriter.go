package admin

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// UnmarshalEditableConfig — обёртка чтобы из server-пакета не зависеть от admin.
func UnmarshalEditableConfig(raw []byte, out *EditableConfig) error {
	return json.Unmarshal(raw, out)
}

// EditableConfig — поля vpn.toml, доступные для правки через UI.
// nil = не менять.
type EditableConfig struct {
	ListenAddress                  *string `json:"listen_address,omitempty"`
	AllowPrivateNet                *bool   `json:"allow_private_net,omitempty"`
	IPv6Available                  *bool   `json:"ipv6_available,omitempty"`
	CacheTTLSecs                   *int    `json:"cache_ttl_secs,omitempty"`
	TLSHandshakeTimeoutSecs        *int    `json:"tls_handshake_timeout_secs,omitempty"`
	ConnectionEstablishTimeoutSecs *int    `json:"connect_timeout_secs,omitempty"`
	TCPConnectionsTimeoutSecs      *int    `json:"tcp_idle_timeout_secs,omitempty"`
	UDPConnectionsTimeoutSecs      *int    `json:"udp_idle_timeout_secs,omitempty"`
}

// PatchVPNConfig мерджит частичные изменения в vpn.toml построчно,
// сохраняя комментарии и порядок ключей. Если ключа нет — добавляет
// в конец top-level секции.
func PatchVPNConfig(path string, edit EditableConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	type patch struct {
		key    string
		value  string
		raw    bool // true = записать как есть (без кавычек)
		applied bool
	}

	var patches []*patch
	add := func(key, val string, raw bool) {
		patches = append(patches, &patch{key: key, value: val, raw: raw})
	}

	if edit.ListenAddress != nil {
		add("listen_address", fmt.Sprintf("%q", *edit.ListenAddress), true)
	}
	if edit.AllowPrivateNet != nil {
		add("allow_private_network_connections", fmt.Sprintf("%t", *edit.AllowPrivateNet), true)
	}
	if edit.IPv6Available != nil {
		add("ipv6_available", fmt.Sprintf("%t", *edit.IPv6Available), true)
	}
	if edit.CacheTTLSecs != nil {
		add("cache_ttl_secs", fmt.Sprintf("%d", *edit.CacheTTLSecs), true)
	}
	if edit.TLSHandshakeTimeoutSecs != nil {
		add("tls_handshake_timeout_secs", fmt.Sprintf("%d", *edit.TLSHandshakeTimeoutSecs), true)
	}
	if edit.ConnectionEstablishTimeoutSecs != nil {
		add("connection_establishment_timeout_secs", fmt.Sprintf("%d", *edit.ConnectionEstablishTimeoutSecs), true)
	}
	if edit.TCPConnectionsTimeoutSecs != nil {
		add("tcp_connections_timeout_secs", fmt.Sprintf("%d", *edit.TCPConnectionsTimeoutSecs), true)
	}
	if edit.UDPConnectionsTimeoutSecs != nil {
		add("udp_connections_timeout_secs", fmt.Sprintf("%d", *edit.UDPConnectionsTimeoutSecs), true)
	}

	if len(patches) == 0 {
		return nil
	}

	lines := strings.Split(string(data), "\n")

	// Найдём начало первой секции (строка с "[xxx]" в начале), чтобы не вставлять
	// top-level ключи внутрь секций.
	firstSectionIdx := len(lines)
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "[") {
			firstSectionIdx = i
			break
		}
	}

	// Заменяем существующие
	for i := 0; i < firstSectionIdx; i++ {
		trimmed := strings.TrimLeft(lines[i], " \t")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// извлекаем ключ
		eq := strings.Index(trimmed, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:eq])
		for _, p := range patches {
			if p.key == key {
				lines[i] = fmt.Sprintf("%s = %s", key, p.value)
				p.applied = true
				break
			}
		}
	}

	// Добавляем отсутствующие — перед первой секцией (или в конец).
	var extras []string
	for _, p := range patches {
		if !p.applied {
			extras = append(extras, fmt.Sprintf("%s = %s", p.key, p.value))
		}
	}
	if len(extras) > 0 {
		before := lines[:firstSectionIdx]
		after := lines[firstSectionIdx:]
		// добавим пустую строку между existing и extras если нужно
		if len(before) > 0 && strings.TrimSpace(before[len(before)-1]) != "" {
			before = append(before, "")
		}
		newLines := make([]string, 0, len(lines)+len(extras)+1)
		newLines = append(newLines, before...)
		newLines = append(newLines, extras...)
		if len(after) > 0 && strings.TrimSpace(after[0]) != "" {
			newLines = append(newLines, "")
		}
		newLines = append(newLines, after...)
		lines = newLines
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
