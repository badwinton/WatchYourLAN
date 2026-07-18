package check

import (
	"bytes"
	"context"
	"encoding/xml"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/aceberg/WatchYourLAN/internal/models"
)

// DNS - returns DNS names of a host
func DNS(host models.Host) (name, dns string) {
	names := lookupHostNames(host.IP)

	if len(names) > 0 {
		name = names[0]
		dns = strings.Join(names, " ")
	}

	return name, dns
}

type hostIdentity struct {
	names            []string
	hardware         string
	deviceType       string
	manufacturer     string
	model            string
	modelNumber      string
	modelDescription string
	serial           string
	presentationURL  string
	services         []string
}

// EnrichHosts fills host names from local DNS, mDNS and SSDP/UPnP discovery.
func EnrichHosts(hosts []models.Host) []models.Host {
	ipSet := make(map[string]struct{})

	for _, host := range hosts {
		if net.ParseIP(host.IP) != nil {
			ipSet[host.IP] = struct{}{}
		}
	}

	avahi := make(map[string]hostIdentity)
	ssdp := make(map[string]hostIdentity)

	// Resolve unknown hardware with a (cached, opt-in) external MAC vendor
	// lookup. Runs first so its vendor name takes precedence over the generic
	// mDNS/SSDP device classes applied below.
	resolveVendors(hosts)

	// Optionally probe common TCP ports so the UI can show what services a
	// host exposes.
	scanPorts(hosts)

	// Optionally run nmap for OS detection and service version scanning.
	scanNmap(hosts)

	// Skip the expensive mDNS/SSDP discovery when every found host already
	// has a usable name and known hardware. DNS reverse lookups below are
	// still attempted for each IP as they are comparatively cheap.
	if needsDiscovery(hosts) {
		avahi = discoverAvahiBrowse(ipSet)
		ssdp = discoverSSDP(ipSet)
	}

	for i := range hosts {
		names := lookupHostNames(hosts[i].IP)

		if identity, ok := avahi[hosts[i].IP]; ok {
			names = appendUnique(names, identity.names...)

			if isUnknownHardware(hosts[i].Hw) && identity.deviceType != "" {
				hosts[i].Hw = identity.deviceType
			}
		}

		if identity, ok := ssdp[hosts[i].IP]; ok {
			names = appendUnique(names, identity.names...)

			if isUnknownHardware(hosts[i].Hw) && identity.hardware != "" {
				hosts[i].Hw = identity.hardware
			}
			if isUnknownHardware(hosts[i].Hw) && identity.deviceType != "" {
				hosts[i].Hw = identity.deviceType
			}
		}

		if len(names) > 0 {
			hosts[i].Name = names[0]
			hosts[i].DNS = strings.Join(names, " ")
		}
	}

	return hosts
}

// needsDiscovery reports whether any host still lacks a usable name or has
// unknown hardware, in which case the (costly) mDNS/SSDP discovery is worth
// running.
func needsDiscovery(hosts []models.Host) bool {
	for _, host := range hosts {
		if strings.TrimSpace(host.Name) == "" || isUnknownHardware(host.Hw) {
			return true
		}
	}
	return false
}

func lookupHostNames(ip string) []string {
	names := []string{}

	dnsNames, _ := net.LookupAddr(ip)
	names = appendUnique(names, dnsNames...)
	names = appendUnique(names, getentNames(ip)...)
	names = appendUnique(names, avahiNames(ip)...)

	return names
}

func getentNames(ip string) []string {
	out, ok := runOutput(700*time.Millisecond, "getent", "hosts", ip)
	if !ok {
		return nil
	}

	fields := strings.Fields(out)
	if len(fields) < 2 {
		return nil
	}

	return fields[1:]
}

func avahiNames(ip string) []string {
	out, ok := runOutput(900*time.Millisecond, "avahi-resolve-address", ip)
	if !ok {
		return nil
	}

	fields := strings.Fields(out)
	if len(fields) < 2 {
		return nil
	}

	return fields[1:]
}

func runOutput(timeout time.Duration, name string, args ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", false
	}

	return string(out), true
}

