//go:build windows

package printer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// psPrinter is the JSON shape returned by PowerShell's Get-Printer.
type psPrinter struct {
	Name          string `json:"Name"`
	PrinterStatus int    `json:"PrinterStatus"`
	Type          int    `json:"Type"`
}

// statusMap translates the PrinterStatus integer codes from Get-Printer / CIM_Printer.
var statusMap = map[int]string{
	0: "Normal",
	1: "Unknown",
	2: "Idle",
	3: "Printing",
	4: "Warming Up",
	5: "Stopped",
	6: "Offline",
	7: "Paused",
}

// sumatraOnce ensures we only try to install SumatraPDF once per session.
var sumatraOnce sync.Once
var sumatraInstallErr error

// ListPrinters enumerates installed printers via PowerShell's Get-Printer.
func (s *Service) ListPrinters() ([]PrinterInfo, error) {
	s.mu.Lock()
	if len(s.cachedPrinters) > 0 && time.Since(s.lastCacheTime) < 10*time.Second {
		printersCopy := make([]PrinterInfo, len(s.cachedPrinters))
		copy(printersCopy, s.cachedPrinters)
		s.mu.Unlock()
		return printersCopy, nil
	}
	s.mu.Unlock()

	// Wrap in @() to guarantee an array even for a single printer.
	psCmd := `@(Get-Printer) | Select-Object Name, PrinterStatus, Type | ConvertTo-Json -Depth 2 -Compress`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
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
	defaultCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
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

	s.mu.Lock()
	s.cachedPrinters = make([]PrinterInfo, len(printers))
	copy(s.cachedPrinters, printers)
	s.lastCacheTime = time.Now()
	s.mu.Unlock()

	return printers, nil
}

// PrintPDF sends a PDF file to the specified printer.
// It tries multiple methods in order of reliability:
// 1. SumatraPDF (best — silent, supports all print settings)
// 2. Adobe Reader (good — widely installed)
// 3. Foxit Reader (good — popular alternative)
// 4. Auto-install SumatraPDF via winget and retry
// 5. Default-printer-swap with system PDF handler (guaranteed fallback)
func (s *Service) PrintPDF(printerName, filePath string, opts PrintOptions) error {
	// 1. Try SumatraPDF (preferred for reliable silent printing).
	if err := s.printViaSumatraPDF(printerName, filePath, opts); err == nil {
		return nil
	}

	// 2. Try Adobe Reader / Acrobat.
	if adobePath, err := findAdobeReader(); err == nil {
		if err := printViaAdobe(adobePath, printerName, filePath); err == nil {
			return nil
		}
	}

	// 3. Try Foxit Reader.
	if foxitPath, err := findFoxitReader(); err == nil {
		if err := printViaFoxit(foxitPath, printerName, filePath); err == nil {
			return nil
		}
	}

	// 4. Auto-install SumatraPDF via winget and retry.
	sumatraOnce.Do(func() {
		sumatraInstallErr = installSumatraPDF()
	})
	if sumatraInstallErr == nil {
		if err := s.printViaSumatraPDF(printerName, filePath, opts); err == nil {
			return nil
		}
	}

	// 5. Guaranteed fallback: temporarily set target as default printer,
	// print with -Verb Print, then restore the original default.
	return printViaDefaultSwap(printerName, filePath)
}

// --- Method 1: SumatraPDF ---

func (s *Service) printViaSumatraPDF(printerName, filePath string, opts PrintOptions) error {
	sumatraPath, err := findSumatraPDF()
	if err != nil {
		return err
	}

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
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("SumatraPDF print failed: %w — %s", err, string(output))
	}
	return nil
}

// --- Method 2: Adobe Reader ---

func printViaAdobe(adobePath, printerName, filePath string) error {
	// Adobe Reader syntax: AcroRd32.exe /t "file.pdf" "PrinterName"
	cmd := exec.Command(adobePath, "/t", filePath, printerName)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Adobe Reader start failed: %w", err)
	}

	// Adobe Reader doesn't exit cleanly after /t; wait with timeout then kill.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("Adobe Reader print failed: %w", err)
		}
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		// Timeout is normal for Adobe Reader /t — the job was still sent.
	}
	return nil
}

// --- Method 3: Foxit Reader ---

func printViaFoxit(foxitPath, printerName, filePath string) error {
	// Foxit syntax: FoxitPDFReader.exe /t "file.pdf" "PrinterName"
	cmd := exec.Command(foxitPath, "/t", filePath, printerName)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Foxit Reader start failed: %w", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("Foxit Reader print failed: %w", err)
		}
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
	}
	return nil
}

// --- Method 5: Default Printer Swap ---

