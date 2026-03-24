# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gob2json is a video silence and static interval detection tool that performs **intersection operations** on two independent analysis results:
- **auto-editor silence segments** (`autoeditor.json`) - Audio silence intervals
- **vcmp static frame segments** (`.pb.zst`) - Video static frame intervals (Protocol Buffers + Zstandard compression)

Outputs auto-editor v1 timeline JSON with intersection intervals marked as speed `0.0` (excluded).

## Build Commands

```bash
# Must run after modifying .proto files
make proto

# Build binary
make build

# Install to system path (requires sudo)
make install

# Clean build files
make clean
```

**Important**: After modifying `proto/analysis.proto`, you must run `make proto` to regenerate code.

## Code Architecture

Four core modules, each with a single responsibility:

- **main.go** - Entry point and parameter handling
  - Threshold priority: Command line arguments > `.pb.zst` `SuggestedThreshold`
  - Auto-find `.pb.zst` and `.json` files in working directory
  - Parameters: `[threshold] [minDuration] [output_base]`

- **vcmp.go** - Video analysis result I/O
  - `AnalysisResult` struct: Video metadata + frame difference count array
  - Protocol Buffers serialization + Zstandard compression
  - Provides `Validate()` method

- **autoeditor.go** - Timeline JSON parsing/generation
  - `Timeline`/`Chunk` structs follow v1 specification
  - Strict validation: version number, time continuity, speed range
  - `validateTimeline()` ensures data integrity

- **merge.go** - Core algorithms
  - `FindExclusionRegionsFromAnalysis()` - Detect static intervals from frame difference data
  - `FindExclusionRegionsFromTimeline()` - Extract silence intervals from timeline
  - `FindOverlappingRegions()` - Calculate intersection of two interval sets
  - `ApplyExclusionToTimeline()` - Apply exclusion regions to timeline (split chunks)

## Data Flow

```
.pb.zst (AnalysisResult) ──┐
                            ├─→ MergeExclusionsAndExport() ─→ output.json
autoeditor.json (Timeline) ─┘
```

## Core Algorithms

**Interval Detection** (`merge.go:30-90`):
- Consecutive frames where difference value > `diffThreshold`
- Must reach `minFrames` (`minDuration * fps`) to form an interval

**Intersection Calculation** (`merge.go:118-139`):
- Iterate through two interval sets to find overlaps: `max(start1, start2)` to `min(end1, end2)`
- Merge adjacent or overlapping intervals

**Apply Exclusion** (`merge.go:174-254`):
- Split timeline chunks into three parts: pre-exclusion, exclusion (speed 0.0), post-exclusion

## Key Constants (merge.go)

```go
MinExclusionDurationSeconds = 20.0   // Minimum exclusion duration (seconds)
ExcludedSpeedMarker = 0.0             // Exclusion speed marker
SkipSpeedHigh = 9999.0                // High speed value for timeline exclusion
SkipSpeedZero = 0.0                   // Zero speed value for timeline exclusion
```

## Timeline Validation Rules (autoeditor.go:120-171)

Strict v1 specification:
1. Version must be "1"
2. First chunk must start at time 0
3. No gaps between chunks (continuity)
4. `Start < End`, speed range `[0.0, 99999.0]`

## Threshold Mechanism

- **Low threshold**: Strict, treats minor frame changes as non-static
- **High threshold**: Lenient, tolerates certain frame changes
- **Auto-suggested value**: Stored in `.pb.zst` `SuggestedThreshold` field

## Protocol Buffers Integration

- Definition: `proto/analysis.proto`
- Generated: `proto/analysis.pb.go` (via `make proto`)
- `diff_counts` uses `packed = true` for storage optimization