func discoverAvahiBrowse(targetIPs map[string]struct{}) map[string]hostIdentity {
	identities := make(map[string]hostIdentity)
	if len(targetIPs) == 0 {
		return identities
	}

	out, ok := runOutput(8*time.Second, "avahi-browse", "-artp")
	if !ok {
		return identities
	}

	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "=") {
			continue
		}

		parts := strings.Split(line, ";")
		if len(parts) < 9 {
			continue
		}

		ip := strings.TrimSpace(parts[7])
		if _, ok := targetIPs[ip]; !ok {
			continue
		}

		identity := identities[ip]
		identity.names = appendUnique(identity.names, parts[3], parts[6])
		identity.services = appendUnique(identity.services, parts[4])
		if deviceType := mdnsDeviceType(parts[4]); deviceType != "" {
			identity.deviceType = deviceType
		}
		identities[ip] = identity
	}

	return identities
}

func discoverSSDP(targetIPs map[string]struct{}) map[string]hostIdentity {
	identities := make(map[string]hostIdentity)
	if len(targetIPs) == 0 {
		return identities
	}

	addr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return identities
	}

	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		slog.Debug("SSDP listen failed", "err", err)
		return identities
	}
	defer conn.Close()

	message := strings.Join([]string{
		"M-SEARCH * HTTP/1.1",
		"HOST:239.255.255.250:1900",
		`MAN:"ssdp:discover"`,
		"MX:1",
		"ST:ssdp:all",
		"",
		"",
	}, "\r\n")

	for i := 0; i < 2; i++ {
		_, _ = conn.WriteTo([]byte(message), addr)
	}

	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	seenLocations := make(map[string]struct{})

	for {
		buffer := make([]byte, 65535)
		n, remote, err := conn.ReadFrom(buffer)
		if err != nil {
			break
		}

		responseIP, _, err := net.SplitHostPort(remote.String())
		if err != nil {
			responseIP = remote.String()
		}

		headers := parseSSDPHeaders(buffer[:n])
		location := strings.TrimSpace(headers["location"])
		if location == "" {
			continue
		}
		if _, seen := seenLocations[location]; seen {
			continue
		}
		seenLocations[location] = struct{}{}

		ip := matchingLocationIP(location, responseIP, targetIPs)
		if ip == "" {
			continue
		}

		identity := identities[ip]
		desc := fetchSSDPDescription(location)
		identity.names = appendUnique(identity.names, desc["friendlyName"])
		identity.hardware = buildSSDPHardware(desc, headers["server"])
		identity.manufacturer = desc["manufacturer"]
		identity.model = desc["modelName"]
		identity.modelNumber = desc["modelNumber"]
		identity.modelDescription = desc["modelDescription"]
		identity.serial = desc["serialNumber"]
		identity.presentationURL = desc["presentationURL"]
		if identity.deviceType == "" {
			identity.deviceType = ssdpDeviceType(desc["deviceType"])
		}
		identities[ip] = identity
	}

	return identities
}

func parseSSDPHeaders(payload []byte) map[string]string {
	headers := make(map[string]string)

	for _, line := range strings.Split(string(payload), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		headers[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}

	return headers
}

func matchingLocationIP(location, responseIP string, targetIPs map[string]struct{}) string {
	if _, ok := targetIPs[responseIP]; ok {
		return responseIP
	}

	parsed, err := url.Parse(location)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}

	locationIP := net.ParseIP(parsed.Hostname())
	if locationIP != nil {
		ip := locationIP.String()
		if _, ok := targetIPs[ip]; ok {
			return ip
		}
		return ""
	}

	ips, err := net.LookupHost(parsed.Hostname())
	if err != nil {
		return ""
	}
	for _, ip := range ips {
		if _, ok := targetIPs[ip]; ok {
			return ip
		}
	}

	return ""
}

func fetchSSDPDescription(location string) map[string]string {
	result := make(map[string]string)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return result
	}
	req.Header.Set("User-Agent", "WatchYourLAN/2.1.4")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return result
	}

	return parseSSDPDescription(body)
}

func parseSSDPDescription(body []byte) map[string]string {
	result := make(map[string]string)
	wanted := map[string]struct{}{
		"friendlyName":     {},
		"manufacturer":     {},
		"modelName":        {},
		"modelNumber":      {},
		"modelDescription": {},
		"serialNumber":     {},
		"deviceType":       {},
		"presentationURL":  {},
		"UDN":              {},
	}

	decoder := xml.NewDecoder(bytes.NewReader(body))
	current := ""

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch item := token.(type) {
		case xml.StartElement:
			if _, ok := wanted[item.Name.Local]; ok {
				current = item.Name.Local
			}
		case xml.CharData:
			if current != "" {
				result[current] = cleanName(string(item))
			}
		case xml.EndElement:
			if item.Name.Local == current {
				current = ""
			}
		}
	}

	return result
}

