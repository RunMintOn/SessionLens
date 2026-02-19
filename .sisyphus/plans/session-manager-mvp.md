# AI 编程助手会话管理器 - 开发计划

## TL;DR

> **快速摘要**: 开发一个 TUI 会话管理器，聚合 Claude Code、OpenCode、Qwen Code 三个平台的会话，以项目为第一级展示，支持一键恢复会话。

> **交付物**:
> - Python + Textual TUI 应用
> - 树状展示：项目 → 会话
> - 左右分栏：左侧会话列表，右侧收藏展示
> - 一键恢复：选中会话按 Enter 恢复

> **预估工作量**: Short（3-5 个核心任务）
> **并行执行**: YES - 2 waves
> **关键路径**: 任务1 → 任务2 → 任务3 → 任务5

---

## Context

### 原始需求
用户希望开发一个轻量级的 AI 编程助手会话管理器，主要解决多平台会话分散难以查找的问题。

### 访谈总结
**关键讨论**:
- 界面设计: 左右分栏，左侧所有会话（按项目分组），右侧收藏
- 展示方式: 项目路径简化显示（~/project-a），会话标题 + 来源标签
- 键盘操作: 上下选择，Enter 恢复，c 收藏，q 退出，r 刷新
- 过滤器: [全部] [Claude] [OpenCode] [Qwen]

**调研发现**:
- OpenCode: SQLite DB，session 表有 id, title, directory, time_updated
- Claude Code: JSONL 文件，sessionId, cwd, type, message.content
- Qwen Code: JSONL 文件，type, message.parts[].text

### Metis 审查
**识别的边界情况**（已处理）:
- 空状态处理: 显示 "No sessions found"
- 工具不存在: 扫描时检测命令，禁用不可用工具的恢复
- 超长标题: 截断处理
- 权限错误: 跳过并警告

---

## Work Objectives

### 核心目标
实现一个可运行的 TUI 会话管理器 MVP，功能包括：
1. 扫描三个平台的会话数据
2. 以项目为第一级展示会话
3. 支持一键恢复会话
4. 左右分栏界面

### 具体交付物
- `main.py` - TUI 主程序
- `scanner/` - 会话扫描模块
- `models.py` - 数据模型
- `config.json` - 配置文件

### 定义完成
- [ ] TUI 界面可正常渲染
- [ ] 三个平台的会话都能正确显示
- [ ] 上下键可以切换选中项
- [ ] Enter 可以恢复选中的会话
- [ ] q 键可以退出程序
- [ ] r 键可以刷新会话列表

### Must Have
- Python + Textual TUI 框架
- 三个平台的会话扫描器
- 树状展示（项目 → 会话）
- 一键恢复功能

### Must NOT Have（Guardrails）
- ❌ Session 删除功能
- ❌ Session 搜索功能
- ❌ Session 元数据编辑
- ❌ 导出/导入功能
- ❌ 自动刷新

---

## Verification Strategy

### Test Decision
- **Infrastructure exists**: NO
- **Automated tests**: NO
- **Framework**: None（MVP 阶段不做自动化测试）
- **Agent QA**: YES - 每个任务完成后，Agent 直接运行程序验证功能

### QA Policy
每个任务必须包含 Agent-Executed QA Scenarios：
- 启动程序验证 TUI 渲染
- 测试会话加载
- 测试键盘交互
- 测试恢复命令

---

## Execution Strategy

### 并行执行 Waves

```
Wave 1 (立即开始 — 基础设施):
├── Task 1: 项目初始化 + 依赖安装
├── Task 2: 数据模型定义
├── Task 3: 会话扫描器（三个平台）
└── Task 4: 配置管理

Wave 2 (After Wave 1 — 核心功能):
├── Task 5: TUI 主界面开发
├── Task 6: 键盘交互实现
├── Task 7: 恢复功能实现
└── Task 8: Agent QA 验证

Critical Path: Task1 → Task2 → Task3 → Task5 → Task6 → Task7
```

---

## TODOs

- [x] 1. 项目初始化 + 依赖安装

  **What to do**:
  - 创建 Python 项目结构
  - 安装依赖: textual, sqlite3 (内置)
  - 创建 pyproject.toml 或 requirements.txt
  - 验证 Textual 安装成功

  **Must NOT do**:
  - 不需要创建复杂的项目结构
  - 不需要 CI/CD 配置

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 基础设置任务，简单直接
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 2, 3, 4
  - **Blocked By**: None (可以开始)

  **References**:
  - Textual 官方文档: https://textual.textualize.io/

  **Acceptance Criteria**:
  - [ ] `python -c "from textual.app import App"` 无报错
  - [ ] 项目目录结构创建完成

  **QA Scenarios**:

  Scenario: 验证项目初始化
    Tool: Bash
    Preconditions: 无
    Steps:
      1. Run `python -c "from textual.app import App"` to verify textual is installed
      2. Check project directory structure exists
    Expected Result: 无报错，目录结构正确
    Evidence: .sisyphus/evidence/task-1-init.{ext}

  **Commit**: NO

