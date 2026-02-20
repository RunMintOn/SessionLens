# Bubble Tea 重构计划

## 当前状态

### ✅ 阶段 1：基础框架 - 已完成
- [x] 安装 Go (`go1.22.2`)
- [x] 安装 gopls (`~/go/bin/gopls`)
- [x] 配置 OpenCode LSP
- [x] 创建 Bubble Tea 脚手架
- [x] 验证基本运行

### 📁 当前文件结构
```
tui_agentSessionManager/
├── main.py                          # Python 原版（main 分支）
├── cmd/session-manager/main.go      # Go 新版本（bubble-tea 分支）
├── go.mod
├── go.sum
├── docs/
│   └── bubble-tea-preparation.md    # 准备资料
└── README_BUBBLETEA.md              # Bubble Tea 说明
```

---

## OpenCode LSP 配置

### ✅ 已配置完成

LSP 配置在 `~/.config/opencode/opencode.json`：

```json
"lsp":{
    "gopls":{
        "enabled": true,
        "command": ["/home/lee/go/bin/gopls"],
        "extensions": [".go"]
    }
}
```

**验证方式：** 重启 OpenCode，访问 `.go` 文件时 LSP 自动激活

---

## 并行执行策略

### Critical Path
```
2.1 → 2.2 → 2.3.1 → 2.3 → 2.6 → 2.7 → 2.8
         ↓         ↓       ↓
         └────────┴───────┘ (2.4, 2.5 并行)
                            ↓
                         2.9 (边缘测试)
```

### 预期并行加速
- **Wave 1**: 2 个任务并行 → 立即开始
- **Wave 2**: 4 个任务并行（2.3 阻塞 2.3.1）→ Wave 1 完成后
- **Wave 3**: 3 个任务顺序 → Wave 2 完成后
- **Wave 4**: 1 个任务 → Wave 3 完成后

**预计比全顺序快 30-40%**

---

## 下一步开发计划

### 阶段 1：基础框架（当前）
- [x] 安装 Go
- [x] 创建 Bubble Tea 脚手架
- [x] 验证基本运行
- [x] 配置 OpenCode LSP ✅

### 阶段 2：读取真实会话数据（Go 重写 Scanner）

> **并行执行策略**：Wave 1 → Wave 2 → Wave 3 → Wave 4

---

#### Wave 1: 基础结构（无依赖，并行）

##### 任务 2.1：创建项目结构和依赖
- [x] 添加纯 Go SQLite 依赖：`go get github.com/glebarez/go-sqlite`（非 CGO 版本）
- [x] 创建 `session/` 目录
- [x] 创建 `session/scanner.go` - Scanner 接口定义
- [x] QA: `CGO_ENABLED=0 go build -o /dev/null ./cmd/session-manager`（验证无 CGO 依赖）

##### 任务 2.2：实现 Session 数据结构
- [x] 在 `session/types.go` 定义结构体（参考 `models.py`）：
  ```go
  type SourceType string  // "opencode" | "claude" | "qwen"
  
  type Session struct {
      ID          string
      Title       string
      SourceTool  SourceType
      ProjectPath string
      LastUpdated int64   // Unix timestamp
  }
  
  type Project struct {
      Path     string
      Sessions []Session
  }
  ```
- [x] QA: `go build ./session/types.go` 无编译错误

---

#### Wave 2: Scanner 实现（依赖 Wave 1，并行）

##### 任务 2.3.1：验证 OpenCode SQLite Schema（阻塞 2.3）
- [x] 查询实际数据库确认列名：`~/.local/share/opencode/opencode.db`
- [x] 验证列：id, title, directory, time_updated, parent_id
- [x] QA: 打印确认的列名

##### 任务 2.3：实现 OpenCode Scanner
- [x] 在 `session/opencode.go` 实现 OpenCodeScanner
- [x] **参考** `scanner/opencode_scanner.py:40-45` 的 SQL 查询：
- [x] 读取 SQLite (`~/.local/share/opencode/opencode.db`)
- [x] QA: 单元测试读取数据库，验证字段正确

