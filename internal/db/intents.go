package db

import "strings"

const (
	IntentPingSweep = "ping_sweep"
	IntentTop1KTCP  = "top_1k_tcp"
	IntentAllTCP    = "all_tcp"
	IntentTopUDP    = "top_udp"
	IntentVulnNSE   = "vuln_nse"
)

const (
	IntentSourceManual = "manual"
	IntentSourceAuto   = "auto"
)

var coverageIntentOrder = []string{
	IntentPingSweep,
	IntentTop1KTCP,
	IntentAllTCP,
	IntentTopUDP,
	IntentVulnNSE,
}

// CoverageIntentOrder returns the fixed intent display order for coverage views.
func CoverageIntentOrder() []string {
	out := make([]string, len(coverageIntentOrder))
	copy(out, coverageIntentOrder)
	return out
}

// ValidIntent reports whether the given intent is supported.
func ValidIntent(intent string) bool {
	switch strings.TrimSpace(strings.ToLower(intent)) {
	case IntentPingSweep, IntentTop1KTCP, IntentAllTCP, IntentTopUDP, IntentVulnNSE:
		return true
	default:
		return false
	}
}

// ValidIntentSource reports whether the given source is supported.
func ValidIntentSource(source string) bool {
	switch strings.TrimSpace(strings.ToLower(source)) {
	case IntentSourceManual, IntentSourceAuto:
		return true
	default:
		return false
	}
}
