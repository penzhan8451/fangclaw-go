# 技能示例

此目录包含各种类型技能的完整示例，帮助你快速上手技能开发。

## 目录

- [01-prompt-only-skill](01-prompt-only-skill/SKILL.md) - 仅使用提示词的技能（无需代码）
- [02-python-skill](02-python-skill/SKILL.md) - Python 语言技能示例
- [03-nodejs-skill](03-nodejs-skill/SKILL.md) - Node.js 语言技能示例

## 技能结构

所有技能都包含以下基本结构：

```
skill-name/           # ⚠️ 目录名（技能 ID）必须使用英文！只能包含 a-z, A-Z, 0-9, -, _
├── SKILL.md           # 技能配置和文档（必须）
├── main.py            # Python 技能入口（Python 类型）
├── index.js           # Node.js 技能入口（Node 类型）
└── ...                # 其他依赖文件
```

### 重要提示：

1. **目录名（技能 ID）**：必须使用英文字符、数字、连字符或下划线，不能包含中文或特殊字符！
2. **SKILL.md 中的 name 字段**：可以使用中文或其他语言，这是显示给用户的技能名称

## SKILL.md Frontmatter 配置

### 基本配置

```yaml
---
name: "技能名称"
description: "技能描述"
version: "1.0.0"
author: "作者名称"
category: "分类"
tags: ["标签1", "标签2"]
---
```

### 运行时配置

#### Prompt-only 类型（仅提示词）

```yaml
runtime:
  runtime_type: "prompt_only"
```

#### Python 类型

```yaml
runtime:
  runtime_type: "python"
  entry: "main.py"       # 入口文件
  version: "3.8+"        # Python 版本要求
```

#### Node.js 类型

```yaml
runtime:
  runtime_type: "node"
  entry: "index.js"      # 入口文件
  version: "14+"         # Node.js 版本要求
```

### 工具配置

```yaml
tools:
  provided:
    - name: "tool_name"
      description: "工具描述"
      parameters:
        type: "object"
        properties:
          param1:
            type: "string"
            description: "参数1说明"
        required:
          - "param1"
```

### 依赖要求

```yaml
requirements:
  python:
    - "requests>=2.0"
  node:
    - "lodash^4.0"
  system:
    - "git"
```

## 使用示例

### 安装技能

将技能目录复制到你的技能目录（通常是 `~/.fangclaw-go/skills/`），然后在界面上刷新或重启服务即可。

### 调用 Python 技能

```bash
# 调用工具
python main.py read_file '{"path": "/tmp/test.txt"}'
```

### 调用 Node.js 技能

```bash
# 调用工具
node index.js format_json '{"input": "{\"a\": 1}", "indent": 4}'
```

## 工具返回格式

所有技能工具返回必须是以下 JSON 格式：

```json
{
  "output": "结果内容",
  "is_error": false
}
```

错误时返回：

```json
{
  "output": "错误信息",
  "is_error": true
}
```

## 分类参考

常见的技能分类：

- `productivity` - 生产力工具
- `utilities` - 实用工具
- `coding` - 编码和开发工具
- `search` - 搜索和研究工具
- `ai` - AI 和机器学习工具
- `data` - 数据处理工具
- `security` - 安全工具
- `communication` - 沟通工具
- `media` - 媒体工具

## 更多信息

查看项目文档了解完整的技能开发指南。
