package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/grandcat/zeroconf"
)

// PrinterMapping defines the remote server configuration for an installed virtual printer.
type PrinterMapping struct {
	ServerIP      string `json:"serverIp"`
	ServerPort    int    `json:"serverPort"`
	RemotePrinter string `json:"remotePrinter"`
	AuthToken     string `json:"authToken"`
}

// DiscoveredRemotePrinter lists discovered printers on the local network.
type DiscoveredRemotePrinter struct {
	Name           string `json:"name"`
	ServerName     string `json:"serverName"`
	ServerIP       string `json:"serverIp"`
	ServerPort     int    `json:"serverPort"`
	AuthToken      string `json:"authToken"`
	Installed      bool   `json:"installed"`
	LocalQueueName string `json:"localQueueName"`
}

type remotePrinterAPI struct {
	Name string `json:"name"`
}

// DiscoverRemotePrinters scans the network for other TakePrint servers and queries their shared printers.
func (a *App) DiscoverRemotePrinters() ([]DiscoveredRemotePrinter, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		err = resolver.Browse(ctx, "_localshareprint._tcp", "local.", entries)
		if err != nil {
			a.addLog("error", fmt.Sprintf("Failed to browse mDNS: %v", err))
		}
	}()

	var remotePrinters []DiscoveredRemotePrinter
	var mu sync.Mutex
	var wg sync.WaitGroup

	installedPrinters, _ := a.GetInstalledTakePrintPrinters()

	// Parse settings to check our own name
	settings := a.loadSettings()

	go func() {
		for entry := range entries {
			// Skip our own server instance
			if entry.Instance == settings.ServerName {
				continue
			}

			var token, ipsStr string
			for _, text := range entry.Text {
				parts := strings.SplitN(text, "=", 2)
				if len(parts) == 2 {
					if parts[0] == "token" {
						token = parts[1]
					} else if parts[0] == "ips" {
						ipsStr = parts[1]
					}
				}
			}

			ip := ""
			if len(entry.AddrIPv4) > 0 {
				ip = entry.AddrIPv4[0].String()
			} else if ipsStr != "" {
				ip = strings.Split(ipsStr, ",")[0]
			}

			if ip == "" {
				continue
			}

			wg.Add(1)
			go func(e *zeroconf.ServiceEntry, ip string, token string) {
				defer wg.Done()
				printers, err := fetchPrinters(ip, e.Port, token)
				if err != nil {
					return
				}

				mu.Lock()
				defer mu.Unlock()
				for _, p := range printers {
					localQueueName := fmt.Sprintf("%s (%s)", p.Name, e.Instance)
					installed := false
					for _, inst := range installedPrinters {
						if inst == localQueueName {
							installed = true
							break
						}
					}

					remotePrinters = append(remotePrinters, DiscoveredRemotePrinter{
						Name:           p.Name,
						ServerName:     e.Instance,
						ServerIP:       ip,
						ServerPort:     e.Port,
						AuthToken:      token,
						Installed:      installed,
						LocalQueueName: localQueueName,
					})
				}
			}(entry, ip, token)
		}
	}()

	<-ctx.Done()
	wg.Wait()

	return remotePrinters, nil
}

func fetchPrinters(ip string, port int, token string) ([]remotePrinterAPI, error) {
	client := &http.Client{Timeout: 1 * time.Second}
	urlStr := fmt.Sprintf("http://%s:%d/printers", ip, port)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var printers []remotePrinterAPI
	if err := json.NewDecoder(resp.Body).Decode(&printers); err != nil {
		return nil, err
	}
	return printers, nil
}

// GetInstalledTakePrintPrinters returns a list of installed TakePrint printer names.
func (a *App) GetInstalledTakePrintPrinters() ([]string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Get-Printer | Where-Object { $_.Name -like \"*TakePrint*\" } | Select-Object -ExpandProperty Name")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var list []string
	lines := strings.Split(string(out), "\r\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			list = append(list, l)
		}
	}
	return list, nil
}

