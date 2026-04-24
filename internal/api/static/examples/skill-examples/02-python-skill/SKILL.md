---
name: "文件处理工具"
description: "提供文件读写、处理和格式转换的 Python 技能"
version: "1.0.0"
author: "Fanggo Team"
category: "utilities"
tags: ["file", "python", "utilities"]
runtime:
  runtime_type: "python"
  entry: "main.py"
  version: "3.8+"
tools:
  provided:
    - name: "read_file"
      description: "读取文件内容"
      parameters:
        type: "object"
        properties:
          path:
            type: "string"
            description: "文件路径"
        required:
          - "path"
    - name: "write_file"
      description: "写入文件内容"
      parameters:
        type: "object"
        properties:
          path:
            type: "string"
            description: "文件路径"
          content:
            type: "string"
            description: "文件内容"
        required:
          - "path"
          - "content"
requirements:
  python: []
---

# 文件处理工具

一个提供基本文件操作功能的 Python 技能，支持读写文件内容。
