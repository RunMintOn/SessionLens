# Go TUI Session Manager - 预想效果文档

> **文档目的**: 明确 Go 版本 TUI 会话管理器的最终实现效果
> 
> **参考版本**: Python/Textual 版本 (`main.py`)
> 
> **实现框架**: Go + Bubble Tea + Lip Gloss

---

## 🎯 核心用户体验

### 启动方式
```bash
# 在 WSL 终端中输入
agent manager

# 或直接运行
./session-manager
```

### 整体布局

```
┌──────────────────────────────────────────────────────────────────────────┐
│  AGENT SESSION MANAGER                              2026-02-20 15:30    │
├──────────────────────────────────────┬───────────────────────────────────┤
│  SESSIONS                            │  FAVORITES                        │
│  ┌────────┐ ┌────────┐ ┌────────┐   │                                   │
│  │  全部  │ │ Claude │ │OpenCode│   │  ★ RAG 向量数据库实现             │
│  └────────┘ └────────┘ └────────┘   │    [Claude] · ~/langchain-study   │
│                                      │                                   │
│  ▼ langchain-study (3)               │  ★ Bubble Tea 重写                │
│    · 向量数据库优化方案 [Claude]     │    [OpenCode] · ~/tui-agentSess.. │
│    · RAG 实现细节 [OpenCode]          │                                   │
│    ★ Embedding 配置问题 [Qwen]        │  · API 调试会话                   │
│                                      │    [Claude] · ~/其他项目           │
│  ▼ tui-agentSessionManager (2)       │                                   │
│    · Scanner 模块实现 [OpenCode]      │                                   │
│    ★ Bubble Tea TUI 美化 [Claude]     │                                   │
│                                      │                                   │
│  ▶ 其他项目 (5)                      │                                   │
│                                      │                                   │
│                                      │                                   │
├──────────────────────────────────────┴───────────────────────────────────┤
│  ↑/k 上  ↓/j 下  ←/h 折叠  →/l 展开  Enter 恢复  c 收藏  r 刷新  q 退出   │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## 📋 详细功能规格

### 1. 左侧面板 - 会话树 (60% 宽度)

#### 1.1 过滤按钮
```
┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐
│  全部  │ │ Claude │ │OpenCode│ │  Qwen  │
└────────┘ └────────┘ └────────┘ └────────┘
   ↑           ↑          ↑          ↑
 全部会话    只显示     只显示     只显示
           Claude    OpenCode    Qwen
```

- **默认**: "全部" 按钮高亮（primary variant）
- **点击**: 过滤对应来源的会话
- **实现**: Bubble Tea 按钮组件 + 状态管理

#### 1.2 树形结构

**层级结构**:
```
根节点 (隐藏，不显示)
├── 项目 A (可展开/折叠)
│   ├── 会话 1 [Claude]
│   ├── 会话 2 [OpenCode]
│   └── 会话 3 [Qwen]
├── 项目 B (可展开/折叠)
│   └── 会话 4 [Claude]
└── 其他项目 (可展开/折叠)
    ├── 会话 5 [OpenCode]
    └── 会话 6 [Qwen]
```

**项目节点显示**:
```
▼ 项目名称 (会话数量)     ← 展开状态
▶ 项目名称 (会话数量)     ← 折叠状态
```

**会话节点显示**:
```
格式：[收藏标记] 标题 [来源]

收藏标记:
  ★ = 已收藏 (star)
  · = 未收藏 (dot)

