package nmapscan

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Result holds the nmap scan output for a single host.
type Result struct {
	OSName  string // e.g. "Apple iOS 15.0 - 15.6"
	DevType string // e.g. "phone", "router", "general purpose"
	Services string // e.g. "http (Apple AirPlay), ssh (OpenSSH 8.9)"
}

// nmapXML is the top-level structure of nmap's XML output.
type nmapXML struct {
	XMLName xml.Name   `xml:"nmaprun"`
	Hosts   []nmapHost `xml:"host"`
}

type nmapHost struct {
	Ports  nmapPorts `xml:"ports"`
	OS     nmapOS    `xml:"os"`
}

type nmapPorts struct {
	Ports []nmapPort `xml:"port"`
}

type nmapPort struct {
	Protocol string     `xml:"protocol,attr"`
	PortID   string     `xml:"portid,attr"`
	Service  nmapService `xml:"service"`
}

type nmapService struct {
	Name    string `xml:"name,attr"`
	Product string `xml:"product,attr"`
	Version string `xml:"version,attr"`
}

type nmapOS struct {
	Matches []nmapOSMatch `xml:"osmatch"`
}

type nmapOSMatch struct {
	Name     string `xml:"name,attr"`
	Accuracy string `xml:"accuracy,attr"`
	Classes  []nmapOSClass `xml:"osclass"`
}

type nmapOSClass struct {
	Type     string `xml:"type,attr"`
	Vendor   string `xml:"vendor,attr"`
	OSFamily string `xml:"osfamily,attr"`
	OSGen    string `xml:"osgen,attr"`
	Accuracy string `xml:"accuracy,attr"`
}

// ScanHost runs nmap OS detection and service version scanning on a single IP.
// Returns a Result with OS, device type, and services, or an error.
func ScanHost(ip string) (Result, error) {
	var result Result

	// -O: OS detection, -sV: service version detection
	// --version-intensity 0: light version probe (faster)
	// -oX -: output XML to stdout
	// -T4: aggressive timing (faster)
	// --host-timeout 15s: cap per-host time
	cmd := exec.Command("nmap", "-O", "-sV", "--version-intensity", "0",
		"-T4", "--host-timeout", "15s", "-oX", "-", ip)

	out, err := cmd.Output()
	if err != nil {
		// nmap returns non-zero for various reasons; try to parse partial output
		if exitErr, ok := err.(*exec.ExitError); ok {
			out = exitErr.Stderr
		}
		if len(out) == 0 {
			return result, fmt.Errorf("nmap failed for %s: %w", ip, err)
		}
		slog.Debug("nmap returned non-zero, parsing partial output", "ip", ip, "err", err)
	}

	var xmlResult nmapXML
	if err := xml.Unmarshal(out, &xmlResult); err != nil {
		return result, fmt.Errorf("nmap XML parse failed for %s: %w", ip, err)
	}

	if len(xmlResult.Hosts) == 0 {
		return result, nil
	}

	host := xmlResult.Hosts[0]

	// Extract OS from best match
	if len(host.OS.Matches) > 0 {
		best := host.OS.Matches[0]
		result.OSName = best.Name
		if len(best.Classes) > 0 {
			result.DevType = best.Classes[0].Type
		}
	}

	// Extract services from open ports
	var services []string
	for _, port := range host.Ports.Ports {
		svc := port.Service.Name
		if port.Service.Product != "" {
			svc = port.Service.Product
			if port.Service.Version != "" {
				svc += " " + port.Service.Version
			}
		}
		services = append(services, fmt.Sprintf("%s/%s", port.PortID, svc))
	}
	if len(services) > 0 {
		result.Services = strings.Join(services, ", ")
	}

	return result, nil
}

// ScanHosts runs nmap on multiple IPs concurrently with a bounded worker pool.
// Returns a map of IP -> Result for hosts where nmap found data.
func ScanHosts(ips []string, maxConcurrency int) map[string]Result {
	results := make(map[string]Result)
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}

	sem := make(chan struct{}, maxConcurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, ip := range ips {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			slog.Debug("nmap scanning", "ip", ip)
			start := time.Now()
			result, err := ScanHost(ip)
			elapsed := time.Since(start)
			if err != nil {
				slog.Debug("nmap scan failed", "ip", ip, "err", err, "elapsed", elapsed)
				return
			}
			slog.Debug("nmap scan complete", "ip", ip, "os", result.OSName, "elapsed", elapsed)

			if result.OSName != "" || result.DevType != "" || result.Services != "" {
				mu.Lock()
				results[ip] = result
				mu.Unlock()
			}
		}(ip)
	}

	wg.Wait()
	return results
}
