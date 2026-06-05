//go:build windows

package printer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// psPrinter is the JSON shape returned by PowerShell's Get-Printer.
type psPrinter struct {
	Name          string `json:"Name"`
	PrinterStatus int    `json:"PrinterStatus"`
	Type          int    `json:"Type"`
}

// statusMap translates the PrinterStatus integer codes from WMI.
var statusMap = map[int]string{
	0: "Normal",
	1: "Paused",
	2: "Error",
	3: "Pending Deletion",
	4: "Paper Jam",
	5: "Paper Out",
	6: "Manual Feed",
	7: "Paper Problem",
	8: "Offline",
}

// ListPrinters enumerates installed printers via PowerShell's Get-Printer.
func (s *Service) ListPrinters() ([]PrinterInfo, error) {
	// Wrap in @() to guarantee an array even for a single printer.
	psCmd := `@(Get-Printer) | Select-Object Name, PrinterStatus, Type | ConvertTo-Json -Depth 2 -Compress`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list printers: %w", err)
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return nil, nil
	}

	var psPrinters []psPrinter

	// PowerShell may return a single object or an array.
	if strings.HasPrefix(raw, "[") {
		if err := json.Unmarshal([]byte(raw), &psPrinters); err != nil {
			return nil, fmt.Errorf("failed to parse printer list: %w", err)
		}
	} else {
		var single psPrinter
		if err := json.Unmarshal([]byte(raw), &single); err != nil {
			return nil, fmt.Errorf("failed to parse printer: %w", err)
		}
		psPrinters = append(psPrinters, single)
	}

	// Determine default printer.
	defaultCmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		`(Get-CimInstance -ClassName Win32_Printer -Filter "Default=True").Name`)
	defaultOut, _ := defaultCmd.Output()
	defaultName := strings.TrimSpace(string(defaultOut))

	printers := make([]PrinterInfo, 0, len(psPrinters))
	for _, p := range psPrinters {
		status, ok := statusMap[p.PrinterStatus]
		if !ok {
			status = "Unknown"
		}
		// Load supplies from the service map or generate defaults.
		s.mu.Lock()
		supplies, exists := s.supplies[p.Name]
		if !exists {
			supplies = GenerateDefaultSupplies(p.Name)
			s.supplies[p.Name] = supplies
		}
		if IsVirtualPrinter(p.Name) {
			supplies = nil
		}
		s.mu.Unlock()

		printers = append(printers, PrinterInfo{
			Name:      p.Name,
			Status:    status,
			IsDefault: strings.EqualFold(p.Name, defaultName),
			Shared:    s.IsPrinterShared(p.Name),
			Supplies:  supplies,
		})
	}
	return printers, nil
}

// PrintPDF sends a PDF file to the specified printer using SumatraPDF, Microsoft Edge, or Google Chrome.
func (s *Service) PrintPDF(printerName, filePath string, opts PrintOptions) error {
	// Try SumatraPDF first (preferred for reliable silent printing).
	sumatraPath, err := findSumatraPDF()
	if err == nil {
		args := []string{
			"-print-to", printerName,
			"-silent",
		}

		// Build SumatraPDF print settings: e.g. "1x,pages=1-5,mono,portrait,paper=A4,simplex"
		var settings []string
		if opts.Copies > 0 {
			settings = append(settings, fmt.Sprintf("%dx", opts.Copies))
		}
		if opts.Pages != "" && opts.Pages != "all" {
			settings = append(settings, fmt.Sprintf("pages=%s", opts.Pages))
		}
		if opts.Color == "mono" || opts.Color == "monochrome" {
			settings = append(settings, "mono")
		} else if opts.Color == "color" {
			settings = append(settings, "color")
		}
		if opts.Orientation == "landscape" {
			settings = append(settings, "landscape")
		} else if opts.Orientation == "portrait" {
			settings = append(settings, "portrait")
		}
		if opts.PaperSize != "" && opts.PaperSize != "default" {
			settings = append(settings, fmt.Sprintf("paper=%s", opts.PaperSize))
		}
		if opts.Duplex == "duplexlong" || opts.Duplex == "duplexshort" || opts.Duplex == "simplex" {
			settings = append(settings, opts.Duplex)
		}

		if len(settings) > 0 {
			args = append(args, "-print-settings", strings.Join(settings, ","))
		}

		args = append(args, filePath)
		cmd := exec.Command(sumatraPath, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("SumatraPDF print failed: %w — %s", err, string(output))
		}
		return nil
	}

	// Try Microsoft Edge next (usually installed on all Windows machines).
	edgePath, err := findEdge()
	if err == nil {
		cmd := exec.Command(edgePath,
			"--headless",
			"--disable-gpu",
			fmt.Sprintf("--print-to-destination=%s", printerName),
			filePath,
		)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("Microsoft Edge print failed: %w — %s", err, string(output))
		}
		return nil
	}

	// Try Google Chrome next.
	chromePath, err := findChrome()
	if err == nil {
		cmd := exec.Command(chromePath,
			"--headless",
			"--disable-gpu",
			fmt.Sprintf("--print-to-destination=%s", printerName),
			filePath,
		)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("Google Chrome print failed: %w — %s", err, string(output))
		}
		return nil
	}

	// Fallback: Use PowerShell Start-Process with the system default PDF handler.
	psCmd := fmt.Sprintf(
		`Start-Process -FilePath "%s" -Verb PrintTo -ArgumentList "%s" -Wait`,
		filePath, printerName,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("PowerShell print failed: %w — %s", err, string(output))
	}
	return nil
}

// findSumatraPDF attempts to locate SumatraPDF on the system.
func findSumatraPDF() (string, error) {
	// Check if it's on PATH first.
	if path, err := exec.LookPath("SumatraPDF.exe"); err == nil {
		return path, nil
	}

	// Check common installation directories.
	commonPaths := []string{
		`C:\Program Files\SumatraPDF\SumatraPDF.exe`,
		`C:\Program Files (x86)\SumatraPDF\SumatraPDF.exe`,
	}
	for _, p := range commonPaths {
		if _, err := exec.LookPath(p); err == nil {
			return p, nil
		}
		// LookPath might not work for absolute paths; try Command directly.
		cmd := exec.Command(p, "-h")
		if err := cmd.Start(); err == nil {
			_ = cmd.Process.Kill()
			return p, nil
		}
	}
	return "", fmt.Errorf("SumatraPDF not found")
}

// findEdge attempts to locate Microsoft Edge on the system.
func findEdge() (string, error) {
	path := `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("Edge not found")
}

// findChrome attempts to locate Google Chrome on the system.
func findChrome() (string, error) {
	paths := []string{
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("Chrome not found")
}
