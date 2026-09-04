package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/luigieli/streaming/server/pkg/http"
	"github.com/luigieli/streaming/server/pkg/hub"
	"github.com/luigieli/streaming/server/pkg/ingest"
	"github.com/luigieli/streaming/utils/config"
)

func main() {
	port := config.GetEnvInt("PORT", 8080)
	streamKey := config.GetEnvString("STREAM_KEY", "")
	stunServers := config.ParseList(config.GetEnvString("STUN_SERVERS", "stun:stun.l.google.com:19302,stun:stun1.l.google.com:19302"))
	natIPs := config.ParseList(config.GetEnvString("NAT_1TO1_IPS", ""))
	webDir := config.GetEnvString("WEB_DIR", "./web")

	fmt.Println("==================================================")
	fmt.Println("   ⚡ Streaming Server (Distribution Hub & Ingest)")
	fmt.Println("==================================================")
	fmt.Printf("--> Port          : %d\n", port)
	if streamKey != "" {
		fmt.Println("--> Stream Key    : [Configured]")
	} else {
		fmt.Println("--> Stream Key    : [Public - No Key Required]")
	}
	fmt.Printf("--> Web Assets    : %s\n", webDir)
	fmt.Printf("--> Ingest URL    : http://localhost:%d/api/publish\n", port)
	fmt.Printf("--> OBS WHIP Ingest: http://localhost:%d/whip\n", port)
	fmt.Printf("--> Viewer URL    : http://localhost:%d\n", port)
	fmt.Println("==================================================")

	wsHub := hub.NewWSHub()
	go wsHub.Run()

	rtcHub, err := hub.NewWebRTCHub(stunServers, natIPs)
	if err != nil {
		fmt.Printf("[!] Warning: WebRTC hub initialization failed: %v\n", err)
	}

	httpIngest := ingest.NewHTTPHandler(wsHub, streamKey)
	server := http.NewServer(port, wsHub, rtcHub, httpIngest, webDir)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := server.Start(); err != nil {
			fmt.Printf("[!] Server exited: %v\n", err)
		}
	}()

	<-sigChan
	fmt.Println("\n[*] Shutting down Streaming Server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = server.Stop(ctx)
	if rtcHub != nil {
		rtcHub.Close()
	}
	fmt.Println("[*] Server shutdown complete.")
}
