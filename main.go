package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// main 是程序的入口函数，负责解析命令行参数、加载数据文件并执行音频排除区间的合并导出操作。
//
// 命令行参数:
//   - threshold (必需): 差异阈值，必须是正整数，用于判断音频片段是否需要排除
//   - minDuration (可选): 最小排除时长（秒），必须是正数，默认使用 MinExclusionDurationSeconds
//   - output_base (可选): 输出文件的基础名称，默认使用输入的 JSON 文件名
//
// 程序会自动在当前工作目录中查找:
//   - 一个 .gob 文件：包含音频分析结果
//   - 一个 .json 文件：包含时间线数据（优先使用 autoeditor.json）
//
// 使用示例:
//   program 30                    // 使用阈值 30，其他参数使用默认值
//   program 30 2.5                // 使用阈值 30，最小时长 2.5 秒
//   program 30 2.5 output         // 指定输出基础名称为 output
//
// 退出状态码:
//   - 0: 成功执行
//   - 1: 发生错误（参数错误、文件未找到、处理失败等）
func main() {
	// 检查最少参数
	if len(os.Args) < 2 {
		printError("缺少 threshold 参数")
		printUsage()
		os.Exit(1)
	}

	// 解析 threshold（第一个参数，必需）
	diffThreshold, err := strconv.Atoi(os.Args[1])
	if err != nil || diffThreshold <= 0 {
		printError(fmt.Sprintf("threshold 必须是正整数: %s", os.Args[1]))
		printUsage()
		os.Exit(1)
	}

	// 解析 minDuration（第二个参数，可选）
	minDuration := MinExclusionDurationSeconds
	if len(os.Args) >= 3 {
		parsedDuration, err := strconv.ParseFloat(os.Args[2], 64)
		if err != nil || parsedDuration <= 0 {
			printError(fmt.Sprintf("minDuration 必须是正数: %s", os.Args[2]))
			printUsage()
			os.Exit(1)
		}
		minDuration = parsedDuration
	}

	// 解析 output base（第三个参数，可选）
	var outputBase string
	if len(os.Args) >= 4 {
		outputBase = os.Args[3]
	}

	// 获取当前工作目录
	workDir, err := os.Getwd()
	if err != nil {
		printError(fmt.Sprintf("获取工作目录时失败: %v", err))
		os.Exit(1)
	}

	// 自动查找 .gob 文件
	gobFile, err := findFileByExtension(workDir, ".gob")
	if err != nil {
		printError(fmt.Sprintf("%v", err))
		os.Exit(1)
	}

	// 自动查找 .json 文件
	jsonFile, err := findFileByExtension(workDir, ".json")
	if err != nil {
		printError(fmt.Sprintf("%v", err))
		os.Exit(1)
	}

	// 未指定输出基础名则使用 json 文件名
	if outputBase == "" {
		outputBase = jsonFile
	}

	// 加载视频分析结果
	fmt.Printf(">> 加载视频分析结果: %s\n", filepath.Base(gobFile))
	analysisResult, err := LoadAnalysisFromGob(gobFile)
	if err != nil {
		printError(fmt.Sprintf("加载 gob 文件时失败: %v", err))
		os.Exit(1)
	}

	// 加载音频分析结果
	fmt.Printf(">> 加载音频分析结果: %s\n", filepath.Base(jsonFile))
	timeline, err := ParseTimelineFromFile(jsonFile)
	if err != nil {
		printError(fmt.Sprintf("加载 json 文件时失败: %v", err))
		os.Exit(1)
	}

	// 处理并导出
	outputFilename, err := MergeExclusionsAndExport(
		analysisResult,
		timeline,
		int32(diffThreshold),
		minDuration,
		outputBase,
	)
	if err != nil {
		fmt.Println()
		printError(fmt.Sprintf("整合时失败: %v", err))
		os.Exit(1)
	}

	fmt.Printf("✓  整合完成 -> %s\n", filepath.Base(outputFilename))
}

// findFileByExtension 在指定目录中查找具有给定扩展名的文件。
//
// 对于 .json 文件，如果存在多个匹配文件，会优先选择名为 "autoeditor.json" 的文件。
// 对于其他扩展名，如果存在多个匹配文件，会选择第一个找到的文件并输出警告信息。
//
// 参数:
//   - dir: 要搜索的目录路径
//   - ext: 文件扩展名（包含点号，如 ".json" 或 ".gob"）
//
// 返回值:
//   - string: 找到的文件完整路径
//   - error: 搜索失败或未找到文件时返回错误
func findFileByExtension(dir, ext string) (string, error) {
	pattern := filepath.Join(dir, "*"+ext)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("搜索 %s 文件时失败: %w", ext, err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("未找到 %s 文件", ext)
	}

	selected := matches[0]
	if len(matches) > 1 {
		// .json 文件优先使用 autoeditor.json
		if ext == ".json" {
			for _, match := range matches {
				if filepath.Base(match) == "autoeditor.json" {
					selected = match
					break
				}
			}
		}
		fmt.Fprintf(os.Stderr, "⚠  发现多个 %s 文件，使用: %s\n", ext, filepath.Base(selected))
	}

	return selected, nil
}

// printUsage 输出程序的使用说明到标准错误流。
//
// 显示命令行参数的格式和要求，帮助用户正确使用程序。
func printUsage() {
	fmt.Fprintf(os.Stderr, "用法: %s <threshold> [minDuration] [output_base]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "需要当前目录下存在 .gob 和 .json 文件\n")
}

// printError 以统一格式输出错误消息到标准错误流。
//
// 参数:
//   - msg: 要显示的错误消息内容
func printError(msg string) {
	fmt.Fprintf(os.Stderr, "✗  %s\n", msg)
}
