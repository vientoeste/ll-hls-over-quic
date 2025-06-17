package media

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/vientoeste/ll-hls-over-quic/pkg/config"
)

// Transcode medias using FFMPEG
type Transcoder struct {
	cfg *config.FfmpegConfig
}

func NewTranscoder(cfg *config.FfmpegConfig) *Transcoder {
	return &Transcoder{cfg: cfg}
}

// creates LL-HLS compatible segments and parts from a source file
func (t *Transcoder) GenerateLLHLS(sourceFile, streamID string) error {
	outputDir := filepath.Join(t.cfg.Path, streamID, "1080p")
	playlistPath := filepath.Join(outputDir, "playlist.m3u8")
	initFilePath := filepath.Join(outputDir, "init.mp4")
	// check the output already exists
	if _, err := os.Stat(initFilePath); err == nil {
		log.Printf("Playlist '%s' already exists. Skipping FFmpeg transcoding.", initFilePath)
		return nil
	} else if !os.IsNotExist(err) {
		// e.g. permission error
		return fmt.Errorf("failed to check for existing playlist: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// cmd := exec.Command("ffmpeg",
	// 	"-re", "-i", sourceFile,
	// 	"-hide_banner", "-y",
	// 	"-c:v", "libx264", "-preset", "veryfast", "-g", "48", "-keyint_min", "48",
	// 	"-c:a", "aac", "-b:a", "128k",
	// 	"-s:v:0", "1920x1080", "-b:v:0", "5000k",

	// 	"-f", "hls",
	// 	"-hls_time", "2",
	// 	"-hls_playlist_type", "event", // Use event for live-style playlist
	// 	"-hls_segment_type", "fmp4",
	// 	"-hls_segment_filename", filepath.Join(outputDir, "seg_%03d.mp4"),
	// 	"-hls_fmp4_init_filename", "init.mp4",
	// 	"-hls_flags", "independent_segments+omit_endlist",

	// 	"-frag_duration", "0.5", // Key for generating parts
	// 	"-hls_fmp4_init_filename", "init.mp4",
	// 	playlistPath,
	// )
	cmd := exec.Command("ffmpeg",
		"-re", "-i", sourceFile,
		"-hide_banner", "-y",

		"-c:v", "libx264", "-preset", "veryfast", "-g", "60", "-keyint_min", "60", "-b:v", "5000k",
		"-c:a", "aac", "-b:a", "128k",
		"-sc_threshold", "0",

		"-f", "hls",
		"-hls_segment_type", "fmp4",
		"-hls_segment_filename", filepath.Join(outputDir, "seg%d.m4s"),
		"-hls_fmp4_init_filename", "init.mp4",
		"-hls_playlist_type", "event",
		"-hls_flags", "independent_segments",

		"-lhls", "1",
		"-hls_time", "0.5",
		playlistPath,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Starting LL-HLS transcoding for stream '%s'...\n", streamID)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("FFmpeg Error: %w", err)
	}

	fmt.Printf("FFmpeg transcoding for stream '%s' Done\n", streamID)
	return nil
}
