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
	// ouiCacheTTL bounds how often an OUI is queried: a resolved (or
	// not-found) OUI is cached for this long, so each OUI is looked up at
	// most once per day even across restarts (the cache is persisted).
	ouiCacheTTL = 24 * time.Hour
)

type ouiEntry struct {
	Vendor string `json:"vendor"`
	Ts     int64  `json:"ts"`
}

var (
	ouiCacheMu sync.Mutex
	ouiCache   = map[string]ouiEntry{}
	ouiLoaded  bool
	cacheDirty bool
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

// NormalizeOUI is the exported wrapper around normalizeOUI for use by other
// packages (e.g. the API handler).
func NormalizeOUI(mac string) string {
	return normalizeOUI(mac)
}

// ResolveVendorForOUI is the exported wrapper around resolveVendor for use by
// other packages (e.g. the API handler). It looks up the vendor for the given
// OUI using the external MAC lookup API.
func ResolveVendorForOUI(oui string) string {
	return resolveVendor(oui)
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
// Results (including not-found) are cached with a TTL and persisted on disk, so
// each OUI is queried at most once per ouiCacheTTL. On rate limiting it enters
// a cooldown (from the X-RateLimit-Reset header or a fixed window).
func resolveVendor(oui string) string {
	loadOUICache()

	ouiCacheMu.Lock()
	if e, ok := ouiCache[oui]; ok {
		if time.Now().Before(time.Unix(e.Ts, 0).Add(ouiCacheTTL)) {
			ouiCacheMu.Unlock()
			return e.Vendor
		}
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
		return "" // don't cache network errors
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		ouiCacheMu.Lock()
		applyCooldown(resp)
		ouiCacheMu.Unlock()
		slog.Warn("MAC lookup rate limited; pausing lookups", "until", cooldownUntil.Format(time.RFC3339))
		return "" // don't cache rate-limit responses
	}
	if resp.StatusCode != http.StatusOK {
		return "" // don't cache server errors
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
		return "" // don't cache decode errors
	}

	vendor := ""
	if data.Success && data.Found && data.Company != "" &&
		data.Company != "*NO COMPANY*" && data.Company != "*PRIVATE*" {
		vendor = data.Company
	}

	// Cache successful responses (including definitive "not found") so
	// known OUIs are not re-queried on every scan. Transient failures
	// (rate-limit, network, server errors) are NOT cached so they will
	// be retried on the next scan.
	ouiCacheMu.Lock()
	ouiCache[oui] = ouiEntry{Vendor: vendor, Ts: time.Now().Unix()}
	cacheDirty = true
	ouiCacheMu.Unlock()
	return vendor
}

// InvalidateOUICache removes the cache entry for the given OUI so the next
// resolveVendor call will fetch fresh data from the API.
func InvalidateOUICache(raw string) {
	loadOUICache()
	normalized := strings.ToUpper(strings.NewReplacer(":", "", "-", "", ".", "").Replace(raw))
	if len(normalized) < 6 {
		return
	}
	ouiCacheMu.Lock()
	delete(ouiCache, normalized[:6])
	cacheDirty = true
	ouiCacheMu.Unlock()
	saveOUICache()
}

// resolveVendors fills the Hardware field of hosts using the external MAC
// lookup API. Resolution runs concurrently with a bounded worker pool but only
// when the feature is enabled. The API result always takes precedence over
// whatever arp-scan's OUI database returned, since the external DB is more
// frequently updated. The cache prevents redundant API calls for known OUIs.
func resolveVendors(hosts []models.Host) {
	if !macLookupEnabled {
		return
	}
	loadOUICache()

	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for i := range hosts {
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
			hosts[idx].HwApi = vendor
			ouiCacheMu.Unlock()
		}(i, oui)
	}

	wg.Wait()

	ouiCacheMu.Lock()
	if cacheDirty {
		saveOUICache()
		cacheDirty = false
	}
	ouiCacheMu.Unlock()
}
