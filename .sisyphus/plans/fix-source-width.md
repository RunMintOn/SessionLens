# 修复：来源列显示不全

## 问题

`self.app.size.width` 返回的不是实际可用宽度，导致来源列被截断。

## 修复方案

使用 Tree 组件的 `size.width` 获取实际可用宽度：

```python
# 在 _refresh_tree_display 中修改：
tree = self.query_one("#sessions-tree")
terminal_width = tree.size.width
```

## 验证

- [x] OpenCode 显示完整 (opencode)
- [x] Claude 显示完整 (claude)  
- [x] Qwen 显示完整 (qwen)
- [x] 拖动窗口时实时更新
