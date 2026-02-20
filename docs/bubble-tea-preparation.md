# Bubble Tea 重写准备资料

## 技术栈

- **语言**: Go
- **主框架**: Bubble Tea (charmbracelet/bubbletea)
- **样式库**: Lip Gloss (charmbracelet/lipgloss)
- **组件库**: Bubbles (charmbracelet/bubbles) - 提供 List, Table 等预制组件

---

## 核心概念

### 1. Model-View-Update 架构

```go
type Model struct {
    // 应用状态
    sessions  []Session
    cursor    int
    selected  *Session
}

func (m Model) Init() tea.Cmd {
    // 初始化，返回初始命令
    return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // 处理消息，更新状态
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "q":
            return m, tea.Quit
        }
    }
    return m, nil
}

func (m Model) View() string {
    // 渲染 UI
    return "Hello World"
}
```

### 2. 运行程序

```go
func main() {
    p := tea.NewProgram(initialModel())
    if _, err := p.Run(); err != nil {
        fmt.Printf("Error: %v", err)
    }
}
```

---

## 关键 API

### 消息类型 (tea.Msg)

| 消息 | 用途 |
|------|------|
| `tea.KeyMsg` | 键盘输入 |
| `tea.WindowSizeMsg` | 窗口大小变化 |
| `tea.MouseMsg` | 鼠标事件 |
| `tea.Quit` | 退出程序 |

### 键盘处理

```go
case tea.KeyMsg:
    switch msg.String() {
    case "q", "ctrl+c":
        return m, tea.Quit
    case "up", "k":
        m.cursor--
    case "down", "j":
        m.cursor++
    case "enter":
        // 选择当前项
    }
```

---

## 样式 (Lip Gloss)

### 颜色

```go
// ANSI 16 颜色
lipgloss.Color("5")   // magenta
lipgloss.Color("9")   // red
lipgloss.Color("12")  // light blue
lipgloss.Color("10")  // green

// True Color (24位)
lipgloss.Color("#86EFAC")  // 绿色 (OpenCode)
lipgloss.Color("#FDBA74")  // 橙色 (Claude)
lipgloss.Color("#93C5FD")  // 蓝色 (Qwen)

// 自适应颜色 (根据终端背景)
lipgloss.AdaptiveColor{Light: "236", Dark: "248"}
```

### 边框

```go
// 预制边框
lipgloss.NormalBorder()    // 普通边框 │
lipgloss.RoundedBorder()   // 圆角边框 ╭╮
lipgloss.ThickBorder()     // 粗边框

// 边框颜色
lipgloss.NewStyle().
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("240"))
```

### 完整样式示例

```go
var panelStyle = lipgloss.NewStyle().
    Foreground(lipgloss.Color("#FAFAFA")).
    Background(lipgloss.Color("#1E1E1E")).
    BorderStyle(lipgloss.RoundedBorder()).
    BorderForeground(lipgloss.Color("#3C3C3C")).
    Padding(1, 2).
    Width(40)
```

---

## UI 组件

### List 组件 (Bubbles)

```go
// 定义 Item
type SessionItem struct {
    title  string
    source string
}

func (i SessionItem) Title() string       { return i.title }
func (i SessionItem) Description() string { return i.source }
func (i SessionItem) FilterValue() string { return i.title }

// 创建 List
items := []list.Item{
    SessionItem{title: "session-1", source: "opencode"},
    SessionItem{title: "session-2", source: "claude"},
}
l := list.New(items, list.NewDefaultDelegate(), 0, 0)
l.Title = "Sessions"

// 处理窗口大小
case tea.WindowSizeMsg:
    h, v := docStyle.GetFrameSize()
    m.list.SetSize(msg.Width-h, msg.Height-v)
```

---

## 项目结构建议

```
agent-session-manager/
├── main.go           # 入口
├── model.go           # Model 定义和数据结构
├── view.go            # View 渲染逻辑
├── update.go          # Update 消息处理
├── session/           # 会话相关
│   └── store.go       # 会话存储
├── style/             # 样式定义
│   └── theme.go       # 主题颜色
└── go.mod
```

---

## 依赖

```go
require (
    github.com/charmbracelet/bubbletea v0.27.0
    github.com/charmbracelet/lipgloss v0.12.0
    github.com/charmbracelet/bubbles v0.20.0
)
```

---

## Mac 终端风格设计参考

### 配色方案

```
背景:   #1E1E1E (深灰)
前景:   #FAFAFA (白)
边框:   #3C3C3C (灰)
选中:   #569CD6 (蓝)
```

### 来源标签颜色

```
OpenCode: #86EFAC (绿)
Claude:   #FDBA74 (橙)
Qwen:     #93C5FD (蓝)
```

### 布局

```
┌─────────────────────────────────────────┐
│ 会话管理器                        [刷新] │
├─────────────────────────────────────────┤
│ 📁 项目 A                               │
│   · session-1        opencode        ★ │
│   · session-2        claude             │
├─────────────────────────────────────────┤
│ ⭐ 收藏夹                              │
│   ★ session-1      opencode            │
├─────────────────────────────────────────┤
│ ↑↓ 导航  ⏎ 选择  f 收藏  q 退出       │
└─────────────────────────────────────────┘
```

---

## 待完成功能

1. 会话列表树形结构（按项目分组）
2. 收藏夹面板
3. 键盘导航
4. 来源标签着色
5. 窗口自适应

---

## 参考资料

- Bubble Tea 文档: https://pkg.go.dev/github.com/charmbracelet/bubbletea
- Lip Gloss 文档: https://github.com/charmbracelet/lipgloss
- Bubbles 组件: https://github.com/charmbracelet/bubbles
- Agent Deck (参考实现): https://github.com/asheshgoplani/agent-deck
  - **确认使用 Bubble Tea + tmux**
  - 功能: 会话管理、Fork、MCP 管理、Skills 管理
  - 快捷键丰富，UI 现代

---

## Agent Deck 功能参考

Agent Deck 是一个很好的参考，它展示了 Bubble Tea 能做到什么程度：

### 主要功能
- 会话列表（按项目分组）
- 状态检测 (Running/Waiting/Idle/Error)
- 会话 Fork（复制会话）
- MCP 管理
- Skills 管理
- 搜索功能
- 快捷键支持

### 快捷键
| 按键 | 功能 |
|------|------|
| Enter | 附加到会话 |
| n | 新建会话 |
| f / F | Fork (快速/对话框) |
| m | MCP 管理 |
| / | 搜索 |
| r | 重启会话 |
| d | 删除 |

---

## 下一步

1. **安装 Go** (如果还没装): `brew install go`
2. **创建项目结构**
3. **开始实现核心功能**
