package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linde12/gowol"

	"github.com/aceberg/WatchYourLAN/internal/check"
	"github.com/aceberg/WatchYourLAN/internal/portscan"
)

// getPortState godoc
// @Summary      Check port state
// @Description  Check whether a given TCP port on an address is open or closed
// @Tags         network
// @Produce      json
// @Param        addr  path      string  true  "IP address or hostname"
// @Param        port  path      string  true  "Port number"
// @Success      200   {boolean}  bool   "true if open, false if closed"
// @Router       /port/{addr}/{port} [get]
func getPortState(c *gin.Context) {
	addr := c.Param("addr")
	port := c.Param("port")
	state := portscan.IsOpen(addr, port)
	c.IndentedJSON(http.StatusOK, state)
}

// getHostDiscovery godoc
// @Summary      Discover a host
// @Description  Run mDNS and SSDP/UPnP discovery for a single IP and return the
// @Description  aggregated manufacturer, model, serial, services and other
// @Description  advertised details.
// @Tags         network
// @Produce      json
// @Param        ip    path      string  true  "IP address of the host"
// @Success      200   {object}  check.HostInfo
// @Router       /host_discovery/{ip} [get]
func getHostDiscovery(c *gin.Context) {
	ip := c.Param("ip")
	info := check.DiscoverHost(ip)
	c.IndentedJSON(http.StatusOK, info)
}

// sendWOL godoc
// @Summary      Send Wake-on-LAN packet
// @Description  Send a magic packet to wake up a host by its MAC address
// @Tags         network
// @Produce      json
// @Param        mac   path      string  true  "MAC address of the host"
// @Success      200   {boolean} bool    "true if sent successfully"
// @Router       /wol/{mac} [get]
func sendWOL(c *gin.Context) {

	mac := c.Param("mac")

	packet, err := gowol.NewMagicPacket(mac)

	if !check.IfError(err) {
		err = packet.Send("255.255.255.255")

		slog.Info("Wake-on-LAN: " + mac)
	}

	c.IndentedJSON(http.StatusOK, !check.IfError(err))
}
