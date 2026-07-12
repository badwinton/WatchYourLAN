package check

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aceberg/WatchYourLAN/internal/models"
)

const (
	ouiCacheFile   = "oui_cache.json"
	macLookupURL   = "https://api.maclookup.app/v2/macs/"
	macLookupTO    = 2 * time.Second
	maxConcurrency = 5
	cooldownPeriod = time.Hour
)

var (
	ouiCacheMu    sync.Mutex
	ouiCache      = map[string]string{}
	ouiLoaded     bool
	cooldownUntil time.Time

	// macLookup configuration is injected by the scan routine (which owns the
	// conf package) to avoid an import cycle between check and conf.
	macLookupEnabled bool
	macLookupKey     string
	macLookupDir     string
)

// SetMacLookup configures the external MAC vendor lookup used by EnrichHosts.
func SetMacLookup(enabled bool, key, dir string) {
	macLookupEnabled = enabled
	macLookupKey = key
	macLookupDir = dir
}

// loadOUICache reads the persisted OUI->vendor map from the config dir. It is
// safe to call repeatedly; the actual read happens only once.
func loadOUICache() {
	ouiCacheMu.Lock()
	defer ouiCacheMu.Unlock()
	if ouiLoaded {
		return
	}
	data, err := os.ReadFile(filepath.Join(macLookupDir, ouiCacheFile))
	if err == nil {
		_ = json.Unmarshal(data, &ouiCache)
	}
	ouiLoaded = true
}

func saveOUICache() {
	data, err := json.MarshalIndent(ouiCache, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(macLookupDir, ouiCacheFile), data, 0644)
}

// applyCooldown pauses MAC lookups. It honours the API's X-RateLimit-Reset
// header when present, otherwise falls back to a fixed window.
func applyCooldown(resp *http.Response) {
	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		if secs, err := strconv.ParseInt(reset, 10, 64); err == nil {
			cooldownUntil = time.Unix(secs, 0)
			return
		}
	}
	cooldownUntil = time.Now().Add(cooldownPeriod)
}

// normalizeOUI returns the 6-char uppercase OUI (first 3 bytes) of a MAC, or
// "" if the input is not a valid MAC.
func normalizeOUI(mac string) string {
	clean := strings.ToUpper(strings.NewReplacer(":", "", "-", "", ".", "").Replace(mac))
	if len(clean) < 6 {
		return ""
	}
	oui := clean[:6]
	for _, c := range oui {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			return ""
		}
	}
	return oui
}

// isLocalAdmin reports whether the MAC's universally/locally-administered bit
// is set. Locally administered addresses have no registered OUI vendor, so
// querying them is pointless.
func isLocalAdmin(mac string) bool {
	clean := strings.ToUpper(strings.NewReplacer(":", "", "-", "", ".", "").Replace(mac))
	if len(clean) < 2 {
		return true
	}
	b, err := strconv.ParseUint(clean[:2], 16, 8)
	if err != nil {
		return true
	}
	return b&0x02 != 0
}

// resolveVendor looks up the vendor for an OUI using the maclookup.app API.
// Results are cached in memory and on disk; on rate limiting it enters a
// cooldown (from the X-RateLimit-Reset header or a fixed window) so subsequent
// scans stop hammering the endpoint. Returns "" when no vendor is resolved.
func resolveVendor(oui string) string {
	loadOUICache()

	ouiCacheMu.Lock()
	if v, ok := ouiCache[oui]; ok {
		ouiCacheMu.Unlock()
		return v
	}
	if time.Now().Before(cooldownUntil) {
		ouiCacheMu.Unlock()
		return ""
	}
	ouiCacheMu.Unlock()

	if !macLookupEnabled {
		return ""
	}

	reqURL := macLookupURL + oui
	if key := strings.TrimSpace(macLookupKey); key != "" {
		reqURL += "?apiKey=" + url.QueryEscape(key)
	}

	client := &http.Client{Timeout: macLookupTO}
	resp, err := client.Get(reqURL)
	if err != nil {
		slog.Debug("MAC lookup request failed", "oui", oui, "err", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		ouiCacheMu.Lock()
		applyCooldown(resp)
		ouiCacheMu.Unlock()
		slog.Warn("MAC lookup rate limited; pausing lookups", "until", cooldownUntil.Format(time.RFC3339))
		return ""
	}
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	// Proactively back off once the quota for this window is exhausted.
	if resp.Header.Get("X-RateLimit-Remaining") == "0" {
		ouiCacheMu.Lock()
		applyCooldown(resp)
		ouiCacheMu.Unlock()
	}

	var data struct {
		Success bool   `json:"success"`
		Found   bool   `json:"found"`
		Company string `json:"company"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ""
	}
	if !data.Success || !data.Found || data.Company == "" ||
		data.Company == "*NO COMPANY*" || data.Company == "*PRIVATE*" {
		return ""
	}

	ouiCacheMu.Lock()
	ouiCache[oui] = data.Company
	ouiCacheMu.Unlock()
	return data.Company
}

// resolveVendors fills the Hardware field of hosts whose vendor is still
// unknown, using the external MAC lookup API. Resolution runs concurrently with
// a bounded worker pool but only when the feature is enabled. Hosts get a
// vendor name that the later mDNS/SSDP step will not override.
func resolveVendors(hosts []models.Host) {
	if !macLookupEnabled {
		return
	}
	loadOUICache()

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var dirtyMu sync.Mutex
	dirty := false

	for i := range hosts {
		if !isUnknownHardware(hosts[i].Hw) {
			continue
		}
		oui := normalizeOUI(hosts[i].Mac)
		if oui == "" || isLocalAdmin(hosts[i].Mac) {
			continue
		}

		wg.Add(1)
		go func(idx int, oui string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			vendor := resolveVendor(oui)
			if vendor == "" {
				return
			}
			ouiCacheMu.Lock()
			hosts[idx].Hw = vendor
			ouiCacheMu.Unlock()
			dirtyMu.Lock()
			dirty = true
			dirtyMu.Unlock()
		}(i, oui)
	}

	wg.Wait()

	ouiCacheMu.Lock()
	if dirty {
		saveOUICache()
	}
	ouiCacheMu.Unlock()
}
