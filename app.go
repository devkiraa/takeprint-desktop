package main

import (
	"context"
	"encoding/json"
	"fmt"
	"crypto/rand"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"takeprint/backend/mdns"
	"takeprint/backend/printer"
	"takeprint/backend/remote"
	"takeprint/backend/server"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// LogEntry represents a single log message for the frontend console.
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
	Level     string `json:"level"` // "info", "warn", "error", "success"
}

// ServerStatus reports the current state of backend services.
type ServerStatus struct {
	MDNSActive   bool   `json:"mdnsActive"`
	HTTPActive   bool   `json:"httpActive"`
	HTTPAddress  string `json:"httpAddress"`
	PrinterCount int    `json:"printerCount"`
}

// Settings represents the server configuration persisted on disk.
type Settings struct {
	ServerName        string                          `json:"serverName"`
	AutoLaunchEnabled bool                            `json:"autoLaunchEnabled"`
	PrinterSupplies   map[string][]printer.SupplyInfo `json:"printerSupplies"`
	VirtualPrinterDir string                          `json:"virtualPrinterDir"`
	ConnectedDevices  []remote.DeviceConfig           `json:"connectedDevices,omitempty"`
	AuthToken         string                          `json:"authToken"`
}

func getDefaultDownloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Downloads")
}

func generateSecureToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "takeprint_fallback_token_12345"
	}
	return hex.EncodeToString(b)
}

func (a *App) loadSettings() Settings {
	file, err := os.Open("settings.json")
	if err != nil {
		// Default to system hostname
		hostname, err := os.Hostname()
		if err != nil || hostname == "" {
			hostname = "TakePrint"
		}
		s := Settings{
			ServerName:        hostname,
			AutoLaunchEnabled: false,
			PrinterSupplies:   make(map[string][]printer.SupplyInfo),
			VirtualPrinterDir: getDefaultDownloadsDir(),
			AuthToken:         generateSecureToken(),
		}
		_ = a.saveSettings(s)
		return s
	}
	defer file.Close()

	var s Settings
	if err := json.NewDecoder(file).Decode(&s); err != nil {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "TakePrint"
		}
		s = Settings{
			ServerName:        hostname,
			AutoLaunchEnabled: false,
			PrinterSupplies:   make(map[string][]printer.SupplyInfo),
			VirtualPrinterDir: getDefaultDownloadsDir(),
			AuthToken:         generateSecureToken(),
		}
		_ = a.saveSettings(s)
		return s
	}
	if s.PrinterSupplies == nil {
		s.PrinterSupplies = make(map[string][]printer.SupplyInfo)
	}
	if s.VirtualPrinterDir == "" {
		s.VirtualPrinterDir = getDefaultDownloadsDir()
	}
	if s.AuthToken == "" {
		s.AuthToken = generateSecureToken()
		_ = a.saveSettings(s)
	}
	return s
}

func (a *App) saveSettings(s Settings) error {
	file, err := os.Create("settings.json")
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(s)
}

// App is the main application struct bound to the Wails frontend.
type App struct {
	ctx            context.Context
	printerService *printer.Service
	mdnsServer     *mdns.Server
	httpServer     *server.Server
	remoteManager  *remote.Manager

	logs   []LogEntry
	logMu  sync.Mutex
	maxLog int
}

// NewApp creates a new App instance.
func NewApp() *App {
	a := &App{
		printerService: printer.NewService(),
		logs:           make([]LogEntry, 0, 200),
		maxLog:         200,
	}
	a.remoteManager = remote.NewManager(func(msg string) {
		a.addLog("info", msg)
	})
	return a
}

// startup is called when the Wails app starts. It launches mDNS and HTTP services.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	a.addLog("info", "🚀 TakePrint starting up...")

	// Load settings
	settings := a.loadSettings()
	a.addLog("info", fmt.Sprintf("⚙️ Loaded server name: %s", settings.ServerName))

	// Sync printer supplies into the service
	a.printerService.SetSupplies(settings.PrinterSupplies)
	a.printerService.SetVirtualPrinterDir(settings.VirtualPrinterDir)

	// Start mDNS service with the custom server name.
	mdnsSrv, err := mdns.Start(settings.ServerName, 8080, settings.AuthToken, a.logCallback)
	if err != nil {
		a.addLog("error", fmt.Sprintf("Failed to start mDNS: %v", err))
	} else {
		a.mdnsServer = mdnsSrv
		a.addLog("success", "mDNS service started successfully")
	}

	// Start HTTP server in a goroutine.
	a.httpServer = server.New(":8080", settings.AuthToken, a.printerService, a.logCallback, func() {
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "job-updated")
		}
	})
	go func() {
		if err := a.httpServer.Start(); err != nil {
			a.addLog("error", fmt.Sprintf("HTTP server error: %v", err))
		}
	}()
	a.addLog("success", "HTTP server started on :8080")

	// Load connected remote devices and start health checker.
	if len(settings.ConnectedDevices) > 0 {
		a.remoteManager.LoadDevices(settings.ConnectedDevices)
		a.addLog("info", fmt.Sprintf("📱 Loaded %d connected device(s)", len(settings.ConnectedDevices)))
	}
	a.remoteManager.StartHealthChecker()
}