---

- [x] 2. 数据模型定义

  **What to do**:
  - 定义 Session 模型: id, title, source_tool, project_path, last_updated
  - 定义 Project 模型: path, sessions[]
  - 定义 Favorite 模型: session_id, added_at
  - 创建 models.py

  **Must NOT do**:
  - 不需要复杂的 ORM
  - 不需要数据库迁移

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 定义数据模型，简单直接

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 3, 5
  - **Blocked By**: Task 1

  **References**:

  **Acceptance Criteria**:
  - [ ] models.py 包含 Session, Project 类
  - [ ] 可以实例化 Session 对象

  **QA Scenarios**:

  Scenario: 验证数据模型
    Tool: Bash
    Preconditions: models.py 已创建
    Steps:
      1. Run `python -c "from models import Session; s = Session('test', 'title', 'claude', '/path', 123); print(s.id)"`
    Expected Result: 输出 'test'
    Evidence: .sisyphus/evidence/task-2-models.{ext}

  **Commit**: YES (with Task 1)

---

- [x] 3. 会话扫描器开发

  **What to do**:
  - 实现 OpenCodeScanner: 读取 SQLite DB
  - 实现 ClaudeCodeScanner: 读取 JSONL 文件
  - 实现 QwenCodeScanner: 读取 JSONL 文件
  - 实现统一接口: scan() -> List[Session]
  - 标题提取逻辑:
    - OpenCode: 直接从 session.title 读取
    - Claude: 从第一条 type="user" 的 message.content 提取
    - Qwen: 从第一条用户消息提取

  **Must NOT do**:
  - 不修改任何会话文件
  - 不做网络请求

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: 需要解析不同格式的会话文件，逻辑较复杂
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES (三个扫描器可以并行开发)
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 5
  - **Blocked By**: Task 2

  **References**:
  - OpenCode: `~/.local/share/opencode/opencode.db`, session 表
  - Claude: `~/.claude/projects/<dir>/<session-id>.jsonl`
  - Qwen: `~/.qwen/projects/-<path>/chats/<session-id>.jsonl`

  **Acceptance Criteria**:
  - [ ] 可以扫描到 OpenCode 会话
  - [ ] 可以扫描到 Claude Code 会话
  - [ ] 可以扫描到 Qwen Code 会话

  **QA Scenarios**:

  Scenario: 验证会话扫描
    Tool: Bash
    Preconditions: 扫描器已实现
    Steps:
      1. Run scanner for each platform
      2. Print session count and first session title
    Expected Result: 打印出各平台的会话数量和标题
    Evidence: .sisyphus/evidence/task-3-scanner.{ext}

  **Commit**: YES

---

- [x] 4. 配置管理

  **What to do**:
  - 定义配置结构: 工具路径、收藏存储路径
  - 实现 Config 类
  - 收藏存储: SQLite 表 (session_id, source, added_at)

  **Must NOT do**:
  - 不需要复杂的配置项
  - 不需要配置 UI

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 简单的配置管理

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1
  - **Blocks**: Task 5, 6
  - **Blocked By**: Task 1

  **References**:

  **Acceptance Criteria**:
  - [ ] Config 类可实例化
  - [ ] favorites.db 可创建

  **QA Scenarios**:

  Scenario: 验证配置
    Tool: Bash
    Preconditions: config.py 已创建
    Steps:
      1. Run `python -c "from config import Config; c = Config(); print(c.data_dir)"`
    Expected Result: 输出配置路径
    Evidence: .sisyphus/evidence/task-4-config.{ext}

  **Commit**: YES

---

- [x] 5. TUI 主界面开发

  **What to do**:
  - 使用 Textual 实现左右分栏布局
  - 左侧: 会话列表（按项目分组）
  - 右侧: 收藏列表
  - 实现项目折叠/展开功能
  - 过滤器实现: [全部] [Claude] [OpenCode] [Qwen]
  - 标题截断处理（超长省略号）

  **Must NOT do**:
  - 不需要复杂的动画效果
  - 不需要自定义主题

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: TUI 界面开发，需要布局和样式
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 6, 7
  - **Blocked By**: Task 3, 4

  **References**:
  - Textual Layout: https://textual.textualize.io/guide/layout/
  - Textual Widgets: https://textual.textualize.io/guide/widgets/

  **Acceptance Criteria**:
  - [ ] 左右分栏界面渲染成功
  - [ ] 会话按项目分组显示
  - [ ] 过滤器按钮可点击/可切换

  **QA Scenarios**:

  Scenario: 验证 TUI 渲染
    Tool: Bash
    Preconditions: main.py 已实现
    Steps:
      1. Run `timeout 5 python main.py` or launch and capture screenshot
      2. Verify left panel shows sessions grouped by project
      3. Verify right panel shows favorites section
    Expected Result: TUI 正确渲染，无报错
    Evidence: .sisyphus/evidence/task-5-tui.{ext}

  **Commit**: YES

