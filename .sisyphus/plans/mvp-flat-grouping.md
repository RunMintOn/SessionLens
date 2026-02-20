# MVP: 扁平分组 + 隐藏功能

> **目标**: 在现有 Go 代码基础上，实现最小可行性产品（MVP）  
> **核心功能**: 扁平分组显示 + 隐藏会话功能  
> **开发策略**: 渐进式修改，保持现有代码可运行  
> **预计工作量**: 2-3 小时

---

## 📊 当前状态 vs MVP 目标

### 当前状态（现有代码）

**布局**:
```
┌─────────────────────────────────────┐
│  AGENT SESSION MANAGER              │
├─────────────────────────────────────┤
│  Sessions:                          │
│                                     │
│  > ses_001  OpenCode  (标题)        │
│    ses_002  Claude   (标题)         │
│    ses_003  Qwen     (标题)         │
│                                     │
├─────────────────────────────────────┤
│  ↑/k up  ↓/j down  Enter attach  q  │
└─────────────────────────────────────┘
```

**特点**:
- ✅ 扁平列表显示
- ✅ 搜索功能（`/` 键）
- ✅ 键盘导航（↑↓jk）
- ✅ 会话恢复（Enter）
- ❌ 无项目分组
- ❌ 显示会话 ID（不够简洁）
- ❌ 无隐藏功能

---

### MVP 目标

**布局**:
```
┌─────────────────────────────────────────────────────────────────────────┐
│  AGENT SESSION MANAGER                               2026-02-20 15:30  │
├─────────────────────────────────────────────────────────────────────────┤
│  [全部] [Claude] [OpenCode] [Qwen]                                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ~/11MyProjrct/langchain-study                                           │
│    > 向量数据库优化方案 [Claude]                                          │
│    · RAG 实现细节 [OpenCode]                                              │
│                                                                          │
│  ~/11MyProjrct/tui-agentSessionManager                                   │
│    · Scanner 模块实现 [OpenCode]                                          │
│                                                                          │
├─────────────────────────────────────────────────────────────────────────┤
│  ↑/k 上  ↓/j 下  Enter 进入/恢复  c 收藏  h 隐藏  H 隐藏列表  / 搜索  q 退出  │
└─────────────────────────────────────────────────────────────────────────┘
```

**新增功能**:
- ✅ 项目路径分组（扁平分组）
- ✅ 不显示会话 ID（只显示标题）
- ✅ 来源标签（颜色）
- ✅ 收藏标记（★/·）
- ✅ 隐藏功能（h/H）
- ✅ 过滤按钮（全部/Claude/OpenCode/Qwen）

---

## 🎯 MVP 范围界定

### Must Have（本次实现）

1. **扁平分组显示**
   - 按 `ProjectPath` 分组
   - 项目路径作为分组标题
   - 会话缩进显示

2. **简洁显示**
   - 不显示会话 ID
   - 格式：`标题 [来源]`
   - 来源颜色：Claude=橙，OpenCode=绿，Qwen=蓝

3. **隐藏功能**
   - 按 `h` 隐藏当前会话
   - 按 `H` 查看隐藏列表
   - 按 `r` 恢复会话
   - 隐藏状态持久化（JSON 文件）

4. **基础过滤**
   - 按来源过滤（全部/Claude/OpenCode/Qwen）
   - 快捷键：`1-4`

### Nice to Have（后续迭代）

- ⏸️ 收藏功能（`c` 键）
- ⏸️ 搜索功能增强（已有基础搜索）
- ⏸️ 窗口 resize 自适应（已有基础实现）

### Out of Scope（本次不做）

- ❌ 树形展开/折叠
- ❌ 项目节点可选中
- ❌ 批量操作
- ❌ 自动隐藏

---

## 📋 开发任务

### Wave 1: 数据结构 + 分组逻辑（30 分钟）

**Task 1.1: 扩展 Session 结构**
```go
// session/types.go
type Session struct {
    ID          string
    Title       string
    SourceTool  SourceType
    ProjectPath string
    LastUpdated int64
    IsHidden    bool  // ← 新增
}
```

**Task 1.2: 实现分组函数**
```go
// session/scanner.go
type GroupedSessions struct {
    ProjectPath string
    Sessions    []Session
}

func GroupByProject(sessions []Session) []GroupedSessions {
    // 按 ProjectPath 分组
    // 返回分组后的列表
}
```

