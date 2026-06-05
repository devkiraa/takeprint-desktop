package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DeviceConfig is the persistable configuration for a connected remote device.
type DeviceConfig struct {
	Name string   `json:"name"`
	IPs  []string `json:"ips"`
	Port int      `json:"port"`
}

// ConnectedDevice is the runtime representation of a remote TakePrint server
// including live status information.
type ConnectedDevice struct {
	Name     string   `json:"name"`
	IPs      []string `json:"ips"`
	Port     int      `json:"port"`
	Status   string   `json:"status"` // "online", "offline", "checking"
	ActiveIP string   `json:"activeIP"`
}

// RemotePrinter represents a printer on a remote TakePrint server.
type RemotePrinter struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	IsDefault  bool   `json:"isDefault"`
	DeviceName string `json:"deviceName"` // Which remote device owns this printer
	DeviceIP   string `json:"deviceIP"`
	DevicePort int    `json:"devicePort"`
}

// PrintOptions holds the settings for a print job.
type PrintOptions struct {
	Pages  string `json:"pages"`
	Color  string `json:"color"`
	Copies int    `json:"copies"`
}

// Manager handles connections to remote TakePrint desktop servers.
type Manager struct {
	devices    map[string]*ConnectedDevice
	mu         sync.RWMutex
	logger     func(string)
	httpClient *http.Client
	stopCh     chan struct{}
}

// NewManager creates a new remote device manager.
func NewManager(logger func(string)) *Manager {
	if logger == nil {
		logger = func(msg string) { fmt.Println(msg) }
	}
	m := &Manager{
		devices: make(map[string]*ConnectedDevice),
		logger:  logger,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		stopCh: make(chan struct{}),
	}
	return m
}

// LoadDevices initializes the manager with persisted device configs.
func (m *Manager) LoadDevices(configs []DeviceConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, cfg := range configs {
		m.devices[cfg.Name] = &ConnectedDevice{
			Name:   cfg.Name,
			IPs:    cfg.IPs,
			Port:   cfg.Port,
			Status: "checking",
		}
	}
}

// StartHealthChecker begins a background goroutine that pings connected devices.
func (m *Manager) StartHealthChecker() {
	go func() {
		// Initial check.
		m.checkAllDevices()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.checkAllDevices()
			case <-m.stopCh:
				return
			}
		}
	}()
}

// Stop shuts down the health checker.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// AddDevice adds a new remote device and starts monitoring it.
func (m *Manager) AddDevice(name string, ips []string, port int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	device := &ConnectedDevice{
		Name:   name,
		IPs:    ips,
		Port:   port,
		Status: "checking",
	}
	m.devices[name] = device
	m.logger(fmt.Sprintf("➕ Added remote device: %s (%s:%d)", name, strings.Join(ips, ", "), port))

	// Check health immediately in background.
	go m.checkDevice(device)
}

// RemoveDevice removes a connected device.
func (m *Manager) RemoveDevice(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.devices, name)
	m.logger(fmt.Sprintf("➖ Removed remote device: %s", name))
}

// GetConnectedDevices returns a snapshot of all connected devices.
func (m *Manager) GetConnectedDevices() []ConnectedDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ConnectedDevice, 0, len(m.devices))
	for _, d := range m.devices {
		result = append(result, *d)
	}
	return result
}

// GetDeviceConfigs returns the persistable config for all connected devices.
func (m *Manager) GetDeviceConfigs() []DeviceConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	configs := make([]DeviceConfig, 0, len(m.devices))
	for _, d := range m.devices {
		configs = append(configs, DeviceConfig{
			Name: d.Name,
			IPs:  d.IPs,
			Port: d.Port,
		})
	}
	return configs
}

