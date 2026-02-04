package importer

import (
	"testing"

	"github.com/sloppy/nmaptracker/internal/db"
)

func TestSuggestIntentsInfersTop1KFromDefaultTCPScans(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		args     string
		wantTop1 bool
	}{
		{
			name:     "explicit top ports 1000",
			filename: "scan.xml",
			args:     "nmap --top-ports 1000 192.0.2.10",
			wantTop1: true,
		},
		{
			name:     "default tcp scan",
			filename: "scan.xml",
			args:     "nmap 192.0.2.10",
			wantTop1: true,
		},
		{
			name:     "default tcp with service detection",
			filename: "scan.xml",
			args:     "nmap -sV -Pn 192.0.2.10",
			wantTop1: true,
		},
		{
			name:     "ping sweep only",
			filename: "scan.xml",
			args:     "nmap -sn 192.0.2.10/24",
			wantTop1: false,
		},
		{
			name:     "udp scan only",
			filename: "scan.xml",
			args:     "nmap -sU 192.0.2.10",
			wantTop1: false,
		},
		{
			name:     "explicit port selection",
			filename: "scan.xml",
			args:     "nmap -p 22,80 192.0.2.10",
			wantTop1: false,
		},
		{
			name:     "no args metadata",
			filename: "scan.xml",
			args:     "",
			wantTop1: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suggested := SuggestIntents(tc.filename, tc.args, Observations{})
			gotTop1 := hasSuggestedIntent(suggested, db.IntentTop1KTCP)
			if gotTop1 != tc.wantTop1 {
				t.Fatalf("top_1k intent mismatch for args %q: got %v want %v (suggested=%+v)", tc.args, gotTop1, tc.wantTop1, suggested)
			}
		})
	}
}

func hasSuggestedIntent(items []SuggestedIntent, intent string) bool {
	for _, item := range items {
		if item.Intent == intent {
			return true
		}
	}
	return false
}
