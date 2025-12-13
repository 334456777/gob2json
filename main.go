package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

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

// findFileByExtension 在目录中查找指定扩展名的文件。
// 如果有多个 .json 文件，优先使用 autoeditor.json。
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

func printUsage() {
	fmt.Fprintf(os.Stderr, "用法: %s <threshold> [minDuration] [output_base]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "需要当前目录下存在 .gob 和 .json 文件\n")
}

func printError(msg string) {
	fmt.Fprintf(os.Stderr, "✗  %s\n", msg)
}
