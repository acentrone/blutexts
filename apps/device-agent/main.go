package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bluesend/device-agent/internal/config"
	agentSync "github.com/bluesend/device-agent/internal/sync"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("BlueSend Device Agent starting...")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	agent, err := agentSync.NewAgent(cfg.APIEndpoint, cfg.DeviceToken, cfg.DeviceName)
	if err != nil {
		log.Fatalf("Agent init error: %v\n\nEnsure:\n  1. Messages.app is signed in to iMessage\n  2. Full Disk Access is granted to this binary\n     (System Settings → Privacy & Security → Full Disk Access)\n  3. DEVICE_TOKEN is set correctly", err)
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go agent.Run()

	<-quit
	log.Println("Shutting down device agent...")
	agent.Stop()
}
