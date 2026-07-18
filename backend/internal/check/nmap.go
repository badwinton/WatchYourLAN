package check

import (
	"log/slog"
	"net"

	"github.com/aceberg/WatchYourLAN/internal/models"
	"github.com/aceberg/WatchYourLAN/internal/nmapscan"
)

var nmapScanEnabled bool

// SetNmapScan configures the automatic nmap OS/service detection.
func SetNmapScan(enabled bool) {
	nmapScanEnabled = enabled
}

// scanNmap runs nmap OS detection and service version scanning for all hosts
// and populates the OsName, DevType, and Services fields.
func scanNmap(hosts []models.Host) {
	if !nmapScanEnabled {
		return
	}

	var ips []string
	for _, h := range hosts {
		if net.ParseIP(h.IP) == nil {
			continue
		}
		ips = append(ips, h.IP)
	}

	if len(ips) == 0 {
		return
	}

	slog.Info("Running nmap OS/service detection", "hosts", len(ips))
	results := nmapscan.ScanHosts(ips, 10)

	for i := range hosts {
		if r, ok := results[hosts[i].IP]; ok {
			if r.OSName != "" {
				hosts[i].OsName = r.OSName
			}
			if r.DevType != "" {
				hosts[i].DevType = r.DevType
			}
			if r.Services != "" {
				hosts[i].Services = r.Services
			}
		}
	}
}
