# Claude Code 会话发现 SOP

## 📋 概述

本文档说明如何从 Claude Code 本地存储中发现并显示会话列表，与 CC 原生界面保持一致。

---

## 🔍 核心发现

### Claude Code 显示规则

通过对比分析，发现 Claude Code 在 `/resume` 命令中**只显示包含用户消息的会话**。

**过滤规则：**
```
显示条件：会话文件中至少包含一条 `type: "user"` 的记录
```

### 数据验证

| 会话文件 | 是否有 user 消息 | CC 是否显示 |
|---------|----------------|------------|
| b2b64f56-....jsonl | ✅ 是 | ✅ 显示 |
| 2e617480-....jsonl | ✅ 是 | ✅ 显示 |
| 5d5ef635-....jsonl | ✅ 是 | ✅ 显示 |
| ba5cf441-....jsonl | ✅ 是 | ✅ 显示 |
| 4537f0cc-....jsonl | ✅ 是 | ✅ 显示 |
| 884965d5-....jsonl | ❌ 否 (只有 system) | ❌ 不显示 |
| 75e47e7e-....jsonl | ❌ 否 (只有 snapshot) | ❌ 不显示 |

---

## 📁 存储结构

### 目录位置
```
~/.claude/projects/
```

### 路径编码规则
```
原始路径：/home/lee
编码目录：-home-lee

原始路径：/home/lee/11MyProjrct/langchain/study
编码目录：-home-lee-11MyProjrct-langchain-study
```

### 会话文件命名
```
{sessionId}.jsonl
例如：2e617480-8fb7-461e-968b-96628baa811e.jsonl
```

---

## 📄 JSONL 文件格式

### 用户消息记录
```json
{
  "type": "user",
  "uuid": "e9d5773b-453a-44cd-9844-03cba16e39f1",
  "sessionId": "2e617480-8fb7-461e-968b-96628baa811e",
  "message": {
    "role": "user",
    "content": "https://github.com/QwenLM/qwen-code\n\n呃，你先看一下这个项目是干什么的"
  },
  "timestamp": "2026-02-17T07:30:18.729Z",
  "cwd": "/home/lee",
  "gitBranch": "HEAD"
}
```

### 系统消息记录（不用于标题提取）
```json
{
  "type": "system",
  "subtype": "local_command",
  "content": "<command-name>/resume</command-name>",
  ...
}
```

### 助手消息记录
```json
{
  "type": "assistant",
  "uuid": "ad04e4d2-d044-4970-aa1e-b990a980bbf2",
  "parentUuid": "e9d5773b-453a-44cd-9844-03cba16e39f1",
  "message": {
    "role": "assistant",
    "content": [...]
  },
  ...
}
```

---

## 🛠️ 实现步骤

### Step 1: 扫描项目目录
```bash
# 找到所有项目目录
find ~/.claude/projects/ -type d -not -name ".*"
```

### Step 2: 扫描会话文件
```bash
# 找到所有 JSONL 文件（排除子代理）
find ~/.claude/projects/ -name "*.jsonl" -not -path "*/subagents/*"
```

### Step 3: 过滤有效会话
```python
def has_user_message(filepath):
    """检查文件是否包含用户消息"""
    with open(filepath, 'r') as f:
        for line in f:
            data = json.loads(line)
            if data.get('type') == 'user':
                return True
    return False
```

### Step 4: 提取会话标题
```python
def get_session_title(filepath):
    """
    提取会话标题 - 模仿 CC 原生行为
    1. 跳过系统消息 (local-command-caveat, local-command-stdout)
    2. 如果有 command-name，提取命令名作为标题 (/xxx)
    3. 否则取第一条真正的用户输入
    """
    command_name = None
    with open(filepath, 'r') as f:
        for line in f:
            data = json.loads(line)
            if data.get('type') == 'user':
                content = data.get('message', {}).get('content', '')
                
                if isinstance(content, str):
                    content = content.strip()
                    
                    # 跳过 local-command-caveat
                    if content.startswith('<local-command-caveat>'):
                        continue
                    
                    # 跳过 local-command-stdout
                    if content.startswith('<local-command-stdout>'):
                        continue
                    
                    # 提取 command-name
                    if content.startswith('<command-name>'):
                        start = content.find('>') + 1
                        end = content.find('</command-name>')
                        if start > 0 and end > start:
                            command_name = content[start:end].strip()
                        continue
                    
                    # 这是真正的用户输入
                    return content[:80]
                
                elif isinstance(content, list):
                    # 多部分内容，提取文本
                    for item in content:
                        if isinstance(item, dict) and item.get('type') == 'text':
                            text = item.get('text', '')
                            if text and not text.startswith('<'):
                                return text[:80]
    
    # 如果没有用户输入但有 command-name，返回命令名
    if command_name:
        return '/' + command_name
    
    return "(无标题)"
```

