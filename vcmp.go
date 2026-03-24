package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	pb "gob2json/proto"

	"github.com/klauspost/compress/zstd"
	"google.golang.org/protobuf/proto"
)

// AnalysisResult stores complete video analysis results
type AnalysisResult struct {
	VideoFile          string   // Path of the analyzed video file
	FPS                float64  // Video frame rate
	Width              int      // Video width in pixels
	Height             int      // Video height in pixels
	TotalFrames        int      // Total number of video frames
	SuggestedThreshold float64  // Auto-calculated suggested threshold
	DiffCounts         []uint32 // Difference pixel count for each frame
}

// toProto converts AnalysisResult to protobuf message
func (r *AnalysisResult) toProto() *pb.AnalysisResult {
	return &pb.AnalysisResult{
		VideoFile:          r.VideoFile,
		Fps:                r.FPS,
		Width:              int32(r.Width),
		Height:             int32(r.Height),
		TotalFrames:        int32(r.TotalFrames),
		SuggestedThreshold: r.SuggestedThreshold,
		DiffCounts:         r.DiffCounts,
	}
}

// fromProto populates AnalysisResult from protobuf message
func (r *AnalysisResult) fromProto(pbResult *pb.AnalysisResult) {
	r.VideoFile = pbResult.VideoFile
	r.FPS = pbResult.Fps
	r.Width = int(pbResult.Width)
	r.Height = int(pbResult.Height)
	r.TotalFrames = int(pbResult.TotalFrames)
	r.SuggestedThreshold = pbResult.SuggestedThreshold
	r.DiffCounts = pbResult.DiffCounts
}

// SaveToFile saves analysis results as a zstd-compressed protobuf file (.pb.zst)
func (r *AnalysisResult) SaveToFile(outputPath string) error {
	if r == nil {
		return errors.New("analysis result is nil")
	}

	if err := r.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	return SaveAnalysisResult(file, r)
}

// SaveAnalysisResult writes AnalysisResult to Writer with zstd compression
func SaveAnalysisResult(w io.Writer, result *AnalysisResult) error {
	if result == nil {
		return errors.New("analysis result is nil")
	}

	if err := result.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Serialize to protobuf
	data, err := proto.Marshal(result.toProto())
	if err != nil {
		return fmt.Errorf("failed to serialize protobuf: %w", err)
	}

	// Compress with zstd
	zw, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		return fmt.Errorf("failed to create zstd writer: %w", err)
	}
	defer zw.Close()

	if _, err := zw.Write(data); err != nil {
		return fmt.Errorf("failed to write compressed data: %w", err)
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("failed to close zstd writer: %w", err)
	}

	return nil
}

// SaveAnalysisResultToFile saves AnalysisResult to file with zstd compression
func SaveAnalysisResultToFile(filename string, result *AnalysisResult) error {
	if result == nil {
		return errors.New("analysis result is nil")
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	return SaveAnalysisResult(file, result)
}

// LoadFromFile loads analysis results from zstd-compressed protobuf file
func (r *AnalysisResult) LoadFromFile(filePath string) error {
	result, err := LoadAnalysisResultFromFile(filePath)
	if err != nil {
		return err
	}

	*r = *result
	return nil
}

// LoadAnalysisResultFromFile loads AnalysisResult from zstd-compressed protobuf file
func LoadAnalysisResultFromFile(filePath string) (*AnalysisResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return LoadAnalysisResult(file)
}

// LoadAnalysisResult reads AnalysisResult from Reader with zstd decompression
func LoadAnalysisResult(r io.Reader) (*AnalysisResult, error) {
	// Create zstd decompressor
	zr, err := zstd.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd reader: %w", err)
	}
	defer zr.Close()

	// Read all decompressed data
	data, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %w", err)
	}

	// Deserialize protobuf
	var pbResult pb.AnalysisResult
	if err := proto.Unmarshal(data, &pbResult); err != nil {
		return nil, fmt.Errorf("failed to deserialize protobuf: %w", err)
	}

	result := &AnalysisResult{}
	result.fromProto(&pbResult)

	if err := result.Validate(); err != nil {
		return nil, fmt.Errorf("验证失败: %w", err)
	}

	return result, nil
}

// Validate checks if AnalysisResult data is valid
func (r *AnalysisResult) Validate() error {
	if r == nil {
		return errors.New("analysis result is nil")
	}

	if r.VideoFile == "" {
		return errors.New("video file path cannot be empty")
	}

	if r.FPS <= 0 {
		return fmt.Errorf("FPS must be positive, got %f", r.FPS)
	}

	if r.Width <= 0 {
		return fmt.Errorf("width must be positive, got %d", r.Width)
	}

	if r.Height <= 0 {
		return fmt.Errorf("height must be positive, got %d", r.Height)
	}

	if r.TotalFrames < 0 {
		return fmt.Errorf("total frames cannot be negative, got %d", r.TotalFrames)
	}

	return nil
}

// NewAnalysisResult creates a new AnalysisResult
func NewAnalysisResult(videoFile string, fps float64, width, height, totalFrames int) *AnalysisResult {
	return &AnalysisResult{
		VideoFile:          videoFile,
		FPS:                fps,
		Width:              width,
		Height:             height,
		TotalFrames:        totalFrames,
		SuggestedThreshold: 0,
		DiffCounts:         make([]uint32, 0),
	}
}

// AddDiffCount appends a difference count (uint32)
func (r *AnalysisResult) AddDiffCount(count uint32) {
	r.DiffCounts = append(r.DiffCounts, count)
}

// SetDiffCounts sets the entire difference count slice
func (r *AnalysisResult) SetDiffCounts(counts []uint32) {
	r.DiffCounts = make([]uint32, len(counts))
	copy(r.DiffCounts, counts)
}

// GetDiffCountsLength returns the count of difference counts
func (r *AnalysisResult) GetDiffCountsLength() int {
	return len(r.DiffCounts)
}

// Duration returns total video duration in seconds
func (r *AnalysisResult) Duration() float64 {
	if r.FPS <= 0 {
		return 0
	}
	return float64(r.TotalFrames) / r.FPS
}

// ClearDiffCounts clears all difference counts
func (r *AnalysisResult) ClearDiffCounts() {
	r.DiffCounts = make([]uint32, 0)
}

// GetDiffCount returns the difference count at specified index, returns error if index is out of bounds
func (r *AnalysisResult) GetDiffCount(index int) (uint32, error) {
	if index < 0 || index >= len(r.DiffCounts) {
		return 0, fmt.Errorf("index %d out of bounds (length: %d)", index, len(r.DiffCounts))
	}
	return r.DiffCounts[index], nil
}
