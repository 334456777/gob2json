package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// Chunk 表示时间线片段，包含开始、结束和速度值
// Start 为闭区间，End 为开区间
type Chunk [3]float64

// Start 返回片段的开始时间
func (c Chunk) Start() float64 {
	return c[0]
}

// End 返回片段的结束时间
func (c Chunk) End() float64 {
	return c[1]
}

// Speed 返回片段的播放速度
func (c Chunk) Speed() float64 {
	return c[2]
}

// Timeline 表示 v1 版本的时间线格式结构
type Timeline struct {
	Version string  `json:"version"`
	Source  string  `json:"source"`
	Chunks  []Chunk `json:"chunks"`
}

// ParseTimeline 从 Reader 解析 v1 时间线 JSON
func ParseTimeline(r io.Reader) (*Timeline, error) {
	var timeline Timeline

	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&timeline); err != nil {
		return nil, fmt.Errorf("解码 JSON 失败: %w", err)
	}

	if err := validateTimeline(&timeline); err != nil {
		return nil, fmt.Errorf("验证失败: %w", err)
	}

	return &timeline, nil
}

// ParseTimelineFromFile 从文件解析 v1 时间线 JSON
func ParseTimelineFromFile(filename string) (*Timeline, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	return ParseTimeline(file)
}

// ParseTimelineFromBytes 从字节切片解析 v1 时间线 JSON
func ParseTimelineFromBytes(data []byte) (*Timeline, error) {
	var timeline Timeline

	if err := json.Unmarshal(data, &timeline); err != nil {
		return nil, fmt.Errorf("解析 JSON 失败: %w", err)
	}

	if err := validateTimeline(&timeline); err != nil {
		return nil, fmt.Errorf("验证失败: %w", err)
	}

	return &timeline, nil
}

// MarshalTimeline 将 Timeline 转换为 JSON 并写入 Writer
func MarshalTimeline(w io.Writer, timeline *Timeline) error {
	if err := validateTimeline(timeline); err != nil {
		return fmt.Errorf("验证失败: %w", err)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(timeline); err != nil {
		return fmt.Errorf("编码 JSON 失败: %w", err)
	}

	return nil
}

// MarshalTimelineToFile 将 Timeline 转换为 JSON 并写入文件
func MarshalTimelineToFile(filename string, timeline *Timeline) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	return MarshalTimeline(file, timeline)
}

// MarshalTimelineToBytes 将 Timeline 转换为 JSON 字节切片
func MarshalTimelineToBytes(timeline *Timeline) ([]byte, error) {
	if err := validateTimeline(timeline); err != nil {
		return nil, fmt.Errorf("验证失败: %w", err)
	}

	data, err := json.MarshalIndent(timeline, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("序列化 JSON 失败: %w", err)
	}

	return data, nil
}

// validateTimeline 根据 v1 规范验证时间线结构
func validateTimeline(timeline *Timeline) error {
	if timeline == nil {
		return errors.New("时间线为空")
	}

	// Version must be "1"
	if timeline.Version != "1" {
		return fmt.Errorf("版本号无效: 期望 \"1\"，得到 %q", timeline.Version)
	}

	// Source must not be empty
	if timeline.Source == "" {
		return errors.New("源文件不能为空")
	}

	// Chunks can be empty, but if not, must follow rules
	if len(timeline.Chunks) == 0 {
		return nil
	}

	// First chunk must start at 0
	if timeline.Chunks[0].Start() != 0 {
		return fmt.Errorf("第一个片段必须从 0 开始，得到 %f", timeline.Chunks[0].Start())
	}

	// Validate each chunk
	for i, chunk := range timeline.Chunks {
		// Start must be less than end
		if chunk.Start() >= chunk.End() {
			return fmt.Errorf("片段 %d: 开始时间 (%f) 必须小于结束时间 (%f)",
				i, chunk.Start(), chunk.End())
		}

		// Speed must be between 0.0 and 99999.0 inclusive
		if chunk.Speed() < 0.0 || chunk.Speed() > 99999.0 {
			return fmt.Errorf("片段 %d: 速度 (%f) 必须在 0.0 到 99999.0 之间",
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

// NewTimeline 创建新的 Timeline
func NewTimeline(source string) *Timeline {
	return &Timeline{
		Version: "1",
		Source:  source,
		Chunks:  make([]Chunk, 0),
	}
}

// AddChunk 向时间线添加片段，违反规则时返回错误
func (t *Timeline) AddChunk(start, end, speed float64) error {
	chunk := Chunk{start, end, speed}

	// Validate the chunk would be valid
	if len(t.Chunks) == 0 {
		if start != 0 {
			return errors.New("第一个片段必须从 0 开始")
		}
	} else {
		lastChunk := t.Chunks[len(t.Chunks)-1]
		if start != lastChunk.End() {
			return fmt.Errorf("片段开始时间 (%f) 必须等于上一片段的结束时间 (%f)",
				start, lastChunk.End())
		}
	}

	if start >= end {
		return fmt.Errorf("开始时间 (%f) 必须小于结束时间 (%f)", start, end)
	}

	if speed < 0.0 || speed > 99999.0 {
		return fmt.Errorf("速度 (%f) 必须在 0.0 到 99999.0 之间", speed)
	}

	t.Chunks = append(t.Chunks, chunk)
	return nil
}

// Duration 返回时间线的总时长
func (t *Timeline) Duration() float64 {
	if len(t.Chunks) == 0 {
		return 0
	}
	return t.Chunks[len(t.Chunks)-1].End()
}