##### 任务 2.4：实现 Claude Code Scanner
- [x] 在 `session/claude.go` 实现 ClaudeCodeScanner
- [x] **参考** `scanner/claude_scanner.py` 的 JSONL 解析逻辑
- [x] 读取 JSONL 文件 (`~/.claude/projects/`)
- [x] 解析第一 user message 作为 title
- [x] QA: 单元测试解析示例 JSONL

##### 任务 2.5：实现 Qwen Scanner
- [x] 在 `session/qwen.go` 实现 QwenScanner
- [x] **参考** `scanner/qwen_scanner.py` 的路径和解析逻辑
- [x] 路径：`~/.qwen/projects/<project-hash>/chats/*.jsonl`
- [x] QA: 单元测试解析示例 JSONL

---

#### Wave 3: 整合（依赖 Wave 2）

##### 任务 2.6：整合 Scanner
- [x] 在 `session/scanner.go` 实现 `func scan_all() ([]Project, error)`
- [x] 遍历三个 Scanner，合并数据
- [x] 错误处理：一个 Scanner 失败不影响其他（记录错误但继续）
- [x] 去重：相同 id + source 视为重复
- [x] 按 LastUpdated 降序排序
- [x] 按 ProjectPath 分组为 []Project
- [x] QA: `go test ./session/ -v`

##### 任务 2.7：更新 main.go 数据结构
- [x] 修改 `main.go` 的 Session 结构体，匹配 `session/types.go`
- [x] 添加 Project 结构体
- [x] 更新 model 使用 `projects []Project`
- [x] QA: `go build ./cmd/session-manager` 无错误

##### 任务 2.8：对接 main.go
- [x] main.go 初始化时调用 `scan_all()`
- [x] 修改 View 函数显示项目分组（项目路径 + 会话列表）
- [x] 显示：会话标题、来源标签（带颜色）、最后更新时间
- [x] QA: 运行程序，验证真实数据显示正确

---

#### Wave 4: 边缘情况测试

##### 任务 2.9：边缘情况处理
- [ ] 测试：数据库文件不存在（首次运行）
- [ ] 测试：JSONL 目录为空
- [ ] 测试：损坏的 SQLite 数据库
- [ ] 测试：格式错误的 JSONL
- [ ] 测试：权限拒绝
- [ ] 测试：Unicode 路径
- [ ] QA: 程序不崩溃，显示空状态而非错误

### 阶段 3：UI 美化
- [ ] 添加边框和面板（Lip Gloss）
- [ ] 实现 Mac 终端风格主题
- [ ] 优化选中高亮

### 阶段 4：会话恢复功能
- [ ] 实现会话附加（tmux attach）
- [ ] 快捷键绑定
- [ ] 错误处理

### 阶段 5：完善功能
- [ ] 搜索功能
- [ ] 窗口自适应
- [ ] 性能优化

---

## 运行命令

### 开发运行
```bash
cd /home/lee/11MyProjrct/tui_agentSessionManager
go run ./cmd/session-manager
```

### 构建
```bash
go build -o session-manager ./cmd/session-manager
```

### 测试
```bash
go test ./...
```

---

## 键盘快捷键

| 按键 | 功能 |
|------|------|
| `↑` / `k` | 上移 |
| `↓` / `j` | 下移 |
| `q` | 退出 |
| `Enter` | 选择（待实现） |
| `f` | 收藏（待实现） |

---

## 参考资料

### Python Scanner 实现（必须参考）
- `scanner/opencode_scanner.py` - SQLite 查询逻辑
- `scanner/claude_scanner.py` - JSONL 解析逻辑
- `scanner/qwen_scanner.py` - Qwen 路径和解析
- `models.py` - Session/Project 数据结构

### 技术文档
- [Bubble Tea 文档](https://pkg.go.dev/github.com/charmbracelet/bubbletea)
- [Lip Gloss 文档](https://github.com/charmbracelet/lipgloss)
- [Bubbles 组件](https://github.com/charmbracelet/bubbles)
- [glebarez/go-sqlite](https://github.com/glebarez/go-sqlite) - 纯 Go SQLite 驱动
- [Agent Deck](https://github.com/asheshgoplani/agent-deck)
- [OpenCode LSP 配置](https://dev.to/pachilo/when-read-this-file-means-run-this-code-lsp-configuration-in-opencode-44g1)
