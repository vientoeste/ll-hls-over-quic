package hls

import (
	"log"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/vientoeste/ll-hls-over-quic/internal/models"
	"github.com/vientoeste/ll-hls-over-quic/pkg/config"
)

type Manager struct {
	mu      sync.RWMutex
	streams map[string]*models.StreamState
	cfg     *config.FfmpegConfig
}

func NewManager(cfg *config.FfmpegConfig) *Manager {
	return &Manager{
		streams: make(map[string]*models.StreamState),
		cfg:     cfg,
	}
}

func (m *Manager) GetOrCreateStream(streamID, rendition string) *models.StreamState {
	m.mu.RLock()
	state, ok := m.streams[streamID+"_"+rendition]
	m.mu.RUnlock()

	if ok {
		return state
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	mapKey := streamID + "_" + rendition
	if state, ok = m.streams[mapKey]; ok {
		return state
	}

	newState := models.NewStreamState(streamID, rendition)
	m.streams[mapKey] = newState

	go m.watchStreamDirectory(newState)

	return newState
}

func (m *Manager) watchStreamDirectory(state *models.StreamState) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Error creating watcher for %s: %v", state.StreamID, err)
		return
	}
	defer watcher.Close()

	// [TODO] update rendition dynamically
	streamDir := filepath.Join(m.cfg.Path, state.StreamID, "1080p")

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// only care about new files being created
				if event.Op&fsnotify.Create == fsnotify.Create {
					if strings.HasSuffix(event.Name, ".m4s") {
						fileName := filepath.Base(event.Name)
						if err := state.AddPart(fileName); err != nil {
							log.Printf("Error adding part: %v", err)
						} else {
							state.NotifyUpdates()
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("watcher error:", err)
			}
		}
	}()

	err = watcher.Add(streamDir)
	if err != nil {
		log.Printf("Error adding directory to watcher for %s: %v", streamDir, err)
	}
	<-done
}
