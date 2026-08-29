package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/luigieli/streaming/ws/pkg/audio"
	"github.com/luigieli/streaming/ws/pkg/config"
	"github.com/luigieli/streaming/ws/pkg/pipeline"
	"github.com/luigieli/streaming/ws/pkg/portal"
	"github.com/luigieli/streaming/ws/pkg/server"
)

func main() {
	cfg := config.Load()

	fmt.Println("=======================================================")
	fmt.Println("   ⚡ Go Wayland WebSocket Streamer (Ultra-Low Latency)")
	fmt.Println("=======================================================")
	fmt.Printf("[*] Configuration: Port=%d, Framerate=%d, Bitrate=%dk, Res=%dx%d\n",
		cfg.Port, cfg.Framerate, cfg.VideoBitrate, cfg.TargetWidth, cfg.TargetHeight)
	fmt.Printf("[*] Audio Filter: Blacklisting=%v, IncludeMic=%v\n",
		cfg.AudioBlacklist, cfg.IncludeMic)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle Graceful Shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Step 1: Initialize WebSocket Hub & Server
	hub := server.NewHub()
	go hub.Run()

	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "web"
	}
	srv := server.NewServer(webDir, hub)
	go func() {
		fmt.Printf("[*] Starting WebSocket Streaming Server on :%d...\n", cfg.Port)
		if err := srv.ListenAndServe(cfg.Port); err != nil {
			fmt.Printf("[!] Server exited: %v\n", err)
		}
	}()

	// Step 2: Start Audio Router & Blacklist Filter
	audioFilter := audio.NewFilter(cfg.AudioBlacklist, cfg.IncludeMic)
	audioRouter := audio.NewRouter(audioFilter, cfg.AudioRouting)
	if err := audioRouter.Start(ctx); err != nil {
		fmt.Printf("[AudioRouter] Notice: %v\n", err)
	}
	defer audioRouter.Stop()

	// Step 3: Trigger Wayland ScreenCast Portal
	fmt.Println("[*] Connecting to Wayland ScreenCast Portal...")
	portalClient, err := portal.NewClient()
	if err != nil {
		fmt.Printf("[!] Failed to connect to DBus: %v\n", err)
		os.Exit(1)
	}
	defer portalClient.Close()

	streamInfo, err := portalClient.RequestScreenCast()
	if err != nil {
		fmt.Printf("[!] Screen selection error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[*] Screen Selected! (NodeID: %d, FD: %d, Resolution: %dx%d)\n",
		streamInfo.NodeID, streamInfo.PipeWireFD, streamInfo.Width, streamInfo.Height)

	// Step 4: Resolve Clean Desktop Audio Source (Exclude Microphone & Blacklist)
	audioSource := cfg.AudioSource
	if !cfg.AudioRouting || strings.ToLower(audioSource) == "mirror" || strings.ToLower(audioSource) == "default" || strings.ToLower(audioSource) == "direct" {
		audioSource = audio.GetDesktopMonitorSource()
		fmt.Printf("[*] Audio Capture: Direct Headphone Mirror (%s) - Headphones & Apps Untouched\n", audioSource)
	} else if !cfg.IncludeMic {
		if audio.IsMicrophone(audioSource) || audioSource == "" {
			audioSource = "stream_sink.monitor"
		}
		fmt.Printf("[*] Audio Capture: Isolated Desktop Audio (%s), Microphone is DISABLED\n", audioSource)
	} else {
		fmt.Printf("[*] Audio Capture: %s (Microphone Included)\n", audioSource)
	}

	runnerOpts := pipeline.Options{
		SourceWidth:  streamInfo.Width,
		SourceHeight: streamInfo.Height,
		TargetWidth:  cfg.TargetWidth,
		TargetHeight: cfg.TargetHeight,
		Framerate:    cfg.Framerate,
		VideoBitrate: cfg.VideoBitrate,
		Encoder:      cfg.Encoder,
		CPUThreads:   cfg.CPUThreads,
		NodeID:       streamInfo.NodeID,
		PipeWireFD:   streamInfo.PipeWireFD,
		AudioSource:  audioSource,
	}

	runner := pipeline.NewRunner(runnerOpts, hub)
	if err := runner.Start(ctx); err != nil {
		fmt.Printf("[!] Failed to start capture pipeline: %v\n", err)
		os.Exit(1)
	}
	defer runner.Stop()

	fmt.Println("")
	fmt.Println("=======================================================")
	fmt.Println("  ⚡ Ultra-Low Latency WebSocket Stream Active!")
	fmt.Printf("  * Local Player URL : http://localhost:%d\n", cfg.Port)
	fmt.Println("=======================================================")
	fmt.Println("")

	select {
	case <-sigChan:
		fmt.Println("\n[!] Received shutdown signal, stopping streaming gracefully...")
	case <-ctx.Done():
	}
}