// mdnsDeviceType maps a DNS-SD service type (e.g. "_airplay._tcp") to a
// human-readable device class. Returns "" when no mapping is known so callers
// can ignore it.
func mdnsDeviceType(serviceType string) string {
	t := strings.ToLower(strings.TrimSpace(serviceType))
	switch {
	case strings.Contains(t, "_airplay"), strings.Contains(t, "_raop"):
		return "Apple TV / AirPlay"
	case strings.Contains(t, "_googlecast"):
		return "Chromecast / Google TV"
	case strings.Contains(t, "_hue"):
		return "Philips Hue"
	case strings.Contains(t, "_hap"):
		return "Apple HomeKit"
	case strings.Contains(t, "_ipp"), strings.Contains(t, "_printer"):
		return "Printer"
	case strings.Contains(t, "_ssh"), strings.Contains(t, "_sftp"):
		return "SSH Server"
	case strings.Contains(t, "_smb"), strings.Contains(t, "samba"):
		return "SMB / File Share"
	case strings.Contains(t, "_nas"), strings.Contains(t, "_adisk"):
		return "NAS / Storage"
	case strings.Contains(t, "_spotify"):
		return "Spotify Connect"
	case strings.Contains(t, "sonos"):
		return "Sonos"
	case strings.Contains(t, "_rfb"):
		return "VNC"
	case strings.Contains(t, "_http"):
		return "Web Device"
	case strings.Contains(t, "_workstation"):
		return "Workstation"
	case strings.Contains(t, "_mediaserver"), strings.Contains(t, "_dacp"):
		return "Media Server"
	case strings.Contains(t, "camera"), strings.Contains(t, "_dvrcam"):
		return "Camera"
	}
	return ""
}

// ssdpDeviceType maps a UPnP deviceType URN (e.g.
// "urn:schemas-upnp-org:device:MediaRenderer:1") to a short class label.
func ssdpDeviceType(deviceType string) string {
	t := strings.ToLower(strings.TrimSpace(deviceType))
	switch {
	case strings.Contains(t, "internetgatewaydevice"), strings.Contains(t, "wanconnectiondevice"):
		return "Router / Gateway"
	case strings.Contains(t, "mediarenderer"), strings.Contains(t, "mediaplayer"):
		return "Media Renderer"
	case strings.Contains(t, "mediaserver"):
		return "Media Server"
	case strings.Contains(t, "remotedisplay"):
		return "Streaming Box"
	case strings.Contains(t, "printer"):
		return "Printer"
	case strings.Contains(t, "camera"), strings.Contains(t, "cameras"):
		return "Camera"
	case strings.Contains(t, "storage"), strings.Contains(t, "nas"):
		return "NAS / Storage"
	case strings.Contains(t, "phone"), strings.Contains(t, "voip"):
		return "VoIP / Phone"
	case strings.Contains(t, "tv"), strings.Contains(t, "television"):
		return "Smart TV"
	}
	return ""
}

// buildSSDPHardware assembles the most useful hardware string from a parsed
// UPnP description, falling back to the SSDP SERVER header when the
// manufacturer/model fields are absent, and appending the serial number when
// present.
func buildSSDPHardware(desc map[string]string, server string) string {
	hw := joinNonEmpty(
		desc["manufacturer"],
		desc["modelName"],
		desc["modelNumber"],
		desc["modelDescription"],
	)
	if hw == "" && server != "" {
		hw = server
	}
	if sn := desc["serialNumber"]; sn != "" {
		if hw == "" {
			hw = "S/N " + sn
		} else {
			hw = hw + " (S/N " + sn + ")"
		}
	}
	return hw
}

func appendUnique(items []string, more ...string) []string {
	seen := make(map[string]struct{}, len(items)+len(more))
	out := make([]string, 0, len(items)+len(more))

	for _, item := range append(items, more...) {
		item = cleanName(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}

	return out
}

func cleanName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, ".")
	value = strings.Join(strings.Fields(value), " ")

	if value == "" || net.ParseIP(value) != nil {
		return ""
	}

	return value
}

func isUnknownHardware(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || strings.HasPrefix(value, "(Unknown")
}

func joinNonEmpty(parts ...string) string {
	out := []string{}
	for _, part := range parts {
		part = cleanName(part)
		if part != "" {
			out = append(out, part)
		}
	}

	return strings.Join(out, " ")
}
