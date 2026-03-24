package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Exclusion detection related constants
const (
	// Minimum exclusion duration in seconds, consecutive frames must reach this duration to be considered an exclusion region
	MinExclusionDurationSeconds = 20.0

	// Speed marker value for exclusion regions
	ExcludedSpeedMarker = 0.0

	// High speed value indicating exclusion in timeline
	SkipSpeedHigh = 9999.0

	// Zero speed value indicating exclusion in timeline
	SkipSpeedZero = 0.0
)

// ExclusionRegion represents a time region to be excluded
type ExclusionRegion struct {
	Start float64 // Start time in seconds
	End   float64 // End time in seconds
}

// FindExclusionRegionsFromAnalysis finds exclusion regions from analysis results
// diffThreshold: Frames with difference value exceeding this threshold are considered exclusion candidates
// minDurationSeconds: Minimum consecutive duration (default 20 seconds)
func FindExclusionRegionsFromAnalysis(result *AnalysisResult, diffThreshold uint32, minDurationSeconds float64) ([]ExclusionRegion, error) {
	if result == nil {
		return nil, fmt.Errorf("analysis result is nil")
	}

	if err := result.Validate(); err != nil {
		return nil, fmt.Errorf("invalid analysis result: %w", err)
	}

	if minDurationSeconds <= 0 {
		minDurationSeconds = MinExclusionDurationSeconds
	}

	fps := result.FPS
	if fps <= 0 {
		return nil, fmt.Errorf("invalid FPS: %f", fps)
	}

	// Minimum number of frames required for exclusion
	minFrames := int(minDurationSeconds * fps)

	var regions []ExclusionRegion
	var startFrame int = -1
	consecutiveCount := 0

	for i, diffCount := range result.DiffCounts {
		if diffCount > diffThreshold {
			if startFrame == -1 {
				startFrame = i
			}
			consecutiveCount++
		} else {
			// End of consecutive region
			if startFrame != -1 && consecutiveCount >= minFrames {
				startTime := float64(startFrame) / fps
				endTime := float64(startFrame+consecutiveCount) / fps
				regions = append(regions, ExclusionRegion{
					Start: startTime,
					End:   endTime,
				})
			}
			startFrame = -1
			consecutiveCount = 0
		}
	}

	// Handle case where exclusion region extends to the end
	if startFrame != -1 && consecutiveCount >= minFrames {
		startTime := float64(startFrame) / fps
		endTime := float64(startFrame+consecutiveCount) / fps
		regions = append(regions, ExclusionRegion{
			Start: startTime,
			End:   endTime,
		})
	}

	return regions, nil
}

// FindExclusionRegionsFromTimeline finds exclusion regions from timeline
// Segments with speed 0.0 or 9999.0 are considered exclusion regions
func FindExclusionRegionsFromTimeline(timeline *Timeline) ([]ExclusionRegion, error) {
	if timeline == nil {
		return nil, fmt.Errorf("timeline is nil")
	}

	if err := validateTimeline(timeline); err != nil {
		return nil, fmt.Errorf("invalid timeline: %w", err)
	}

	var regions []ExclusionRegion

	for _, chunk := range timeline.Chunks {
		speed := chunk.Speed()
		if speed == SkipSpeedZero || speed == SkipSpeedHigh {
			regions = append(regions, ExclusionRegion{
				Start: chunk.Start(),
				End:   chunk.End(),
			})
		}
	}

	return regions, nil
}

// FindOverlappingRegions finds overlapping parts of two sets of exclusion regions
func FindOverlappingRegions(regions1, regions2 []ExclusionRegion) []ExclusionRegion {
	var overlapping []ExclusionRegion

	for _, r1 := range regions1 {
		for _, r2 := range regions2 {
			// Check if overlapping
			overlapStart := max(r1.Start, r2.Start)
			overlapEnd := min(r1.End, r2.End)

			if overlapStart < overlapEnd {
				overlapping = append(overlapping, ExclusionRegion{
					Start: overlapStart,
					End:   overlapEnd,
				})
			}
		}
	}

	// Merge overlapping regions
	return mergeOverlappingRegions(overlapping)
}

// mergeOverlappingRegions merges overlapping or adjacent regions
func mergeOverlappingRegions(regions []ExclusionRegion) []ExclusionRegion {
	if len(regions) <= 1 {
		return regions
	}

	// Sort by start time
	for i := 0; i < len(regions)-1; i++ {
		for j := i + 1; j < len(regions); j++ {
			if regions[j].Start < regions[i].Start {
				regions[i], regions[j] = regions[j], regions[i]
			}
		}
	}

	var merged []ExclusionRegion
	current := regions[0]

	for i := 1; i < len(regions); i++ {
		if regions[i].Start <= current.End {
			// Overlapping or adjacent, extend current region
			current.End = max(current.End, regions[i].End)
		} else {
			// No overlap, save current region and start new region
			merged = append(merged, current)
			current = regions[i]
		}
	}
	merged = append(merged, current)

	return merged
}

