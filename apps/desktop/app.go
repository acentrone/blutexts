package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	agent "github.com/bluesend/device-agent/pkg/agent"
)

const configDir = "BlueSend"
const configFile = "config.json"

// StoredConfig persists device setup to ~/Library/Application Support/BlueSend/
type StoredConfig struct {
	DeviceToken string `json:"device_token"`
	APIEndpoint string `json:"api_endpoint"`
	DeviceName  string `json:"device_name"`
}

// App is the main application struct bound to the Wails frontend.
type App struct {
	ctx   context.Context
	agent *agent.Agent
	cfg   *StoredConfig
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg, err := loadConfig()
	if err == nil && cfg.DeviceToken != "" {
		a.cfg = cfg
		go a.startAgent()
	}
}

func (a *App) shutdown(ctx context.Context) {
	if a.agent != nil {
		a.agent.Stop()
	}
}

// IsConfigured returns true if a device token has been saved.
func (a *App) IsConfigured() bool {
	if a.cfg != nil && a.cfg.DeviceToken != "" {
		return true
	}
	cfg, err := loadConfig()
	if err != nil {
		return false
	}
	a.cfg = cfg
	return cfg.DeviceToken != ""
}

// SaveSetup saves the device config and starts the agent.
func (a *App) SaveSetup(token, endpoint, name string) error {
	if token == "" {
		return fmt.Errorf("device token is required")
	}
	if endpoint == "" {
		endpoint = "wss://devices.blutexts.com"
	}
	if name == "" {
		hostname, _ := os.Hostname()
		name = hostname
	}

	cfg := &StoredConfig{
		DeviceToken: token,
		APIEndpoint: endpoint,
		DeviceName:  name,
	}
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	a.cfg = cfg

	go a.startAgent()
	return nil
}

// GetStatus returns the current agent status for the UI.
func (a *App) GetStatus() agent.StatusInfo {
	if a.agent == nil {
		return agent.StatusInfo{
			Connected:  false,
			DeviceName: a.getDeviceName(),
		}
	}
	return a.agent.GetStatus()
}

// GetActivityLog returns recent activity for the UI.
func (a *App) GetActivityLog() []agent.LogEntry {
	if a.agent == nil {
		return []agent.LogEntry{}
	}
	return a.agent.GetActivityLog()
}

// GetDeviceName returns the configured or hostname device name.
func (a *App) getDeviceName() string {
	if a.cfg != nil && a.cfg.DeviceName != "" {
		return a.cfg.DeviceName
	}
	hostname, _ := os.Hostname()
	return hostname
}

// CheckDiskAccess checks if we can read chat.db (Full Disk Access required).
func (a *App) CheckDiskAccess() bool {
	u, err := user.Current()
	if err != nil {
		return false
	}
	chatDB := filepath.Join(u.HomeDir, "Library", "Messages", "chat.db")
	f, err := os.Open(chatDB)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// OpenSystemPrefs opens the Full Disk Access settings page.
func (a *App) OpenSystemPrefs() {
	// This opens System Settings to Privacy > Full Disk Access on macOS Ventura+
	cmd := "open"
	args := []string{"x-apple.systempreferences:com.apple.preference.security?Privacy_AllFiles"}
	exec := os.Args[0] // unused, just for reference
	_ = exec
	osCmd := &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	}
	_ = osCmd
	// Use simple exec
	p, err := os.StartProcess("/usr/bin/open", append([]string{cmd}, args...), &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err == nil {
		p.Release()
	}
}

func (a *App) startAgent() {
	if a.cfg == nil {
		return
	}

	ag, err := agent.NewAgent(a.cfg.APIEndpoint, a.cfg.DeviceToken, a.cfg.DeviceName)
	if err != nil {
		fmt.Printf("Agent init error: %v\n", err)
		return
	}
	a.agent = ag
	a.agent.Run()
}

// Config file helpers

func configPath() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(u.HomeDir, "Library", "Application Support", configDir)
	os.MkdirAll(dir, 0700)
	return filepath.Join(dir, configFile), nil
}

func loadConfig() (*StoredConfig, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg StoredConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func saveConfig(cfg *StoredConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
