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

// toProto 将 AnalysisResult 转换为 protobuf 消息
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

// fromProto 从 protobuf 消息填充 AnalysisResult
func (r *AnalysisResult) fromProto(pbResult *pb.AnalysisResult) {
	r.VideoFile = pbResult.VideoFile
	r.FPS = pbResult.Fps
	r.Width = int(pbResult.Width)
	r.Height = int(pbResult.Height)
	r.TotalFrames = int(pbResult.TotalFrames)
	r.SuggestedThreshold = pbResult.SuggestedThreshold
	r.DiffCounts = pbResult.DiffCounts
}

// SaveToFile 将分析结果保存为 zstd 压缩的 protobuf 文件 (.pb.zst)
func (r *AnalysisResult) SaveToFile(outputPath string) error {
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

// SaveAnalysisResult 将 AnalysisResult 以 zstd 压缩方式写入 Writer
func SaveAnalysisResult(w io.Writer, result *AnalysisResult) error {
	if result == nil {
		return errors.New("分析结果为空")
	}

	if err := result.Validate(); err != nil {
		return fmt.Errorf("验证失败: %w", err)
	}

	// 序列化为 protobuf
	data, err := proto.Marshal(result.toProto())
	if err != nil {
		return fmt.Errorf("序列化 protobuf 失败: %w", err)
	}

	// 使用 zstd 压缩
	zw, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.SpeedBestCompression))
	if err != nil {
		return fmt.Errorf("创建 zstd 写入器失败: %w", err)
	}
	defer zw.Close()

	if _, err := zw.Write(data); err != nil {
		return fmt.Errorf("写入压缩数据失败: %w", err)
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("关闭 zstd 写入器失败: %w", err)
	}

	return nil
}

// SaveAnalysisResultToFile 将 AnalysisResult 以 zstd 压缩方式保存到文件
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

// LoadFromFile 从 zstd 压缩的 protobuf 文件加载分析结果
func (r *AnalysisResult) LoadFromFile(filePath string) error {
	result, err := LoadAnalysisResultFromFile(filePath)
	if err != nil {
		return err
	}

	*r = *result
	return nil
}

// LoadAnalysisResultFromFile 从 zstd 压缩的 protobuf 文件加载 AnalysisResult
func LoadAnalysisResultFromFile(filePath string) (*AnalysisResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	return LoadAnalysisResult(file)
}

// LoadAnalysisResult 从 Reader 以 zstd 解压方式读取 AnalysisResult
func LoadAnalysisResult(r io.Reader) (*AnalysisResult, error) {
	// 创建 zstd 解压器
	zr, err := zstd.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("创建 zstd 读取器失败: %w", err)
	}
	defer zr.Close()

	// 读取所有解压后的数据
	data, err := io.ReadAll(zr)
	if err != nil {
		return nil, fmt.Errorf("读取压缩数据失败: %w", err)
	}

	// 反序列化 protobuf
	var pbResult pb.AnalysisResult
	if err := proto.Unmarshal(data, &pbResult); err != nil {
		return nil, fmt.Errorf("反序列化 protobuf 失败: %w", err)
	}

	result := &AnalysisResult{}
	result.fromProto(&pbResult)

	if err := result.Validate(); err != nil {
		return nil, fmt.Errorf("验证失败: %w", err)
	}

	return result, nil
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

// SetDiffCounts 设置整个差异计数切片
func (r *AnalysisResult) SetDiffCounts(counts []uint32) {
	r.DiffCounts = make([]uint32, len(counts))
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
	r.DiffCounts = make([]uint32, 0)
}

// GetDiffCount 返回指定索引的差异计数，索引越界时返回错误
func (r *AnalysisResult) GetDiffCount(index int) (uint32, error) {
	if index < 0 || index >= len(r.DiffCounts) {
		return 0, fmt.Errorf("索引 %d 越界(长度: %d)", index, len(r.DiffCounts))
	}
	return r.DiffCounts[index], nil
}
