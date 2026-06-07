//go:build windows

package printer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Win32 Printer Status constants
const (
	PRINTER_STATUS_PAUSED            = 0x00000001
	PRINTER_STATUS_ERROR             = 0x00000002
	PRINTER_STATUS_PENDING_DELETION  = 0x00000004
	PRINTER_STATUS_PAPER_JAM         = 0x00000008
	PRINTER_STATUS_PAPER_OUT         = 0x00000010
	PRINTER_STATUS_MANUAL_FEED       = 0x00000020
	PRINTER_STATUS_PAPER_PROBLEM     = 0x00000040
	PRINTER_STATUS_OFFLINE           = 0x00000080
	PRINTER_STATUS_IO_ACTIVE         = 0x00000100
	PRINTER_STATUS_BUSY              = 0x00000200
	PRINTER_STATUS_PRINTING          = 0x00000400
	PRINTER_STATUS_OUTPUT_BIN_FULL   = 0x00000800
	PRINTER_STATUS_NOT_AVAILABLE     = 0x00001000
	PRINTER_STATUS_WAITING           = 0x00002000
	PRINTER_STATUS_PROCESSING        = 0x00004000
	PRINTER_STATUS_INITIALIZING      = 0x00008000
	PRINTER_STATUS_WARMING_UP        = 0x00010000
	PRINTER_STATUS_TONER_LOW         = 0x00020000
	PRINTER_STATUS_NO_TONER          = 0x00040000
	PRINTER_STATUS_PAGE_PUNT         = 0x00080000
	PRINTER_STATUS_USER_INTERVENTION = 0x00100000
	PRINTER_STATUS_OUT_OF_MEMORY     = 0x00200000
	PRINTER_STATUS_DOOR_OPEN         = 0x00400000
	PRINTER_STATUS_SERVER_UNKNOWN    = 0x00800000
	PRINTER_STATUS_POWER_SAVE        = 0x01000000
)

const (
	PRINTER_ENUM_LOCAL       = 0x00000002
	PRINTER_ENUM_CONNECTIONS = 0x00000004
)

type PRINTER_INFO_2W struct {
	ServerName         *uint16
	PrinterName        *uint16
	ShareName          *uint16
	PortName           *uint16
	DriverName         *uint16
	Comment            *uint16
	Location           *uint16
	DevMode            uintptr
	SepFile            *uint16
	PrintProcessor     *uint16
	Datatype           *uint16
	Parameters         *uint16
	SecurityDescriptor uintptr
	Attributes         uint32
	Priority           uint32
	DefaultPriority    uint32
	StartTime          uint32
	UntilTime          uint32
	Status             uint32
	JobsCount          uint32
	AveragePPM         uint32
}

var (
	winspoolW             = syscall.NewLazyDLL("winspool.drv")
	procEnumPrinters      = winspoolW.NewProc("EnumPrintersW")
	procGetDefaultPrinter = winspoolW.NewProc("GetDefaultPrinterW")
)

func utf16PtrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	var buf []uint16
	ptr := uintptr(unsafe.Pointer(p))
	for {
		val := *(*uint16)(unsafe.Pointer(ptr))
		if val == 0 {
			break
		}
		buf = append(buf, val)
		ptr += 2
	}
	return syscall.UTF16ToString(buf)
}

func enumPrintersW(flags uint32) ([]PRINTER_INFO_2W, error) {
	var needed, returned uint32
	procEnumPrinters.Call(
		uintptr(flags),
		0,
		2,
		0,
		0,
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&returned)),
	)
	if needed == 0 {
		return nil, nil
	}

	buf := make([]byte, needed)
	r1, _, err := procEnumPrinters.Call(
		uintptr(flags),
		0,
		2,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(needed),
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&returned)),
	)
	if r1 == 0 {
		if err != nil && err != syscall.Errno(0) {
			return nil, err
		}
		return nil, fmt.Errorf("EnumPrintersW failed")
	}

	if returned == 0 {
		return nil, nil
	}

	infos := unsafe.Slice((*PRINTER_INFO_2W)(unsafe.Pointer(&buf[0])), returned)
	res := make([]PRINTER_INFO_2W, len(infos))
	copy(res, infos)
	return res, nil
}

