package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/quic-go/quic-go/http3"
)

const (
	ffmpegOutputDir = "./ffmpeg_output"
	serverAddr      = "0.0.0.0:3000"
	certFile        = "localhost+2.pem" // [TODO] 추후 환경 변수로 대체
	keyFile         = "localhost+2-key.pem"
)

func middlewareContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filePath := r.URL.Path

		if strings.HasSuffix(filePath, ".m3u8") {
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		} else if strings.HasSuffix(filePath, ".mpd") {
			w.Header().Set("Content-Type", "application/dash+xml")
		} else if strings.HasSuffix(filePath, ".m4s") || strings.HasSuffix(filePath, ".mp4") {
			w.Header().Set("Content-Type", "video/mp4")
		} else if strings.HasSuffix(filePath, ".ts") {
			w.Header().Set("Content-Type", "video/MP2T")
		}

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")

		// for OPTIONS req
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	if _, err := os.Stat(ffmpegOutputDir); os.IsNotExist(err) {
		log.Fatalf("FFmpeg output directory '%s' does not exist. Please create it and ensure FFmpeg writes files there.", ffmpegOutputDir)
	}

	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir(ffmpegOutputDir))
	mux.Handle("/video/", http.StripPrefix("/video/", middlewareContentType(fileServer)))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Print("dddd")
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("QUIC/HTTP/3 LL-HLS Server is running!"))
	})

	log.Printf("Starting QUIC/HTTP/3 server on %s", serverAddr)
	log.Printf("Serving LL-HLS content from: %s", ffmpegOutputDir)

	server := &http3.Server{
		Addr:    serverAddr,
		Handler: mux,
	}

	err := server.ListenAndServeTLS(certFile, keyFile)
	if err != nil {
		log.Fatalf("Failed to listen and serve: %v", err)
	}
}
