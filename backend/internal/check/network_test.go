package check

import "testing"

func TestMDNSDeviceType(t *testing.T) {
	cases := map[string]string{
		"_airplay._tcp":            "Apple TV / AirPlay",
		"_raop._tcp":               "Apple TV / AirPlay",
		"_googlecast._tcp":         "Chromecast / Google TV",
		"_hue._tcp":                "Philips Hue",
		"_ipp._tcp":                "Printer",
		"_printer._tcp":            "Printer",
		"_ssh._tcp":                "SSH Server",
		"_smb._tcp":                "SMB / File Share",
		"_nas._tcp":                "NAS / Storage",
		"_spotify-connect._tcp":    "Spotify Connect",
		"_rfb._tcp":                "VNC",
		"_media._http._tcp":        "Web Device",
		"some-unknown-service._tcp": "",
		"":                         "",
	}
	for in, want := range cases {
		if got := mdnsDeviceType(in); got != want {
			t.Errorf("mdnsDeviceType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSSDPDeviceType(t *testing.T) {
	cases := map[string]string{
		"urn:schemas-upnp-org:device:InternetGatewayDevice:1": "Router / Gateway",
		"urn:schemas-upnp-org:device:MediaRenderer:1":         "Media Renderer",
		"urn:schemas-upnp-org:device:MediaServer:1":            "Media Server",
		"urn:schemas-upnp-org:device:Printer:1":                "Printer",
		"urn:schemas-upnp-org:device:TV:1":                     "Smart TV",
		"urn:something:device:Camera:2":                        "Camera",
		"":   "",
		"urn:not-a-known:device:Thing:1": "",
	}
	for in, want := range cases {
		if got := ssdpDeviceType(in); got != want {
			t.Errorf("ssdpDeviceType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildSSDPHardware(t *testing.T) {
	cases := []struct {
		name   string
		desc   map[string]string
		server string
		want   string
	}{
		{
			name: "manufacturer and model",
			desc: map[string]string{
				"manufacturer": "Sagemcom",
				"modelName":    "F@st5360",
				"modelNumber":  "1.0",
			},
			want: "Sagemcom F@st5360 1.0",
		},
		{
			name: "model description appended",
			desc: map[string]string{
				"manufacturer":     "Apple",
				"modelName":        "AppleTV",
				"modelDescription": "Gateway",
			},
			want: "Apple AppleTV Gateway",
		},
		{
			name:   "server header fallback",
			desc:   map[string]string{},
			server: "Linux/3.14 UPnP/1.0 MiniUPnPd/2.1",
			want:   "Linux/3.14 UPnP/1.0 MiniUPnPd/2.1",
		},
		{
			name: "serial number appended",
			desc: map[string]string{
				"manufacturer": "Sonos",
				"modelName":    "One",
				"serialNumber": "ABC123",
			},
			want: "Sonos One (S/N ABC123)",
		},
		{
			name: "serial only",
			desc: map[string]string{"serialNumber": "XYZ"},
			want: "S/N XYZ",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := buildSSDPHardware(c.desc, c.server); got != c.want {
				t.Errorf("buildSSDPHardware() = %q, want %q", got, c.want)
			}
		})
	}
}
