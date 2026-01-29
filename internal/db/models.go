package db

import "time"

// Project represents the top-level grouping.
type Project struct {
	ID        int64
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ScopeDefinition captures includes/excludes.
type ScopeDefinition struct {
	ID         int64
	ProjectID  int64
	Definition string
	Type       string
	CreatedAt  time.Time
}

// ScanImport tracks import history metadata.
type ScanImport struct {
	ID         int64
	ProjectID  int64
	Filename   string
	ImportTime time.Time
	HostsFound int
	PortsFound int
}

// Host represents a scanned host.
type Host struct {
	ID        int64
	ProjectID int64
	IPAddress string
	Hostname  string
	OSGuess   string
	InScope   bool
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Port represents a port observation for a host.
type Port struct {
	ID           int64
	HostID       int64
	PortNumber   int
	Protocol     string
	State        string
	Service      string
	Version      string
	Product      string
	ExtraInfo    string
	WorkStatus   string
	ScriptOutput string
	Notes        string
	LastSeen     time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
