# gob2json

**视频静音静止区间检测与合并工具**

一个用于检测视频中同时满足"静音"和"静止画面"条件的区间，并生成可供 [auto-editor](https://github.com/WyattBlue/auto-editor) 使用的时间线文件的 Go 工具。

## 🎯 核心功能

将两个独立的分析结果进行**交集运算**：

1.  **auto-editor 的静音片段** (`autoeditor.json`) - 包含音频静音区间
2.  **vcmp 的静止画面片段** (`.gob` 文件) - 包含视频画面静止区间

输出：**同时静音且静止的区间**，格式为 auto-editor v1 时间线 JSON，可直接用于视频剪辑。

## 📋 使用场景

  - 自动移除视频中的"冻结"片段（画面静止 + 无声音）
  - 批量处理视频教程、录屏、直播回放中的无效片段
  - 精确定位需要手动编辑的问题区域

## 🚀 快速开始

### 安装

```bash
# 克隆仓库
git clone <repository-url>
cd gob2json

# 构建并安装到系统路径
make install
```

### 使用步骤

1.  **准备输入文件**

    在工作目录下放置两个文件：

      - `autoeditor.json` - 由 auto-editor 生成的时间线文件
      - `*.gob` - 由 vcmp 生成的视频分析结果文件（需包含建议阈值数据）

2.  **运行程序**

    ```bash
    gob2json [threshold] [minDuration] [output_base]
    ```

    **参数说明：**

      - `threshold` (可选): 差异阈值（整数）。
          - **如果不填**: 程序将自动使用 `.gob` 文件中记录的 `SuggestedThreshold`。
          - **如果填入**: 将覆盖自动建议值，强制使用指定的阈值。
      - `minDuration` (可选): 最小排除时长（秒），必须是正数，默认使用 `20.0` 秒。
          - *注意：若要指定 `minDuration`，必须显式提供 `threshold` 参数。*
      - `output_base` (可选): 输出文件的基础名称，默认使用输入的 JSON 文件名。

3.  **使用输出文件**

    程序会生成带时间戳的 JSON 文件，例如 `autoeditor_20231215_143022.json`，可直接用于 auto-editor：

    ```bash
    auto-editor input.mp4 --edit timeline:autoeditor_20231215_143022.json
    ```

### 示例

```bash
# 模式 1: 全自动模式
# 直接使用 gob 文件内的建议阈值，默认最小排除时长 20秒
gob2json

# 模式 2: 手动指定阈值
# 强制使用阈值 30，默认最小排除时长 20秒
gob2json 30

# 模式 3: 指定阈值和时长
# 强制使用阈值 30，并将最小排除时长设为 2.5 秒
gob2json 30 2.5

# 模式 4: 指定输出文件名
gob2json 30 2.5 cleaned_output
```

## 🔧 工作原理

```
┌─────────────────┐     ┌─────────────────┐
│  autoeditor.json│     │   analysis.gob  │
│   (静音区间)    │     │  (静止画面区间) │
└────────┬────────┘     └────────┬────────┘
         │                       │
         └───────────┬───────────┘
                     ↓
            ┌─────────────────┐
            │   交集运算      │
            │ (重叠区间检测)  │
            └────────┬────────┘
                     ↓
         ┌───────────────────────┐
         │  output_timestamp.json │
         │  (静音+静止的区间)     │
         │   速度标记为 0.0       │
         └───────────────────────┘
```

### 关键逻辑

1.  **解析输入文件**

      - 从 `.gob` 文件读取帧差异数据（`DiffCounts`）和建议阈值（`SuggestedThreshold`）
      - 从 `autoeditor.json` 读取时间线片段（`Chunks`）

2.  **区间检测**

      - **静止画面区间**: 差异值超过阈值的连续帧（≥最小时长）
      - **静音区间**: 时间线中速度为 `0.0` 或 `9999.0` 的片段

3.  **交集计算**

      - 找出两组区间的重叠部分
      - 合并相邻或重叠的区间

4.  **生成输出**

      - 将交集区间的播放速度设置为 `0.0`（排除标记）
      - 保持其他区间的原始速度
      - 输出符合 auto-editor v1 规范的 JSON

## 📁 项目结构

```
gob2json/
├── main.go          # 入口函数，参数解析与自动阈值逻辑
├── vcmp.go          # .gob 文件读写，AnalysisResult 数据结构定义
├── autoeditor.go    # auto-editor JSON 格式解析与生成
├── merge.go         # 核心算法：区间检测、交集计算、合并导出
├── go.mod           # Go 模块定义
├── Makefile         # 构建脚本
└── README.md        # 本文档
```

## 🛠️ 开发命令

```bash
# 构建二进制文件
make build

# 安装到系统路径 (需要 sudo)
make install

# 卸载
make uninstall

# 清理构建文件
make clean

# 显示帮助信息
make help
```

## 📊 数据格式

### 输入格式

**auto-editor JSON (v1):**

```json
{
  "version": "1",
  "source": "video.mp4",
  "chunks": [
    [0.0, 10.5, 1.0],     // [开始, 结束, 速度]
    [10.5, 15.0, 0.0],    // 速度 0.0 = 静音片段
    [15.0, 30.0, 1.0]
  ]
}
```

**vcmp .gob 文件结构:**

```go
// AnalysisResult 保存视频分析的完整结果
type AnalysisResult struct {
   VideoFile          string   // 被分析的视频文件路径
   FPS                float64  // 视频帧率
   Width              int      // 视频宽度（像素）
   Height             int      // 视频高度（像素）
   TotalFrames        int      // 视频总帧数
   SuggestedThreshold float64  // 自动计算的建议阈值
   DiffCounts         []uint32 // 每一帧的差异像素数量 (注意类型为 uint32)
}
```

### 输出格式

与输入的 auto-editor JSON 格式相同，但交集区间的速度被设置为 `0.0`。

## ⚙️ 配置说明

### 常量（可在 `merge.go` 中修改）

```go
const (
    MinExclusionDurationSeconds = 20.0   // 默认最小排除时长
    ExcludedSpeedMarker = 0.0           // 排除区域的速度标记
    SkipSpeedHigh = 9999.0              // 高速度标记
    SkipSpeedZero = 0.0                 // 零速度标记
)
```

### 阈值机制

程序优先读取命令行参数。如果未提供命令行参数，则会尝试使用 `.gob` 文件中存储的 `SuggestedThreshold`。如果两者都不可用（例如旧版 gob 文件且未指定参数），程序将报错退出。

  - **低阈值**: 更严格，可能将微小的动作（如鼠标抖动）也视为非静止。
  - **高阈值**: 更宽松，容忍一定的画面变化。

## 🔍 故障排除

### 常见问题

1.  **找不到输入文件**

    ```
    ✗ 未找到 .gob 文件
    ```

    → 确保工作目录下存在 `.gob` 和 `.json` 文件。

2.  **缺少阈值**

    ```
    ✗ 缺少 threshold 参数，且分析结果中未包含有效建议阈值
    ```

    → 说明 `.gob` 文件可能由旧版工具生成，不包含建议阈值。此时必须在命令行手动指定一个整数阈值（如 `gob2json 30`）。

3.  **发现多个 JSON 文件**

    ```
    ⚠ 发现多个 .json 文件，使用: autoeditor.json
    ```

    → 程序会优先使用名为 `autoeditor.json` 的文件。

4.  **版本错误**

    ```
    ✗ 版本号无效: 期望 "1"，得到 "3"
    ```

    → 当前仅支持 auto-editor v1 格式 timeline。

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

### 开发环境要求

  - Go 1.21 或更高版本
  - 熟悉 auto-editor 和 vcmp 工具的使用

## 📄 许可证

本项目采用 [GPL-3.0 License](https://www.google.com/search?q=LICENSE) 开源。

## 🔗 相关项目

  - [auto-editor](https://github.com/WyattBlue/auto-editor) - 自动视频编辑工具
  - [vcmp](https://github.com/334456777/vcmp) - 视频画面比较工具
