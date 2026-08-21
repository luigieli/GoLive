package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/luigieli/streaming/webrtc/pkg/audio"
	"github.com/luigieli/streaming/webrtc/pkg/config"
	"github.com/luigieli/streaming/webrtc/pkg/pipeline"
	"github.com/luigieli/streaming/webrtc/pkg/portal"
	"github.com/luigieli/streaming/webrtc/pkg/server"
	webrtcPkg "github.com/luigieli/streaming/webrtc/pkg/webrtc"
)

func main() {
	cfg := config.Load()

	fmt.Println("=======================================================")
	fmt.Println("   ⚡ Go Wayland WebRTC Streamer (<150ms Direct UDP)")
	fmt.Println("=======================================================")
	fmt.Printf("[*] Configuration: Port=%d, Framerate=%d, Bitrate=%dk\n",
		cfg.Port, cfg.Framerate, cfg.VideoBitrate)
	fmt.Printf("[*] Audio Filter: Blacklisting=%v, IncludeMic=%v\n",
		cfg.AudioBlacklist, cfg.IncludeMic)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Step 1: Initialize WebRTC Broadcaster
	broadcaster, err := webrtcPkg.NewBroadcaster(cfg.ICEServers)
	if err != nil {
		fmt.Printf("[!] Failed to initialize WebRTC broadcaster: %v\n", err)
		os.Exit(1)
	}
	defer broadcaster.Close()

	webDir := os.Getenv("WEB_DIR")
	if webDir == "" {
		webDir = "web"
	}
	srv := server.NewServer(webDir, broadcaster)
	go func() {
		fmt.Printf("[*] Starting WebRTC HTTP Signaling & Player Server on :%d...\n", cfg.Port)
		if err := srv.ListenAndServe(cfg.Port); err != nil {
			fmt.Printf("[!] Server exited: %v\n", err)
		}
	}()

	// Step 2: Start Audio Router & Blacklist Filter
	audioFilter := audio.NewFilter(cfg.AudioBlacklist, cfg.IncludeMic)
	audioRouter := audio.NewRouter(audioFilter)
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

	// Step 4: Resolve Clean Desktop Audio Source
	audioSource := cfg.AudioSource
	if !cfg.IncludeMic {
		if audio.IsMicrophone(audioSource) || audioSource == "default" || audioSource == "" {
			audioSource = "stream_sink.monitor"
		}
		fmt.Printf("[*] Audio Capture: Isolated Desktop Audio (%s), Microphone is DISABLED\n", audioSource)
	} else {
		fmt.Printf("[*] Audio Capture: %s (Microphone Included)\n", audioSource)
	}

	runnerOpts := pipeline.Options{
		Width:        streamInfo.Width,
		Height:       streamInfo.Height,
		Framerate:    cfg.Framerate,
		VideoBitrate: cfg.VideoBitrate,
		NodeID:       streamInfo.NodeID,
		PipeWireFD:   streamInfo.PipeWireFD,
		AudioSource:  audioSource,
	}

	runner := pipeline.NewRunner(runnerOpts, broadcaster)
	if err := runner.Start(ctx); err != nil {
		fmt.Printf("[!] Failed to start capture pipeline: %v\n", err)
		os.Exit(1)
	}
	defer runner.Stop()

	fmt.Println("")
	fmt.Println("=======================================================")
	fmt.Println("  ⚡ WebRTC Stream Active!")
	fmt.Printf("  * Local Player URL : http://localhost:%d\n", cfg.Port)
	fmt.Println("=======================================================")
	fmt.Println("")

	select {
	case <-sigChan:
		fmt.Println("\n[!] Received shutdown signal, stopping streaming gracefully...")
	case <-ctx.Done():
	}
}
