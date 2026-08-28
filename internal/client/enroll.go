package client

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type EnrollState struct {
	URL         string `json:"url"`
	Fingerprint string `json:"fingerprint"`
	ProfileID   string `json:"profile_id,omitempty"`
}

type enrollRequest struct {
	Fingerprint string `json:"fingerprint"`
	Name        string `json:"name,omitempty"`
	User        string `json:"user,omitempty"`
}

type enrollError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type EnrollResult struct {
	OK        bool   `json:"ok"`
	ProfileID string `json:"profile_id,omitempty"`
	Error     string `json:"error,omitempty"`
	Message   string `json:"message,omitempty"`
	Revoked   bool   `json:"revoked,omitempty"`
}

func DeviceFingerprint() (string, error) {
	mid, err := machineID()
	if err != nil {
		return "", fmt.Errorf("machine id: %w", err)
	}
	h := sha256.Sum256([]byte(mid + ":firetunnel"))
	return hex.EncodeToString(h[:]), nil
}

func Enroll(enrollURL string, profiles *ProfileStore) (*EnrollResult, error) {
	fp, err := DeviceFingerprint()
	if err != nil {
		return nil, err
	}

	hostname, _ := os.Hostname()
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME")
	}

	body, _ := json.Marshal(enrollRequest{
		Fingerprint: fp,
		Name:        hostname,
		User:        username,
	})

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(enrollURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("enroll request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusOK {
		return handleEnrollSuccess(enrollURL, fp, respBody, profiles)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return &EnrollResult{Error: "rate_limit", Message: "Слишком частые запросы, повторите позже"}, nil
	}

	var apiErr enrollError
	json.Unmarshal(respBody, &apiErr)

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		deleteEnrolledProfile(enrollURL, profiles)
		removeEnrollState(enrollURL)
		return &EnrollResult{
			Revoked: true,
			Error:   apiErr.Error,
			Message: apiErr.Message,
		}, nil
	}

	if resp.StatusCode >= 500 {
		slog.Warn("enroll server error, keeping existing config", "status", resp.StatusCode)
		return &EnrollResult{Error: "server_error", Message: fmt.Sprintf("Сервер вернул %d, используем существующий конфиг", resp.StatusCode)}, nil
	}

	return &EnrollResult{Error: apiErr.Error, Message: apiErr.Message}, nil
}

func handleEnrollSuccess(enrollURL, fp string, configData []byte, profiles *ProfileStore) (*EnrollResult, error) {
	var doc ProfileTOML
	if err := toml.Unmarshal(configData, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	name := profileNameFromURL(enrollURL)
	if doc.Endpoint.Hostname != "" {
		name = doc.Endpoint.Hostname
	}

	existingID := findEnrolledProfileID(enrollURL)

	if existingID != "" {
		if p, err := profiles.Get(existingID); err == nil {
			os.WriteFile(p.Path, configData, 0600)
			slog.Info("enroll: updated existing profile", "id", existingID, "name", name)
			saveEnrollState(enrollURL, fp, existingID)
			return &EnrollResult{OK: true, ProfileID: existingID}, nil
		}
	}

	p, err := profiles.ImportContent(configData, name+".toml")
	if err != nil {
		return nil, fmt.Errorf("save profile: %w", err)
	}

	saveEnrollState(enrollURL, fp, p.ID)
	slog.Info("enroll: new profile created", "id", p.ID, "name", name)
	return &EnrollResult{OK: true, ProfileID: p.ID}, nil
}

func profileNameFromURL(url string) string {
	parts := strings.Split(strings.TrimRight(url, "/"), "/")
	if len(parts) > 0 {
		token := parts[len(parts)-1]
		if len(token) > 8 {
			return "enroll-" + token[:8]
		}
		return "enroll-" + token
	}
	return "enrolled"
}

func enrollDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(base, "ttclient", "enroll")
	os.MkdirAll(dir, 0700)
	return dir
}

func enrollStateFile(enrollURL string) string {
	h := sha256.Sum256([]byte(enrollURL))
	return filepath.Join(enrollDir(), hex.EncodeToString(h[:8])+".json")
}

func saveEnrollState(enrollURL, fp, profileID string) {
	data, _ := json.Marshal(EnrollState{
		URL:         enrollURL,
		Fingerprint: fp,
		ProfileID:   profileID,
	})
	os.WriteFile(enrollStateFile(enrollURL), data, 0600)
}

func removeEnrollState(enrollURL string) {
	os.Remove(enrollStateFile(enrollURL))
}

func findEnrolledProfileID(enrollURL string) string {
	data, err := os.ReadFile(enrollStateFile(enrollURL))
	if err != nil {
		return ""
	}
	var state EnrollState
	json.Unmarshal(data, &state)
	return state.ProfileID
}

func deleteEnrolledProfile(enrollURL string, profiles *ProfileStore) {
	id := findEnrolledProfileID(enrollURL)
	if id != "" && profiles != nil {
		profiles.Delete(id)
		slog.Info("enroll: deleted revoked profile", "id", id)
	}
}

func LoadAllEnrollStates() []EnrollState {
	dir := enrollDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var states []EnrollState
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var s EnrollState
		if json.Unmarshal(data, &s) == nil && s.URL != "" {
			states = append(states, s)
		}
	}
	return states
}

func CheckAllEnrollments(profiles *ProfileStore) []EnrollResult {
	states := LoadAllEnrollStates()
	if len(states) == 0 {
		return nil
	}
	var results []EnrollResult
	for _, s := range states {
		result, err := Enroll(s.URL, profiles)
		if err != nil {
			slog.Warn("enroll check failed", "url", maskURL(s.URL), "err", err)
			continue
		}
		if result.Revoked {
			slog.Warn("enroll: device revoked", "url", maskURL(s.URL), "message", result.Message)
		}
		results = append(results, *result)
	}
	return results
}

func maskURL(u string) string {
	parts := strings.Split(u, "/")
	if len(parts) > 0 {
		last := parts[len(parts)-1]
		if len(last) > 4 {
			parts[len(parts)-1] = last[:4] + "***"
		}
	}
	return strings.Join(parts, "/")
}
