package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// main 是程序的入口函数
//
// 命令行参数:
//   - threshold (可选): 差异阈值，必须是正整数，用于判断音频片段是否需要排除，默认使用gob内推荐参数
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
	// 1. 获取工作目录
	workDir, err := os.Getwd()
	if err != nil {
		printError(fmt.Sprintf("获取工作目录时失败: %v", err))
		os.Exit(1)
	}

	// 2. 自动查找文件 (提前查找，以便读取建议阈值)
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

	// 3. 加载数据 (提前加载)
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

	// 4. 确定阈值 (DiffThreshold)
	var diffThreshold uint32

	if len(os.Args) >= 2 {
		// 优先使用命令行参数
		val, err := strconv.Atoi(os.Args[1])
		if err != nil || val <= 0 {
			printError(fmt.Sprintf("threshold 必须是正整数: %s", os.Args[1]))
			printUsage()
			os.Exit(1)
		}
		diffThreshold = uint32(val)
		fmt.Printf(">> 使用手动阈值: %d\n", diffThreshold)
	} else {
		// 未提供参数，尝试使用建议阈值
		if analysisResult.SuggestedThreshold > 0 {
			diffThreshold = uint32(analysisResult.SuggestedThreshold)
			fmt.Printf(">> 使用建议阈值: %d\n", diffThreshold)
		} else {
			// 既没参数也没建议值
			printError("缺少 threshold 参数，且分析结果中未包含有效建议阈值")
			printUsage()
			os.Exit(1)
		}
	}

	// 5. 解析 minDuration（第二个参数，可选）
	// 注意：如果想设置 minDuration，必须在命令行显式提供 threshold
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

	// 6. 解析 output base（第三个参数，可选）
	outputBase := jsonFile // 默认
	if len(os.Args) >= 4 {
		outputBase = os.Args[3]
	}

	// 7. 处理并导出
	outputFilename, err := MergeExclusionsAndExport(
		analysisResult,
		timeline,
		diffThreshold,
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
	fmt.Fprintf(os.Stderr, "用法: %s [threshold] [minDuration] [output_base]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "提示: 如果省略 threshold，将使用 .gob 文件中的建议阈值\n")
}

// printError 以统一格式输出错误消息到标准错误流。
//
// 参数:
//   - msg: 要显示的错误消息内容
func printError(msg string) {
	fmt.Fprintf(os.Stderr, "✗  %s\n", msg)
}