// shutdown is called when the Wails app is closing.
func (a *App) shutdown(ctx context.Context) {
	a.addLog("info", "Shutting down services...")

	if a.remoteManager != nil {
		a.remoteManager.Stop()
	}
	if a.httpServer != nil {
		a.httpServer.Stop()
	}
	if a.mdnsServer != nil {
		a.mdnsServer.Stop()
	}

	a.addLog("info", "👋 TakePrint stopped")
}

// --- Bound Methods (callable from frontend) ---

// GetPrinters returns the list of locally installed printers.
func (a *App) GetPrinters() ([]printer.PrinterInfo, error) {
	printers, err := a.printerService.ListPrinters()
	if err != nil {
		a.addLog("error", fmt.Sprintf("Failed to list printers: %v", err))
		return nil, err
	}

	// Save any newly generated default supplies back to Settings
	settings := a.loadSettings()
	supplies := a.printerService.GetSupplies()
	settings.PrinterSupplies = supplies
	_ = a.saveSettings(settings)

	return printers, nil
}

// TogglePrinterShare enables or disables sharing for a printer.
func (a *App) TogglePrinterShare(name string, shared bool) {
	a.printerService.TogglePrinterShare(name, shared)
	if shared {
		a.addLog("success", fmt.Sprintf("🔓 Enabled sharing for printer '%s'", name))
	} else {
		a.addLog("warn", fmt.Sprintf("🔒 Disabled sharing for printer '%s'", name))
	}
}

// GetServerName returns the current customized server name.
func (a *App) GetServerName() string {
	return a.loadSettings().ServerName
}

// GetAuthToken returns the current server authentication token.
func (a *App) GetAuthToken() string {
	return a.loadSettings().AuthToken
}

// UpdateServerName updates the customized server name and restarts the mDNS server.
func (a *App) UpdateServerName(newName string) error {
	if newName == "" {
		return fmt.Errorf("server name cannot be empty")
	}

	// Save to settings file.
	s := a.loadSettings()
	s.ServerName = newName
	if err := a.saveSettings(s); err != nil {
		return fmt.Errorf("failed to save settings: %w", err)
	}

	a.addLog("info", fmt.Sprintf("🔄 Updating print server name to: %s", newName))

	// Stop current mDNS
	if a.mdnsServer != nil {
		a.mdnsServer.Stop()
	}

	// Start new mDNS with updated name
	mdnsSrv, err := mdns.Start(newName, 8080, s.AuthToken, a.logCallback)
	if err != nil {
		a.addLog("error", fmt.Sprintf("Failed to restart mDNS: %v", err))
		return err
	}
	a.mdnsServer = mdnsSrv
	a.addLog("success", fmt.Sprintf("📡 Server name changed. mDNS broadcasting as '%s'", newName))
	return nil
}

// GetVirtualPrinterDir returns the folder where virtual print jobs are saved.
func (a *App) GetVirtualPrinterDir() string {
	return a.loadSettings().VirtualPrinterDir
}

// UpdateVirtualPrinterDir saves a new directory for virtual print files.
func (a *App) UpdateVirtualPrinterDir(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	s := a.loadSettings()
	s.VirtualPrinterDir = path

	// Sync it with the printerService
	a.printerService.SetVirtualPrinterDir(path)

	a.addLog("info", fmt.Sprintf("⚙️ Save location for virtual printers updated to: %s", path))
	return a.saveSettings(s)
}

// SelectVirtualPrinterDir opens a native folder selection dialog and saves the selection.
func (a *App) SelectVirtualPrinterDir() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("context not initialized")
	}

	currentDir := a.loadSettings().VirtualPrinterDir
	selected, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		DefaultDirectory: currentDir,
		Title:            "Select Save Location for Virtual Printers",
	})
	if err != nil {
		return "", err
	}
	if selected == "" {
		return currentDir, nil // User canceled
	}

	err = a.UpdateVirtualPrinterDir(selected)
	return selected, err
}

// GetLogs returns the recent log entries.
func (a *App) GetLogs() []LogEntry {
	a.logMu.Lock()
	defer a.logMu.Unlock()

	// Return a copy.
	result := make([]LogEntry, len(a.logs))
	copy(result, a.logs)
	return result
}

// GetJobs returns the list of active/past print jobs.
func (a *App) GetJobs() []server.PrintJob {
	if a.httpServer == nil {
		return nil
	}
	return a.httpServer.GetJobs()
}

// GetLocalIPs returns non-loopback IPv4 addresses of the desktop machine.
func (a *App) GetLocalIPs() ([]string, error) {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		a.addLog("error", fmt.Sprintf("Failed to get network interfaces: %v", err))
		return nil, err
	}
	for _, iface := range ifaces {
		// Skip interfaces that are down or loopback
		name := strings.ToLower(iface.Name)
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
	return ips, nil
}

