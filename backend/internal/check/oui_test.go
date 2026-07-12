package check

import (
	"testing"
	"time"
)

func TestNormalizeOUI(t *testing.T) {
	cases := map[string]string{
		"78:20:51:01:8f:bc": "782051",
		"78-20-51-01-8F-BC": "782051",
		"782051":            "782051",
		"78.20.51":          "782051",
		"12:34:56:78:9A:BC": "123456",
		"":                  "",
		"zz:zz:zz":          "",
		"12:34":             "",
	}
	for in, want := range cases {
		if got := normalizeOUI(in); got != want {
			t.Errorf("normalizeOUI(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsLocalAdmin(t *testing.T) {
	cases := map[string]bool{
		"78:20:51:01:8f:bc": false, // TP-Link, universally administered
		"02:00:00:00:00:00": true,  // locally administered
		"0E:00:00:00:00:00": true,
		"12:34:56:78:9A:BC": true, // bit1 set -> locally administered
	}
	for in, want := range cases {
		if got := isLocalAdmin(in); got != want {
			t.Errorf("isLocalAdmin(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestOUICacheRoundTrip verifies that resolved entries survive a save/load
// cycle, which is what makes the "query at most once per day" guarantee hold
// across process restarts.
func TestOUICacheRoundTrip(t *testing.T) {
	macLookupDir = t.TempDir()
	ouiLoaded = false
	ouiCache = map[string]ouiEntry{
		"782051": {Vendor: "TP-Link", Ts: time.Now().Unix()},
	}

	saveOUICache()

	ouiCache = nil
	ouiLoaded = false
	loadOUICache()

	if got := ouiCache["782051"].Vendor; got != "TP-Link" {
		t.Errorf("cached vendor not restored after round-trip, got %q", got)
	}
}
