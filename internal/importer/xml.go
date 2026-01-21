package importer

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Internal parsing structs matching nmap XML.
type nmapRun struct {
	Hosts []nmapHost `xml:"host"`
}

type nmapHost struct {
	Addresses []nmapAddress `xml:"address"`
	Hostnames nmapHostnames `xml:"hostnames"`
	Ports     []nmapPort    `xml:"ports>port"`
	OS        nmapOS        `xml:"os"`
}

type nmapAddress struct {
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"`
}

type nmapHostnames struct {
	Hostnames []nmapHostname `xml:"hostname"`
}

type nmapHostname struct {
	Name string `xml:"name,attr"`
}

type nmapPort struct {
	Protocol string       `xml:"protocol,attr"`
	PortID   int          `xml:"portid,attr"`
	State    nmapState    `xml:"state"`
	Service  nmapService  `xml:"service"`
	Scripts  []nmapScript `xml:"script"`
}

type nmapState struct {
	State string `xml:"state,attr"`
}

type nmapService struct {
	Name      string `xml:"name,attr"`
	Product   string `xml:"product,attr"`
	Version   string `xml:"version,attr"`
	ExtraInfo string `xml:"extrainfo,attr"`
}

type nmapScript struct {
	ID     string `xml:"id,attr"`
	Output string `xml:"output,attr"`
}

type nmapOS struct {
	Matches []nmapOSMatch `xml:"osmatch"`
}

type nmapOSMatch struct {
	Name string `xml:"name,attr"`
}

func parseXML(r io.Reader) (Observations, error) {
	var run nmapRun
	dec := xml.NewDecoder(r)
	if err := dec.Decode(&run); err != nil {
		return Observations{}, fmt.Errorf("decode xml: %w", err)
	}

	var obs Observations
	for _, h := range run.Hosts {
		host := HostObservation{
			IPAddress: firstIPv4(h.Addresses),
			Hostname:  firstHostname(h.Hostnames),
			OSGuess:   firstOS(h.OS),
		}
		for _, p := range h.Ports {
			host.Ports = append(host.Ports, PortObservation{
				PortNumber:   p.PortID,
				Protocol:     strings.ToLower(p.Protocol),
				State:        strings.ToLower(p.State.State),
				Service:      p.Service.Name,
				Version:      p.Service.Version,
				Product:      p.Service.Product,
				ExtraInfo:    p.Service.ExtraInfo,
				ScriptOutput: joinScripts(p.Scripts),
			})
		}
		obs.Hosts = append(obs.Hosts, host)
	}
	return obs, nil
}

func firstIPv4(addrs []nmapAddress) string {
	for _, a := range addrs {
		if strings.ToLower(a.AddrType) == "ipv4" {
			return a.Addr
		}
	}
	if len(addrs) > 0 {
		return addrs[0].Addr
	}
	return ""
}

func firstHostname(h nmapHostnames) string {
	if len(h.Hostnames) == 0 {
		return ""
	}
	return h.Hostnames[0].Name
}

func firstOS(os nmapOS) string {
	if len(os.Matches) == 0 {
		return ""
	}
	return os.Matches[0].Name
}

func joinScripts(scripts []nmapScript) string {
	if len(scripts) == 0 {
		return ""
	}
	var parts []string
	for _, s := range scripts {
		if s.ID != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", s.ID, s.Output))
		} else {
			parts = append(parts, s.Output)
		}
	}
	return strings.Join(parts, "\n")
}
