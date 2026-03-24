package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// Chunk represents a timeline segment with start, end, and speed values
// Start is inclusive, End is exclusive
type Chunk [3]float64

// Start returns the segment start time
func (c Chunk) Start() float64 {
	return c[0]
}

// End returns the segment end time
func (c Chunk) End() float64 {
	return c[1]
}

// Speed returns the segment playback speed
func (c Chunk) Speed() float64 {
	return c[2]
}

// Timeline represents the v1 timeline format structure
type Timeline struct {
	Version string  `json:"version"`
	Source  string  `json:"source"`
	Chunks  []Chunk `json:"chunks"`
}

// ParseTimeline parses v1 timeline JSON from Reader
func ParseTimeline(r io.Reader) (*Timeline, error) {
	var timeline Timeline

	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&timeline); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	if err := validateTimeline(&timeline); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &timeline, nil
}

// ParseTimelineFromFile parses v1 timeline JSON from file
func ParseTimelineFromFile(filename string) (*Timeline, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return ParseTimeline(file)
}

// ParseTimelineFromBytes parses v1 timeline JSON from byte slice
func ParseTimelineFromBytes(data []byte) (*Timeline, error) {
	var timeline Timeline

	if err := json.Unmarshal(data, &timeline); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if err := validateTimeline(&timeline); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &timeline, nil
}

// MarshalTimeline converts Timeline to JSON and writes to Writer
func MarshalTimeline(w io.Writer, timeline *Timeline) error {
	if err := validateTimeline(timeline); err != nil {
		return fmt.Errorf("验证失败: %w", err)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(timeline); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

// MarshalTimelineToFile converts Timeline to JSON and writes to file
func MarshalTimelineToFile(filename string, timeline *Timeline) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	return MarshalTimeline(file, timeline)
}

// MarshalTimelineToBytes converts Timeline to JSON byte slice
func MarshalTimelineToBytes(timeline *Timeline) ([]byte, error) {
	if err := validateTimeline(timeline); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	data, err := json.MarshalIndent(timeline, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize JSON: %w", err)
	}

	return data, nil
}

// validateTimeline validates timeline structure according to v1 specification
func validateTimeline(timeline *Timeline) error {
	if timeline == nil {
		return errors.New("timeline is nil")
	}

	// Version must be "1"
	if timeline.Version != "1" {
		return fmt.Errorf("invalid version: expected \"1\", got %q", timeline.Version)
	}

	// Source must not be empty
	if timeline.Source == "" {
		return errors.New("source cannot be empty")
	}

	// Chunks can be empty, but if not, must follow rules
	if len(timeline.Chunks) == 0 {
		return nil
	}

	// First chunk must start at time 0
	if timeline.Chunks[0].Start() != 0 {
		return fmt.Errorf("first chunk must start at 0, got %f", timeline.Chunks[0].Start())
	}

	// Validate each chunk
	for i, chunk := range timeline.Chunks {
		// Start must be less than end
		if chunk.Start() >= chunk.End() {
			return fmt.Errorf("chunk %d: start time (%f) must be less than end time (%f)",
				i, chunk.Start(), chunk.End())
		}

		// Speed must be between 0.0 and 99999.0 inclusive
		if chunk.Speed() < 0.0 || chunk.Speed() > 99999.0 {
			return fmt.Errorf("chunk %d: speed (%f) must be between 0.0 and 99999.0",
				i, chunk.Speed())
		}

		// Check continuity (no gaps between chunks)
		if i > 0 {
			prevEnd := timeline.Chunks[i-1].End()
			if chunk.Start() != prevEnd {
				return fmt.Errorf("片段 %d: 开始时间 (%f) 必须等于上一片段的结束时间 (%f)",
					i, chunk.Start(), prevEnd)
			}
		}
	}

	return nil
}

// NewTimeline creates a new Timeline
func NewTimeline(source string) *Timeline {
	return &Timeline{
		Version: "1",
		Source:  source,
		Chunks:  make([]Chunk, 0),
	}
}

// AddChunk adds a chunk to the timeline, returns error if rules are violated
func (t *Timeline) AddChunk(start, end, speed float64) error {
	chunk := Chunk{start, end, speed}

	// Validate the chunk would be valid
	if len(t.Chunks) == 0 {
		if start != 0 {
			return errors.New("first chunk must start at 0")
		}
	} else {
		lastChunk := t.Chunks[len(t.Chunks)-1]
		if start != lastChunk.End() {
			return fmt.Errorf("chunk start time (%f) must equal previous chunk's end time (%f)",
				start, lastChunk.End())
		}
	}

	if start >= end {
		return fmt.Errorf("start time (%f) must be less than end time (%f)", start, end)
	}

	if speed < 0.0 || speed > 99999.0 {
		return fmt.Errorf("speed (%f) must be between 0.0 and 99999.0", speed)
	}

	t.Chunks = append(t.Chunks, chunk)
	return nil
}

// Duration returns the total duration of the timeline
func (t *Timeline) Duration() float64 {
	if len(t.Chunks) == 0 {
		return 0
	}
	return t.Chunks[len(t.Chunks)-1].End()
}
