package scope

import (
	"fmt"
	"net/netip"
	"strings"
)

// Matcher evaluates whether an IP is in scope given include/exclude rules.
// If no includes are provided, nothing is in scope unless the "includeAllByDefault"
// flag is set at construction (we default to false for safety).
type Matcher struct {
	includeAllByDefault bool
	includes            []predicate
	excludes            []predicate
}

type predicate func(netip.Addr) bool

// Definition represents a scope entry.
type Definition struct {
	Definition string
	Type       string // "include" or "exclude"
}

// NewMatcher builds a matcher from definitions. includeAllByDefault controls
// behavior when no includes are present (spec defaults to nothing in scope).
func NewMatcher(defs []Definition, includeAllByDefault bool) (*Matcher, error) {
	m := &Matcher{includeAllByDefault: includeAllByDefault}
	for _, d := range defs {
		pred, err := parseDefinition(d.Definition)
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", d.Definition, err)
		}
		switch strings.ToLower(d.Type) {
		case "include":
			m.includes = append(m.includes, pred)
		case "exclude":
			m.excludes = append(m.excludes, pred)
		default:
			return nil, fmt.Errorf("unknown definition type %q", d.Type)
		}
	}
	return m, nil
}

// InScope applies exclusion precedence: any matching exclude returns false,
// otherwise returns true if an include matches (or includeAllByDefault).
func (m *Matcher) InScope(ip string) (bool, error) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false, fmt.Errorf("parse ip %q: %w", ip, err)
	}

	for _, ex := range m.excludes {
		if ex(addr) {
			return false, nil
		}
	}

	for _, inc := range m.includes {
		if inc(addr) {
			return true, nil
		}
	}

	return m.includeAllByDefault, nil
}

func parseDefinition(def string) (predicate, error) {
	def = strings.TrimSpace(def)
	if def == "" {
		return nil, fmt.Errorf("empty definition")
	}

	if strings.Contains(def, "/") {
		prefix, err := netip.ParsePrefix(def)
		if err != nil {
			return nil, fmt.Errorf("parse cidr: %w", err)
		}
		return func(a netip.Addr) bool { return prefix.Contains(a) }, nil
	}

	if strings.Contains(def, "-") {
		parts := strings.Split(def, "-")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid range %q", def)
		}
		start, err := netip.ParseAddr(strings.TrimSpace(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("parse range start: %w", err)
		}
		end, err := netip.ParseAddr(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("parse range end: %w", err)
		}
		if start.Compare(end) > 0 {
			start, end = end, start
		}
		return func(a netip.Addr) bool {
			return start.Compare(a) <= 0 && end.Compare(a) >= 0
		}, nil
	}

	addr, err := netip.ParseAddr(def)
	if err != nil {
		return nil, fmt.Errorf("parse ip: %w", err)
	}
	return func(a netip.Addr) bool { return a == addr }, nil
}