来源标签:
  [Claude]   = 橙色 (#FDBA74)
  [OpenCode] = 绿色 (#86EFAC)
  [Qwen]     = 蓝色 (#93C5FD)
```

**示例**:
```
▼ langchain-study (3)
  · 向量数据库优化方案 [Claude]
  ★ RAG 实现细节 [OpenCode]
  · Embedding 配置 [Qwen]
```

#### 1.3 标题截断规则

```go
// 最大长度 22 字符
if len(title) > 22 {
    title = title[:19] + "..."
}

// 示例
"向量数据库优化方案详细讨论记录" → "向量数据库优化方案详..."
"RAG 实现" → "RAG 实现" (不截断)
```

#### 1.4 项目路径简化

```go
// 完整路径
/home/lee/11MyProjrct/langchain-study

// 简化显示
~/langchain-study

// 实现逻辑
if path.StartsWith(home) {
    return "~" + path[len(home):]
}
```

---

### 2. 右侧面板 - 收藏列表 (40% 宽度)

#### 2.1 空状态
```
FAVORITES
┌─────────────────────────────────────┐
│                                     │
│      No favorites yet               │
│                                     │
│   (按 c 键收藏当前选中的会话)          │
│                                     │
└─────────────────────────────────────┘
```

#### 2.2 有收藏状态
```
FAVORITES
┌─────────────────────────────────────┐
│  ★ RAG 向量数据库实现                │
│    [Claude] · ~/langchain-study     │
│                                     │
│  ★ Bubble Tea 重写                  │
│    [OpenCode] · ~/tui-agentSess...  │
│                                     │
│  · API 调试会话                      │
│    [Claude] · ~/其他项目             │
└─────────────────────────────────────┘
```

**显示格式**:
```
行 1: [收藏标记] 标题 (截断，最大 22 字符)
行 2: [来源] · 简化路径
```

---

### 3. 键盘交互

| 按键 | 功能 | 说明 |
|------|------|------|
| `↑` / `k` | 向上移动 | 在树节点间移动光标 |
| `↓` / `j` | 向下移动 | 在树节点间移动光标 |
| `←` / `h` | 折叠 | 折叠当前项目节点 |
| `→` / `l` | 展开 | 展开当前项目节点 |
| `Enter` | 恢复会话 | 恢复选中的会话 |
| `c` | 收藏/取消收藏 | 切换当前会话的收藏状态 |
| `r` | 刷新 | 重新扫描所有会话 |
| `q` | 退出 | 退出程序 |
| `1` | 全部 | 切换到全部过滤器 |
| `2` | Claude | 切换到 Claude 过滤器 |
| `3` | OpenCode | 切换到 OpenCode 过滤器 |
| `4` | Qwen | 切换到 Qwen 过滤器 |

---

### 4. 会话恢复行为

#### 4.1 恢复逻辑

```go
// 伪代码
func restoreSession(sess Session) {
    // 1. 获取项目路径
    projectPath := sess.ProjectPath
    
    // 2. 根据来源工具构建命令
    var cmd []string
    switch sess.SourceTool {
    case "Claude":
        cmd = []string{"claude", "-r", sess.ID}
    case "OpenCode":
        cmd = []string{"opencode", "-s", sess.ID}
    case "Qwen":
        cmd = []string{"qwen", "-r", sess.ID}
    }
    
    // 3. 先 cd 到项目目录，再执行恢复命令
    fullCmd := fmt.Sprintf("cd %s && %s", projectPath, strings.Join(cmd, " "))
    
    // 4. 启动新会话（不等待）
    exec.Command("bash", "-c", fullCmd)
        .Start()  // 非阻塞
    
    // 5. 程序退出，让 AI CLI 接管终端
    return m, tea.Quit
}
```

#### 4.2 预期效果

```
用户操作:
1. 在 TUI 中选中 "RAG 实现细节 [OpenCode]"
2. 按 Enter

程序执行:
1. cd ~/langchain-study
2. opencode -s ses_xxxxx

终端状态:
- Session Manager 退出
- OpenCode AI CLI 启动，恢复会话
- 用户在原终端窗口与 AI 继续对话
```

---

### 5. 收藏功能

#### 5.1 收藏数据存储

**位置**: `~/.local/share/agent-session-manager/favorites.db`

**表结构**:
```sql
CREATE TABLE favorites (
    session_id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    added_at INTEGER NOT NULL
);
```

#### 5.2 收藏操作流程

```
1. 用户选中某个会话节点
2. 按 c 键
3. 如果未收藏 → 添加到数据库，显示 ★
4. 如果已收藏 → 从数据库删除，显示 ·
5. 刷新左右面板显示
```

---

### 6. 视觉风格

#### 6.1 颜色方案

| 元素 | 颜色 | Hex |
|------|------|-----|
| 背景 | 深灰 | `#1E1E1E` |
| 边框 | 灰色 | `#3C3C3C` |
| 文本 | 白色 | `#FAFAFA` |
| 选中背景 | 蓝色 | `#569CD6` |
| Claude 来源 | 橙色 | `#FDBA74` |
| OpenCode 来源 | 绿色 | `#86EFAC` |
| Qwen 来源 | 蓝色 | `#93C5FD` |
| 收藏标记 | 黄色 | `#FBBF24` |

#### 6.2 边框样式

```
使用 Lip Gloss RoundedBorder

┌─────────────────────────────────────┐
│                                     │
│                                     │
└─────────────────────────────────────┘
```

#### 6.3 面板布局

```
Header (1 行)
├─ 标题左对齐
└─ 时间右对齐

Main Content (动态高度)
├─ Left Panel (60%)
│  ├─ "SESSIONS" 标题
│  ├─ 过滤按钮 (Horizontal)
│  └─ Tree 会话树
└─ Right Panel (40%)
   ├─ "FAVORITES" 标题
   └─ 收藏列表

Footer (1 行)
└─ 快捷键帮助 (居中，灰色)
```

---

## 📊 数据结构

### Session (会话)

```go
type Session struct {
    ID          string      // 会话 ID，如 "ses_abc123"
    Title       string      // 会话标题
    SourceTool  SourceType  // CLAUDE | OPENCODE | QWEN
    ProjectPath string      // 项目完整路径
    LastUpdated int64       // Unix 时间戳
}
```

### Project (项目)

```go
type Project struct {
    Path     string     // 项目完整路径
    Name     string     // 简化名称 (如 "~/langchain-study")
    Sessions []Session  // 会话列表
}
```

### Tree Model (树模型)

```go
type TreeModel struct {
    Projects       []Project        // 项目列表
    ExpandedNodes  map[string]bool  // 展开状态
    CursorIndex    int              // 当前光标位置
    Filter         SourceType       // 当前过滤器
    Favorites      map[string]bool  // 收藏会话 ID 集合
}
```

---

## 🔧 技术实现要点

### 1. 树形控件实现

**方案 A**: 使用 `bubbles/treeview` (如果存在)
**方案 B**: 自定义实现 (推荐，更灵活)

```go
// 自定义树节点
type TreeNode struct {
    Label      string
    Style      lipgloss.Style
    Children   []TreeNode
    Data       interface{}  // 存储 Session
    Expanded   bool
    Depth      int
}
```

### 2. 收藏持久化

```go
// 使用 SQLite (与 Scanner 一致)
import "github.com/glebarez/sqlite"

// 或使用简单 JSON
import "encoding/json"

// 推荐 SQLite，支持查询和扩展
```

### 3. 会话恢复进程管理

```go
// 关键：使用 syscall.Setsid 创建新会话
import "syscall"

cmd := exec.Command("bash", "-c", fullCmd)
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
cmd.Start()

// 然后退出程序
return m, tea.Quit
```

---

## ✅ 验收标准

### 功能验收

- [ ] 启动后显示树形项目列表
- [ ] 项目可展开/折叠
- [ ] 会话显示标题 + 来源标签
- [ ] 过滤按钮工作正常
- [ ] 收藏功能工作正常
- [ ] 按 Enter 恢复会话并退出程序
- [ ] AI CLI 正确启动并恢复会话

### UI 验收

- [ ] Mac 终端风格（圆角边框、深色主题）
- [ ] 来源颜色正确显示
- [ ] 选中状态高亮
- [ ] 窗口 resize 自适应
- [ ] 收藏列表正确显示

### 性能验收

- [ ] 启动时间 < 1 秒
- [ ] 会话扫描 < 2 秒
- [ ] 键盘响应 < 100ms

---

## 📝 与 Python 版本的差异

| 特性 | Python 版本 | Go 版本 (计划) |
|------|------------|---------------|
| UI 框架 | Textual | Bubble Tea |
| 树形控件 | Tree (内置) | 自定义实现 |
| 收藏存储 | SQLite | SQLite (一致) |
| 会话恢复 | Popen + start_new_session | exec.Command + Setsid |
| 过滤按钮 | Button 组件 | Button 组件 |
| 颜色主题 | Textual $surface | 自定义 Mac 风格 |
| 快捷键 | 类似 | 类似 + 数字键过滤 |

---

## 🚀 实现优先级

### P0 - 核心功能 (必须先实现)
1. 树形项目分组显示
2. 会话选择 + 键盘导航
3. 会话恢复（cd + CLI + 退出）

### P1 - 重要功能
4. 过滤按钮
5. 收藏功能（存储 + 显示）
6. 来源颜色标签

### P2 - 优化功能
7. 窗口 resize 自适应
8. 搜索功能
9. 快捷键数字过滤

---

## 📌 备注

- **会话 ID 显示**: 默认不显示完整 ID，如需调试可在右侧详情栏显示
- **右侧详情栏**: 后续可选功能，显示选中会话的详细信息
- **项目分组**: 按 `project_path` 分组，相同路径的会话归为一个项目
- **其他项目**: 如果项目路径为空或无法解析，归入"其他项目"

---

**文档版本**: 1.0  
**创建日期**: 2026-02-20  
**最后更新**: 2026-02-20
