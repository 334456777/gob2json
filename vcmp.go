package main

import (
	"compress/gzip"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
)

// AnalysisResult 保存视频分析的完整结果
type AnalysisResult struct {
	VideoFile          string   // 被分析的视频文件路径
	FPS                float64  // 视频帧率
	Width              int      // 视频宽度（像素）
	Height             int      // 视频高度（像素）
	TotalFrames        int      // 视频总帧数
	SuggestedThreshold float64  // 自动计算的建议阈值
	DiffCounts         []uint32 // 每一帧的差异像素数量
}

// SaveToGob 将分析结果保存为 gzip 压缩的 Gob 文件
func (r *AnalysisResult) SaveToGob(outputPath string) error {
	if r == nil {
		return errors.New("分析结果为空")
	}

	if err := r.Validate(); err != nil {
		return fmt.Errorf("验证失败: %w", err)
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	return SaveAnalysisResult(file, r)
}

// SaveAnalysisResult 将 AnalysisResult 以 gzip 压缩方式写入 Writer
func SaveAnalysisResult(w io.Writer, result *AnalysisResult) error {
	if result == nil {
		return errors.New("分析结果为空")
	}

	gw := gzip.NewWriter(w)
	defer gw.Close()

	encoder := gob.NewEncoder(gw)
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("编码分析结果失败: %w", err)
	}

	return gw.Close()
}

// LoadAnalysisFromGob 从 gzip 压缩的 Gob 文件加载 AnalysisResult
func LoadAnalysisFromGob(filePath string) (*AnalysisResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	return LoadAnalysisResult(file)
}

// LoadAnalysisResult 从 Reader 以 gzip 解压方式读取 AnalysisResult
func LoadAnalysisResult(r io.Reader) (*AnalysisResult, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("创建 gzip 读取器失败: %w", err)
	}
	defer gr.Close()

	var result AnalysisResult
	decoder := gob.NewDecoder(gr)
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("解码分析结果失败: %w", err)
	}

	return &result, nil
}

// Validate 检查 AnalysisResult 数据是否有效
func (r *AnalysisResult) Validate() error {
	if r == nil {
		return errors.New("分析结果为空")
	}

	if r.VideoFile == "" {
		return errors.New("视频文件路径不能为空")
	}

	if r.FPS <= 0 {
		return fmt.Errorf("FPS 必须为正数，得到 %f", r.FPS)
	}
	
	if r.TotalFrames < 0 {
		return fmt.Errorf("总帧数不能为负数，得到 %d", r.TotalFrames)
	}

	return nil
}

// NewAnalysisResult 创建新的 AnalysisResult
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

// AddDiffCount 追加一个差异计数 (uint32)
func (r *AnalysisResult) AddDiffCount(count uint32) {
	r.DiffCounts = append(r.DiffCounts, count)
}

// GetDiffCount 返回指定索引的差异计数 (uint32)
func (r *AnalysisResult) GetDiffCount(index int) (uint32, error) {
	if index < 0 || index >= len(r.DiffCounts) {
		return 0, fmt.Errorf("索引 %d 越界", index)
	}
	return r.DiffCounts[index], nil
}
