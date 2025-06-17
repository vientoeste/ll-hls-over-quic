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

func (h *Handler) servePlaylist(w http.ResponseWriter, r *http.Request, streamID, rendition string) {
	streamState := h.manager.GetOrCreateStream(streamID, rendition)
	playlistData := streamState.Playlist()

	clientMSNStr := r.URL.Query().Get("_HLS_msn")
	clientPartStr := r.URL.Query().Get("_HLS_part")

	// [TODO] improve blocking logic
	if clientMSNStr != "" && clientPartStr != "" {
		clientMSN, errMSN := strconv.Atoi(clientMSNStr)
		clientPart, errPart := strconv.Atoi(clientPartStr)
		// check if the client is up-to-date:
		// server should block only if it has NO new parts to offer the client
		isClientUpToDate := false
		if errMSN == nil && errPart == nil && playlistData.LastPart != nil {
			lastSeg := playlistData.Segments[playlistData.MediaSequence]
			if lastSeg != nil && clientMSN >= lastSeg.Sequence && clientPart >= playlistData.LastPart.Sequence {
				isClientUpToDate = true
			}
		}
		if isClientUpToDate {
			updateChan := streamState.SubscribeToUpdates()
			ctx := r.Context()
			select {
			case <-updateChan:
				log.Printf("update received for stream '%s', sending new playlist.", streamID)
			case <-time.After(5 * time.Second):
				log.Printf("blocking request for stream '%s' timed out.", streamID)
			case <-ctx.Done():
				log.Printf("client disconnected while waiting for stream '%s'.", streamID)
				return
			}
			// after waiting, get the fresh playlist data
			playlistData = streamState.Playlist()
		}
	}

	playlist, err := h.generatePlaylist(r, &playlistData)
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

// func (h *Handler) generatePlaylist(r *http.Request, state *models.StreamState) (string, error) {
// 	playlistData := state.Playlist()

// 	var sb strings.Builder

// 	// Header
// 	sb.WriteString("#EXTM3U\n")
// 	sb.WriteString("#EXT-X-VERSION:6\n")
// 	fmt.Fprintf(&sb, "#EXT-X-TARGETDURATION:%.0f\n", playlistData.TargetDuration)
// 	fmt.Fprintf(&sb, "#EXT-X-SERVER-CONTROL:CAN-BLOCK-RELOAD=YES,PART-HOLD-BACK=%.2f,CAN-SKIP-UNTIL=%.2f\n", playlistData.TargetDuration*1.2, playlistData.TargetDuration*6)
// 	fmt.Fprintf(&sb, "#EXT-X-PART-INF:PART-TARGET=%.2f\n", 0.5) // Hardcoded for now
// 	fmt.Fprintf(&sb, "#EXT-X-MEDIA-SEQUENCE:%d\n", playlistData.MediaSequence)

// 	// Body - Segments and Parts
// 	for i := playlistData.MediaSequence; i < playlistData.MediaSequence+len(playlistData.Segments); i++ {
// 		segment, ok := playlistData.Segments[i]
// 		if !ok {
// 			continue
// 		}
// 		fmt.Fprintf(&sb, "#EXTINF:%.3f,\n", segment.Duration)
// 		for _, part := range segment.Parts {
// 			independentStr := ""
// 			if part.Independent {
// 				independentStr = ",INDEPENDENT=YES"
// 			}
// 			fmt.Fprintf(&sb, "#EXT-X-PART:DURATION=%.3f,URI=\"%s\"%s\n", part.Duration, part.URI, independentStr)
// 		}
// 	}

// 	// Preload Hint - Server Push
// 	if playlistData.LastPart != nil {
// 		lastPart := playlistData.LastPart
// 		nextPartSeq := lastPart.Sequence + 1
// 		nextPartURI := strings.Replace(lastPart.URI, fmt.Sprintf(".part%d", lastPart.Sequence), fmt.Sprintf(".part%d", nextPartSeq), 1)

// 		fmt.Fprintf(&sb, "#EXT-X-PRELOAD-HINT:TYPE=PART,URI=\"%s\"\n", nextPartURI)

// 		if pusher, ok := r.Context().Value(http.ServerContextKey).(http.Pusher); ok {
// 			err := pusher.Push(nextPartURI, nil)
// 			if err != nil {
// 				log.Printf("Failed to push %s: %v", nextPartURI, err)
// 			} else {
// 				log.Printf("Pushed %s", nextPartURI)
// 			}
// 		}
// 	}

// 	return sb.String(), nil
// }

func (h *Handler) generatePlaylist(r *http.Request, playlistData *models.PlaylistData) (string, error) {
	var sb strings.Builder

	sb.WriteString("#EXTM3U\n")
	sb.WriteString("#EXT-X-VERSION:6\n")
	fmt.Fprintf(&sb, "#EXT-X-TARGETDURATION:%.0f\n", playlistData.TargetDuration)
	fmt.Fprintf(&sb, "#EXT-X-SERVER-CONTROL:CAN-BLOCK-RELOAD=YES,PART-HOLD-BACK=%.2f,CAN-SKIP-UNTIL=%.2f\n", 1.5, playlistData.TargetDuration*6)
	fmt.Fprintf(&sb, "#EXT-X-PART-INF:PART-TARGET=%.2f\n", 0.5)
	fmt.Fprintf(&sb, "#EXT-X-MEDIA-SEQUENCE:%d\n", playlistData.MediaSequence)

	// body - segments/parts: ONLY LIST a reasonable number of segments
	startSequence := playlistData.MediaSequence
	if len(playlistData.Segments) > 3 {
		startSequence = playlistData.MediaSequence - (len(playlistData.Segments) - 3)
		if startSequence < 0 {
			startSequence = 0
		}
	}

	for i := startSequence; i <= playlistData.MediaSequence; i++ {
		segment, ok := playlistData.Segments[i]
		if !ok {
			continue
		}

		if len(segment.Parts) == 0 {
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
		
		baseName := strings.TrimSuffix(lastPart.URI, ".m4s") // e.g. 1080p/seg_001.mp4.part2
		uriParts := strings.Split(baseName, ".part")
		if len(uriParts) == 2 {
			nextPartSeq := lastPart.Sequence + 1
			nextPartURI := fmt.Sprintf("%s.part%d.m4s", uriParts[0], nextPartSeq)
			fmt.Fprintf(&sb, "#EXT-X-PRELOAD-HINT:TYPE=PART,URI=\"%s\"\n", nextPartURI)

			if pusher, ok := r.Context().Value(http.ServerContextKey).(http.Pusher); ok {
				if err := pusher.Push(nextPartURI, nil); err != nil {
					log.Printf("Failed to push %s: %v", nextPartURI, err)
				}
			}
		}
	}

	return sb.String(), nil
}