// FetchRemotePrinters fetches the shared printers from a specific device.
func (m *Manager) FetchRemotePrinters(deviceName string) ([]RemotePrinter, error) {
	m.mu.RLock()
	device, ok := m.devices[deviceName]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("device '%s' not found", deviceName)
	}

	baseURL := m.resolveBaseURL(device)
	if baseURL == "" {
		return nil, fmt.Errorf("device '%s' is offline", deviceName)
	}

	resp, err := m.httpClient.Get(baseURL + "/printers")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch printers from '%s': %w", deviceName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad response from '%s': %d", deviceName, resp.StatusCode)
	}

	var rawPrinters []struct {
		Name      string `json:"name"`
		Status    string `json:"status"`
		IsDefault bool   `json:"isDefault"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawPrinters); err != nil {
		return nil, fmt.Errorf("failed to decode printers from '%s': %w", deviceName, err)
	}

	printers := make([]RemotePrinter, 0, len(rawPrinters))
	for _, rp := range rawPrinters {
		printers = append(printers, RemotePrinter{
			Name:       rp.Name,
			Status:     rp.Status,
			IsDefault:  rp.IsDefault,
			DeviceName: deviceName,
			DeviceIP:   device.ActiveIP,
			DevicePort: device.Port,
		})
	}
	return printers, nil
}

// GetAllRemotePrinters aggregates printers from all online connected devices.
func (m *Manager) GetAllRemotePrinters() []RemotePrinter {
	m.mu.RLock()
	deviceNames := make([]string, 0, len(m.devices))
	for name, d := range m.devices {
		if d.Status == "online" {
			deviceNames = append(deviceNames, name)
		}
	}
	m.mu.RUnlock()

	var allPrinters []RemotePrinter
	for _, name := range deviceNames {
		printers, err := m.FetchRemotePrinters(name)
		if err != nil {
			m.logger(fmt.Sprintf("⚠️ Failed to fetch printers from '%s': %v", name, err))
			continue
		}
		allPrinters = append(allPrinters, printers...)
	}
	return allPrinters
}

// ForwardPrintJob sends a PDF print job to a remote TakePrint server.
func (m *Manager) ForwardPrintJob(deviceName, printerName, filePath string, opts PrintOptions) error {
	m.mu.RLock()
	device, ok := m.devices[deviceName]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("device '%s' not found", deviceName)
	}

	baseURL := m.resolveBaseURL(device)
	if baseURL == "" {
		return fmt.Errorf("device '%s' is offline", deviceName)
	}

	// Open the file.
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Build multipart form.
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("printer", printerName)
	_ = writer.WriteField("pages", opts.Pages)
	_ = writer.WriteField("color", opts.Color)
	_ = writer.WriteField("copies", fmt.Sprintf("%d", opts.Copies))

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}
	writer.Close()

	// Use a longer timeout for file upload.
	uploadClient := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("POST", baseURL+"/print", body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := uploadClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send print job to '%s': %w", deviceName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote print failed (%d): %s", resp.StatusCode, string(respBody))
	}

	m.logger(fmt.Sprintf("📤 Forwarded print job to %s/%s", deviceName, printerName))
	return nil
}

// --- Internal helpers ---

// checkAllDevices pings every connected device.
func (m *Manager) checkAllDevices() {
	m.mu.RLock()
	devices := make([]*ConnectedDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	for _, d := range devices {
		wg.Add(1)
		go func(dev *ConnectedDevice) {
			defer wg.Done()
			m.checkDevice(dev)
		}(d)
	}
	wg.Wait()
}

// checkDevice pings a single device across all its known IPs.
func (m *Manager) checkDevice(device *ConnectedDevice) {
	m.mu.RLock()
	ips := make([]string, len(device.IPs))
	copy(ips, device.IPs)
	port := device.Port
	m.mu.RUnlock()

	for _, ip := range ips {
		url := fmt.Sprintf("http://%s:%d/health", ip, port)
		resp, err := m.httpClient.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				m.mu.Lock()
				device.Status = "online"
				device.ActiveIP = ip
				m.mu.Unlock()
				return
			}
		}
	}

	m.mu.Lock()
	device.Status = "offline"
	device.ActiveIP = ""
	m.mu.Unlock()
}

// resolveBaseURL returns the working base URL for a device, or "" if offline.
func (m *Manager) resolveBaseURL(device *ConnectedDevice) string {
	if device.ActiveIP != "" {
		return fmt.Sprintf("http://%s:%d", device.ActiveIP, device.Port)
	}

	// Try all IPs as a fallback.
	for _, ip := range device.IPs {
		url := fmt.Sprintf("http://%s:%d", ip, device.Port)
		resp, err := m.httpClient.Get(url + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				m.mu.Lock()
				device.ActiveIP = ip
				device.Status = "online"
				m.mu.Unlock()
				return url
			}
		}
	}
	return ""
}
