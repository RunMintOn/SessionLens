# 修复计划：UI 显示优化

## TL;DR

> 修复 TUI 界面显示问题：
> 1. 移除顶层"会话管理器"节点
> 2. 禁用会话节点的展开/收起功能
> 3. 来源标签改用浅色小字显示（不用 []）

---

## Context

### 原始问题
- 顶层"会话管理器"占用横向空间
- 会话节点有展开箭头（无意义）
- 来源用 `[]` 括号显示，土

### 配色方案（浅色）
| 来源 | 颜色 | 十六进制 |
|------|------|----------|
| OpenCode | 浅绿色 | `#86EFAC` |
| Qwen | 浅蓝色 | `#93C5FD` |
| Claude | 浅橙色 | `#FDBA74` |

---

## Context

### 原始问题
- 顶层"会话管理器"占用横向空间
- 会话节点有展开箭头（无意义）
- 来源标签 [opencode] 被截断

### 调研发现
- Textual Tree 组件默认所有节点都可展开
- 标签过长被截断，需要更多横向空间或换组件

---

## Work Objectives

### 核心目标
修复界面显示问题，提升用户体验

### 具体修改
1. 移除顶层"会话管理器"节点，直接显示项目
2. 项目节点可展开/收起，会话节点禁用展开
3. 来源标签完整显示（调整宽度或换组件）

---

## Execution Strategy

### 任务分解

```
Task 1: 移除顶层节点 + 禁用会话展开
  - 修改 main.py tree 构建逻辑
  - 会话节点不允许展开

Task 2: 调整显示宽度/换组件
  - 方案A: 换用 ListView 组件（更灵活）
  - 方案B: 调整 Tree 样式，增加宽度
```

---

## TODOs

- [x] 1. 移除顶层"会话管理器" + 禁用会话展开

  **What to do**:
  - 修改 `_refresh_tree_display` 方法
  - 不再添加顶层节点，直接添加项目节点
  - 项目节点可展开，会话节点用 `allow_expand=False`

  **QA Scenarios**:

  Scenario: 验证界面显示
    Tool: Bash
    Preconditions: main.py 已修改
    Steps:
      1. Run `timeout 3 python3 main.py`
      2. Verify no top-level "会话管理器" node
      3. Verify sessions don't have expand arrows
    Expected Result: 顶层节点消失，会话无展开箭头

  **Commit**: YES

---

- [x] 2. 验证来源标签显示完整

  **What to do**:
  - 运行程序检查来源标签是否完整显示
  - 如仍有问题，考虑换组件

  **QA Scenarios**:

  Scenario: 验证来源显示
    Tool: Bash
    Preconditions: Task 1 完成
    Steps:
      1. Run TUI
      2. Check if [opencode], [claude], [qwen] tags are visible
    Expected Result: 来源标签完整显示

  **Commit**: YES

---

## Success Criteria

- [x] 无顶层"会话管理器"节点
- [x] 会话节点无展开箭头
- [x] 来源标签完整显示