// InstallRemotePrinter creates a local port and virtual printer that redirects print output to the remote server.
func (a *App) InstallRemotePrinter(serverName, serverIP string, serverPort int, remotePrinter string, authToken string) error {
	// Automatically resolve authToken from connected devices if not passed
	if authToken == "" {
		settings := a.loadSettings()
		for _, d := range settings.ConnectedDevices {
			if d.Name == serverName {
				authToken = d.Token
				break
			}
		}
	}

	queueName := fmt.Sprintf("%s (%s)", remotePrinter, serverName)
	cleanQueueName := strings.ReplaceAll(queueName, "/", "_")
	cleanQueueName = strings.ReplaceAll(cleanQueueName, "\\", "_")
	cleanQueueName = strings.ReplaceAll(cleanQueueName, " ", "_")

	portPath := fmt.Sprintf("C:\\Users\\Public\\Documents\\TakePrint_Printers\\%s\\job.pdf", cleanQueueName)

	// Create directories
	os.MkdirAll(filepath.Dir(portPath), 0777)

	// PowerShell script to add port and printer using Admin rights (Start-Process)
	psCommand := fmt.Sprintf(`
		$portName = "%s"
		$queueName = "%s"
		if (-not (Get-PrinterPort -Name $portName -ErrorAction SilentlyContinue)) {
			Add-PrinterPort -Name $portName
		}
		if (-not (Get-Printer -Name $queueName -ErrorAction SilentlyContinue)) {
			Add-Printer -Name $queueName -DriverName "Microsoft Print to PDF" -PortName $portName
		}
	`, portPath, queueName)

	// Run PowerShell elevated
	cmd := exec.Command("powershell", "-NoProfile", "-Command", fmt.Sprintf("Start-Process powershell -Verb RunAs -ArgumentList '-NoProfile', '-Command', '%s' -Wait", psCommand))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to launch elevated setup: %w", err)
	}

	// Verify installation
	installed, _ := a.GetInstalledTakePrintPrinters()
	success := false
	for _, inst := range installed {
		if inst == queueName {
			success = true
			break
		}
	}

	if !success {
		return fmt.Errorf("printer setup elevated session cancelled or rejected")
	}

	// Save mapping in settings
	settings := a.loadSettings()
	if settings.LocalMappedPrinters == nil {
		settings.LocalMappedPrinters = make(map[string]PrinterMapping)
	}
	settings.LocalMappedPrinters[queueName] = PrinterMapping{
		ServerIP:      serverIP,
		ServerPort:    serverPort,
		RemotePrinter: remotePrinter,
		AuthToken:     authToken,
	}
	a.saveSettings(settings)

	a.addLog("success", fmt.Sprintf("Installed network printer queue: %s", queueName))
	return nil
}

// UninstallRemotePrinter deletes the local virtual printer queue and port.
func (a *App) UninstallRemotePrinter(queueName string) error {
	cleanQueueName := strings.ReplaceAll(queueName, "/", "_")
	cleanQueueName = strings.ReplaceAll(cleanQueueName, "\\", "_")
	cleanQueueName = strings.ReplaceAll(cleanQueueName, " ", "_")
	portPath := fmt.Sprintf("C:\\Users\\Public\\Documents\\TakePrint_Printers\\%s\\job.pdf", cleanQueueName)

	psCommand := fmt.Sprintf(`
		$portName = "%s"
		$queueName = "%s"
		if (Get-Printer -Name $queueName -ErrorAction SilentlyContinue) {
			Remove-Printer -Name $queueName
		}
		if (Get-PrinterPort -Name $portName -ErrorAction SilentlyContinue) {
			Remove-PrinterPort -Name $portName
		}
	`, portPath, queueName)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", fmt.Sprintf("Start-Process powershell -Verb RunAs -ArgumentList '-NoProfile', '-Command', '%s' -Wait", psCommand))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to uninstall elevated: %w", err)
	}

	// Clean up mapping
	settings := a.loadSettings()
	if settings.LocalMappedPrinters != nil {
		delete(settings.LocalMappedPrinters, queueName)
		a.saveSettings(settings)
	}

	// Clean up directories
	os.RemoveAll(filepath.Dir(portPath))

	a.addLog("info", fmt.Sprintf("Uninstalled network printer queue: %s", queueName))
	return nil
}