// GetServerStatus returns the current status of backend services.
func (a *App) GetServerStatus() ServerStatus {
	printers, _ := a.printerService.ListPrinters()
	return ServerStatus{
		MDNSActive:   a.mdnsServer != nil,
		HTTPActive:   a.httpServer != nil,
		HTTPAddress:  ":8080",
		PrinterCount: len(printers),
	}
}

// SetAutoLaunch toggles the Windows registry key to launch TakePrint on boot.
func (a *App) SetAutoLaunch(enable bool) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("auto-launch is only supported on Windows")
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	var psCmd string
	if enable {
		psCmd = fmt.Sprintf(`Set-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -Name 'TakePrint' -Value '"%s"'`, execPath)
	} else {
		psCmd = `Remove-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -Name 'TakePrint' -ErrorAction SilentlyContinue`
	}

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to toggle auto-launch registry: %w", err)
	}

	s := a.loadSettings()
	s.AutoLaunchEnabled = enable
	a.saveSettings(s)

	if enable {
		a.addLog("success", "⚙️ Auto-Launch enabled. TakePrint will start on Windows boot.")
	} else {
		a.addLog("info", "⚙️ Auto-Launch disabled.")
	}
	return nil
}

// IsAutoLaunchEnabled checks if the Windows boot registry key is set.
func (a *App) IsAutoLaunchEnabled() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	psCmd := `Get-ItemProperty -Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Run' -Name 'TakePrint' -ErrorAction SilentlyContinue`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		return false
	}
	return strings.Contains(string(out), "TakePrint")
}

// --- Remote Device Methods (callable from frontend) ---

// ScanForDevices scans the LAN for other TakePrint servers via mDNS.
func (a *App) ScanForDevices() ([]mdns.DiscoveredDevice, error) {
	settings := a.loadSettings()
	devices, err := mdns.ScanForDevices(settings.ServerName, 4*time.Second, a.logCallback)
	if err != nil {
		a.addLog("error", fmt.Sprintf("Device scan failed: %v", err))
		return nil, err
	}
	return devices, nil
}

// AddRemoteDevice adds a remote TakePrint server to the connected list.
func (a *App) AddRemoteDevice(name string, ips []string, port int, token string) error {
	if name == "" || len(ips) == 0 {
		return fmt.Errorf("name and at least one IP are required")
	}
	a.remoteManager.AddDevice(name, ips, port, token)

	// Persist to settings.
	s := a.loadSettings()
	s.ConnectedDevices = a.remoteManager.GetDeviceConfigs()
	if err := a.saveSettings(s); err != nil {
		a.addLog("warn", fmt.Sprintf("Failed to save device config: %v", err))
	}
	a.addLog("success", fmt.Sprintf("✅ Connected to remote device: %s", name))
	return nil
}

// RemoveRemoteDevice removes a remote TakePrint server from the connected list.
func (a *App) RemoveRemoteDevice(name string) {
	a.remoteManager.RemoveDevice(name)

	// Persist to settings.
	s := a.loadSettings()
	s.ConnectedDevices = a.remoteManager.GetDeviceConfigs()
	_ = a.saveSettings(s)
	a.addLog("info", fmt.Sprintf("🔌 Disconnected from device: %s", name))
}

// GetConnectedDevices returns the list of connected remote devices with status.
func (a *App) GetConnectedDevices() []remote.ConnectedDevice {
	return a.remoteManager.GetConnectedDevices()
}

// GetRemotePrinters fetches printers from a specific connected device.
func (a *App) GetRemotePrinters(deviceName string) ([]remote.RemotePrinter, error) {
	return a.remoteManager.FetchRemotePrinters(deviceName)
}

// GetAllRemotePrinters aggregates printers from all online connected devices.
func (a *App) GetAllRemotePrinters() []remote.RemotePrinter {
	return a.remoteManager.GetAllRemotePrinters()
}

// PrintToRemote forwards a print job to a remote device's printer.
func (a *App) PrintToRemote(deviceName, printerName, filePath string, pages string, color string, copies int) error {
	opts := remote.PrintOptions{
		Pages:  pages,
		Color:  color,
		Copies: copies,
	}
	err := a.remoteManager.ForwardPrintJob(deviceName, printerName, filePath, opts)
	if err != nil {
		a.addLog("error", fmt.Sprintf("❌ Remote print failed: %v", err))
		return err
	}
	a.addLog("success", fmt.Sprintf("📤 Print job sent to %s → %s", deviceName, printerName))
	return nil
}

// --- Internal helpers ---

// logCallback is the logging function passed to backend services.
func (a *App) logCallback(msg string) {
	a.addLog("info", msg)
}

// addLog appends a log entry and emits a Wails event for real-time updates.
func (a *App) addLog(level, message string) {
	entry := LogEntry{
		Timestamp: time.Now().Format("15:04:05"),
		Message:   message,
		Level:     level,
	}

	a.logMu.Lock()
	a.logs = append(a.logs, entry)
	if len(a.logs) > a.maxLog {
		a.logs = a.logs[len(a.logs)-a.maxLog:]
	}
	a.logMu.Unlock()

	// Emit event to the frontend if context is available.
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "log", entry)
	}
}
