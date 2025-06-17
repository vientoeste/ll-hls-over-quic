package main

import (
	"fmt"
	"log"

	"github.com/vientoeste/ll-hls-over-quic/pkg/config"
	"github.com/vientoeste/ll-hls-over-quic/pkg/hls"
	"github.com/vientoeste/ll-hls-over-quic/pkg/media"
	"github.com/vientoeste/ll-hls-over-quic/pkg/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to get configs: %v", err)
	}

	go func() {
		transcoder := media.NewTranscoder(&cfg.Ffmpeg)
		err := transcoder.GenerateLLHLS("assets/videos/source.mp4", "movie1")
		if err != nil {
			log.Printf("Failed to transcode video: %v", err)
		}
	}()

	hlsManager := hls.NewManager(&cfg.Ffmpeg)
	hlsHandler := hls.NewHandler(&cfg.Ffmpeg, hlsManager)
	httpServer := server.NewServer(&cfg.Server, hlsHandler)

	fmt.Println("Server started on port " + cfg.Server.Port)
	if err := httpServer.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
