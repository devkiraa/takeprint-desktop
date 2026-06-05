package printer

import (
	"strings"
	"sync"
)

// SupplyInfo represents a cartridge supply level (e.g. Cyan, Magenta, Yellow, Black).
type SupplyInfo struct {
	Name    string  `json:"name"`    // e.g. "Black", "Cyan", "Magenta", "Yellow"
	Type    string  `json:"type"`    // "ink" or "toner"
	Percent float64 `json:"percent"` // 0.0 to 100.0
}

// PrinterInfo represents a local OS printer.
type PrinterInfo struct {
	Name      string       `json:"name"`
	Status    string       `json:"status"`
	IsDefault bool         `json:"isDefault"`
	Shared    bool         `json:"shared"`
	Supplies  []SupplyInfo `json:"supplies"`
}

// PrintOptions holds the settings for printing a PDF document.
type PrintOptions struct {
	Pages       string `json:"pages"`       // e.g. "all", "1-5", "1,2,5"
	Color       string `json:"color"`       // "color" or "mono"
	Copies      int    `json:"copies"`      // number of copies
	Orientation string `json:"orientation"` // "portrait" or "landscape"
	PaperSize   string `json:"paperSize"`   // e.g. "A4", "letter", "legal", "A5"
	Duplex      string `json:"duplex"`      // "simplex", "duplexlong", "duplexshort"
}

// Service defines the cross-platform printer operations.
// Implementations are provided via build tags in windows.go and unix.go.
type Service struct {
	disabledPrinters  map[string]bool
	supplies          map[string][]SupplyInfo
	virtualPrinterDir string
	mu                sync.Mutex
}

// NewService creates a new printer service.
func NewService() *Service {
	return &Service{
		disabledPrinters: make(map[string]bool),
		supplies:         make(map[string][]SupplyInfo),
	}
}

// SetVirtualPrinterDir configures the directory where virtual prints are saved.
func (s *Service) SetVirtualPrinterDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.virtualPrinterDir = dir
}

// GetVirtualPrinterDir returns the directory where virtual prints are saved.
func (s *Service) GetVirtualPrinterDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.virtualPrinterDir
}

// SetSupplies configures the current cartridge levels in the service.
func (s *Service) SetSupplies(supplies map[string][]SupplyInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.supplies = supplies
}

// GetSupplies returns the current cartridge levels map.
func (s *Service) GetSupplies() map[string][]SupplyInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	cpy := make(map[string][]SupplyInfo)
	for k, v := range s.supplies {
		cpy[k] = v
	}
	return cpy
}

// DecrementSupplies reduces ink/toner cartridge levels for a printed job.
func (s *Service) DecrementSupplies(name string, pages int, color string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	levels, ok := s.supplies[name]
	if !ok {
		return
	}

	colorDec := float64(pages) * 0.5
	monoDec := float64(pages) * 0.2

	for i, c := range levels {
		var dec float64
		if color == "mono" || color == "monochrome" {
			if c.Name == "Black" {
				dec = monoDec
			}
		} else {
			if c.Name == "Black" {
				dec = monoDec
			} else {
				dec = colorDec
			}
		}

		newVal := c.Percent - dec
		if newVal < 0 {
			newVal = 0
		}
		levels[i].Percent = newVal
	}
	s.supplies[name] = levels
}

// IsVirtualPrinter checks if a printer is digital-only / virtual (e.g. PDF/XPS/OneNote).
func IsVirtualPrinter(name string) bool {
	nameLower := strings.ToLower(name)
	return strings.Contains(nameLower, "pdf") ||
		strings.Contains(nameLower, "xps") ||
		strings.Contains(nameLower, "onenote") ||
		strings.Contains(nameLower, "writer") ||
		strings.Contains(nameLower, "fax") ||
		strings.Contains(nameLower, "send to")
}

// GenerateDefaultSupplies creates a mock set of CMYK or K cartridges.
func GenerateDefaultSupplies(printerName string) []SupplyInfo {
	if IsVirtualPrinter(printerName) {
		return nil
	}

	nameLower := strings.ToLower(printerName)
	isMono := strings.Contains(nameLower, "mono") ||
		strings.Contains(nameLower, "laserjet")

	if isMono {
		return []SupplyInfo{
			{Name: "Black", Type: "toner", Percent: 85.0},
		}
	}

	return []SupplyInfo{
		{Name: "Cyan", Type: "ink", Percent: 78.0},
		{Name: "Magenta", Type: "ink", Percent: 64.0},
		{Name: "Yellow", Type: "ink", Percent: 92.0},
		{Name: "Black", Type: "ink", Percent: 82.0},
	}
}

// TogglePrinterShare enables or disables sharing for a specific printer.
func (s *Service) TogglePrinterShare(name string, shared bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if shared {
		delete(s.disabledPrinters, name)
	} else {
		s.disabledPrinters[name] = true
	}
}

// IsPrinterShared checks if a printer is currently shared.
func (s *Service) IsPrinterShared(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.disabledPrinters[name]
}

// ListSharedPrinters returns only the printers that are enabled for sharing.
func (s *Service) ListSharedPrinters() ([]PrinterInfo, error) {
	all, err := s.ListPrinters()
	if err != nil {
		return nil, err
	}
	shared := make([]PrinterInfo, 0, len(all))
	for _, p := range all {
		if p.Shared {
			shared = append(shared, p)
		}
	}
	return shared, nil
}
