package mdns

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

// DiscoveredDevice represents a TakePrint server found on the network.
type DiscoveredDevice struct {
	Name  string   `json:"name"`
	IPs   []string `json:"ips"`
	Port  int      `json:"port"`
	Token string   `json:"token"`
}

// ScanForDevices browses the local network for other TakePrint instances
// using mDNS/zeroconf. It filters out the local machine's own service.
func ScanForDevices(selfName string, timeout time.Duration, logger func(string)) ([]DiscoveredDevice, error) {
	if logger == nil {
		logger = func(msg string) {}
	}

	logger("🔍 Scanning for TakePrint devices on the network...")

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create mDNS resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry, 20)
	var devices []DiscoveredDevice

	// Collect the local machine IPs for self-filtering.
	localIPs := getLocalIPs()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	go func() {
		for entry := range entries {
			// Skip our own service.
			if strings.EqualFold(entry.Instance, selfName) {
				continue
			}
			if isSelf(entry, localIPs) {
				continue
			}

			ips := make([]string, 0, len(entry.AddrIPv4))
			for _, ip := range entry.AddrIPv4 {
				ips = append(ips, ip.String())
			}
			// Include IPv6 as fallback
			for _, ip := range entry.AddrIPv6 {
				ips = append(ips, ip.String())
			}

			if len(ips) == 0 {
				continue
			}

			var token string
			for _, txt := range entry.Text {
				if strings.HasPrefix(txt, "token=") {
					token = strings.TrimPrefix(txt, "token=")
				}
			}

			device := DiscoveredDevice{
				Name:  entry.Instance,
				IPs:   ips,
				Port:  entry.Port,
				Token: token,
			}
			devices = append(devices, device)
			logger(fmt.Sprintf("📡 Found device: %s at %s:%d (Auth Enabled: %v)", device.Name, strings.Join(ips, ", "), device.Port, token != ""))
		}
	}()

	err = resolver.Browse(ctx, "_localshareprint._tcp", "local.", entries)
	if err != nil {
		return nil, fmt.Errorf("mDNS browse failed: %w", err)
	}

	// Wait for the context to finish (timeout or cancel).
	<-ctx.Done()

	logger(fmt.Sprintf("🔍 Scan complete. Found %d device(s).", len(devices)))
	return devices, nil
}

// isSelf checks if a discovered entry matches any of the local machine's IPs.
func isSelf(entry *zeroconf.ServiceEntry, localIPs map[string]bool) bool {
	for _, ip := range entry.AddrIPv4 {
		if localIPs[ip.String()] {
			return true
		}
	}
	for _, ip := range entry.AddrIPv6 {
		if localIPs[ip.String()] {
			return true
		}
	}
	return false
}

// getLocalIPs returns a set of all local IP addresses for self-filtering.
func getLocalIPs() map[string]bool {
	ips := make(map[string]bool)
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				ips[ipnet.IP.String()] = true
			}
		}
	}
	return ips
}
