package web

import (
	"strings"

	"github.com/sloppy/nmaptracker/internal/db"
)

func portServiceSummary(port db.Port) string {
	parts := []string{port.Service, port.Product, port.Version, port.ExtraInfo}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func hostScopeLabel(host db.Host) string {
	if host.InScope {
		return "In scope"
	}
	return "Out of scope"
}
