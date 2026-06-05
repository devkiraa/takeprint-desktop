package mdns

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/grandcat/zeroconf"
)

// Server wraps the zeroconf mDNS service registration.
type Server struct {
	zc     *zeroconf.Server
	Logger func(msg string)
}

// Start registers the _localshareprint._tcp mDNS service on the given port.
// The logger callback is invoked with status messages for the frontend console.
func Start(serviceName string, port int, token string, logger func(string)) (*Server, error) {
	if logger == nil {
		logger = func(msg string) { log.Println(msg) }
	}

	// Resolve the machine's local IP for logging.
	localIP := getLocalIP()

	// Gather all active non-virtual IPs to publish in mDNS TXT.
	nonVirtualIPs := getNonVirtualIPs()
	if len(nonVirtualIPs) == 0 {
		nonVirtualIPs = []string{localIP}
	}
	ipsStr := strings.Join(nonVirtualIPs, ",")

	server, err := zeroconf.Register(
		serviceName,                // Service instance name
		"_localshareprint._tcp",    // Service type
		"local.",                   // Domain
		port,                       // Port
		[]string{"version=1.0", "platform=desktop", "token=" + token, "ips=" + ipsStr}, // TXT records
		nil, // Use all network interfaces
	)
	if err != nil {
		return nil, fmt.Errorf("mDNS registration failed: %w", err)
	}

	logger(fmt.Sprintf("📡 mDNS broadcasting '_localshareprint._tcp' on %s:%d (Available IPs: %s)", localIP, port, ipsStr))

	return &Server{zc: server, Logger: logger}, nil
}

// Stop gracefully shuts down the mDNS service.
func (s *Server) Stop() {
	if s.zc != nil {
		s.zc.Shutdown()
		if s.Logger != nil {
			s.Logger("🛑 mDNS service stopped")
		}
	}
}

// getLocalIP returns the preferred local IPv4 address.
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// getNonVirtualIPs returns non-loopback IPv4 addresses of the desktop machine, skipping virtual interfaces.
func getNonVirtualIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		name := strings.ToLower(iface.Name)
		// Skip interfaces that are down or loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		// Skip virtual adapters commonly used by VMs, containers, and WSL
		if strings.Contains(name, "virtual") || strings.Contains(name, "vbox") ||
			strings.Contains(name, "vmware") || strings.Contains(name, "docker") ||
			strings.Contains(name, "wsl") || strings.Contains(name, "host-only") ||
			strings.Contains(name, "vethernet") || strings.Contains(name, "vboxnet") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && !ipnet.IP.IsLinkLocalUnicast() {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP.String())
				}
			}
		}
	}
	return ips
}
