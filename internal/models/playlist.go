package models

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Part struct {
	URI         string
	Duration    float64
	Independent bool
	Sequence    int
}

type Segment struct {
	Sequence  int
	URI       string
	Duration  float64
	Parts     []*Part
	IsPartial bool
}

type PlaylistData struct {
	MediaSequence  int
	TargetDuration float64
	Segments       map[int]*Segment
	LastPart       *Part
}

type StreamState struct {
	mu sync.RWMutex

	StreamID       string
	MediaSequence  int
	PartSequence   int
	TargetDuration float64
	Segments       map[int]*Segment
	IsLive         bool
	rendition      string

	updateChan chan struct{}
}

func NewStreamState(streamID, rendition string) *StreamState {
	return &StreamState{
		StreamID:       streamID,
		TargetDuration: 2.0,
		Segments:       make(map[int]*Segment),
		IsLive:         true,
		rendition:      rendition,
		updateChan:     make(chan struct{}),
	}
}

func (s *StreamState) AddPart(fileName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// e.g., seg_001.mp4.part2.m4s
	base := strings.TrimSuffix(fileName, ".m4s")
	parts := strings.Split(base, ".part")
	if len(parts) != 2 {
		return fmt.Errorf("invalid part filename format: %s", fileName)
	}

	segStr := strings.TrimPrefix(parts[0], "seg_")
	segBaseName := parts[0]
	segSeq, err := strconv.Atoi(strings.TrimLeft(segStr, "0"))
	if err != nil {
		return fmt.Errorf("could not parse segment sequence: %s", fileName)
	}

	partSeq, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("could not parse part sequence: %s", fileName)
	}

	if _, ok := s.Segments[segSeq]; !ok {
		s.Segments[segSeq] = &Segment{
			Sequence:  segSeq,
			URI:       segBaseName,
			Duration:  2.0,
			Parts:     make([]*Part, 0),
			IsPartial: true,
		}
		if segSeq > s.MediaSequence {
			s.MediaSequence = segSeq
		}
	}

	newPart := &Part{
		URI:         filepath.Join(s.rendition, fileName),
		Duration:    0.5,
		Independent: partSeq == 0,
		Sequence:    partSeq,
	}

	s.Segments[segSeq].Parts = append(s.Segments[segSeq].Parts, newPart)
	s.PartSequence = partSeq

	fmt.Printf("Added part to stream '%s': Seg %d, Part %d\n", s.StreamID, segSeq, partSeq)
	return nil
}

func (s *StreamState) Playlist() PlaylistData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// deep copy segments and parts to avoid race conditions after the lock is released
	segmentsCopy := make(map[int]*Segment)
	var lastPart *Part
	for seq, seg := range s.Segments {
		partsCopy := make([]*Part, len(seg.Parts))
		for i, part := range seg.Parts {
			partCopy := *part // copy part
			partsCopy[i] = &partCopy
			lastPart = &partCopy
		}
		segmentsCopy[seq] = &Segment{
			Sequence:  seg.Sequence,
			Duration:  seg.Duration,
			Parts:     partsCopy,
			IsPartial: seg.IsPartial,
		}
	}

	return PlaylistData{
		MediaSequence:  s.MediaSequence,
		TargetDuration: s.TargetDuration,
		Segments:       segmentsCopy,
		LastPart:       lastPart,
	}
}

func (s *StreamState) NotifyUpdates() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.updateChan != nil {
		close(s.updateChan)
	}
	s.updateChan = make(chan struct{})
}

func (s *StreamState) SubscribeToUpdates() <-chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updateChan
}
