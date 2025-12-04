package main

import (
	"compress/gzip"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
)

// AnalysisResult 视频分析结果数据结构
type AnalysisResult struct {
	VideoFile    string  // 视频文件路径
	AnalysisTime string  // 分析时间戳
	FPS          float64 // 帧率
	Width        int     // 视频宽度（像素）
	Height       int     // 视频高度（像素）
	TotalFrames  int     // 总帧数
	DiffCounts   []int32 // 帧差异计数
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

	if err := SaveAnalysisResult(file, r); err != nil {
		return err
	}

	return nil
}

// SaveAnalysisResult 将 AnalysisResult 以 gzip 压缩方式写入 Writer
func SaveAnalysisResult(w io.Writer, result *AnalysisResult) error {
	if result == nil {
		return errors.New("分析结果为空")
	}

	if err := result.Validate(); err != nil {
		return fmt.Errorf("验证失败: %w", err)
	}

	gw := gzip.NewWriter(w)
	defer gw.Close()

	encoder := gob.NewEncoder(gw)
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("编码分析结果失败: %w", err)
	}

	if err := gw.Close(); err != nil {
		return fmt.Errorf("关闭 gzip 写入器失败: %w", err)
	}

	return nil
}

// SaveAnalysisResultToFile 将 AnalysisResult 以 gzip 压缩方式保存到文件
func SaveAnalysisResultToFile(filename string, result *AnalysisResult) error {
	if result == nil {
		return errors.New("分析结果为空")
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	return SaveAnalysisResult(file, result)
}

// LoadFromGob 从 gzip 压缩的 Gob 文件加载分析结果
func (r *AnalysisResult) LoadFromGob(filePath string) error {
	result, err := LoadAnalysisFromGob(filePath)
	if err != nil {
		return err
	}

	*r = *result
	return nil
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

	if err := result.Validate(); err != nil {
		return nil, fmt.Errorf("验证失败: %w", err)
	}

	return &result, nil
}

// LoadAnalysisResultFromFile 从文件以 gzip 解压方式加载 AnalysisResult
func LoadAnalysisResultFromFile(filename string) (*AnalysisResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	return LoadAnalysisResult(file)
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

	if r.Width <= 0 {
		return fmt.Errorf("宽度必须为正数，得到 %d", r.Width)
	}

	if r.Height <= 0 {
		return fmt.Errorf("高度必须为正数，得到 %d", r.Height)
	}

	if r.TotalFrames < 0 {
		return fmt.Errorf("总帧数不能为负数，得到 %d", r.TotalFrames)
	}

	return nil
}

// NewAnalysisResult 创建新的 AnalysisResult
func NewAnalysisResult(videoFile, analysisTime string, fps float64, width, height, totalFrames int) *AnalysisResult {
	return &AnalysisResult{
		VideoFile:    videoFile,
		AnalysisTime: analysisTime,
		FPS:          fps,
		Width:        width,
		Height:       height,
		TotalFrames:  totalFrames,
		DiffCounts:   make([]int32, 0),
	}
}

// AddDiffCount 追加一个差异计数
func (r *AnalysisResult) AddDiffCount(count int32) {
	r.DiffCounts = append(r.DiffCounts, count)
}

// SetDiffCounts 设置整个差异计数切片
func (r *AnalysisResult) SetDiffCounts(counts []int32) {
	r.DiffCounts = make([]int32, len(counts))
	copy(r.DiffCounts, counts)
}

// GetDiffCountsLength 返回差异计数的数量
func (r *AnalysisResult) GetDiffCountsLength() int {
	return len(r.DiffCounts)
}

// Duration 返回视频总时长（秒）
func (r *AnalysisResult) Duration() float64 {
	if r.FPS <= 0 {
		return 0
	}
	return float64(r.TotalFrames) / r.FPS
}

// ClearDiffCounts 清空所有差异计数
func (r *AnalysisResult) ClearDiffCounts() {
	r.DiffCounts = make([]int32, 0)
}

// GetDiffCount 返回指定索引的差异计数，索引越界时返回错误
func (r *AnalysisResult) GetDiffCount(index int) (int32, error) {
	if index < 0 || index >= len(r.DiffCounts) {
		return 0, fmt.Errorf("索引 %d 越界（长度: %d）", index, len(r.DiffCounts))
	}
	return r.DiffCounts[index], nil
}
