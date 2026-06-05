package mdns

import (
	"fmt"
	"log"
	"net"

	"github.com/grandcat/zeroconf"
)

// Server wraps the zeroconf mDNS service registration.
type Server struct {
	zc     *zeroconf.Server
	Logger func(msg string)
}

// Start registers the _localshareprint._tcp mDNS service on the given port.
// The logger callback is invoked with status messages for the frontend console.
func Start(serviceName string, port int, logger func(string)) (*Server, error) {
	if logger == nil {
		logger = func(msg string) { log.Println(msg) }
	}

	// Resolve the machine's local IP for logging.
	localIP := getLocalIP()

	server, err := zeroconf.Register(
		serviceName,                // Service instance name
		"_localshareprint._tcp",    // Service type
		"local.",                   // Domain
		port,                       // Port
		[]string{"version=1.0", "platform=desktop"}, // TXT records
		nil, // Use all network interfaces
	)
	if err != nil {
		return nil, fmt.Errorf("mDNS registration failed: %w", err)
	}

	logger(fmt.Sprintf("📡 mDNS broadcasting '_localshareprint._tcp' on %s:%d", localIP, port))

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
