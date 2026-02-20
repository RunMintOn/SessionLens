# Go TUI Session Manager - Tree View + Favorites 实现计划

## TL;DR

> **核心目标**: 将当前扁平列表的 Go TUI 重写为树形项目分组 + 右侧收藏列表布局，完全对齐 Python 版本功能
>
> **交付物**:
> - 树形项目分组显示（可展开/折叠）
> - 过滤按钮（全部/Claude/OpenCode/Qwen）
> - 右侧收藏面板（SQLite 持久化）
> - 增强的会话恢复（cd 项目目录 + CLI + 退出）
>
> **预计工作量**: 中等（6-8 小时）
> **并行执行**: 是 - 4 个 Wave
> **关键路径**: 数据结构 → 树形渲染 → 键盘导航 → 收藏功能 → 会话恢复

---

## Context

### 原始需求

用户要求 Go 版本完全对齐 Python/Textual 版本的功能：
1. 按项目分组显示会话（树形结构）
2. 左侧显示会话树，右侧显示收藏列表
3. 过滤按钮切换来源
4. Enter 恢复会话时先 cd 到项目目录，再执行 AI CLI，然后退出 TUI

### 当前状态

**已有功能** (✅ 已完成):
- Scanner 模块（OpenCode/Claude/Qwen 三个扫描器）
- 基础 TUI（扁平列表、搜索、窗口自适应）
- 会话恢复代码（但未 cd 项目目录）

**缺失功能** (❌ 待实现):
- 树形项目分组（当前是扁平列表）
- 过滤按钮
- 收藏功能（SQLite 持久化）
- 增强的会话恢复（cd + 退出）

### 研究结果汇总

#### 1. 树形视图实现 (bg_019f4135)

**推荐库**: `github.com/Digital-Shane/treeview`
- 最完整的文件浏览器示例
- Viewport 集成（处理大树滚动）
- 内置文件系统支持
- 搜索和过滤功能
- 可自定义键位绑定

**备选库**:
- `savannahostrowski/tree-bubble` - 简单，基础导航
- `mariusor/bubbles-tree` - 更复杂的状态管理

**关键实现模式**:
```go
// 数据结构
type Node struct {
    Value    string
    Children []Node
    Expanded bool
    Data     interface{}  // 存储 Session
}

// 渲染模式
func renderTree(nodes []Node, indent int) string {
    // 使用树形符号：└── ├── │
    // 递归渲染子节点
}
```

**键盘导航**:
- `↑/k` - 向上移动
- `↓/j` - 向下移动
- `←/h` - 折叠当前项目
- `→/l` - 展开当前项目
- `Enter` - 选择/激活

#### 2. 过滤按钮实现 (websearch)

**参考**: Bubble Tea 官方 tabs 示例
- 没有内置 button 组件
- 使用 Lip Gloss 自定义按钮样式
- 通过键盘事件（数字键/Tab）切换活动按钮

**实现模式**:
```go
type model struct {
    Tabs       []string  // ["全部", "Claude", "OpenCode", "Qwen"]
    activeTab  int       // 当前选中的索引
}

// 渲染按钮行
func renderTabs(tabs []string, active int) string {
    // 使用 lipgloss 渲染每个按钮
    // 活动按钮高亮背景
}
```

#### 3. SQLite 收藏持久化 (bg_1618dd76)

**驱动**: `github.com/glebarez/go-sqlite` (纯 Go，无 CGO)

**数据库路径**: `~/.local/share/agent-session-manager/favorites.db`

**表结构**:
```sql
CREATE TABLE favorites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    source TEXT NOT NULL,
    added_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);
```

**Repository 模式**:
```go
type FavoriteRepository struct {
    db *sql.DB
}

func (r *FavoriteRepository) Add(ctx context.Context, sessionID, source string) error
func (r *FavoriteRepository) Exists(ctx context.Context, sessionID string) (bool, error)
func (r *FavoriteRepository) Remove(ctx context.Context, sessionID string) error
func (r *FavoriteRepository) GetAll(ctx context.Context) ([]Favorite, error)
```

#### 4. 会话恢复进程管理 (bg_a00a2241)

**关键代码**:
```go
cmd := exec.Command("claude", "-r", sessionID)
cmd.Dir = projectPath  // cd 到项目目录
cmd.SysProcAttr = &syscall.SysProcAttr{
    Setpgid: true,  // 新进程组，不继承信号
}
cmd.Stdin = os.Stdin
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr

if err := cmd.Start(); err != nil {
    return m, nil
}

return m, tea.Quit  // 退出 TUI，让 AI CLI 接管终端
```

