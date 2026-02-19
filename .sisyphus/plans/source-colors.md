# 修复：来源颜色显示 + 自适应对齐 + 实时渲染

## 问题

1. ~~颜色问题~~ ✅ 已解决
2. ~~对齐问题~~ ✅ 基本解决
3. **新问题：实时渲染** - 拖动窗口时不会重新渲染
4. **新问题：对齐微调** - 树缩进字符宽度计算不准确

## 修复方案

### 1. 添加 on_resize 事件监听

在 `SessionManagerApp` 类中添加：

```python
def on_resize(self, event: events.Resize) -> None:
    """窗口大小变化时重新渲染"""
    self._refresh_tree_display()
```

### 2. 修正缩进宽度计算

当前: `left_padding = 4` (固定值)
修正: 根据实际树深度计算缩进

```python
def format_label(title: str, source: str, is_favorite: bool, terminal_width: int, tree_depth: int = 2) -> Text:
    # 树深度会影响左边缩进
    # depth 0: ├──  (3 字符)
    # depth 1: │   ├── (6 字符)
    # depth 2: │   │   ├── (9 字符)
    left_padding = 3 + (tree_depth - 1) * 3  # 动态计算
    ...
```

## 验证

- [x] 拖动窗口时实时更新（resize 事件触发）
- [x] 不同深度节点对齐正确
