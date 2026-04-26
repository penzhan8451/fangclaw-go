---
name: "JSON 处理工具"
description: "提供 JSON 解析、格式化和验证的 Node.js 技能"
version: "1.0.0"
author: "Fanggo Team"
category: "utilities"
tags: ["json", "nodejs", "utilities"]
runtime:
  runtime_type: "node"
  entry: "index.js"
  version: "14+"
tools:
  provided:
    - name: "format_json"
      description: "格式化 JSON 字符串"
      parameters:
        type: "object"
        properties:
          input:
            type: "string"
            description: "需要格式化的 JSON 字符串"
          indent:
            type: "number"
            description: "缩进空格数（默认 2）"
            default: 2
        required:
          - "input"
    - name: "validate_json"
      description: "验证 JSON 是否有效"
      parameters:
        type: "object"
        properties:
          input:
            type: "string"
            description: "需要验证的 JSON 字符串"
        required:
          - "input"
requirements:
  node: []
---

# JSON 处理工具

一个提供 JSON 解析、格式化和验证功能的 Node.js 技能。
