package scope

import "testing"

func TestMatcherInclusionAndExclusion(t *testing.T) {
	defs := []Definition{
		{Definition: "10.0.0.0/24", Type: "include"},
		{Definition: "10.0.0.50-10.0.0.60", Type: "exclude"},
		{Definition: "10.0.0.5", Type: "exclude"},
	}
	m, err := NewMatcher(defs, false)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}

	tests := []struct {
		ip   string
		want bool
	}{
		{"10.0.0.1", true},
		{"10.0.0.5", false},    // excluded single IP
		{"10.0.0.55", false},   // excluded range override
		{"10.0.1.200", false},  // outside include
		{"192.168.1.1", false}, // outside include
	}

	for _, tt := range tests {
		got, err := m.InScope(tt.ip)
		if err != nil {
			t.Fatalf("InScope(%s): %v", tt.ip, err)
		}
		if got != tt.want {
			t.Fatalf("InScope(%s)=%v want %v", tt.ip, got, tt.want)
		}
	}
}

func TestCIDRRangeSingle(t *testing.T) {
	defs := []Definition{
		{Definition: "192.168.1.0/30", Type: "include"},
		{Definition: "192.168.2.5-192.168.2.7", Type: "include"},
		{Definition: "2001:db8::1", Type: "include"},
	}
	m, err := NewMatcher(defs, false)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}

	cases := map[string]bool{
		"192.168.1.0": true,
		"192.168.1.2": true,
		"192.168.1.4": false,
		"192.168.2.6": true,
		"192.168.2.8": false,
		"2001:db8::1": true,
		"2001:db8::2": false,
	}
	for ip, want := range cases {
		got, err := m.InScope(ip)
		if err != nil {
			t.Fatalf("InScope(%s): %v", ip, err)
		}
		if got != want {
			t.Fatalf("InScope(%s)=%v want %v", ip, got, want)
		}
	}
}

func TestNoIncludeDefaultsToOut(t *testing.T) {
	m, err := NewMatcher(nil, false)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}
	if got, _ := m.InScope("10.0.0.1"); got {
		t.Fatalf("expected default out-of-scope when no includes")
	}
}

func TestIncludeAllByDefault(t *testing.T) {
	m, err := NewMatcher(nil, true)
	if err != nil {
		t.Fatalf("new matcher: %v", err)
	}
	if got, _ := m.InScope("10.0.0.1"); !got {
		t.Fatalf("expected default in-scope when includeAllByDefault")
	}
}

func TestInvalidDefinitionsError(t *testing.T) {
	badDefs := []Definition{
		{Definition: "not-an-ip", Type: "include"},
		{Definition: "10.0.0.1-abc", Type: "include"},
		{Definition: "10.0.0.0/33", Type: "include"},
		{Definition: "10.0.0.1", Type: "unknown"},
	}
	for _, d := range badDefs {
		if _, err := NewMatcher([]Definition{d}, false); err == nil {
			t.Fatalf("expected error for definition %+v", d)
		}
	}
}