// Watcher implementation for forwarding jobs
func (a *App) startLocalPrinterWatcher() {
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for range ticker.C {
			a.checkLocalPrintJobs()
		}
	}()
}

func (a *App) checkLocalPrintJobs() {
	baseDir := "C:\\Users\\Public\\Documents\\TakePrint_Printers"
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return
	}

	dirs, err := os.ReadDir(baseDir)
	if err != nil {
		return
	}

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}

		queueName := d.Name()
		// Reconstruct original name from folder name (spaces were replaced by underscores)
		// We'll match against keys in settings mappings instead
		jobPath := filepath.Join(baseDir, queueName, "job.pdf")

		info, err := os.Stat(jobPath)
		if err != nil {
			continue
		}

		// Wait until file is unlocked / fully written
		file, err := os.OpenFile(jobPath, os.O_RDWR, 0)
		if err != nil {
			continue
		}
		file.Close()

		if info.Size() == 0 {
			continue
		}

		// Find the correct mapping key
		mappingKey := ""
		settings := a.loadSettings()
		for key := range settings.LocalMappedPrinters {
			cleanKey := strings.ReplaceAll(key, "/", "_")
			cleanKey = strings.ReplaceAll(cleanKey, "\\", "_")
			cleanKey = strings.ReplaceAll(cleanKey, " ", "_")
			if cleanKey == queueName {
				mappingKey = key
				break
			}
		}

		if mappingKey == "" {
			os.Remove(jobPath)
			continue
		}

		mapping := settings.LocalMappedPrinters[mappingKey]

		// Execute forward
		go a.forwardJobToRemoteServer(mappingKey, jobPath, mapping)
	}
}

func (a *App) getPrinterMapping(queueName string) (PrinterMapping, bool) {
	settings := a.loadSettings()
	if settings.LocalMappedPrinters == nil {
		return PrinterMapping{}, false
	}
	m, found := settings.LocalMappedPrinters[queueName]
	return m, found
}

func (a *App) forwardJobToRemoteServer(queueName, jobPath string, mapping PrinterMapping) {
	a.addLog("info", fmt.Sprintf("Intercepted print job for %s. Forwarding to network server http://%s:%d...", queueName, mapping.ServerIP, mapping.ServerPort))

	// Open file
	file, err := os.Open(jobPath)
	if err != nil {
		a.addLog("error", fmt.Sprintf("Failed to read job file: %v", err))
		return
	}
	defer file.Close()

	// Perform copy to memory/pipe
	bodyReader, bodyWriter := io.Pipe()
	formWriter := multipart.NewWriter(bodyWriter)

	go func() {
		defer bodyWriter.Close()
		defer formWriter.Close()

		part, err := formWriter.CreateFormFile("file", "PrintJob.pdf")
		if err != nil {
			return
		}
		_, _ = io.Copy(part, file)
	}()

	urlStr := fmt.Sprintf("http://%s:%d/print?printer=%s&copies=1&color=color", mapping.ServerIP, mapping.ServerPort, url.QueryEscape(mapping.RemotePrinter))
	req, err := http.NewRequest("POST", urlStr, bodyReader)
	if err != nil {
		a.addLog("error", fmt.Sprintf("Failed to build print forward request: %v", err))
		return
	}

	req.Header.Set("Content-Type", formWriter.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+mapping.AuthToken)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		a.addLog("error", fmt.Sprintf("Failed to send job to remote print server: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		a.addLog("success", fmt.Sprintf("Print job sent successfully to remote printer '%s'", mapping.RemotePrinter))
		// Delete job file
		os.Remove(jobPath)
	} else {
		bodyBytes, _ := io.ReadAll(resp.Body)
		a.addLog("error", fmt.Sprintf("Remote server rejected job: %s (Status: %d)", string(bodyBytes), resp.StatusCode))
		// Delete job file to clean up
		os.Remove(jobPath)
	}
}
