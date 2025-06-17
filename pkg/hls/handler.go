package hls

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vientoeste/ll-hls-over-quic/internal/models"
	"github.com/vientoeste/ll-hls-over-quic/pkg/config"
)

type Handler struct {
	cfg     *config.FfmpegConfig
	manager *Manager
}

func NewHandler(cfg *config.FfmpegConfig, manager *Manager) *Handler {
	return &Handler{
		cfg:     cfg,
		manager: manager,
	}
}

func (h *Handler) ServeHLS(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	path := chi.URLParam(r, "*")

	if strings.HasSuffix(path, ".m3u8") {
		rendition := filepath.Dir(path)
		h.servePlaylist(w, r, streamID, rendition)
		return
	}

	if strings.HasSuffix(path, ".mp4") || strings.HasSuffix(path, ".m4s") {
		filePath := filepath.Join(h.cfg.Path, streamID, path)
		if !strings.HasPrefix(filePath, h.cfg.Path) {
			http.Error(w, "Invalid file path", http.StatusBadRequest)
			return
		}
		http.ServeFile(w, r, filePath)
		return
	}

	http.Error(w, "Unsupported file type", http.StatusNotFound)
}

// [TODO] playlist must be implemented
func (h *Handler) servePlaylist(w http.ResponseWriter, r *http.Request, streamID, rendition string) {
	streamState := h.manager.GetOrCreateStream(streamID, rendition)

	// [TODO] Add full support for _HLS_msn, _HLS_part to enable Delta Updates
	isBlockingRequest, _ := strconv.ParseBool(r.URL.Query().Get("_HLS_msn"))

	// [TODO] improve blocking logic
	if isBlockingRequest {
		updateChan := streamState.SubscribeToUpdates()
		ctx := r.Context()
		select {
		case <-updateChan:
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			return
		}
	}

	playlist, err := h.generatePlaylist(r, streamState)
	if err != nil {
		log.Printf("Error generating playlist for %s: %v", streamID, err)
		http.Error(w, "Failed to generate playlist", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(playlist))
}

func (h *Handler) generatePlaylist(r *http.Request, state *models.StreamState) (string, error) {
	playlistData := state.Playlist()

	var sb strings.Builder

	// Header
	sb.WriteString("#EXTM3U\n")
	sb.WriteString("#EXT-X-VERSION:6\n")
	fmt.Fprintf(&sb, "#EXT-X-TARGETDURATION:%.0f\n", playlistData.TargetDuration)
	fmt.Fprintf(&sb, "#EXT-X-SERVER-CONTROL:CAN-BLOCK-RELOAD=YES,PART-HOLD-BACK=%.2f,CAN-SKIP-UNTIL=%.2f\n", playlistData.TargetDuration*1.2, playlistData.TargetDuration*6)
	fmt.Fprintf(&sb, "#EXT-X-PART-INF:PART-TARGET=%.2f\n", 0.5) // Hardcoded for now
	fmt.Fprintf(&sb, "#EXT-X-MEDIA-SEQUENCE:%d\n", playlistData.MediaSequence)

	// Body - Segments and Parts
	for i := playlistData.MediaSequence; i < playlistData.MediaSequence+len(playlistData.Segments); i++ {
		segment, ok := playlistData.Segments[i]
		if !ok {
			continue
		}
		fmt.Fprintf(&sb, "#EXTINF:%.3f,\n", segment.Duration)
		for _, part := range segment.Parts {
			independentStr := ""
			if part.Independent {
				independentStr = ",INDEPENDENT=YES"
			}
			fmt.Fprintf(&sb, "#EXT-X-PART:DURATION=%.3f,URI=\"%s\"%s\n", part.Duration, part.URI, independentStr)
		}
	}

	// Preload Hint - Server Push
	if playlistData.LastPart != nil {
		lastPart := playlistData.LastPart
		nextPartSeq := lastPart.Sequence + 1
		nextPartURI := strings.Replace(lastPart.URI, fmt.Sprintf(".part%d", lastPart.Sequence), fmt.Sprintf(".part%d", nextPartSeq), 1)

		fmt.Fprintf(&sb, "#EXT-X-PRELOAD-HINT:TYPE=PART,URI=\"%s\"\n", nextPartURI)

		if pusher, ok := r.Context().Value(http.ServerContextKey).(http.Pusher); ok {
			err := pusher.Push(nextPartURI, nil)
			if err != nil {
				log.Printf("Failed to push %s: %v", nextPartURI, err)
			} else {
				log.Printf("Pushed %s", nextPartURI)
			}
		}
	}

	return sb.String(), nil
}
