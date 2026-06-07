package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// UpdateCheckResult details the results of checking the GitHub releases API.
type UpdateCheckResult struct {
	UpdateAvailable bool   `json:"updateAvailable"`
	LatestVersion   string `json:"latestVersion"`
	DownloadURL     string `json:"downloadUrl"`
	ReleaseNotes    string `json:"releaseNotes"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// GetVersion returns the currently running application version.
func (a *App) GetVersion() string {
	return AppVersion
}

// CheckForUpdate queries the GitHub Releases API to check if a newer version is available.
func (a *App) CheckForUpdate() (UpdateCheckResult, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/devkiraa/takeprint-desktop/releases/latest", nil)
	if err != nil {
		return UpdateCheckResult{}, err
	}
	req.Header.Set("User-Agent", "TakePrint-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return UpdateCheckResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UpdateCheckResult{}, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return UpdateCheckResult{}, err
	}

	current := AppVersion
	latest := rel.TagName
	updateAvailable := isNewerVersion(current, latest)

	var downloadURL string
	// Try to find installer.exe first
	for _, asset := range rel.Assets {
		if strings.Contains(strings.ToLower(asset.Name), "installer.exe") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	// Fallback to first .exe if no installer found
	if downloadURL == "" {
		for _, asset := range rel.Assets {
			if strings.HasSuffix(strings.ToLower(asset.Name), ".exe") {
				downloadURL = asset.BrowserDownloadURL
				break
			}
		}
	}

	return UpdateCheckResult{
		UpdateAvailable: updateAvailable,
		LatestVersion:   latest,
		DownloadURL:     downloadURL,
		ReleaseNotes:    rel.Body,
	}, nil
}

// StartUpdate downloads and runs the installer in the background.
func (a *App) StartUpdate(downloadURL string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("auto-update is only supported on Windows")
	}
	if downloadURL == "" {
		return fmt.Errorf("download URL cannot be empty")
	}

	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "TakePrint-Setup.exe")

	go func() {
		a.addLog("info", "Starting update download...")
		err := a.downloadFile(downloadURL, tempFile)
		if err != nil {
			a.addLog("error", fmt.Sprintf("Failed to download update: %v", err))
			wailsRuntime.EventsEmit(a.ctx, "update_error", err.Error())
			return
		}

		a.addLog("info", "Update downloaded successfully. Launching installer...")
		wailsRuntime.EventsEmit(a.ctx, "update_status", "launching")

		// Launch the installer detached from the current process
		cmd := exec.Command(tempFile)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    false,
			CreationFlags: 0x00000008, // DETACHED_PROCESS
		}

		err = cmd.Start()
		if err != nil {
			a.addLog("error", fmt.Sprintf("Failed to run installer: %v", err))
			wailsRuntime.EventsEmit(a.ctx, "update_error", err.Error())
			return
		}

		// Exit the current app so that the installer doesn't run into file-locking conflicts
		os.Exit(0)
	}()

	return nil
}

// Helper to track download progress
type progressWriter struct {
	total      int64
	downloaded int64
	onProgress func(percent float64)
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.downloaded += int64(n)
	if pw.total > 0 {
		percent := float64(pw.downloaded) / float64(pw.total) * 100
		pw.onProgress(percent)
	}
	return n, nil
}

func (a *App) downloadFile(url string, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad response status: %s", resp.Status)
	}

	pw := &progressWriter{
		total: resp.ContentLength,
		onProgress: func(percent float64) {
			wailsRuntime.EventsEmit(a.ctx, "update_progress", percent)
		},
	}

	_, err = io.Copy(out, io.TeeReader(resp.Body, pw))
	return err
}

// Compare semantic version strings (e.g. "v1.0.2" < "v1.0.3")
func isNewerVersion(current, latest string) bool {
	current = strings.TrimPrefix(current, "v")
	latest = strings.TrimPrefix(latest, "v")

	currParts := strings.Split(current, ".")
	lateParts := strings.Split(latest, ".")

	for i := 0; i < 3; i++ {
		var currVal, lateVal int
		if i < len(currParts) {
			currVal, _ = strconv.Atoi(currParts[i])
		}
		if i < len(lateParts) {
			lateVal, _ = strconv.Atoi(lateParts[i])
		}
		if lateVal > currVal {
			return true
		}
		if currVal > lateVal {
			return false
		}
	}
	return false
}