---

- [x] 6. 键盘交互实现

  **What to do**:
  - 上下键: 选择会话/项目
  - Enter: 恢复选中的会话
  - c: 收藏/取消收藏选中会话
  - q: 退出程序
  - r: 刷新会话列表
  - 过滤器快捷键: 1/2/3/4 或点击

  **Must NOT do**:
  - 不需要鼠标支持（除非简单）
  - 不需要复杂的光标移动

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: 需要绑定键盘事件到 TUI 组件

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 8
  - **Blocked By**: Task 5

  **References**:

  **Acceptance Criteria**:
  - [ ] 上下键可以切换选中项
  - [ ] q 键可以退出
  - [ ] r 键可以刷新

  **QA Scenarios**:

  Scenario: 验证键盘交互
    Tool: interactive_bash
    Preconditions: TUI 已运行
    Steps:
      1. Launch TUI
      2. Press up/down arrow - verify selection changes
      3. Press q - verify app exits
    Expected Result: 键盘响应正确
    Evidence: .sisyphus/evidence/task-6-keyboard.{ext}

  **Commit**: YES

---

- [x] 7. 恢复功能实现

  **What to do**:
  - 实现恢复命令构建:
    - Claude: `claude -r <session-id>`
    - OpenCode: `opencode -s <session-id>`
    - Qwen: `qwen -r <session-id>`
  - 检测工具是否可用（PATH 中存在）
  - 工具不可用时禁用恢复或提示
  - 按 Enter 执行恢复命令

  **Must NOT do**:
  - 不等待命令执行完成（启动后退出）
  - 不处理命令输出

  **Recommended Agent Profile**:
  - **Category**: `quick`
    - Reason: 调用系统命令，简单直接

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2
  - **Blocks**: Task 8
  - **Blocked By**: Task 5

  **References**:
  - Claude: `claude -r <session-id>`
  - OpenCode: `opencode -s <session-id>`
  - Qwen: `qwen -r <session-id>`

  **Acceptance Criteria**:
  - [ ] 选中 Claude 会话 + Enter → 执行 `claude -r xxx`
  - [ ] 选中 OpenCode 会话 + Enter → 执行 `opencode -s xxx`
  - [ ] 选中 Qwen 会话 + Enter → 执行 `qwen -r xxx`

  **QA Scenarios**:

  Scenario: 验证恢复功能
    Tool: interactive_bash
    Preconditions: 键盘交互已实现
    Steps:
      1. Select a session in TUI
      2. Press Enter
      3. Verify correct command is executed (check process list or output)
    Expected Result: 正确的恢复命令被执行
    Evidence: .sisyphus/evidence/task-7-resume.{ext}

  **Commit**: YES

---

- [x] 8. Agent QA 验证

  **What to do**:
  - 运行完整程序
  - 验证所有功能正常工作
  - 测试边界情况（空状态、工具不存在等）

  **Must NOT do**:
  - 不需要写自动化测试代码
  - 不需要性能测试

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: 全面测试和验证

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 2 (Final)
  - **Blocks**: None
  - **Blocked By**: Task 6, 7

  **References**:

  **Acceptance Criteria**:
  - [ ] 程序可正常启动
  - [ ] 三个平台的会话都能显示
  - [ ] 键盘交互正常
  - [ ] 恢复功能正常

  **QA Scenarios**:

  Scenario: 完整功能验证
    Tool: interactive_bash
    Preconditions: 所有任务完成
    Steps:
      1. Launch the application
      2. Verify sessions are displayed
      3. Test keyboard navigation
      4. Test resume with Enter
      5. Test quit with q
    Expected Result: 所有功能正常
    Evidence: .sisyphus/evidence/task-8-qa.{ext}

  **Commit**: YES (if fixes needed)

---

## Final Verification Wave

- [ ] F1. **Plan Compliance Audit** — `oracle`
  验证所有 Must Have 都已实现，Must NOT Have 未实现

- [ ] F2. **Code Quality Review** — `unspecified-high`
  检查代码质量，无明显问题

- [ ] F3. **Real Manual QA** — `unspecified-high`
  手动测试所有功能

---

## Commit Strategy

- **Wave 1**: `feat: 项目初始化和数据模型`
- **Wave 2**: `feat: TUI 界面和交互功能`

---

## Success Criteria

### Verification Commands
```bash
python main.py  # 启动 TUI
```

### Final Checklist
- [ ] 所有 Must Have 存在
- [ ] 所有 Must NOT Have 不存在
- [ ] 三个平台会话都能扫描
- [ ] 键盘交互正常
- [ ] 恢复命令正确执行