**正确顺序**:
1. 启动子进程（`cmd.Start()`）
2. 立即退出 TUI（`tea.Quit`）
3. 子进程继承干净的终端状态

#### 5. 双面板布局 (websearch)

**参考**: Bubble Tea tabs 示例 + Lip Gloss
- 使用 `tea.WindowSizeMsg` 获取窗口尺寸
- 用 Lip Gloss `Width()` 设置面板宽度
- 左右面板用字符串拼接（`left + "\n" + right`）

**布局计算**:
```go
func (m model) View() string {
    leftWidth := m.width * 60 / 100
    rightWidth := m.width - leftWidth
    
    leftPanel := leftView(m.sessions).Width(leftWidth)
    rightPanel := favoritesView(m.favorites).Width(rightWidth)
    
    return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}
```

---

## Work Objectives

### 核心目标

用 Go + Bubble Tea 实现与 Python/Textual 版本功能对等的 TUI 会话管理器：
1. 树形项目分组显示
2. 左右双面板布局（会话树 + 收藏列表）
3. 过滤按钮
4. SQLite 收藏持久化
5. 增强的会话恢复（cd + CLI + 退出）

### 具体交付物

1. **cmd/session-manager/main.go** - 重写的主程序（约 400-500 行）
2. **internal/database/favorites.go** - 收藏数据库管理
3. **internal/tree/tree.go** - 树形视图组件
4. **internal/filter/filter.go** - 过滤按钮组件

### 完成定义

- [ ] `go build ./cmd/session-manager` 通过
- [ ] `go test ./...` 所有测试通过
- [ ] 实际运行程序，树形展开/折叠正常
- [ ] 收藏功能工作（添加/删除/持久化）
- [ ] Enter 恢复会话后程序退出，AI CLI 正常启动

### Must Have

1. 树形项目分组（按 `project_path` 分组）
2. 项目节点可展开/折叠（`←/→` 或 `h/l`）
3. 会话节点显示：`[收藏标记] 标题 [来源]`
4. 来源颜色：Claude=橙色，OpenCode=绿色，Qwen=蓝色
5. 过滤按钮（全部/Claude/OpenCode/Qwen）
6. 右侧收藏列表（标题 + 来源 + 路径）
7. `c` 键切换收藏状态
8. SQLite 持久化收藏数据
9. Enter 恢复会话：`cd 项目目录 && AI_CLI -r sessionID`，然后退出

### Must NOT Have (Guardrails)

1. ❌ 不要使用 GORM - 直接用 `database/sql`
2. ❌ 不要实现搜索功能 - 后续再说
3. ❌ 不要实现项目详情面板 - 只做收藏列表
4. ❌ 不要修改现有 scanner 模块 - 只读使用
5. ❌ 不要用 tmux attach - 必须用 AI CLI 命令

---

## Verification Strategy

### 测试决策

- **基础设施存在**: 是（已有 `go test` 框架）
- **自动化测试**: 是（TDD 模式）
- **框架**: `testing` + 标准库（无额外依赖）
- **Agent-Executed QA**: 所有任务必须包含

### QA 策略

每个任务必须包含：
1. 单元测试（如适用）
2. Agent-Executed QA 场景（直接运行验证）

**QA 执行方式**:
- **TUI 组件**: `interactive_bash` 运行程序，发送按键，验证输出
- **数据库**: `bash` 运行测试，验证 SQLite 数据
- **集成**: `bash` 构建并运行，截图验证

---

## Execution Strategy

### 并行执行 Waves

```
Wave 1 (基础架构 - 3 任务并行):
├── Task 1: SQLite 收藏数据库模块 [quick]
├── Task 2: 树形数据结构定义 [quick]
└── Task 3: 过滤按钮组件 [quick]

Wave 2 (核心组件 - 3 任务并行):
├── Task 4: 树形视图渲染器 [deep]
├── Task 5: 树形键盘导航 [unspecified-high]
└── Task 6: 收藏列表渲染器 [quick]

Wave 3 (集成与增强 - 3 任务并行):
├── Task 7: 双面板布局集成 [unspecified-high]
├── Task 8: 收藏切换功能 [quick]
└── Task 9: 增强的会话恢复 [quick]

Wave 4 (验证 - 2 任务并行):
├── Task 10: 集成测试 [deep]
└── Task 11: 手动 QA 验证 [unspecified-high]

Wave FINAL (独立审查 - 4 任务并行):
├── F1: 计划合规审计 (oracle)
├── F2: 代码质量审查 (unspecified-high)
├── F3: 真实手动 QA (unspecified-high)
└── F4: 范围保真检查 (deep)

关键路径：Task 2 → Task 4 → Task 5 → Task 7 → Task 10 → F1-F4
并行加速：~65% 快于顺序执行
最大并发：3 (Waves 1, 2, 3)
```