func getDefaultPrinterW() (string, error) {
	var size uint32
	procGetDefaultPrinter.Call(
		0,
		uintptr(unsafe.Pointer(&size)),
	)
	if size == 0 {
		return "", nil
	}

	buf := make([]uint16, size)
	r1, _, err := procGetDefaultPrinter.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r1 == 0 {
		if err != nil && err != syscall.Errno(0) {
			return "", err
		}
		return "", fmt.Errorf("GetDefaultPrinterW failed")
	}

	return syscall.UTF16ToString(buf), nil
}

func translateStatus(status uint32) string {
	if status == 0 {
		return "Idle"
	}
	if status&PRINTER_STATUS_ERROR != 0 {
		return "Error"
	}
	if status&PRINTER_STATUS_PAPER_JAM != 0 {
		return "Paper Jam"
	}
	if status&PRINTER_STATUS_PAPER_OUT != 0 {
		return "Paper Out"
	}
	if status&PRINTER_STATUS_OFFLINE != 0 {
		return "Offline"
	}
	if status&PRINTER_STATUS_PAUSED != 0 {
		return "Paused"
	}
	if status&PRINTER_STATUS_PRINTING != 0 {
		return "Printing"
	}
	if status&PRINTER_STATUS_WARMING_UP != 0 {
		return "Warming Up"
	}
	if status&PRINTER_STATUS_BUSY != 0 {
		return "Busy"
	}
	if status&PRINTER_STATUS_INITIALIZING != 0 {
		return "Initializing"
	}
	if status&PRINTER_STATUS_TONER_LOW != 0 {
		return "Toner Low"
	}
	if status&PRINTER_STATUS_NO_TONER != 0 {
		return "No Toner"
	}
	if status&PRINTER_STATUS_DOOR_OPEN != 0 {
		return "Door Open"
	}
	return "Normal"
}

// sumatraOnce ensures we only try to install SumatraPDF once per session.
var sumatraOnce sync.Once
var sumatraInstallErr error

// ListPrinters enumerates installed printers natively via Win32 Spooler.
func (s *Service) ListPrinters() ([]PrinterInfo, error) {
	s.mu.Lock()
	if len(s.cachedPrinters) > 0 && time.Since(s.lastCacheTime) < 10*time.Second {
		printersCopy := make([]PrinterInfo, len(s.cachedPrinters))
		copy(printersCopy, s.cachedPrinters)
		s.mu.Unlock()
		return printersCopy, nil
	}
	s.mu.Unlock()

	defaultName, _ := getDefaultPrinterW()
	defaultName = strings.TrimSpace(defaultName)

	psPrinters, err := enumPrintersW(PRINTER_ENUM_LOCAL | PRINTER_ENUM_CONNECTIONS)
	if err != nil {
		return nil, fmt.Errorf("failed to list printers natively: %w", err)
	}

	printers := make([]PrinterInfo, 0, len(psPrinters))
	for _, p := range psPrinters {
		name := utf16PtrToString(p.PrinterName)
		status := translateStatus(p.Status)

		// Load supplies from the service map or generate defaults.
		s.mu.Lock()
		supplies, exists := s.supplies[name]
		if !exists {
			supplies = GenerateDefaultSupplies(name)
			s.supplies[name] = supplies
		}
		if IsVirtualPrinter(name) {
			supplies = nil
		}
		s.mu.Unlock()

		printers = append(printers, PrinterInfo{
			Name:      name,
			Status:    status,
			IsDefault: strings.EqualFold(name, defaultName),
			Shared:    s.IsPrinterShared(name),
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