**Task 1.3: 隐藏列表管理**
```go
// internal/hidden/manager.go
type HiddenManager struct {
    hiddenFile string  // ~/.local/share/agent-session-manager/hidden-sessions.json
}

func (m *HiddenManager) Add(sessionID string) error
func (m *HiddenManager) Remove(sessionID string) error
func (m *HiddenManager) IsHidden(sessionID string) bool
func (m *HiddenManager) GetAll() ([]string, error)
```

**验收**:
- [ ] `go build ./...` 通过
- [ ] 分组函数单元测试通过
- [ ] 隐藏管理单元测试通过

---

### Wave 2: UI 渲染（45 分钟）

**Task 2.1: 修改 View 函数 - 分组渲染**
```go
// cmd/session-manager/main.go:View()
func (m model) View() string {
    var s string
    
    // 遍历分组
    for _, group := range m.groupedSessions {
        // 渲染项目路径标题
        s += renderProjectTitle(group.ProjectPath)
        
        // 渲染会话列表
        for _, sess := range group.Sessions {
            s += renderSession(sess, m.cursor)
        }
        
        s += "\n" // 分组间距
    }
    
    return s
}
```

**Task 2.2: 渲染项目路径标题**
```go
func renderProjectTitle(path string) string {
    // 简化路径：/home/lee/project -> ~/project
    simplified := simplifyPath(path)
    return lipgloss.NewStyle().Bold(true).Render(simplified) + "\n"
}
```

**Task 2.3: 渲染会话行**
```go
func renderSession(sess Session, cursor int) string {
    // 格式：`  [光标] [收藏标记] 标题 [来源]`
    cursor := "  "
    if m.cursor == i {
        cursor = "> "
    }
    
    favMarker := "·"
    if sess.IsFavorite {
        favMarker = "★"
    }
    
    sourceColor := getSourceColor(sess.SourceTool)
    
    line := fmt.Sprintf("%s%s %s [%s]",
        cursor,
        favMarker,
        sess.Title,
        sourceColor.Render(string(sess.SourceTool)),
    )
    
    return itemStyle.Render(line) + "\n"
}
```

**验收**:
- [ ] 项目路径正确显示（简化格式）
- [ ] 会话按项目分组
- [ ] 来源颜色正确
- [ ] 光标显示正确

---

### Wave 3: 隐藏功能（45 分钟）

**Task 3.1: 添加隐藏状态**
```go
// cmd/session-manager/main.go
type model struct {
    cursor         int
    sessions       []session.Session
    groupedSessions []GroupedSessions
    hiddenManager  *hidden.Manager  // ← 新增
    showHiddenList bool             // ← 新增：是否显示隐藏列表
    // ...
}
```

**Task 3.2: 实现 h 键处理**
```go
// cmd/session-manager/main.go:Update()
case "h":
    // 隐藏当前选中的会话
    sess := m.sessions[m.cursor]
    m.hiddenManager.Add(sess.ID)
    
    // 从列表中移除
    m.sessions = removeSession(m.sessions, m.cursor)
    m.groupedSessions = session.GroupByProject(m.sessions)
    
    return m, nil
```

**Task 3.3: 实现 H 键处理（显示隐藏列表）**
```go
case "H":
    // 显示隐藏列表覆盖层
    m.showHiddenList = true
    m.hiddenCursor = 0
    return m, nil
```

**Task 3.4: 隐藏列表 UI**
```go
func (m model) View() string {
    if m.showHiddenList {
        return m.renderHiddenList()
    }
    
    // 正常渲染主界面
    return m.renderMainView()
}

func (m model) renderHiddenList() string {
    // 渲染覆盖层
    // 显示所有隐藏的会话
    // 支持 r（恢复）和 a（全部恢复）
}
```

**Task 3.5: 恢复功能**
```go
case "r":
    if m.showHiddenList {
        // 恢复选中的隐藏会话
        hiddenID := m.hiddenSessions[m.hiddenCursor]
        m.hiddenManager.Remove(hiddenID)
        
        // 重新扫描，会话重新出现
        m.sessions = scanAll()
        m.groupedSessions = session.GroupByProject(m.sessions)
    }
    return m, nil
```

**验收**:
- [ ] 按 `h` 隐藏会话
- [ ] 按 `H` 显示隐藏列表
- [ ] 按 `r` 恢复会话
- [ ] 隐藏状态持久化

---

### Wave 4: 过滤功能（30 分钟）

**Task 4.1: 添加过滤状态**
```go
type model struct {
    // ...
    filter string  // "all", "claude", "opencode", "qwen"
}
```