### 依赖矩阵

| 任务 | 依赖于 | 阻塞
|------|--------|------
| 1-3 | 无 | 4, 5, 6
| 4 | 2 | 7
| 5 | 2, 4 | 7
| 6 | 1 | 7
| 7 | 4, 5, 6 | 10
| 8 | 1, 7 | 10
| 9 | 7 | 10
| 10 | 7, 8, 9 | F1-F4
| 11 | 10 | -
| F1-F4 | 10 | -

### Agent 调度摘要

- **Wave 1**: 3 任务 - T1→`quick`, T2→`quick`, T3→`quick`
- **Wave 2**: 3 任务 - T4→`deep`, T5→`unspecified-high`, T6→`quick`
- **Wave 3**: 3 任务 - T7→`unspecified-high`, T8→`quick`, T9→`quick`
- **Wave 4**: 2 任务 - T10→`deep`, T11→`unspecified-high`
- **FINAL**: 4 任务 - F1→`oracle`, F2→`unspecified-high`, F3→`unspecified-high`, F4→`deep`

---

## TODOs

- [ ] 1. SQLite 收藏数据库模块

  **做什么**:
  - 创建 `internal/database/favorites.go`
  - 实现数据库初始化（创建表）
  - 实现 CRUD 操作（Add/Exists/Remove/GetAll）
  - 使用 XDG 路径（`~/.local/share/agent-session-manager/favorites.db`）

  **必须不做**:
  - 不使用 GORM
  - 不实现迁移系统（后续再说）
  - 不修改现有 scanner 模块

  **推荐 Agent Profile**:
  - **Category**: `quick`
  - **Skills**: 无特殊技能需求
  - **理由**: 标准 SQLite CRUD 模式，研究结果已提供完整代码

  **并行化**:
  - **可并行**: YES
  - **并行组**: Wave 1 (与 Task 2, 3)
  - **阻塞**: Task 6, Task 8
  - **被阻塞于**: 无

  **参考**:
  - `bg_1618dd76` 研究结果 - 完整 CRUD 代码示例
  - `session/opencode.go` - 现有 SQLite 使用模式

  **验收标准**:
  - [ ] `go build ./internal/database` 通过
  - [ ] `go test ./internal/database -v` 通过（至少 4 个测试：Add/Exists/Remove/GetAll）
  - [ ] 数据库文件创建在正确路径

  **QA 场景**:

  ```
  场景：添加收藏
    工具：bash (go test)
    前置：数据库初始化完成
    步骤:
      1. 调用 repo.Add(ctx, "ses_test123", "Claude")
      2. 调用 repo.Exists(ctx, "ses_test123")
    预期结果：Exists 返回 true
    证据：.sisyphus/evidence/task-1-add-favorite-test.txt

  场景：删除收藏
    工具：bash (go test)
    前置：已存在收藏 ses_test123
    步骤:
      1. 调用 repo.Remove(ctx, "ses_test123")
      2. 调用 repo.Exists(ctx, "ses_test123")
    预期结果：Exists 返回 false
    证据：.sisyphus/evidence/task-1-remove-favorite-test.txt
  ```

  **Commit**: YES (与 Task 2, 3 一起)
  - Message: `feat(database): add SQLite favorites persistence`
  - Files: `internal/database/favorites.go`, `internal/database/favorites_test.go`
  - Pre-commit: `go test ./internal/database/...`

- [ ] 2. 树形数据结构定义

  **做什么**:
  - 创建 `internal/tree/types.go`
  - 定义 `TreeNode` 结构（包含 Session 数据）
  - 定义 `TreeModel` 结构（包含展开状态、光标位置）
  - 实现 `GroupByProject()` 函数（将扁平会话列表分组为树）

  **必须不做**:
  - 不实现渲染逻辑
  - 不实现键盘导航
  - 不使用第三方树库（自己实现简单版本）

  **推荐 Agent Profile**:
  - **Category**: `quick`
  - **Skills**: 无
  - **理由**: 纯数据结构定义，无复杂逻辑

  **并行化**:
  - **可并行**: YES
  - **并行组**: Wave 1 (与 Task 1, 3)
  - **阻塞**: Task 4, Task 5
  - **被阻塞于**: 无

  **参考**:
  - `bg_019f4135` 研究结果 - 数据结构模式
  - `session/types.go` - 现有 Session 定义

  **验收标准**:
  - [ ] `go build ./internal/tree` 通过
  - [ ] `GroupByProject()` 正确将 []Session 转换为 map[string][]Session
  - [ ] 单元测试验证分组逻辑

  **QA 场景**:

  ```
  场景：会话分组
    工具：bash (go test)
    前置：准备 5 个测试会话（3 个 project-a，2 个 project-b）
    步骤:
      1. 调用 GroupByProject(sessions)
      2. 验证返回 map 包含 2 个项目
      3. 验证 project-a 有 3 个会话
    预期结果：分组正确
    证据：.sisyphus/evidence/task-2-group-test.txt
  ```

  **Commit**: YES (与 Task 1, 3 一起)

