package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	url := c.Address + "/users/kick?username=" + username
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err // сервер недоступен — не страшно
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("admin api: %s", resp.Status)
	}
	return nil
}

// ActiveUsers возвращает map[username]connections_count.
func (c *AdminClient) ActiveUsers() (map[string]int, error) {
	if c == nil || c.Address == "" {
		return nil, nil
	}
	req, _ := http.NewRequest(http.MethodGet, c.Address+"/users/active", nil) //nolint:errcheck
	req.Header.Set("Authorization", "Bearer "+c.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
