package importer

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseXMLSampleFile(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(filename), "testdata", "sampleNmap.xml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	obs, err := ParseXML(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("parse xml: %v", err)
	}
	if len(obs.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(obs.Hosts))
	}
	host := obs.Hosts[0]
	if host.IPAddress != "127.0.0.1" {
		t.Fatalf("unexpected ip: %s", host.IPAddress)
	}
	if host.Hostname != "localhost" {
		t.Fatalf("unexpected hostname: %s", host.Hostname)
	}
	if host.HostState != "up" {
		t.Fatalf("expected host state up, got %q", host.HostState)
	}
	if len(host.Ports) != 25 {
		t.Fatalf("expected 25 ports, got %d", len(host.Ports))
	}
	for _, p := range host.Ports {
		if p.Protocol != "tcp" {
			t.Fatalf("expected tcp protocol, got %s", p.Protocol)
		}
		if p.State == "" {
			t.Fatalf("expected state set for port %d", p.PortNumber)
		}
		if p.Service == "" {
			t.Fatalf("expected service name for port %d", p.PortNumber)
		}
	}
}

func TestParseXMLWithMetadataIncludesNmapArgs(t *testing.T) {
	xml := `<nmaprun args="nmap -sn 10.0.0.0/24"><host><status state="up"/><address addr="10.0.0.1" addrtype="ipv4"/></host></nmaprun>`
	obs, metadata, err := ParseXMLWithMetadata(strings.NewReader(xml))
	if err != nil {
		t.Fatalf("parse xml with metadata: %v", err)
	}
	if metadata.NmapArgs != "nmap -sn 10.0.0.0/24" {
		t.Fatalf("unexpected nmap args: %q", metadata.NmapArgs)
	}
	if len(obs.Hosts) != 1 || obs.Hosts[0].HostState != "up" {
		t.Fatalf("unexpected observation output: %#v", obs)
	}
}

func TestParseXMLWithScriptsAndVersion(t *testing.T) {
	xml := `
<nmaprun>
  <host>
    <address addr="192.0.2.10" addrtype="ipv4"/>
    <hostnames><hostname name="example.com"/></hostnames>
    <os><osmatch name="linux"/></os>
    <ports>
      <port protocol="tcp" portid="443">
        <state state="open"/>
        <service name="https" product="nginx" version="1.25.4" extrainfo="ubuntu"/>
        <script id="ssl-cert" output="CN=example.com"/>
        <script id="http-title" output="Welcome"/>
      </port>
      <port protocol="udp" portid="53">
        <state state="open"/>
        <service name="domain"/>
      </port>
    </ports>
  </host>
</nmaprun>`
	obs, err := ParseXML(strings.NewReader(xml))
	if err != nil {
		t.Fatalf("parse xml: %v", err)
	}
	if len(obs.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(obs.Hosts))
	}
	h := obs.Hosts[0]
	if h.OSGuess != "linux" {
		t.Fatalf("expected OS linux, got %s", h.OSGuess)
	}
	if len(h.Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(h.Ports))
	}
	var https, dns PortObservation
	for _, p := range h.Ports {
		if p.PortNumber == 443 {
			https = p
		}
		if p.PortNumber == 53 && p.Protocol == "udp" {
			dns = p
		}
	}
	if https.Service != "https" || https.Product != "nginx" || https.Version != "1.25.4" || https.ExtraInfo != "ubuntu" {
		t.Fatalf("https fields unexpected: %#v", https)
	}
	if !strings.Contains(https.ScriptOutput, "ssl-cert") || !strings.Contains(https.ScriptOutput, "http-title") {
		t.Fatalf("script output missing expected entries: %s", https.ScriptOutput)
	}
	if dns.Protocol != "udp" || dns.Service != "domain" {
		t.Fatalf("udp port parse failed: %#v", dns)
	}
}