- [ ] 3. 过滤按钮组件

  **做什么**:
  - 创建 `internal/filter/filter.go`
  - 定义 4 个按钮：全部、Claude、OpenCode、Qwen
  - 实现按钮渲染（Lip Gloss 样式）
  - 实现按钮切换逻辑（数字键 1-4 或 Tab）

  **必须不做**:
  - 不使用第三方 button 库
  - 不实现鼠标点击（仅键盘）
  - 不实现按钮禁用状态

  **推荐 Agent Profile**:
  - **Category**: `quick`
  - **Skills**: 无
  - **理由**: 简单 UI 组件，研究结果已提供 tabs 示例

  **并行化**:
  - **可并行**: YES
  - **并行组**: Wave 1 (与 Task 1, 2)
  - **阻塞**: Task 7
  - **被阻塞于**: 无

  **参考**:
  - `websearch` 结果 - Bubble Tea tabs 示例
  - `cmd/session-manager/main.go` - 现有样式定义

  **验收标准**:
  - [ ] `go build ./internal/filter` 通过
  - [ ] 渲染 4 个按钮，活动按钮高亮
  - [ ] 按 1-4 切换活动按钮

  **QA 场景**:

  ```
  场景：按钮渲染
    工具：bash (运行示例程序)
    步骤:
      1. 创建简单测试程序渲染 filter
      2. 截图验证
    预期结果：4 个按钮水平排列，活动按钮有背景色
    证据：.sisyphus/evidence/task-3-buttons-render.png

  场景：按钮切换
    工具：interactive_bash
    步骤:
      1. 运行测试程序
      2. 按 "2" 键
      3. 验证 Claude 按钮高亮
    预期结果：活动按钮正确切换
    证据：.sisyphus/evidence/task-3-buttons-switch.png
  ```

  **Commit**: YES (与 Task 1, 2 一起)

- [ ] 4. 树形视图渲染器

  **做什么**:
  - 创建 `internal/tree/render.go`
  - 实现 `renderTree()` 函数
  - 使用树形符号（└── ├── │）
  - 递归渲染项目和会话
  - 支持展开/折叠状态
  - 应用来源颜色（Claude/Orange, OpenCode/Green, Qwen/Blue）
  - 收藏标记（★/·）

  **必须不做**:
  - 不实现键盘导航（Task 5 负责）
  - 不实现滚动（后续优化）
  - 不使用第三方树库

  **推荐 Agent Profile**:
  - **Category**: `deep`
  - **Skills**: `frontend-design`
  - **理由**: 复杂 UI 渲染逻辑，需要精心设计视觉层次

  **并行化**:
  - **可并行**: NO
  - **并行组**: Wave 2 (在 Task 2 之后)
  - **阻塞**: Task 7
  - **被阻塞于**: Task 2

  **参考**:
  - `bg_019f4135` 研究结果 - 渲染模式代码
  - `main.py:format_label()` - Python 版本标签格式化

  **验收标准**:
  - [ ] `go build ./internal/tree` 通过
  - [ ] 渲染输出包含正确的树形符号
  - [ ] 展开/折叠状态正确显示
  - [ ] 来源颜色正确应用

  **QA 场景**:

  ```
  场景：树形渲染 - 展开状态
    工具：bash (运行测试程序)
    前置：准备 2 个项目，每个项目 2 个会话
    步骤:
      1. 设置 expanded=true
      2. 渲染树
      3. 截图
    预期结果：显示完整树形结构，所有会话可见
    证据：.sisyphus/evidence/task-4-tree-expanded.png

  场景：树形渲染 - 折叠状态
    工具：bash
    前置：同上
    步骤:
      1. 设置 expanded=false
      2. 渲染树
      3. 截图
    预期结果：项目折叠，只显示项目名
    证据：.sisyphus/evidence/task-4-tree-collapsed.png
  ```

  **Commit**: YES (与 Task 5 一起)
  - Message: `feat(tree): implement tree view renderer`
  - Files: `internal/tree/render.go`, `internal/tree/render_test.go`
  - Pre-commit: `go test ./internal/tree/...`