// ApplyExclusionToTimeline applies exclusion regions to timeline
// Excluded parts have speed set to 0.0
func ApplyExclusionToTimeline(timeline *Timeline, exclusions []ExclusionRegion) (*Timeline, error) {
	if timeline == nil {
		return nil, fmt.Errorf("timeline is nil")
	}

	if err := validateTimeline(timeline); err != nil {
		return nil, fmt.Errorf("invalid timeline: %w", err)
	}

	if len(exclusions) == 0 {
		// No exclusion regions, return copy of original timeline
		newTimeline := NewTimeline(timeline.Source)
		for _, chunk := range timeline.Chunks {
			if err := newTimeline.AddChunk(chunk.Start(), chunk.End(), chunk.Speed()); err != nil {
				return nil, fmt.Errorf("failed to copy chunk: %w", err)
			}
		}
		return newTimeline, nil
	}

	// Merge and sort exclusion regions
	exclusions = mergeOverlappingRegions(exclusions)

	newTimeline := NewTimeline(timeline.Source)

	for _, chunk := range timeline.Chunks {
		chunkStart := chunk.Start()
		chunkEnd := chunk.End()
		chunkSpeed := chunk.Speed()

		// Find all exclusion regions overlapping with this chunk
		var overlappingExclusions []ExclusionRegion
		for _, excl := range exclusions {
			if excl.Start < chunkEnd && excl.End > chunkStart {
				overlappingExclusions = append(overlappingExclusions, excl)
			}
		}

		if len(overlappingExclusions) == 0 {
			// No overlapping exclusion regions, keep original chunk
			if err := newTimeline.AddChunk(chunkStart, chunkEnd, chunkSpeed); err != nil {
				return nil, fmt.Errorf("failed to add chunk: %w", err)
			}
			continue
		}

		// Split chunk based on exclusion regions
		currentPos := chunkStart
		for _, excl := range overlappingExclusions {
			exclStart := max(excl.Start, chunkStart)
			exclEnd := min(excl.End, chunkEnd)

			// Add part before exclusion region
			if currentPos < exclStart {
				if err := newTimeline.AddChunk(currentPos, exclStart, chunkSpeed); err != nil {
					return nil, fmt.Errorf("failed to add pre-exclusion chunk: %w", err)
				}
			}

			// Add exclusion part with speed set to 0.0
			if exclStart < exclEnd {
				if err := newTimeline.AddChunk(exclStart, exclEnd, ExcludedSpeedMarker); err != nil {
					return nil, fmt.Errorf("failed to add exclusion chunk: %w", err)
				}
			}

			currentPos = exclEnd
		}

		// Add remaining part after all exclusion regions
		if currentPos < chunkEnd {
			if err := newTimeline.AddChunk(currentPos, chunkEnd, chunkSpeed); err != nil {
				return nil, fmt.Errorf("failed to add post-exclusion chunk: %w", err)
			}
		}
	}

	return newTimeline, nil
}

// MergeExclusionsAndExport processes analysis results and timeline, finds overlapping exclusion regions and exports
// diffThreshold: Difference threshold
// minDurationSeconds: Minimum exclusion duration (0 means use default value of 20 seconds)
// baseFilename: Output file base name (timestamp will be appended)
func MergeExclusionsAndExport(
	analysisResult *AnalysisResult,
	timeline *Timeline,
	diffThreshold uint32,
	minDurationSeconds float64,
	baseFilename string,
) (string, error) {
	// Validate inputs
	if analysisResult == nil {
		return "", fmt.Errorf("analysis result is nil")
	}
	if timeline == nil {
		return "", fmt.Errorf("timeline is nil")
	}

	// Use default value if duration not specified
	if minDurationSeconds <= 0 {
		minDurationSeconds = MinExclusionDurationSeconds
	}

	// Find exclusion regions from analysis results
	analysisExclusions, err := FindExclusionRegionsFromAnalysis(analysisResult, diffThreshold, minDurationSeconds)
	if err != nil {
		return "", fmt.Errorf("failed to find exclusion regions from analysis: %w", err)
	}

	// Find exclusion regions from timeline
	timelineExclusions, err := FindExclusionRegionsFromTimeline(timeline)
	if err != nil {
		return "", fmt.Errorf("failed to find exclusion regions from timeline: %w", err)
	}

	// Find overlapping regions
	overlappingExclusions := FindOverlappingRegions(analysisExclusions, timelineExclusions)

	// Apply exclusion regions to timeline
	newTimeline, err := ApplyExclusionToTimeline(timeline, overlappingExclusions)
	if err != nil {
		return "", fmt.Errorf("failed to apply exclusion regions: %w", err)
	}

	// Generate output filename with timestamp
	outputFilename := generateOutputFilename(baseFilename)

	// Write to file
	if err := MarshalTimelineToFile(outputFilename, newTimeline); err != nil {
		return "", fmt.Errorf("failed to write timeline file: %w", err)
	}

	return outputFilename, nil
}

// generateOutputFilename generates output filename with timestamp
func generateOutputFilename(baseFilename string) string {
	// Remove extension
	ext := filepath.Ext(baseFilename)
	baseName := strings.TrimSuffix(baseFilename, ext)
	// Ensure .json extension
	return fmt.Sprintf("%s.json", baseName)
}

// Helper functions for Go versions before 1.21
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
