# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

gob2json 是一个视频静音静止区间检测工具，对两个独立分析结果进行**交集运算**：
- **auto-editor 静音片段** (`autoeditor.json`) - 音频静音区间
- **vcmp 静止画面片段** (`.pb.zst`) - 视频画面静止区间（Protocol Buffers + Zstandard 压缩）

输出 auto-editor v1 时间线 JSON，交集区间速度标记为 `0.0`（排除）。

## 构建命令

```bash
# 修改 .proto 文件后必须执行
make proto

# 构建二进制文件
make build

# 安装到系统路径（需要 sudo）
make install

# 清理构建文件
make clean
```

**重要**：修改 `proto/analysis.proto` 后必须运行 `make proto` 重新生成代码。

## 代码架构

四个核心模块，每个文件单一职责：

- **main.go** - 入口与参数处理
  - 阈值优先级：命令行参数 > `.pb.zst` 的 `SuggestedThreshold`
  - 自动查找工作目录中的 `.pb.zst` 和 `.json` 文件
  - 参数：`[threshold] [minDuration] [output_base]`

- **vcmp.go** - 视频分析结果读写
  - `AnalysisResult` 结构体：视频元数据 + 帧差异计数数组
  - Protocol Buffers 序列化 + Zstandard 压缩
  - 提供 `Validate()` 验证方法

- **autoeditor.go** - 时间线 JSON 解析/生成
  - `Timeline`/`Chunk` 结构体遵循 v1 规范
  - 严格验证：版本号、时间连续性、速度范围
  - `validateTimeline()` 确保数据完整性

- **merge.go** - 核心算法
  - `FindExclusionRegionsFromAnalysis()` - 从帧差异数据检测静止区间
  - `FindExclusionRegionsFromTimeline()` - 从时间线提取静音区间
  - `FindOverlappingRegions()` - 计算两组区间交集
  - `ApplyExclusionToTimeline()` - 将排除区域应用到时间线（拆分片段）

## 数据流

```
.pb.zst (AnalysisResult) ──┐
                            ├─→ MergeExclusionsAndExport() ─→ output.json
autoeditor.json (Timeline) ─┘
```

## 核心算法

**区间检测** (`merge.go:30-90`)：
- 差异值 > `diffThreshold` 的连续帧
- 达到 `minFrames`（`minDuration * fps`）才形成区间

**交集计算** (`merge.go:118-139`)：
- 遍历两组区间找重叠：`max(start1, start2)` 到 `min(end1, end2)`
- 合并相邻或重叠的区间

**应用排除** (`merge.go:174-254`)：
- 拆分时间线片段为三部分：排除前、排除（速度 0.0）、排除后

## 关键常量 (merge.go)

```go
MinExclusionDurationSeconds = 20.0   // 最小排除时长（秒）
ExcludedSpeedMarker = 0.0             // 排除速度标记
SkipSpeedHigh = 9999.0                // 时间线排除的高速度值
SkipSpeedZero = 0.0                   // 时间线排除的零速度值
```

## 时间线验证规则 (autoeditor.go:120-171)

严格的 v1 规范：
1. 版本号必须为 "1"
2. 第一个 chunk 必须从时间 0 开始
3. chunk 之间无间隙（连续性）
4. `Start < End`，速度范围 `[0.0, 99999.0]`

## 阈值机制

- **低阈值**：严格，微小画面变化也视为非静止
- **高阈值**：宽松，容忍一定画面变化
- **自动建议值**：存储在 `.pb.zst` 的 `SuggestedThreshold` 字段

## Protocol Buffers 集成

- 定义：`proto/analysis.proto`
- 生成：`proto/analysis.pb.go`（`make proto`）
- `diff_counts` 使用 `packed = true` 优化存储
