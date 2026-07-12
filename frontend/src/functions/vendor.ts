// Map hardware/model description keywords to Bootstrap icon names.
// This is a lightweight visual hint only; it matches keywords in the
// hardware string (vendor, mDNS service type or UPnP device class) and is
// purely cosmetic. OUI-to-vendor resolution, if needed, belongs on the
// backend, not in this hardcoded frontend table.
const hwIcons: Record<string, string> = {
  // Networking
  "router":       "bi-router",
  "gateway":      "bi-router",
  "switch":       "bi-router-fill",
  "access point": "bi-wifi",
  "firewall":     "bi-shield-lock",
  "modem":        "bi-reception-4",
  "ubiquiti":     "bi-router",
  "mikrotik":     "bi-router",
  "tp-link":      "bi-router",
  "netgear":      "bi-router",
  "asus":         "bi-router",
  "d-link":       "bi-router",
  "linksys":      "bi-router",
  "cisco":        "bi-router",

  // Computers
  "apple":        "bi-laptop",
  "macbook":      "bi-laptop",
  "imac":         "bi-display",
  "dell":         "bi-pc-display",
  "lenovo":       "bi-pc-display",
  "hp":           "bi-pc-display",
  "microsoft":    "bi-pc-display",
  "intel":        "bi-cpu",
  "amd":          "bi-cpu",
  "server":       "bi-server",
  "nas":          "bi-hdd",
  "storage":      "bi-hdd",
  "synology":     "bi-hdd",
  "qnap":         "bi-hdd",

  // Mobile
  "samsung":      "bi-phone",
  "huawei":       "bi-phone",
  "xiaomi":       "bi-phone",
  "oneplus":      "bi-phone",
  "pixel":        "bi-phone",
  "iphone":       "bi-phone",
  "motorola":     "bi-phone",
  "oppo":         "bi-phone",
  "vivo":         "bi-phone",
  "android":      "bi-phone",

  // IoT / Smart Home
  "espressif":    "bi-cpu",
  "shelly":       "bi-plug",
  "tasmota":      "bi-plug",
  "sonoff":       "bi-plug",
  "tuya":         "bi-plug",
  "zigbee":       "bi-lightning",
  "philips":      "bi-lightbulb",
  "hue":          "bi-lightbulb",
  "nest":         "bi-thermometer",
  "ring":         "bi-camera",
  "echo":         "bi-speaker",
  "alexa":        "bi-speaker",
  "google":       "bi-speaker",
  "amazon":       "bi-speaker",

  // Printers
  "printer":      "bi-printer",
  "epson":        "bi-printer",
  "brother":      "bi-printer",
  "canon":        "bi-printer",
  "ricoh":        "bi-printer",

  // Cameras
  "camera":       "bi-camera-video",
  "hikvision":    "bi-camera-video",
  "dahua":        "bi-camera-video",
  "reolink":      "bi-camera-video",

  // Media / TV / Streaming
  "tv":           "bi-tv",
  "television":   "bi-tv",
  "roku":         "bi-tv",
  "chromecast":   "bi-tv",
  "appletv":      "bi-tv",
  "airplay":      "bi-tv",
  "sony":         "bi-tv",
  "lg":           "bi-tv",
  "media":        "bi-tv",
  "spotify":      "bi-music-note-beamed",
  "sonos":        "bi-speaker",

  // Game consoles
  "playstation":  "bi-controller",
  "xbox":         "bi-controller",
  "nintendo":     "bi-controller",

  // VoIP / Phone
  "voip":         "bi-telephone",
  "phone":        "bi-telephone",
};

const escapeRe = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

// vendorIcon returns a Bootstrap icon name (e.g. "bi-router") for the given
// hardware description, or a generic "bi-pc-display" fallback.
export function vendorIcon(hw: string): string {
  const text = (hw || "").toLowerCase();
  for (const [keyword, icon] of Object.entries(hwIcons)) {
    const re = new RegExp("\\b" + escapeRe(keyword) + "\\b", "i");
    if (re.test(text)) {
      return icon;
    }
  }
  return "bi-pc-display";
}
