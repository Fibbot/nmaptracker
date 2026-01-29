package scope

import (
	"testing"
)

func TestMatcher(t *testing.T) {
	defs := []string{
		"10.0.0.0/24",
		"192.168.1.5",
	}
	m, _ := NewMatcher(defs)

	tests := []struct {
		ip   string
		want bool
	}{
		{"10.0.0.1", true},
		{"10.0.0.254", true},
		{"10.0.1.1", false},
		{"192.168.1.5", true},
		{"192.168.1.6", false},
		{"8.8.8.8", false},
	}

	for _, tc := range tests {
		got := m.InScope(tc.ip)
		if got != tc.want {
			t.Errorf("InScope(%q) = %v; want %v", tc.ip, got, tc.want)
		}
	}
}

func TestMatcherEmpty(t *testing.T) {
	m, _ := NewMatcher([]string{})
	if !m.InScope("1.2.3.4") {
		t.Error("Empty matcher should include everything by default")
	}
}
