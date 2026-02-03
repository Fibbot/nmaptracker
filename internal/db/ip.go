package db

import "net/netip"

func ipv4ToInt(ip string) (int64, bool) {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is4() {
		return 0, false
	}
	octets := addr.As4()
	value := uint32(octets[0])<<24 | uint32(octets[1])<<16 | uint32(octets[2])<<8 | uint32(octets[3])
	return int64(value), true
}
