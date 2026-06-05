package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"takeprint/backend/printer"
)

// PrintJob represents a tracked print job in the server queue.
type PrintJob struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	Printer     string `json:"printer"`
	Status      string `json:"status"` // "printing", "completed", "failed"
	SubmittedAt string `json:"submittedAt"`
	Pages       string `json:"pages"`
	Color       string `json:"color"`
	Copies      int    `json:"copies"`
	Error       string `json:"error,omitempty"`
}

// Server wraps the HTTP server for receiving print jobs.
type Server struct {
	httpServer     *http.Server
	printerService *printer.Service
	Logger         func(msg string)
	jobs           []PrintJob
	jobsMu         sync.RWMutex
}

// New creates and configures the HTTP print server.
func New(addr string, ps *printer.Service, logger func(string)) *Server {
	s := &Server{
		printerService: ps,
		Logger:         logger,
		jobs:           make([]PrintJob, 0),
	}
	if s.Logger == nil {
		s.Logger = func(msg string) { fmt.Println(msg) }
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/printers", s.handlePrinters)
	mux.HandleFunc("/print", s.handlePrint)
	mux.HandleFunc("/jobs", s.handleJobs)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      corsMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return s
}

// Start begins listening on the configured address.
func (s *Server) Start() error {
	s.Logger(fmt.Sprintf("🌐 HTTP server listening on %s", s.httpServer.Addr))
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.Logger(fmt.Sprintf("⚠️ HTTP shutdown error: %v", err))
	} else {
		s.Logger("🛑 HTTP server stopped")
	}
}

// --- Job Queue Management ---

func (s *Server) addJob(job PrintJob) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	// Insert at the beginning of the slice (newest first)
	s.jobs = append([]PrintJob{job}, s.jobs...)
	if len(s.jobs) > 20 {
		s.jobs = s.jobs[:20]
	}
}

func (s *Server) updateJobStatus(id string, status string, errStr string) {
	s.jobsMu.Lock()
	defer s.jobsMu.Unlock()
	for i, job := range s.jobs {
		if job.ID == id {
			s.jobs[i].Status = status
			s.jobs[i].Error = errStr
			break
		}
	}
}

// GetJobs returns a copy of the recent print jobs.
func (s *Server) GetJobs() []PrintJob {
	s.jobsMu.RLock()
	defer s.jobsMu.RUnlock()
	cpy := make([]PrintJob, len(s.jobs))
	copy(cpy, s.jobs)
	return cpy
}

// --- Handlers ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.GetJobs())
}

func (s *Server) handlePrinters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	printers, err := s.printerService.ListSharedPrinters()
	if err != nil {
		s.Logger(fmt.Sprintf("❌ Failed to list printers: %v", err))
		http.Error(w, fmt.Sprintf("Failed to list printers: %v", err), http.StatusInternalServerError)
		return
	}

	s.Logger(fmt.Sprintf("📋 Listed %d printer(s)", len(printers)))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(printers)
}

func (s *Server) handlePrint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit upload size to 50 MB.
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	printerName := r.FormValue("printer")
	if printerName == "" {
		http.Error(w, "Missing 'printer' field", http.StatusBadRequest)
		return
	}

	// Verify the printer is actually shared.
	if !s.printerService.IsPrinterShared(printerName) {
		s.Logger(fmt.Sprintf("⚠️ Blocked print job targeting unshared printer: %s", printerName))
		http.Error(w, "Target printer is not shared", http.StatusForbidden)
		return
	}

	// Parse print settings.
	pages := r.FormValue("pages")
	if pages == "" {
		pages = "all"
	}
	color := r.FormValue("color")
	if color == "" {
		color = "color"
	}
	copies := 1
	if copiesStr := r.FormValue("copies"); copiesStr != "" {
		if val, err := strconv.Atoi(copiesStr); err == nil && val > 0 {
			copies = val
		}
	}

	opts := printer.PrintOptions{
		Pages:  pages,
		Color:  color,
		Copies: copies,
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing 'file' field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file extension.
	ext := filepath.Ext(header.Filename)
	if ext != ".pdf" {
		http.Error(w, "Only PDF files are supported", http.StatusBadRequest)
		return
	}

	s.Logger(fmt.Sprintf("📥 Received print job: %s → %s (Pages: %s, Color: %s, Copies: %d, Size: %d bytes)",
		header.Filename, printerName, opts.Pages, opts.Color, opts.Copies, header.Size))

	// Save to temporary file.
	tmpFile, err := os.CreateTemp("", "takeprint-*.pdf")
	if err != nil {
		s.Logger(fmt.Sprintf("❌ Failed to create temp file: %v", err))
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, file); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		s.Logger(fmt.Sprintf("❌ Failed to save uploaded file: %v", err))
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	tmpFile.Close()

	// Track job
	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	job := PrintJob{
		ID:          jobID,
		Filename:    header.Filename,
		Printer:     printerName,
		Status:      "printing",
		SubmittedAt: time.Now().Format("15:04:05"),
		Pages:       opts.Pages,
		Color:       opts.Color,
		Copies:      opts.Copies,
	}
	s.addJob(job)

	// Execute the print job.
	go func() {
		defer os.Remove(tmpPath)

		if printer.IsVirtualPrinter(printerName) {
			saveDir := s.printerService.GetVirtualPrinterDir()
			if saveDir == "" {
				home, _ := os.UserHomeDir()
				saveDir = filepath.Join(home, "Downloads")
			}
			destPath := filepath.Join(saveDir, header.Filename)
			destPath = getUniquePath(destPath)

			if err := copyFile(tmpPath, destPath); err != nil {
				s.Logger(fmt.Sprintf("❌ Failed to save virtual print to '%s': %v", destPath, err))
				s.updateJobStatus(jobID, "failed", err.Error())
			} else {
				s.Logger(fmt.Sprintf("💾 Saved virtual print successfully to '%s'", destPath))
				s.updateJobStatus(jobID, "saved", "")
			}
			return
		}

		if err := s.printerService.PrintPDF(printerName, tmpPath, opts); err != nil {
			s.Logger(fmt.Sprintf("❌ Print failed for '%s': %v", header.Filename, err))
			s.updateJobStatus(jobID, "failed", err.Error())
		} else {
			s.Logger(fmt.Sprintf("✅ Printed '%s' on '%s' (Pages: %s, Color: %s, Copies: %d)",
				header.Filename, printerName, opts.Pages, opts.Color, opts.Copies))
			s.updateJobStatus(jobID, "completed", "")

			// Decrement printer supplies on successful print.
			pagesCount := estimatePages(opts.Pages) * opts.Copies
			s.printerService.DecrementSupplies(printerName, pagesCount, opts.Color)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"message": fmt.Sprintf("Print job '%s' queued for printer '%s'", header.Filename, printerName),
	})
}

// estimatePages calculates the approximate page count from printer request settings.
func estimatePages(pagesStr string) int {
	if pagesStr == "" || pagesStr == "all" {
		return 1
	}
	count := 0
	for _, part := range strings.Split(pagesStr, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) == 2 {
				start, _ := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
				end, _ := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
				if end >= start && start > 0 {
					count += (end - start + 1)
					continue
				}
			}
		}
		count++
	}
	if count <= 0 {
		return 1
	}
	return count
}

// --- Middleware ---

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func getUniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)

	for i := 1; ; i++ {
		newPath := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}
}

