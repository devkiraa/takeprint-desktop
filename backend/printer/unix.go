//go:build !windows

package printer

import (
	"fmt"
	"os/exec"
	"strings"
)

// ListPrinters enumerates installed printers via CUPS (lpstat).
func (s *Service) ListPrinters() ([]PrinterInfo, error) {
	// Get list of printers.
	cmd := exec.Command("lpstat", "-p")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list printers: %w", err)
	}

	// Get default printer.
	defaultCmd := exec.Command("lpstat", "-d")
	defaultOut, _ := defaultCmd.Output()
	defaultName := ""
	if parts := strings.SplitN(strings.TrimSpace(string(defaultOut)), ":", 2); len(parts) == 2 {
		defaultName = strings.TrimSpace(parts[1])
	}

	var printers []PrinterInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// lpstat -p format: "printer <name> is idle." or "printer <name> disabled ..."
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "printer" {
			continue
		}
		name := fields[1]
		status := "Idle"
		if strings.Contains(line, "disabled") {
			status = "Disabled"
		} else if strings.Contains(line, "idle") {
			status = "Idle"
		} else if strings.Contains(line, "printing") {
			status = "Printing"
		}
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
			IsDefault: name == defaultName,
			Shared:    s.IsPrinterShared(name),
			Supplies:  supplies,
		})
	}
	return printers, nil
}

// PrintPDF sends a PDF file to the specified printer via CUPS (lp).
func (s *Service) PrintPDF(printerName, filePath string, opts PrintOptions) error {
	args := []string{"-d", printerName}
	if opts.Copies > 1 {
		args = append(args, "-n", fmt.Sprintf("%d", opts.Copies))
	}
	if opts.Pages != "" && opts.Pages != "all" {
		args = append(args, "-P", opts.Pages)
	}
	if opts.Color == "mono" || opts.Color == "monochrome" {
		args = append(args, "-o", "ColorModel=Gray")
	} else if opts.Color == "color" {
		args = append(args, "-o", "ColorModel=Color")
	}
	args = append(args, filePath)

	cmd := exec.Command("lp", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("lp print failed: %w — %s", err, string(output))
	}
	return nil
}