- [ ] 5. 树形键盘导航

  **做什么**:
  - 创建 `internal/tree/navigation.go`
  - 实现 `Update()` 方法处理按键
  - `↑/k` - 向上移动光标
  - `↓/j` - 向下移动光标
  - `←/h` - 折叠当前项目
  - `→/l` - 展开当前项目
  - 跟踪扁平化的可见节点列表（用于光标导航）
  - 处理边界情况（第一个/最后一个节点）

  **必须不做**:
  - 不实现页面滚动（后续优化）
  - 不实现 Home/End 键
  - 不实现搜索跳转

  **推荐 Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: 无
  - **理由**: 复杂的状态管理和边界处理

  **并行化**:
  - **可并行**: NO
  - **并行组**: Wave 2 (在 Task 2, 4 之后)
  - **阻塞**: Task 7
  - **被阻塞于**: Task 2, Task 4

  **参考**:
  - `bg_019f4135` 研究结果 - 键盘导航模式
  - `cmd/session-manager/main.go:Update()` - 现有导航逻辑

  **验收标准**:
  - [ ] `go build ./internal/tree` 通过
  - [ ] 光标在可见节点间正确移动
  - [ ] 展开/折叠操作正确更新状态
  - [ ] 边界检查正确（不会越界）

  **QA 场景**:

  ```
  场景：向下导航
    工具：interactive_bash
    前置：启动程序，3 个项目各 2 个会话
    步骤:
      1. 按 "j" 5 次
      2. 验证光标位置
    预期结果：光标移动到第 6 个可见节点
    证据：.sisyphus/evidence/task-5-nav-down.png

  场景：折叠项目
    工具：interactive_bash
    前置：光标在项目节点上
    步骤:
      1. 按 "h"
      2. 验证项目折叠，子节点隐藏
    预期结果：项目折叠，光标移动到下一个可见节点
    证据：.sisyphus/evidence/task-5-collapse.png

  场景：展开项目
    工具：interactive_bash
    前置：光标在折叠的项目节点上
    步骤:
      1. 按 "l"
      2. 验证项目展开，子节点显示
    预期结果：项目展开，子节点可见
    证据：.sisyphus/evidence/task-5-expand.png
  ```

  **Commit**: YES (与 Task 4 一起)

- [ ] 6. 收藏列表渲染器

  **做什么**:
  - 创建 `internal/favorites/render.go`
  - 实现 `renderFavorites()` 函数
  - 空状态显示 "No favorites yet"
  - 有收藏时显示：`★ 标题 \n [来源] · 简化路径`
  - 路径简化（`/home/lee/project` → `~/project`）

  **必须不做**:
  - 不实现滚动（收藏数量通常较少）
  - 不实现收藏内导航（只读显示）
  - 不实现收藏分组

  **推荐 Agent Profile**:
  - **Category**: `quick`
  - **Skills**: 无
  - **理由**: 简单列表渲染

  **并行化**:
  - **可并行**: NO
  - **并行组**: Wave 2 (在 Task 1 之后)
  - **阻塞**: Task 7
  - **被阻塞于**: Task 1

  **参考**:
  - `main.py:update_favorites()` - Python 版本收藏渲染
  - `bg_1618dd76` 研究结果 - 获取收藏数据

  **验收标准**:
  - [ ] `go build ./internal/favorites` 通过
  - [ ] 空状态正确显示
  - [ ] 收藏列表格式正确

  **QA 场景**:

  ```
  场景：空收藏列表
    工具：bash
    前置：数据库无收藏
    步骤:
      1. 调用 renderFavorites([])
      2. 验证输出
    预期结果：显示 "No favorites yet"
    证据：.sisyphus/evidence/task-6-favorites-empty.txt

  场景：有收藏列表
    工具：bash
    前置：数据库有 3 个收藏
    步骤:
      1. 调用 renderFavorites(favorites)
      2. 验证格式
    预期结果：每行显示 "★ 标题 [来源] · 路径"
    证据：.sisyphus/evidence/task-6-favorites-list.txt
  ```

  **Commit**: YES (独立)
  - Message: `feat(favorites): implement favorites list renderer`
  - Files: `internal/favorites/render.go`
  - Pre-commit: `go test ./internal/favorites/...`