func printViaDefaultSwap(printerName, filePath string) error {
	// 1. Get the current default printer
	getDefaultPS := `(Get-CimInstance -ClassName Win32_Printer -Filter "Default=True").Name`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", getDefaultPS)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, _ := cmd.Output()
	originalDefault := strings.TrimSpace(string(out))

	// 2. Set the target printer as default
	setDefaultPS := fmt.Sprintf(
		`(Get-WmiObject -Query "SELECT * FROM Win32_Printer WHERE Name='%s'").SetDefaultPrinter()`,
		strings.ReplaceAll(printerName, "'", "''"),
	)
	cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", setDefaultPS)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set default printer to '%s': %w", printerName, err)
	}

	// 3. Print the file using -Verb Print (sends to default printer).
	// The -Verb Print is more widely supported than -Verb PrintTo.
	printPS := fmt.Sprintf(
		`Start-Process -FilePath '%s' -Verb Print -WindowStyle Hidden -Wait -PassThru`,
		strings.ReplaceAll(filePath, "'", "''"),
	)
	cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", printPS)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	printOutput, printErr := cmd.CombinedOutput()

	// 4. Restore the original default printer (always, even on error)
	if originalDefault != "" && !strings.EqualFold(originalDefault, printerName) {
		restorePS := fmt.Sprintf(
			`(Get-WmiObject -Query "SELECT * FROM Win32_Printer WHERE Name='%s'").SetDefaultPrinter()`,
			strings.ReplaceAll(originalDefault, "'", "''"),
		)
		restoreCmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", restorePS)
		restoreCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = restoreCmd.Run()
	}

	if printErr != nil {
		return fmt.Errorf("print via default-swap failed: %w — %s", printErr, string(printOutput))
	}
	return nil
}

// --- Tool Finders ---

// findSumatraPDF attempts to locate SumatraPDF on the system.
func findSumatraPDF() (string, error) {
	// Check if it's on PATH first.
	if path, err := exec.LookPath("SumatraPDF.exe"); err == nil {
		return path, nil
	}

	// Build list of common installation directories.
	commonPaths := []string{
		`C:\Program Files\SumatraPDF\SumatraPDF.exe`,
		`C:\Program Files (x86)\SumatraPDF\SumatraPDF.exe`,
	}

	// Also check %LOCALAPPDATA% (winget install location) and %APPDATA%/TakePrint
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		commonPaths = append(commonPaths,
			filepath.Join(localAppData, "SumatraPDF", "SumatraPDF.exe"),
		)
	}
	if appData := os.Getenv("APPDATA"); appData != "" {
		commonPaths = append(commonPaths,
			filepath.Join(appData, "TakePrint", "tools", "SumatraPDF.exe"),
		)
	}

	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("SumatraPDF not found")
}

// findAdobeReader attempts to locate Adobe Reader or Adobe Acrobat.
func findAdobeReader() (string, error) {
	paths := []string{
		`C:\Program Files\Adobe\Acrobat DC\Acrobat\Acrobat.exe`,
		`C:\Program Files (x86)\Adobe\Acrobat DC\Acrobat\Acrobat.exe`,
		`C:\Program Files\Adobe\Acrobat Reader DC\Reader\AcroRd32.exe`,
		`C:\Program Files (x86)\Adobe\Acrobat Reader DC\Reader\AcroRd32.exe`,
		`C:\Program Files\Adobe\Reader 11.0\Reader\AcroRd32.exe`,
		`C:\Program Files (x86)\Adobe\Reader 11.0\Reader\AcroRd32.exe`,
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("Adobe Reader not found")
}

// findFoxitReader attempts to locate Foxit PDF Reader.
func findFoxitReader() (string, error) {
	paths := []string{
		`C:\Program Files\Foxit Software\Foxit PDF Reader\FoxitPDFReader.exe`,
		`C:\Program Files (x86)\Foxit Software\Foxit PDF Reader\FoxitPDFReader.exe`,
		`C:\Program Files\Foxit Software\Foxit Reader\FoxitReader.exe`,
		`C:\Program Files (x86)\Foxit Software\Foxit Reader\FoxitReader.exe`,
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("Foxit Reader not found")
}

// installSumatraPDF attempts to install SumatraPDF silently via winget.
func installSumatraPDF() error {
	// Try winget first (built into Windows 10 1809+ and Windows 11).
	cmd := exec.Command("winget", "install",
		"--id", "SumatraPDF.SumatraPDF",
		"--exact", "--silent",
		"--accept-package-agreements",
		"--accept-source-agreements",
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("winget install failed: %w — %s", err, string(output))
	}
	return nil
}
