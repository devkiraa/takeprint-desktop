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
	Status      string `json:"status"` // "printing", "completed", "failed", "saved"
	SubmittedAt string `json:"submittedAt"`
	Pages       string `json:"pages"`
	Color       string `json:"color"`
	Copies      int    `json:"copies"`
	Orientation string `json:"orientation,omitempty"`
	PaperSize   string `json:"paperSize,omitempty"`
	Duplex      string `json:"duplex,omitempty"`
	Error       string `json:"error,omitempty"`
}

type pendingJob struct {
	jobID       string
	printerName string
	filePath    string
	filename    string
	opts        printer.PrintOptions
}

// Server wraps the HTTP server for receiving print jobs.
type Server struct {
	httpServer     *http.Server
	printerService *printer.Service
	Logger         func(msg string)
	jobs           []PrintJob
	jobsMu         sync.RWMutex
	authToken      string
	printQueue     chan pendingJob
	onJobUpdate    func()
	onJobNotify    func(status, filename, printerName, errMsg string)
}

// New creates and configures the HTTP print server.
func New(addr string, authToken string, ps *printer.Service, logger func(string), onJobUpdate func(), onJobNotify func(status, filename, printerName, errMsg string)) *Server {
	s := &Server{
		printerService: ps,
		Logger:         logger,
		jobs:           make([]PrintJob, 0),
		authToken:      authToken,
		printQueue:     make(chan pendingJob, 100),
		onJobUpdate:    onJobUpdate,
		onJobNotify:    onJobNotify,
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
		ReadTimeout:  60 * time.Second,  // Extended for slow file uploads
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go s.startQueueWorker()

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

// --- Token Verification ---
func (s *Server) verifyToken(w http.ResponseWriter, r *http.Request) bool {
	if s.authToken == "" {
		return true
	}
	authHeader := r.Header.Get("Authorization")
	var token string
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		token = r.Header.Get("X-TakePrint-Token")
		if token == "" {
			token = r.URL.Query().Get("token")
		}
	}

	if token != s.authToken {
		s.Logger(fmt.Sprintf("🔒 Rejected unauthorized access from %s", r.RemoteAddr))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

// --- Job Queue Management ---

func (s *Server) addJob(job PrintJob) {
	s.jobsMu.Lock()
	// Insert at the beginning of the slice (newest first)
	s.jobs = append([]PrintJob{job}, s.jobs...)
	if len(s.jobs) > 20 {
		s.jobs = s.jobs[:20]
	}
	s.jobsMu.Unlock()
	if s.onJobUpdate != nil {
		s.onJobUpdate()
	}
}

func (s *Server) updateJobStatus(id string, status string, errStr string) {
	s.jobsMu.Lock()
	updated := false
	for i, job := range s.jobs {
		if job.ID == id {
			s.jobs[i].Status = status
			s.jobs[i].Error = errStr
			updated = true
			break
		}
	}
	s.jobsMu.Unlock()
	if updated && s.onJobUpdate != nil {
		s.onJobUpdate()
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

// --- Sequential Print Queue Worker ---

func (s *Server) startQueueWorker() {
	for job := range s.printQueue {
		s.processPrintJob(job)
	}
}

func (s *Server) processPrintJob(job pendingJob) {
	defer os.Remove(job.filePath)

	if printer.IsVirtualPrinter(job.printerName) {
		saveDir := s.printerService.GetVirtualPrinterDir()
		if saveDir == "" {
			home, _ := os.UserHomeDir()
			saveDir = filepath.Join(home, "Downloads")
		}
		destPath := filepath.Join(saveDir, job.filename)
		destPath = getUniquePath(destPath)

		if err := copyFile(job.filePath, destPath); err != nil {
			s.Logger(fmt.Sprintf("❌ Failed to save virtual print to '%s': %v", destPath, err))
			s.updateJobStatus(job.jobID, "failed", err.Error())
			if s.onJobNotify != nil {
				s.onJobNotify("failed", job.filename, job.printerName, err.Error())
			}
		} else {
			s.Logger(fmt.Sprintf("💾 Saved virtual print successfully to '%s'", destPath))
			s.updateJobStatus(job.jobID, "saved", "")
			if s.onJobNotify != nil {
				s.onJobNotify("saved", job.filename, job.printerName, "")
			}
		}
		return
	}

	if err := s.printerService.PrintPDF(job.printerName, job.filePath, job.opts); err != nil {
		s.Logger(fmt.Sprintf("❌ Print failed for '%s': %v", job.filename, err))
		s.updateJobStatus(job.jobID, "failed", err.Error())
		if s.onJobNotify != nil {
			s.onJobNotify("failed", job.filename, job.printerName, err.Error())
		}
	} else {
		s.Logger(fmt.Sprintf("✅ Printed '%s' on '%s' (Pages: %s, Color: %s, Copies: %d)",
			job.filename, job.printerName, job.opts.Pages, job.opts.Color, job.opts.Copies))
		s.updateJobStatus(job.jobID, "completed", "")
		if s.onJobNotify != nil {
			s.onJobNotify("completed", job.filename, job.printerName, "")
		}

		// Decrement printer supplies on successful print.
		pagesCount := estimatePages(job.opts.Pages) * job.opts.Copies
		s.printerService.DecrementSupplies(job.printerName, pagesCount, job.opts.Color)
	}
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
	if !s.verifyToken(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.GetJobs())
}

func (s *Server) handlePrinters(w http.ResponseWriter, r *http.Request) {
	if !s.verifyToken(w, r) {
		return
	}
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
	if !s.verifyToken(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit total request size to 50 MB
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20)

	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "Expected multipart/form-data", http.StatusBadRequest)
		return
	}

	var printerName string
	var pages string = "all"
	var color string = "color"
	var orientation string = "portrait"
	var paperSize string = "A4"
	var duplex string = "simplex"
	var copies int = 1
	var tmpPath string
	var headerFilename string

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			if tmpPath != "" {
				os.Remove(tmpPath)
			}
			http.Error(w, "Error parsing form data", http.StatusBadRequest)
			return
		}

		formName := part.FormName()
		if formName == "file" {
			headerFilename = filepath.Base(part.FileName())
			if filepath.Ext(headerFilename) != ".pdf" {
				part.Close()
				if tmpPath != "" {
					os.Remove(tmpPath)
				}
				http.Error(w, "Only PDF files are supported", http.StatusBadRequest)
				return
			}

			// Create temporary file on disk to stream upload directly
			tmpFile, err := os.CreateTemp("", "takeprint-*.pdf")
			if err != nil {
				part.Close()
				s.Logger(fmt.Sprintf("❌ Failed to create temp file: %v", err))
				http.Error(w, "Server error", http.StatusInternalServerError)
				return
			}
			tmpPath = tmpFile.Name()

			_, err = io.Copy(tmpFile, part)
			tmpFile.Close()
			part.Close()
			if err != nil {
				os.Remove(tmpPath)
				s.Logger(fmt.Sprintf("❌ Failed to save uploaded file: %v", err))
				http.Error(w, "Failed to save file", http.StatusInternalServerError)
				return
			}
		} else {
			fieldBytes, err := io.ReadAll(part)
			part.Close()
			if err != nil {
				if tmpPath != "" {
					os.Remove(tmpPath)
				}
				http.Error(w, "Error reading form field", http.StatusBadRequest)
				return
			}
			val := string(fieldBytes)

			switch formName {
			case "printer":
				printerName = val
			case "pages":
				pages = val
			case "color":
				color = val
			case "orientation":
				orientation = val
			case "paperSize":
				paperSize = val
			case "duplex":
				duplex = val
			case "copies":
				if valInt, err := strconv.Atoi(val); err == nil && valInt > 0 {
					copies = valInt
				}
			}
		}
	}

	if printerName == "" {
		if tmpPath != "" {
			os.Remove(tmpPath)
		}
		http.Error(w, "Missing 'printer' field", http.StatusBadRequest)
		return
	}

	if tmpPath == "" {
		http.Error(w, "Missing 'file' field", http.StatusBadRequest)
		return
	}

	// Verify the printer is actually shared.
	if !s.printerService.IsPrinterShared(printerName) {
		os.Remove(tmpPath)
		s.Logger(fmt.Sprintf("⚠️ Blocked print job targeting unshared printer: %s", printerName))
		http.Error(w, "Target printer is not shared", http.StatusForbidden)
		return
	}

	opts := printer.PrintOptions{
		Pages:       pages,
		Color:       color,
		Copies:      copies,
		Orientation: orientation,
		PaperSize:   paperSize,
		Duplex:      duplex,
	}

	s.Logger(fmt.Sprintf("📥 Received print job: %s → %s (Pages: %s, Color: %s, Copies: %d, Orientation: %s, PaperSize: %s, Duplex: %s)",
		headerFilename, printerName, opts.Pages, opts.Color, opts.Copies, opts.Orientation, opts.PaperSize, opts.Duplex))

	// Track job
	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	job := PrintJob{
		ID:          jobID,
		Filename:    headerFilename,
		Printer:     printerName,
		Status:      "printing",
		SubmittedAt: time.Now().Format("15:04:05"),
		Pages:       opts.Pages,
		Color:       opts.Color,
		Copies:      opts.Copies,
		Orientation: opts.Orientation,
		PaperSize:   opts.PaperSize,
		Duplex:      opts.Duplex,
	}
	s.addJob(job)

	// Queue the print job for sequential execution
	s.printQueue <- pendingJob{
		jobID:       jobID,
		printerName: printerName,
		filePath:    tmpPath,
		filename:    headerFilename,
		opts:        opts,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"message": fmt.Sprintf("Print job '%s' queued for printer '%s'", headerFilename, printerName),
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-TakePrint-Token")

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