- [ ] 7. 双面板布局集成

  **做什么**:
  - 重写 `cmd/session-manager/main.go` 的 `model` 结构
  - 集成树形视图（左侧 60%）
  - 集成收藏列表（右侧 40%）
  - 集成过滤按钮（左侧顶部）
  - 处理 `tea.WindowSizeMsg` 动态计算宽度
  - 使用 `lipgloss.JoinHorizontal()` 拼接左右面板

  **必须不做**:
  - 不修改 scanner 模块
  - 不实现搜索功能
  - 不实现面板 resize 拖拽

  **推荐 Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: `frontend-design`
  - **理由**: 复杂布局集成，需要协调多个组件

  **并行化**:
  - **可并行**: NO
  - **并行组**: Wave 3 (在 Task 4, 5, 6 之后)
  - **阻塞**: Task 8, Task 9, Task 10
  - **被阻塞于**: Task 4, Task 5, Task 6

  **参考**:
  - `websearch` 结果 - 双面板布局模式
  - `main.py:compose()` - Python 版本布局
  - `cmd/session-manager/main.go:View()` - 现有渲染逻辑

  **验收标准**:
  - [ ] `go build ./cmd/session-manager` 通过
  - [ ] 左右面板正确显示（60%/40%）
  - [ ] 窗口 resize 时自适应
  - [ ] 过滤按钮在左侧顶部

  **QA 场景**:

  ```
  场景：双面板渲染
    工具：interactive_bash
    步骤:
      1. 运行程序
      2. 截图验证布局
    预期结果：左侧会话树，右侧收藏列表，中间有分隔
    证据：.sisyphus/evidence/task-7-layout.png

  场景：窗口 resize
    工具：interactive_bash
    步骤:
      1. 运行程序
      2. resize 终端窗口
      3. 验证布局自适应
    预期结果：面板宽度按比例调整
    证据：.sisyphus/evidence/task-7-resize.png
  ```

  **Commit**: YES
  - Message: `feat(layout): integrate two-panel layout with tree and favorites`
  - Files: `cmd/session-manager/main.go`
  - Pre-commit: `go build ./cmd/session-manager`

- [ ] 8. 收藏切换功能

  **做什么**:
  - 在 `main.go:Update()` 中添加 `c` 键处理
  - 调用数据库 Add/Remove
  - 刷新树形视图（更新收藏标记）
  - 刷新收藏列表

  **必须不做**:
  - 不修改数据库接口
  - 不实现批量操作
  - 不实现收藏排序

  **推荐 Agent Profile**:
  - **Category**: `quick`
  - **Skills**: 无
  - **理由**: 简单的事件处理

  **并行化**:
  - **可并行**: NO
  - **并行组**: Wave 3 (在 Task 7 之后)
  - **阻塞**: Task 10
  - **被阻塞于**: Task 7, Task 1

  **参考**:
  - `main.py:action_toggle_favorite()` - Python 版本收藏切换
  - `internal/database/favorites.go` - Task 1 实现的数据库

  **验收标准**:
  - [ ] `go build ./cmd/session-manager` 通过
  - [ ] 按 `c` 键切换收藏状态
  - [ ] 收藏标记实时更新

  **QA 场景**:

  ```
  场景：添加收藏
    工具：interactive_bash
    前置：选中一个未收藏的会话
    步骤:
      1. 按 "c"
      2. 验证会话前显示 ★
      3. 验证右侧收藏列表出现该会话
    预期结果：收藏状态正确更新
    证据：.sisyphus/evidence/task-8-add-favorite.png

  场景：移除收藏
    工具：interactive_bash
    前置：选中一个已收藏的会话
    步骤:
      1. 按 "c"
      2. 验证会话前显示 ·
      3. 验证右侧收藏列表移除该会话
    预期结果：收藏状态正确移除
    证据：.sisyphus/evidence/task-8-remove-favorite.png
  ```

  **Commit**: YES
  - Message: `feat(favorites): add toggle favorite functionality`
  - Files: `cmd/session-manager/main.go`
  - Pre-commit: `go build ./cmd/session-manager`

