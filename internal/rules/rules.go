package rules

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
)

type Rule struct {
	CIDR               string `toml:"cidr"`
	ClientRandomPrefix string `toml:"client_random_prefix"`
	Action             Action `toml:"action"`

	// compiled
	network *net.IPNet
	prefix  []byte
	mask    []byte
}

type Engine struct {
	mu    sync.RWMutex
	rules []*Rule
	path  string // запоминаем для Save()
}

func New() *Engine {
	return &Engine{}
}

// Path возвращает путь к rules.toml (для отображения в UI).
func (e *Engine) Path() string {
	return e.path
}

// Snapshot возвращает копию rules с экспортируемыми полями для admin API.
type RuleView struct {
	CIDR               string `json:"cidr"`
	ClientRandomPrefix string `json:"client_random_prefix"`
	Action             string `json:"action"`
}

func (e *Engine) Snapshot() []RuleView {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]RuleView, 0, len(e.rules))
	for _, r := range e.rules {
		out = append(out, RuleView{
			CIDR:               r.CIDR,
			ClientRandomPrefix: r.ClientRandomPrefix,
			Action:             string(r.Action),
		})
	}
	return out
}

// Set заменяет полный набор правил, компилирует и сохраняет в файл если path задан.
func (e *Engine) Set(views []RuleView) error {
	compiled := make([]*Rule, 0, len(views))
	for i, v := range views {
		r := &Rule{
			CIDR:               v.CIDR,
			ClientRandomPrefix: v.ClientRandomPrefix,
			Action:             Action(v.Action),
		}
		if err := compile(r); err != nil {
			return fmt.Errorf("rule %d: %w", i, err)
		}
		compiled = append(compiled, r)
	}
	e.mu.Lock()
	e.rules = compiled
	e.mu.Unlock()
	if e.path == "" {
		return nil
	}
	return saveRulesFile(e.path, views)
}

func saveRulesFile(path string, views []RuleView) error {
	var sb strings.Builder
	for _, v := range views {
		sb.WriteString("[[rule]]\n")
		if v.CIDR != "" {
			sb.WriteString(fmt.Sprintf("cidr = %q\n", v.CIDR))
		}
		if v.ClientRandomPrefix != "" {
			sb.WriteString(fmt.Sprintf("client_random_prefix = %q\n", v.ClientRandomPrefix))
		}
		sb.WriteString(fmt.Sprintf("action = %q\n\n", v.Action))
	}
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

func (e *Engine) LoadFile(path string) error {
	e.mu.Lock()
	e.path = path
	e.mu.Unlock()
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// rules.toml не обязателен
		return nil
	}
	type rulesFile struct {
		Rules []*Rule `toml:"rule"`
	}
	var rf rulesFile
	if _, err := toml.DecodeFile(path, &rf); err != nil {
		return fmt.Errorf("parse rules: %w", err)
	}
	for _, r := range rf.Rules {
		if err := compile(r); err != nil {
			return fmt.Errorf("compile rule: %w", err)
		}
	}
	e.mu.Lock()
	e.rules = rf.Rules
	e.mu.Unlock()
	return nil
}

// compile parses CIDR and client_random_prefix into binary form.
func compile(r *Rule) error {
	if r.CIDR != "" {
		_, network, err := net.ParseCIDR(r.CIDR)
		if err != nil {
			return fmt.Errorf("invalid cidr %q: %w", r.CIDR, err)
		}
		r.network = network
	}
	if r.ClientRandomPrefix != "" {
		if err := parseClientRandom(r); err != nil {
			return err
		}
	}
	if r.Action != ActionAllow && r.Action != ActionDeny {
		return fmt.Errorf("invalid action %q", r.Action)
	}
	return nil
}

// parseClientRandom handles:
//
//	"aabbcc"      — simple prefix match
//	"a0b0/f0f0"  — bitwise match with mask
func parseClientRandom(r *Rule) error {
	s := r.ClientRandomPrefix
	if idx := strings.Index(s, "/"); idx >= 0 {
		prefix, err := hex.DecodeString(s[:idx])
		if err != nil {
			return fmt.Errorf("client_random_prefix hex: %w", err)
		}
		mask, err := hex.DecodeString(s[idx+1:])
		if err != nil {
			return fmt.Errorf("client_random_prefix mask: %w", err)
		}
		if len(prefix) != len(mask) {
			return fmt.Errorf("client_random_prefix/mask length mismatch")
		}
		r.prefix = prefix
		r.mask = mask
	} else {
		prefix, err := hex.DecodeString(s)
		if err != nil {
			return fmt.Errorf("client_random_prefix: %w", err)
		}
		r.prefix = prefix
		r.mask = nil
	}
	return nil
}

// Allow returns true if the connection should be permitted.
// clientIP — remote IP, clientRandom — 32-byte TLS ClientHello random.
func (e *Engine) Allow(clientIP net.IP, clientRandom []byte) bool {
	e.mu.RLock()
	rules := e.rules
	e.mu.RUnlock()

	for _, r := range rules {
		if !matchIP(r, clientIP) {
			continue
		}
		if !matchRandom(r, clientRandom) {
			continue
		}
		return r.Action == ActionAllow
	}
	return true // default allow
}

func matchIP(r *Rule, ip net.IP) bool {
	if r.network == nil {
		return true
	}
	return r.network.Contains(ip)
}

func matchRandom(r *Rule, random []byte) bool {
	if r.prefix == nil {
		return true
	}
	if len(random) < len(r.prefix) {
		return false
	}
	if r.mask == nil {
		return string(random[:len(r.prefix)]) == string(r.prefix)
	}
	for i := range r.prefix {
		if random[i]&r.mask[i] != r.prefix[i]&r.mask[i] {
			return false
		}
	}
	return true
}
