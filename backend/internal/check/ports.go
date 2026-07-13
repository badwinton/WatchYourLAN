package check

import (
	"net"
	"strings"
	"sync"

	"github.com/aceberg/WatchYourLAN/internal/models"
	"github.com/aceberg/WatchYourLAN/internal/portscan"
)

// commonPorts is the curated list of TCP ports probed during auto port scan.
// Kept short and high-signal to limit scan time and network noise.
var commonPorts = []string{
	"21", "22", "23", "53", "80", "81", "110", "135", "139", "143",
	"443", "445", "587", "631", "3306", "3389", "5357", "5900",
	"8080", "8443", "8883", "9100",
}

const maxPortConcurrency = 50

// portScanEnabled is injected by the scan routine (which owns the conf package)
// to avoid an import cycle between check and conf.
var portScanEnabled bool

// SetPortScan configures the automatic port scan used by EnrichHosts.
func SetPortScan(enabled bool) {
	portScanEnabled = enabled
}

// scanPorts probes common TCP ports for every host and stores the open ones in
// the Ports field as a comma-separated list. Runs concurrently with a bounded
// worker pool; only hosts with a valid IP are scanned.
func scanPorts(hosts []models.Host) {
	if !portScanEnabled {
		return
	}

	sem := make(chan struct{}, maxPortConcurrency)
	var wg sync.WaitGroup

	for i := range hosts {
		ip := hosts[i].IP
		if net.ParseIP(ip) == nil {
			continue
		}
		wg.Add(1)
		go func(idx int, ip string) {
			defer wg.Done()

			open := make([]string, 0, len(commonPorts))
			for _, port := range commonPorts {
				sem <- struct{}{}
				ok := portscan.IsOpen(ip, port)
				<-sem
				if ok {
					open = append(open, port)
				}
			}
			if len(open) > 0 {
				hosts[idx].Ports = strings.Join(open, ",")
			}
		}(i, ip)
	}

	wg.Wait()
}