- [ ] 9. 增强的会话恢复

  **做什么**:
  - 修改 `main.go:Update()` 的 Enter 处理
  - 先 `cd` 到项目目录
  - 执行 AI CLI 命令（`claude -r` / `opencode -s` / `qwen -r`）
  - 使用 `syscall.SysProcAttr{Setpgid: true}` 分离进程
  - 调用 `tea.Quit` 退出 TUI

  **必须不做**:
  - 不使用 tmux attach
  - 不等待进程完成
  - 不显示错误提示（静默失败）

  **推荐 Agent Profile**:
  - **Category**: `quick`
  - **Skills**: 无
  - **理由**: 简单但关键的进程管理

  **并行化**:
  - **可并行**: NO
  - **并行组**: Wave 3 (在 Task 7 之后)
  - **阻塞**: Task 10
  - **被阻塞于**: Task 7

  **参考**:
  - `bg_a00a2241` 研究结果 - 完整进程管理代码
  - `main.py:action_restore_session()` - Python 版本恢复逻辑

  **验收标准**:
  - [ ] `go build ./cmd/session-manager` 通过
  - [ ] Enter 后程序退出
  - [ ] AI CLI 在正确目录启动

  **QA 场景**:

  ```
  场景：恢复 OpenCode 会话
    工具：interactive_bash
    前置：选中一个 OpenCode 会话
    步骤:
      1. 按 "Enter"
      2. 验证程序退出
      3. 验证 opencode -r ses_xxx 启动
    预期结果：TUI 退出，OpenCode 在正确目录恢复会话
    证据：.sisyphus/evidence/task-9-restore-opencode.txt

  场景：恢复 Claude 会话
    工具：interactive_bash
    前置：选中一个 Claude 会话
    步骤:
      1. 按 "Enter"
      2. 验证程序退出
      3. 验证 claude -r ses_xxx 启动
    预期结果：TUI 退出，Claude 在正确目录恢复会话
    证据：.sisyphus/evidence/task-9-restore-claude.txt
  ```

  **Commit**: YES
  - Message: `feat(restore): enhance session restore with cd and process detachment`
  - Files: `cmd/session-manager/main.go`
  - Pre-commit: `go build ./cmd/session-manager`

- [ ] 10. 集成测试

  **做什么**:
  - 创建 `cmd/session-manager/main_test.go`
  - 测试完整流程：启动 → 导航 → 收藏 → 恢复
  - 模拟数据库操作
  - 验证 UI 状态

  **必须不做**:
  - 不测试外部 CLI（mock）
  - 不测试真实终端交互

  **推荐 Agent Profile**:
  - **Category**: `deep`
  - **Skills**: 无
  - **理由**: 复杂集成测试需要仔细设计

  **并行化**:
  - **可并行**: NO
  - **并行组**: Wave 4 (在所有功能完成后)
  - **阻塞**: Task 11, F1-F4
  - **被阻塞于**: Task 7, Task 8, Task 9

  **参考**:
  - `session/*_test.go` - 现有测试模式
  - `bg_1618dd76` 研究结果 - 测试用 `:memory:` SQLite

  **验收标准**:
  - [ ] `go test ./cmd/session-manager -v` 通过
  - [ ] 至少 5 个集成测试用例

  **QA 场景**:

  ```
  场景：运行集成测试
    工具：bash
    步骤:
      1. go test ./cmd/session-manager -v -cover
      2. 验证所有测试通过
    预期结果：测试通过率 100%
    证据：.sisyphus/evidence/task-10-integration-tests.txt
  ```

  **Commit**: YES
  - Message: `test: add integration tests for session manager`
  - Files: `cmd/session-manager/main_test.go`
  - Pre-commit: `go test ./cmd/session-manager/...`

- [ ] 11. 手动 QA 验证

  **做什么**:
  - 实际运行程序
  - 手动测试所有功能
  - 截图记录
  - 验证与 Python 版本功能对等

  **必须不做**:
  - 不自动化（纯手动）
  - 不跳过任何功能点

  **推荐 Agent Profile**:
  - **Category**: `unspecified-high`
  - **Skills**: `playwright` (如需浏览器验证，但此处不需要)
  - **理由**: 需要真实终端环境验证

  **并行化**:
  - **可并行**: NO
  - **并行组**: Wave 4 (在 Task 10 之后)
  - **阻塞**: F1-F4
  - **被阻塞于**: Task 10

  **参考**:
  - `.sisyphus/drafts/go-tui-vision.md` - 预想效果文档

  **验收标准**:
  - [ ] 所有功能手动验证通过
  - [ ] 截图保存到 `.sisyphus/evidence/task-11-qa/`

  **QA 场景**:

  ```
  场景：完整用户流程
    工具：interactive_bash
    步骤:
      1. 运行 ./session-manager
      2. 用方向键导航
      3. 按 ←/→ 展开/折叠项目
      4. 按 c 切换收藏
      5. 按 1-4 切换过滤
      6. 按 Enter 恢复会话
    预期结果：所有功能正常工作
    证据：.sisyphus/evidence/task-11-qa/full-flow.gif
  ```

  **Commit**: NO

---

## Final Verification Wave

> 4 个审查 Agent 并行执行，必须全部通过

