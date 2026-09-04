package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/luigieli/streaming/client/pkg/audio"
	"github.com/luigieli/streaming/client/pkg/pipeline"
	"github.com/luigieli/streaming/client/pkg/portal"
	"github.com/luigieli/streaming/utils/config"
	"github.com/luigieli/streaming/utils/types"
)

func main() {
	serverURL := config.GetEnvString("SERVER_URL", "http://localhost:8080/api/publish")
	streamKey := config.GetEnvString("STREAM_KEY", "")
	encoder := strings.ToLower(config.GetEnvString("ENCODER", "gpu"))
	framerate := config.GetEnvInt("FRAMERATE", 60)
	bitrate := config.ParseBitrate(config.GetEnvString("VIDEO_BITRATE", "6000"))
	cpuThreads := config.GetEnvInt("CPU_THREADS", 4)
	includeMic := config.GetEnvBool("INCLUDE_MIC", false)

	defaultBlacklist := "discord,Discord,vesktop,webcord,slack,zoom,teams"
	blacklist := config.ParseList(config.GetEnvString("AUDIO_BLACKLIST", defaultBlacklist))

	audioSource := config.GetEnvString("AUDIO_SOURCE", "stream_sink.monitor")
	audioRouting := config.GetEnvBool("AUDIO_ROUTING", true)
	if strings.ToLower(audioSource) == "mirror" || strings.ToLower(audioSource) == "default" || strings.ToLower(audioSource) == "direct" {
		audioRouting = false
		audioSource = audio.GetDesktopMonitorSource()
	}

	fmt.Println("==================================================")
	fmt.Println("   ⚡ Streaming Client (Wayland Capture & Sender)")
	fmt.Println("==================================================")
	fmt.Printf("--> Target Server : %s\n", serverURL)
	fmt.Printf("--> Video Encoder : %s\n", encoder)
	fmt.Printf("--> Framerate     : %d fps\n", framerate)
	fmt.Printf("--> Bitrate       : %d kbps\n", bitrate)
	fmt.Printf("--> Audio Mode    : Routing=%v, Source=%s\n", audioRouting, audioSource)
	fmt.Println("==================================================")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Audio Routing
	audioFilter := audio.NewFilter(blacklist, includeMic)
	audioRouter := audio.NewRouter(audioFilter, audioRouting)
	if err := audioRouter.Start(ctx); err != nil {
		fmt.Printf("[!] Audio router warning: %v\n", err)
	}
	defer audioRouter.Stop()

	// 2. Request Wayland Screencast via Portal
	portalClient, err := portal.NewClient()
	if err != nil {
		fmt.Printf("[!] Failed to connect to desktop portal: %v\n", err)
		os.Exit(1)
	}
	defer portalClient.Close()

	streamInfo, err := portalClient.RequestScreenCast()
	if err != nil {
		fmt.Printf("[!] Failed to negotiate screencast: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[+] Screencast session acquired: NodeID=%d, Resolution=%dx%d\n", streamInfo.NodeID, streamInfo.Width, streamInfo.Height)

	// 3. Sender & Pipeline
	sender := pipeline.NewSender(serverURL, streamKey)
	opts := types.CaptureOptions{
		Width:        streamInfo.Width,
		Height:       streamInfo.Height,
		Framerate:    framerate,
		VideoBitrate: bitrate,
		Encoder:      encoder,
		CPUThreads:   cpuThreads,
		NodeID:       streamInfo.NodeID,
		PipeWireFD:   streamInfo.PipeWireFD,
		AudioSource:  audioSource,
	}

	runner := pipeline.NewRunner(opts, sender)
	streamReader, err := runner.Start(ctx)
	if err != nil {
		fmt.Printf("[!] Failed to start capture pipeline: %v\n", err)
		os.Exit(1)
	}
	defer runner.Stop()

	fmt.Println("[+] Streaming capture started. Sending stream to server...")

	// 4. Stream data to server in background
	go func() {
		if sendErr := sender.Send(ctx, streamReader); sendErr != nil {
			fmt.Printf("[!] Stream transmission ended: %v\n", sendErr)
			cancel()
		}
	}()

	select {
	case <-sigChan:
		fmt.Println("\n[*] Received interrupt. Shutting down capture client...")
	case <-ctx.Done():
		fmt.Println("\n[*] Stream context canceled. Shutting down...")
	}
}