### Step 5: 获取元数据
```python
def get_session_metadata(filepath):
    """获取会话元数据"""
    import os
    from datetime import datetime
    
    # 文件大小 (KB)
    size_kb = os.path.getsize(filepath) / 1024
    
    # 修改时间
    mtime = datetime.fromtimestamp(os.path.getmtime(filepath))
    
    # 读取 gitBranch（从第一条 user 或 assistant 消息）
    git_branch = "HEAD"
    with open(filepath, 'r') as f:
        for line in f:
            data = json.loads(line)
            if data.get('type') in ('user', 'assistant'):
                git_branch = data.get('gitBranch', 'HEAD')
                break
    
    return {
        'size_kb': size_kb,
        'mtime': mtime,
        'git_branch': git_branch
    }
```

### Step 6: 格式化显示
```
显示格式（与 CC 原生一致）:
{标题}
{时间} · {gitBranch} · {大小}

示例:
https://github.com/QwenLM/qwen-code 呃，你先看一下这个项目是干什么的
3 days ago · HEAD · 164.9 KB

说一声你好
3 days ago · HEAD · 1.0 KB

/clear
3 days ago · HEAD · 2.5 KB
```

---

## 📊 完整示例代码

```python
#!/usr/bin/env python3
"""
Claude Code Session Discovery Tool
发现并显示所有有效的会话（与 CC 原生/resume 一致）
"""

import json
import os
from pathlib import Path
from datetime import datetime

def get_first_user_message(filepath):
    """获取第一条用户消息内容"""
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            for line in f:
                data = json.loads(line.strip())
                if data.get('type') == 'user':
                    content = data.get('message', {}).get('content', '')
                    if isinstance(content, list):
                        for item in content:
                            if isinstance(item, dict) and item.get('type') == 'text':
                                return item.get('text', '')
                    elif isinstance(content, str):
                        return content
    except Exception as e:
        return None
    return None

def get_git_branch(filepath):
    """获取 git 分支"""
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            for line in f:
                data = json.loads(line.strip())
                if data.get('type') in ('user', 'assistant'):
                    return data.get('gitBranch', 'HEAD')
    except:
        pass
    return 'HEAD'

def format_time_ago(dt):
    """格式化为 'X days ago'"""
    now = datetime.now()
    diff = now - dt
    days = diff.days
    if days == 0:
        return "today"
    elif days == 1:
        return "yesterday"
    else:
        return f"{days} days ago"

def discover_sessions(cwd_path):
    """发现指定工作目录的所有会话"""
    # 编码项目路径
    encoded_path = cwd_path.replace('/', '-').strip('-')
    if cwd_path.startswith('/'):
        encoded_path = '-' + encoded_path
    
    projects_dir = Path.home() / '.claude' / 'projects'
    session_dir = projects_dir / encoded_path
    
    if not session_dir.exists():
        return []
    
    sessions = []
    for jsonl_file in session_dir.glob('*.jsonl'):
        # 跳过子代理
        if 'subagents' in str(jsonl_file):
            continue
        
        # 检查是否有用户消息
        title = get_first_user_message(jsonl_file)
        if not title:
            continue  # 跳过没有用户消息的会话
        
        # 获取元数据
        mtime = datetime.fromtimestamp(jsonl_file.stat().st_mtime)
        size_kb = jsonl_file.stat().st_size / 1024
        git_branch = get_git_branch(jsonl_file)
        
        sessions.append({
            'file': jsonl_file,
            'title': title,
            'mtime': mtime,
            'size_kb': size_kb,
            'git_branch': git_branch
        })
    
    # 按时间倒序排序
    sessions.sort(key=lambda x: x['mtime'], reverse=True)
    return sessions

def display_sessions(sessions):
    """显示会话列表"""
    for session in sessions:
        # 截断长标题
        title = session['title']
        if len(title) > 100:
            title = title[:100] + '...'
        
        # 替换多行标题为单行
        title = ' '.join(title.split())
        
        time_ago = format_time_ago(session['mtime'])
        size_str = f"{session['size_kb']:.1f} KB"
        
        print(f"\n{title}")
        print(f"{time_ago} · {session['git_branch']} · {size_str}")

# 主程序
if __name__ == '__main__':
    cwd = '/home/lee'
    sessions = discover_sessions(cwd)
    print(f"找到 {len(sessions)} 条会话\n")
    print("=" * 60)
    display_sessions(sessions)
```

---

## ✅ 验证清单

- [ ] 只扫描 `~/.claude/projects/` 目录
- [ ] 跳过 `subagents/` 子目录
- [ ] 过滤掉没有 `type: "user"` 记录的文件
- [ ] 提取第一条用户消息作为标题
- [ ] 按修改时间倒序排序
- [ ] 显示时间（X days ago 格式）
- [ ] 显示 git 分支（HEAD）
- [ ] 显示文件大小（KB）

---

## 🔧 常见问题

### Q1: 为什么有些会话不显示？
A: 只有包含 `type: "user"` 记录的会话才会显示。纯系统命令（如 `/resume` 取消）不会显示。

### Q2: 标题太长怎么办？
A: 建议截断到 80-100 字符，与 CC 原生行为一致。

### Q3: 多行标题如何处理？
A: 将换行符替换为空格，显示为单行。

### Q4: 如何区分不同项目？
A: 通过编码的目录名区分，如 `-home-lee` 对应 `/home/lee`。

---

## 📝 版本信息

- 最后更新：2026-02-21
- Claude Code 版本：2.1.44+
- 存储格式：JSONL