- [ ] F1. **计划合规审计** — `oracle`
  逐条阅读 "Must Have" 列表，验证每个需求都已实现
  逐条阅读 "Must NOT Have" 列表，搜索代码确认无违反
  检查证据文件：`.sisyphus/evidence/task-{N}-*`
  输出：`Must Have [9/9] | Must NOT Have [5/5] | Tasks [11/11] | VERDICT: APPROVE/REJECT`

- [ ] F2. **代码质量审查** — `unspecified-high`
  运行 `tsc --noEmit` + linter + `go test ./...`
  检查：`as any`/`@ts-ignore` 等价物、空 catch、未使用 import
  检查 AI slop：过度注释、过度抽象、通用变量名
  输出：`Build [PASS/FAIL] | Tests [N pass] | Files [N clean] | VERDICT`

- [ ] F3. **真实手动 QA** — `unspecified-high`
  从干净状态启动程序
  执行每个任务的 QA 场景
  测试跨任务集成（树形 + 收藏 + 恢复 协同工作）
  测试边界情况：空项目、空收藏、快速按键
  输出：`Scenarios [N/N pass] | Integration [N/N] | Edge Cases [N tested] | VERDICT`

- [ ] F4. **范围保真检查** — `deep`
  对每个任务：读 "What to do"，读实际 diff
  验证 1:1 实现（无遗漏，无 creep）
  检查跨任务污染：Task N 修改 Task M 的文件
  输出：`Tasks [N/N compliant] | Contamination [CLEAN/N issues] | VERDICT`

---

## Commit Strategy

- **1-3**: `feat(database): add SQLite favorites + tree types + filter buttons`
  - Files: `internal/database/favorites.go`, `internal/tree/types.go`, `internal/filter/filter.go`
  - Pre-commit: `go test ./internal/...`

- **4-5**: `feat(tree): implement tree view renderer + navigation`
  - Files: `internal/tree/render.go`, `internal/tree/navigation.go`
  - Pre-commit: `go test ./internal/tree/...`

- **6**: `feat(favorites): implement favorites list renderer`
  - Files: `internal/favorites/render.go`
  - Pre-commit: `go test ./internal/favorites/...`

- **7**: `feat(layout): integrate two-panel layout with tree and favorites`
  - Files: `cmd/session-manager/main.go`
  - Pre-commit: `go build ./cmd/session-manager`

- **8**: `feat(favorites): add toggle favorite functionality`
  - Files: `cmd/session-manager/main.go`
  - Pre-commit: `go build ./cmd/session-manager`

- **9**: `feat(restore): enhance session restore with cd and process detachment`
  - Files: `cmd/session-manager/main.go`
  - Pre-commit: `go build ./cmd/session-manager`

- **10**: `test: add integration tests for session manager`
  - Files: `cmd/session-manager/main_test.go`
  - Pre-commit: `go test ./cmd/session-manager/...`

---

## Success Criteria

### 验证命令

```bash
# 构建
go build ./cmd/session-manager  # Expected: 无错误

# 测试
go test ./... -v  # Expected: 所有测试通过

# 运行
./session-manager  # Expected: TUI 正常启动

# 功能验证
# 1. 树形展开/折叠正常
# 2. 收藏切换正常
# 3. Enter 恢复会话后程序退出
```

### 最终检查清单

- [ ] 所有 "Must Have" 已实现
- [ ] 所有 "Must NOT Have" 未违反
- [ ] 所有测试通过
- [ ] 证据文件完整（`.sisyphus/evidence/task-*`）
- [ ] 代码无 AI slop
- [ ] 与 Python 版本功能对等

---

## 附录：文件结构

```
cmd/session-manager/
├── main.go              # 重写的主程序（约 400-500 行）
└── main_test.go         # 集成测试

internal/
├── database/
│   ├── favorites.go     # SQLite 收藏数据库
│   └── favorites_test.go
├── tree/
│   ├── types.go         # 树形数据结构
│   ├── render.go        # 树形渲染
│   ├── navigation.go    # 键盘导航
│   └── render_test.go
├── filter/
│   └── filter.go        # 过滤按钮组件
└── favorites/
    └── render.go        # 收藏列表渲染

.sisyphus/
├── drafts/go-tui-vision.md         # 预想效果文档
├── plans/go-tui-tree-favorites.md  # 本计划
└── evidence/
    ├── task-{N}-{scenario}.*       # QA 证据
    └── task-11-qa/                 # 手动 QA 截图
```

---

**计划版本**: 1.0  
**创建日期**: 2026-02-20  
**最后更新**: 2026-02-20
