package rules

import (
	"encoding/hex"
	"fmt"
	"net"
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
}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) LoadFile(path string) error {
	if path == "" {
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
