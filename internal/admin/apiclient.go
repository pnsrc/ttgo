package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AdminClient вызывает admin API сервера для мгновенного кика юзера.
type AdminClient struct {
	Address string // например "http://127.0.0.1:9090"
	Token   string
}

// KickUser инвалидирует кэш и посылает GOAWAY активным соединениям.
// Возвращает nil если API недоступен (просто логируем, не критично —
// файл-стор сам перечитается за ≤10с).
func (c *AdminClient) KickUser(username string) error {
	if c == nil || c.Address == "" || c.Token == "" {
		return nil
	}
	return c.post("/users/kick?username=" + username)
}

// KickSession кикает одно конкретное соединение по remote_addr.
func (c *AdminClient) KickSession(username, remoteAddr string) error {
	if c == nil || c.Address == "" || c.Token == "" {
		return nil
	}
	q := "username=" + username + "&remote_addr=" + remoteAddr
	return c.post("/sessions/kick?" + q)
}

func (c *AdminClient) post(path string) error {
	req, err := http.NewRequest(http.MethodPost, c.Address+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("admin api %s: %s", path, resp.Status)
	}
	return nil
}

// ActiveUsers возвращает map[username]connections_count.
func (c *AdminClient) ActiveUsers() (map[string]int, error) {
	if c == nil || c.Address == "" {
		return nil, nil
	}
	var result map[string]int
	if err := c.getJSON("/users/active", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SessionInfo дублирует server.SessionInfo (avoid import cycle).
type SessionInfo struct {
	Username     string    `json:"username"`
	RemoteAddr   string    `json:"remote_addr"`
	ConnectedAt  time.Time `json:"connected_at"`
	BytesIn      uint64    `json:"bytes_in"`
	BytesOut     uint64    `json:"bytes_out"`
	OpenTunnels  int32     `json:"open_tunnels"`
	TotalTunnels uint64    `json:"total_tunnels"`
}

// UserStats дублирует server.UserStats.
type UserStats struct {
	Username     string    `json:"username"`
	ActiveConns  int       `json:"active_conns"`
	BytesIn      uint64    `json:"bytes_in"`
	BytesOut     uint64    `json:"bytes_out"`
	OpenTunnels  int32     `json:"open_tunnels"`
	TotalTunnels uint64    `json:"total_tunnels"`
	FirstSeenAt  time.Time `json:"first_seen_at"`
}

func (c *AdminClient) Sessions() ([]SessionInfo, error) {
	if c == nil || c.Address == "" {
		return nil, nil
	}
	var out []SessionInfo
	if err := c.getJSON("/sessions", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *AdminClient) UserStats() ([]UserStats, error) {
	if c == nil || c.Address == "" {
		return nil, nil
	}
	var out []UserStats
	if err := c.getJSON("/stats", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *AdminClient) getJSON(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.Address+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("admin api %s: %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
