package scope

import (
	"net/netip"
)

type Rule struct {
	Definition string
	Type       string // "ip" or "cidr"
	prefix     netip.Prefix
	addr       netip.Addr
}

type Matcher struct {
	rules []Rule
}

func NewMatcher(definitions []string) (*Matcher, error) {
	var rules []Rule
	for _, def := range definitions {
		rule := Rule{Definition: def}

		// Try parsing as CIDR first
		if prefix, err := netip.ParsePrefix(def); err == nil {
			rule.Type = "cidr"
			rule.prefix = prefix
			rules = append(rules, rule)
			continue
		}

		// Try parsing as IP
		if addr, err := netip.ParseAddr(def); err == nil {
			rule.Type = "ip"
			rule.addr = addr
			rules = append(rules, rule)
			continue
		}

		// Invalid - skip invalid entries as per plan robustness
	}

	return &Matcher{rules: rules}, nil
}

func (m *Matcher) InScope(ip string) bool {
	if len(m.rules) == 0 {
		// No rules defined = everything in scope
		return true
	}

	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	for _, rule := range m.rules {
		switch rule.Type {
		case "cidr":
			if rule.prefix.Contains(addr) {
				return true
			}
		case "ip":
			if rule.addr == addr {
				return true
			}
		}
	}

	return false
}