**Task 4.2: 渲染过滤按钮**
```go
func (m model) renderFilterButtons() string {
    buttons := []string{"全部", "Claude", "OpenCode", "Qwen"}
    var s string
    
    for i, btn := range buttons {
        style := inactiveStyle
        if m.filter == getFilterKey(i) {
            style = activeStyle
        }
        s += style.Render(" " + btn + " ")
    }
    
    return s
}
```

**Task 4.3: 实现过滤逻辑**
```go
// cmd/session-manager/main.go:Update()
case "1":
    m.filter = "all"
    m.sessions = filterSessions(m.allSessions, "all")
    m.groupedSessions = session.GroupByProject(m.sessions)
    return m, nil
case "2":
    m.filter = "claude"
    // ...
```

**Task 4.4: 更新 View 函数**
```go
func (m model) View() string {
    // 1. 渲染标题
    header := renderTitle()
    
    // 2. 渲染过滤按钮
    filterRow := m.renderFilterButtons()
    
    // 3. 渲染分组会话
    sessionList := m.renderGroupedSessions()
    
    // 4. 渲染底部帮助
    footer := renderFooter()
    
    return header + "\n" + filterRow + "\n" + sessionList + "\n" + footer
}
```

**验收**:
- [ ] 过滤按钮正确显示
- [ ] 按 `1-4` 切换过滤
- [ ] 过滤后会话列表正确更新

---

### Wave 5: 集成测试 + 优化（30 分钟）

**Task 5.1: 完整流程测试**
```bash
# 1. 构建
go build ./cmd/session-manager

# 2. 运行
./session-manager

# 3. 测试导航
↑/j 向下，k/↑向上

# 4. 测试隐藏
选中会话 → 按 h → 按 H → 按 r

# 5. 测试过滤
按 1/2/3/4 切换过滤
```

**Task 5.2: 性能优化**
- 缓存分组结果
- 优化隐藏列表加载
- 减少重复渲染

**Task 5.3: 错误处理**
- 隐藏文件不存在时创建
- JSON 解析错误处理
- 空列表处理

**验收**:
- [ ] 所有功能正常工作
- [ ] 无明显性能问题
- [ ] 错误处理优雅

---

## 📁 文件结构

### 修改的文件

```
cmd/session-manager/main.go        ← 主要修改（+200 行）
session/types.go                   ← 添加 IsHidden 字段
session/scanner.go                 ← 添加 GroupByProject 函数
```

### 新增的文件

```
internal/hidden/manager.go         ← 隐藏功能管理（+100 行）
internal/hidden/manager_test.go    ← 单元测试
```

### 总计

- **新增代码**: ~300 行
- **修改代码**: ~100 行
- **测试代码**: ~100 行
- **总计**: ~500 行

---

## ⏱️ 时间估算

| Wave | 任务 | 预计时间 |
|------|------|----------|
| **Wave 1** | 数据结构 + 分组逻辑 | 30 分钟 |
| **Wave 2** | UI 渲染 | 45 分钟 |
| **Wave 3** | 隐藏功能 | 45 分钟 |
| **Wave 4** | 过滤功能 | 30 分钟 |
| **Wave 5** | 集成测试 + 优化 | 30 分钟 |
| **总计** | | **3 小时** |

---

## ✅ 验收标准

### 功能验收

- [ ] 启动后显示扁平分组的会话列表
- [ ] 项目路径作为分组标题（简化格式：`~/project`）
- [ ] 不显示会话 ID（只显示标题）
- [ ] 来源标签颜色正确
- [ ] 按 `h` 隐藏选中的会话
- [ ] 按 `H` 查看隐藏列表
- [ ] 按 `r` 恢复隐藏会话
- [ ] 按 `1-4` 切换过滤
- [ ] 隐藏状态持久化

### 视觉验收

- [ ] Mac 终端风格（圆角边框、深色主题）
- [ ] 项目路径分组清晰
- [ ] 来源颜色正确
- [ ] 无闪烁、无错位

### 性能验收

- [ ] 启动时间 <500ms
- [ ] 键盘响应 <100ms
- [ ] 隐藏操作 <50ms

---

## 🚀 开始开发

**准备执行**:
```bash
/start-work mvp-flat-grouping
```

**开发原则**:
1. 保持现有代码可运行
2. 渐进式修改，每步验证
3. 先实现功能，再优化视觉
4. 测试驱动，确保质量

**准备好了吗？** 🎯
