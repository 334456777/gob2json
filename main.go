package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// main is the program entry point
//
// Command line arguments:
//   - threshold (optional): Difference threshold, must be a positive integer, used to determine if audio segments should be excluded, defaults to recommended value in pb.zst
//   - minDuration (optional): Minimum exclusion duration in seconds, must be positive, defaults to MinExclusionDurationSeconds
//   - output_base (optional): Output file base name, defaults to input JSON filename
//
// The program automatically searches in the current working directory for:
//   - A .pb.zst file: Contains video analysis results (Protocol Buffers + Zstandard compressed)
//   - A .json file: Contains timeline data (prefers autoeditor.json)
//
// Usage examples:
//
//	program 30                    // Use threshold 30, other parameters use defaults
//	program 30 2.5                // Use threshold 30, min duration 2.5 seconds
//	program 30 2.5 output         // Specify output base name as output
//
// Exit status codes:
//   - 0: Successful execution
//   - 1: Error occurred (parameter error, file not found, processing failure, etc.)
func main() {
	// 1. Get working directory
	workDir, err := os.Getwd()
	if err != nil {
		printError(fmt.Sprintf("Failed to get working directory: %v", err))
		os.Exit(1)
	}

	// 2. Auto-find files (early search to read suggested threshold)
	pbzstFile, err := findFileByExtension(workDir, ".pb.zst")
	if err != nil {
		printError(fmt.Sprintf("%v", err))
		os.Exit(1)
	}

	// Auto-find .json file
	jsonFile, err := findFileByExtension(workDir, ".json")
	if err != nil {
		printError(fmt.Sprintf("%v", err))
		os.Exit(1)
	}

	// 3. Load data (early load)
	fmt.Printf(">> Loading video analysis results: %s\n", filepath.Base(pbzstFile))
	analysisResult, err := LoadAnalysisResultFromFile(pbzstFile)
	if err != nil {
		printError(fmt.Sprintf("Failed to load pb.zst file: %v", err))
		os.Exit(1)
	}

	// Load audio analysis results
	fmt.Printf(">> Loading audio analysis results: %s\n", filepath.Base(jsonFile))
	timeline, err := ParseTimelineFromFile(jsonFile)
	if err != nil {
		printError(fmt.Sprintf("Failed to load json file: %v", err))
		os.Exit(1)
	}

	// 4. Determine threshold (DiffThreshold)
	var diffThreshold uint32

	if len(os.Args) >= 2 {
		// Prefer command line arguments
		val, err := strconv.Atoi(os.Args[1])
		if err != nil || val <= 0 {
			printError(fmt.Sprintf("threshold must be a positive integer: %s", os.Args[1]))
			printUsage()
			os.Exit(1)
		}
		diffThreshold = uint32(val)
	} else {
		if analysisResult.SuggestedThreshold > 0 {
			diffThreshold = uint32(analysisResult.SuggestedThreshold)
		} else {
			// No parameter or suggested value available
			printError("Missing threshold parameter and no valid suggested threshold in analysis results")
			printUsage()
			os.Exit(1)
		}
	}

	// 5. Parse minDuration (second parameter, optional)
	// Note: To set minDuration, threshold must be explicitly provided on command line
	minDuration := MinExclusionDurationSeconds
	if len(os.Args) >= 3 {
		parsedDuration, err := strconv.ParseFloat(os.Args[2], 64)
		if err != nil || parsedDuration <= 0 {
			printError(fmt.Sprintf("minDuration must be positive: %s", os.Args[2]))
			printUsage()
			os.Exit(1)
		}
		minDuration = parsedDuration
	}

	// 6. Parse output base (third parameter, optional)
	outputBase := "autoeditor" // default
	if len(os.Args) >= 4 {
		outputBase = os.Args[3]
	}

	// 7. Process and export
	outputFilename, err := MergeExclusionsAndExport(
		analysisResult,
		timeline,
		diffThreshold,
		minDuration,
		outputBase,
	)
	if err != nil {
		fmt.Println()
		printError(fmt.Sprintf("Failed to merge: %v", err))
		os.Exit(1)
	}

	fmt.Printf("✓  Merge complete -> %s\n", filepath.Base(outputFilename))
}

// findFileByExtension searches for files with a given extension in a specified directory.
//
// For .json files, if multiple matching files exist, "autoeditor.json" is preferred.
// For other extensions, if multiple matching files exist, the first found file is selected and a warning is displayed.
//
// Parameters:
//   - dir: Directory path to search
//   - ext: File extension (including dot, e.g., ".json" or ".pb.zst")
//
// Returns:
//   - string: Full path of the found file
//   - error: Error if search fails or no file is found
func findFileByExtension(dir, ext string) (string, error) {
	pattern := filepath.Join(dir, "*"+ext)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("Failed to search for %s files: %w", ext, err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("No %s files found", ext)
	}

	selected := matches[0]
	if len(matches) > 1 {
		// .json files prefer autoeditor.json
		if ext == ".json" {
			for _, match := range matches {
				if filepath.Base(match) == "autoeditor.json" {
					selected = match
					break
				}
			}
		}
		fmt.Fprintf(os.Stderr, "⚠  发现多个 %s 文件，使用 %s\n", ext, filepath.Base(selected))
	}

	return selected, nil
}

// printUsage outputs program usage instructions to stderr.
//
// Displays command line argument format and requirements to help users use the program correctly.
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [threshold] [minDuration] [output_base]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Note: If threshold is omitted, the suggested threshold from .pb.zst file will be used\n")
}

// printError outputs error messages in a unified format to stderr.
//
// Parameters:
//   - msg: Error message content to display
func printError(msg string) {
	fmt.Fprintf(os.Stderr, "✗  %s\n", msg)
}
