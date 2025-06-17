package server

import (
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/quic-go/quic-go/http3"
	"github.com/vientoeste/ll-hls-over-quic/pkg/config"
	"github.com/vientoeste/ll-hls-over-quic/pkg/hls"
)

type Server struct {
	cfg        *config.ServerConfig
	hlsHandler *hls.Handler
}

func NewServer(cfg *config.ServerConfig, hlsHandler *hls.Handler) *Server {
	return &Server{
		cfg:        cfg,
		hlsHandler: hlsHandler,
	}
}

func (s *Server) Start() error {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Alt-Svc", `h3=":443"; ma=2592000`)
			// [TODO] use proper cors in prod
			// w.Header().Set("Access-Control-Allow-Origin", "*")
			h.ServeHTTP(w, r)
		})
	})

	// e.g. /live/someVideoId/1080p/playlist.m3u8
	//              fileName: ^                 ^
	router.Get("/live/{streamID}/*", s.hlsHandler.ServeHLS)

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("assets", "index.html"))
	})

	addr := s.cfg.Port

	certFile := "fullchain.pem"
	keyFile := "privkey.pem"

	if s.cfg.HTTPVersion == 3 {
		fmt.Printf("Starting QUIC (HTTP/3) Server at %s\n", addr)
		return http3.ListenAndServeTLS(addr, certFile, keyFile, router)
	}

	fmt.Printf("Starting SPDY (HTTP/2) Server at %s\n", addr)
	return http.ListenAndServeTLS(addr, certFile, keyFile, router)
}
