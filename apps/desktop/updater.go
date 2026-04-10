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
	"strings"
	"time"
)

const AppVersion = "1.2.2"

// VersionInfo is returned by the API's /api/agent/version endpoint.
type VersionInfo struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Notes       string `json:"notes"`
	Required    bool   `json:"required"`
}

// UpdateStatus is returned to the frontend.
type UpdateStatus struct {
	Available   bool   `json:"available"`
	Version     string `json:"version"`
	Notes       string `json:"notes"`
	Required    bool   `json:"required"`
	Downloading bool   `json:"downloading"`
	Progress    int    `json:"progress"` // 0-100
	Error       string `json:"error"`
}

var currentUpdate *VersionInfo

// GetCurrentVersion returns the running app version.
func (a *App) GetCurrentVersion() string {
	return AppVersion
}

// CheckForUpdate checks the API for a newer version.
func (a *App) CheckForUpdate() UpdateStatus {
	apiBase := "https://api.blutexts.com"
	if a.cfg != nil && a.cfg.APIEndpoint != "" {
		// Derive HTTP URL from WSS endpoint
		base := strings.Replace(a.cfg.APIEndpoint, "wss://", "https://", 1)
		base = strings.Replace(base, "ws://", "http://", 1)
		apiBase = base
	}

	url := fmt.Sprintf("%s/api/agent/version?current=%s&os=%s&arch=%s",
		apiBase, AppVersion, runtime.GOOS, runtime.GOARCH)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return UpdateStatus{Error: "Could not check for updates"}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return UpdateStatus{} // No update info available, that's fine
	}

	var info VersionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return UpdateStatus{Error: "Invalid update response"}
	}

	if info.Version == "" || info.Version == AppVersion {
		return UpdateStatus{}
	}

	currentUpdate = &info
	return UpdateStatus{
		Available: true,
		Version:   info.Version,
		Notes:     info.Notes,
		Required:  info.Required,
	}
}

// DownloadAndInstallUpdate downloads the new DMG and triggers install.
func (a *App) DownloadAndInstallUpdate() UpdateStatus {
	if currentUpdate == nil || currentUpdate.DownloadURL == "" {
		return UpdateStatus{Error: "No update available"}
	}

	// Download to temp
	tmpDir := os.TempDir()
	dmgPath := filepath.Join(tmpDir, "BlueSend-update.dmg")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(currentUpdate.DownloadURL)
	if err != nil {
		return UpdateStatus{Error: fmt.Sprintf("Download failed: %v", err)}
	}
	defer resp.Body.Close()

	out, err := os.Create(dmgPath)
	if err != nil {
		return UpdateStatus{Error: fmt.Sprintf("Could not save update: %v", err)}
	}

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		return UpdateStatus{Error: fmt.Sprintf("Download incomplete: %v", err)}
	}

	// Mount DMG, copy app, unmount
	if err := installFromDMG(dmgPath); err != nil {
		return UpdateStatus{Error: fmt.Sprintf("Install failed: %v", err)}
	}

	return UpdateStatus{
		Available: true,
		Version:   currentUpdate.Version,
		Notes:     "Update installed. Restart to apply.",
	}
}

func installFromDMG(dmgPath string) error {
	// Mount — don't use -quiet so we get the mount point in output
	mountOut, err := exec.Command("hdiutil", "attach", dmgPath, "-nobrowse").CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount dmg: %w — output: %s", err, string(mountOut))
	}

	// Find mount point — look for /Volumes/ in any part of the output
	mountPoint := ""
	for _, line := range strings.Split(string(mountOut), "\n") {
		idx := strings.Index(line, "/Volumes/")
		if idx >= 0 {
			mountPoint = strings.TrimSpace(line[idx:])
			break
		}
	}
	if mountPoint == "" {
		return fmt.Errorf("could not find mount point in: %s", string(mountOut))
	}
	defer exec.Command("hdiutil", "detach", mountPoint, "-quiet", "-force").Run()

	// Find .app in mounted volume
	entries, err := os.ReadDir(mountPoint)
	if err != nil {
		return fmt.Errorf("read mount: %w", err)
	}

	var appName string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".app") {
			appName = e.Name()
			break
		}
	}
	if appName == "" {
		return fmt.Errorf("no .app found in DMG")
	}

	srcApp := filepath.Join(mountPoint, appName)
	dstApp := filepath.Join("/Applications", appName)

	// Stage the new app to a temp location, then swap on restart
	stagePath := filepath.Join(os.TempDir(), "BlueSend-staged.app")
	os.RemoveAll(stagePath)
	cmd := exec.Command("cp", "-R", srcApp, stagePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stage app: %w — %s", err, string(out))
	}

	// Write a swap script that runs after we quit
	swapScript := filepath.Join(os.TempDir(), "bluesend-update.sh")
	script := fmt.Sprintf(`#!/bin/bash
sleep 2
rm -rf "%s"
mv "%s" "%s"
open -n "%s"
rm -f "%s"
`, dstApp, stagePath, dstApp, dstApp, swapScript)

	os.WriteFile(swapScript, []byte(script), 0755)

	// Clean up DMG
	os.Remove(dmgPath)

	return nil
}

// RestartApp runs the swap script and quits so the new version can take over.
func (a *App) RestartApp() {
	swapScript := filepath.Join(os.TempDir(), "bluesend-update.sh")
	if _, err := os.Stat(swapScript); err == nil {
		exec.Command("bash", swapScript).Start()
	} else {
		// No swap script, just relaunch
		exec.Command("open", "-n", "/Applications/bluesend.app").Start()
	}
	os.Exit(0)
}
