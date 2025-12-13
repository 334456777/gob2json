package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// 排除检测相关常量
const (
	// 最小排除时长（秒），连续帧需达到此时长才被视为排除区域
	MinExclusionDurationSeconds = 20.0

	// 排除区域的速度标记值
	ExcludedSpeedMarker = 0.0

	// 时间线中表示排除的高速度值
	SkipSpeedHigh = 9999.0

	// 时间线中表示排除的零速度值
	SkipSpeedZero = 0.0
)

// ExclusionRegion 表示需要排除的时间区域
type ExclusionRegion struct {
	Start float64 // 开始时间（秒）
	End   float64 // 结束时间（秒）
}

// FindExclusionRegionsFromAnalysis 从分析结果中查找排除区域
// diffThreshold: 差异值超过此阈值的帧被视为排除候选
// minDurationSeconds: 最小连续时长（默认 20 秒）
func FindExclusionRegionsFromAnalysis(result *AnalysisResult, diffThreshold uint32, minDurationSeconds float64) ([]ExclusionRegion, error) {
	if result == nil {
		return nil, fmt.Errorf("分析结果为空")
	}

	if err := result.Validate(); err != nil {
		return nil, fmt.Errorf("分析结果无效: %w", err)
	}

	if minDurationSeconds <= 0 {
		minDurationSeconds = MinExclusionDurationSeconds
	}

	fps := result.FPS
	if fps <= 0 {
		return nil, fmt.Errorf("FPS 无效: %f", fps)
	}

	// 排除所需的最小帧数
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
			// 连续区域结束
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

	// 处理排除区域延伸到末尾的情况
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

// FindExclusionRegionsFromTimeline 从时间线中查找排除区域
// 速度为 0.0 或 9999.0 的片段被视为排除区域
func FindExclusionRegionsFromTimeline(timeline *Timeline) ([]ExclusionRegion, error) {
	if timeline == nil {
		return nil, fmt.Errorf("时间线为空")
	}

	if err := validateTimeline(timeline); err != nil {
		return nil, fmt.Errorf("时间线无效: %w", err)
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

// FindOverlappingRegions 查找两组排除区域的重叠部分
func FindOverlappingRegions(regions1, regions2 []ExclusionRegion) []ExclusionRegion {
	var overlapping []ExclusionRegion

	for _, r1 := range regions1 {
		for _, r2 := range regions2 {
			// 检查是否重叠
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

	// 合并重叠区域
	return mergeOverlappingRegions(overlapping)
}

// mergeOverlappingRegions 合并重叠或相邻的区域
func mergeOverlappingRegions(regions []ExclusionRegion) []ExclusionRegion {
	if len(regions) <= 1 {
		return regions
	}

	// 按开始时间排序
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
			// 重叠或相邻，扩展当前区域
			current.End = max(current.End, regions[i].End)
		} else {
			// 无重叠，保存当前区域并开始新区域
			merged = append(merged, current)
			current = regions[i]
		}
	}
	merged = append(merged, current)

	return merged
}

// ApplyExclusionToTimeline 将排除区域应用到时间线
// 被排除的部分速度设为 0.0
func ApplyExclusionToTimeline(timeline *Timeline, exclusions []ExclusionRegion) (*Timeline, error) {
	if timeline == nil {
		return nil, fmt.Errorf("时间线为空")
	}

	if err := validateTimeline(timeline); err != nil {
		return nil, fmt.Errorf("时间线无效: %w", err)
	}

	if len(exclusions) == 0 {
		// 无排除区域，返回原时间线副本
		newTimeline := NewTimeline(timeline.Source)
		for _, chunk := range timeline.Chunks {
			if err := newTimeline.AddChunk(chunk.Start(), chunk.End(), chunk.Speed()); err != nil {
				return nil, fmt.Errorf("复制片段时失败: %w", err)
			}
		}
		return newTimeline, nil
	}

	// 合并并排序排除区域
	exclusions = mergeOverlappingRegions(exclusions)

	newTimeline := NewTimeline(timeline.Source)

	for _, chunk := range timeline.Chunks {
		chunkStart := chunk.Start()
		chunkEnd := chunk.End()
		chunkSpeed := chunk.Speed()

		// 查找与此片段重叠的所有排除区域
		var overlappingExclusions []ExclusionRegion
		for _, excl := range exclusions {
			if excl.Start < chunkEnd && excl.End > chunkStart {
				overlappingExclusions = append(overlappingExclusions, excl)
			}
		}

		if len(overlappingExclusions) == 0 {
			// 无重叠排除区域，保留原片段
			if err := newTimeline.AddChunk(chunkStart, chunkEnd, chunkSpeed); err != nil {
				return nil, fmt.Errorf("添加片段时失败: %w", err)
			}
			continue
		}

		// 根据排除区域拆分片段
		currentPos := chunkStart
		for _, excl := range overlappingExclusions {
			exclStart := max(excl.Start, chunkStart)
			exclEnd := min(excl.End, chunkEnd)

			// 添加排除区域之前的部分
			if currentPos < exclStart {
				if err := newTimeline.AddChunk(currentPos, exclStart, chunkSpeed); err != nil {
					return nil, fmt.Errorf("添加排除前片段时失败: %w", err)
				}
			}

			// 添加排除部分，速度设为 0.0
			if exclStart < exclEnd {
				if err := newTimeline.AddChunk(exclStart, exclEnd, ExcludedSpeedMarker); err != nil {
					return nil, fmt.Errorf("添加排除片段时失败: %w", err)
				}
			}

			currentPos = exclEnd
		}

		// 添加所有排除区域之后的剩余部分
		if currentPos < chunkEnd {
			if err := newTimeline.AddChunk(currentPos, chunkEnd, chunkSpeed); err != nil {
				return nil, fmt.Errorf("添加排除后片段时失败: %w", err)
			}
		}
	}

	return newTimeline, nil
}

// MergeExclusionsAndExport 处理分析结果和时间线，找出重叠的排除区域并导出
// diffThreshold: 差异阈值
// minDurationSeconds: 最小排除时长（0 表示使用默认值 20 秒）
// baseFilename: 输出文件基础名（会追加时间戳）
func MergeExclusionsAndExport(
	analysisResult *AnalysisResult,
	timeline *Timeline,
	diffThreshold uint32,
	minDurationSeconds float64,
	baseFilename string,
) (string, error) {
	// 验证输入
	if analysisResult == nil {
		return "", fmt.Errorf("分析结果为空")
	}
	if timeline == nil {
		return "", fmt.Errorf("时间线为空")
	}

	// 未指定时长则使用默认值
	if minDurationSeconds <= 0 {
		minDurationSeconds = MinExclusionDurationSeconds
	}

	// 从分析结果查找排除区域
	analysisExclusions, err := FindExclusionRegionsFromAnalysis(analysisResult, diffThreshold, minDurationSeconds)
	if err != nil {
		return "", fmt.Errorf("查找分析结果排除区域时失败: %w", err)
	}

	// 从时间线查找排除区域
	timelineExclusions, err := FindExclusionRegionsFromTimeline(timeline)
	if err != nil {
		return "", fmt.Errorf("查找时间线排除区域时失败: %w", err)
	}

	// 查找重叠区域
	overlappingExclusions := FindOverlappingRegions(analysisExclusions, timelineExclusions)

	// 应用排除区域到时间线
	newTimeline, err := ApplyExclusionToTimeline(timeline, overlappingExclusions)
	if err != nil {
		return "", fmt.Errorf("应用排除区域时失败: %w", err)
	}

	// 生成带时间戳的输出文件名
	outputFilename := generateOutputFilename(baseFilename)

	// 写入文件
	if err := MarshalTimelineToFile(outputFilename, newTimeline); err != nil {
		return "", fmt.Errorf("写入时间线文件时失败: %w", err)
	}

	return outputFilename, nil
}

// generateOutputFilename 生成带时间戳的输出文件名
func generateOutputFilename(baseFilename string) string {
	// 移除扩展名
	ext := filepath.Ext(baseFilename)
	baseName := strings.TrimSuffix(baseFilename, ext)
	// 确保 .json 扩展名
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
