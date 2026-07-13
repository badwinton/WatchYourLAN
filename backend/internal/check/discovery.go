package check

import "net"

// HostInfo is the structured, per-host discovery result surfaced on the host
// detail page. It aggregates what mDNS and SSDP/UPnP advertisement for a single
// IP without needing to persist it in the database.
type HostInfo struct {
	IP               string   `json:"ip"`
	Names            []string `json:"names"`
	Manufacturer     string   `json:"manufacturer"`
	Model            string   `json:"model"`
	ModelNumber      string   `json:"modelNumber"`
	ModelDescription string   `json:"modelDescription"`
	Serial           string   `json:"serial"`
	PresentationURL  string   `json:"presentationURL"`
	Hardware         string   `json:"hardware"`
	DeviceType       string   `json:"deviceType"`
	Services         []string `json:"services"`
}

// DiscoverHost runs mDNS and SSDP discovery for a single IP and returns the
// merged, structured result. It reuses the same discovery helpers used during
// the full scan, so it reflects the device's current advertisements.
func DiscoverHost(ip string) HostInfo {
	info := HostInfo{IP: ip}
	if net.ParseIP(ip) == nil {
		return info
	}

	ipSet := map[string]struct{}{ip: {}}
	avahi := discoverAvahiBrowse(ipSet)
	ssdp := discoverSSDP(ipSet)

	if a, ok := avahi[ip]; ok {
		info.Names = a.names
		info.Services = a.services
		info.DeviceType = a.deviceType
	}
	if s, ok := ssdp[ip]; ok {
		info.Names = appendUnique(info.Names, s.names...)
		info.Manufacturer = s.manufacturer
		info.Model = s.model
		info.ModelNumber = s.modelNumber
		info.ModelDescription = s.modelDescription
		info.Serial = s.serial
		info.PresentationURL = s.presentationURL
		info.Hardware = s.hardware
		if info.DeviceType == "" {
			info.DeviceType = s.deviceType
		}
	}

	return info
}
