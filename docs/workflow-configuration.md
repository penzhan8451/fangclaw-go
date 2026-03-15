# Workflow 配置指南

## 概述

Workflow 是由多个步骤（Step）组成的自动化流水线，每个步骤会运行一个 Agent 并执行指定的任务。步骤的输出可以传递给下一个步骤作为输入。

## 基本结构

一个 Workflow JSON 文件的基本结构如下：

```json
{
  "id": "wf-1234567890",
  "name": "My Workflow",
  "description": "This is a sample workflow",
  "steps": [
    // 步骤配置
  ],
  "created_at": "2026-03-12T17:42:07.60332+08:00"
}
```

## 步骤（Step）配置

每个步骤包含以下字段：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 步骤名称 |
| `agent` | object | 是 | Agent 配置 |
| `prompt_template` | string | 是 | 提示模板（使用 Handlebars 语法） |
| `mode` | string \| object | 是 | 执行模式 |
| `timeout_secs` | number | 否 | 超时时间（秒），默认 120 |
| `error_mode` | string \| object | 否 | 错误处理模式，默认 "fail" |
| `output_var` | string | 否 | 输出变量名 |

### Agent 配置

Agent 可以通过 ID 或名称指定：

```json
// 通过名称指定
"agent": {
  "name": "Data Analyst"
}

// 或通过 ID 指定
"agent": {
  "id": "agent-123"
}
```

## 执行模式（Mode）

### 1. Sequential（顺序执行）

默认模式，步骤按顺序执行。

```json
"mode": "sequential"
```

### 2. Fan Out（并行执行）

步骤并行执行多个实例。

```json
"mode": "fan_out"
```

### 3. Conditional（条件执行）

根据条件决定是否执行步骤。

```json
"mode": {
  "condition": "steps.analyze.output.includes('ERROR')"
}
```

**条件表达式语法：**
- 使用 JavaScript 表达式
- 可以访问前面步骤的输出：`steps.step_name.output`
- 可以使用标准的 JavaScript 比较运算符和函数

**示例：**
```javascript
// 检查输出是否包含特定字符串
steps.analyze.output.includes('需要修复')

// 检查输出长度
steps.summarize.output.length > 100

// 使用正则表达式
/ERROR/i.test(steps.check.output)

// 多条件组合
steps.analyze.output.includes('错误') && steps.analyze.output.includes('严重')
```

### 4. Loop（循环执行）

重复执行步骤直到满足条件或达到最大迭代次数。

```json
"mode": {
  "max_iterations": 5,
  "until": "steps.review.output.includes('满意')"
}
```

## 错误处理模式（Error Mode）

### 1. Fail（失败停止）

默认模式，步骤失败时停止整个 Workflow。

```json
"error_mode": "fail"
```

### 2. Skip（跳过继续）

步骤失败时跳过该步骤，继续执行后续步骤。

```json
"error_mode": "skip"
```

### 3. Retry（重试）

步骤失败时自动重试。

```json
"error_mode": {
  "max_retries": 3
}
```

## Handlebars 提示模板语法

提示模板使用 Handlebars 模板引擎，可以动态引用前面步骤的输出。

### 基本变量引用

```
{{input}}
```

引用 Workflow 的输入参数。

### 引用步骤输出

```
{{steps.step_name.output}}
```

引用指定步骤的输出。

### 条件判断

```handlebars
{{#if steps.fix.output}}
  {{steps.fix.output}}
{{else}}
  {{steps.analyze.output}}
{{/if}}
```

### 循环

```handlebars
{{#each steps.results}}
  - {{this}}
{{/each}}
```

### 辅助函数

Handlebars 提供了一些内置辅助函数：

```handlebars
{{#unless steps.error}}
  执行成功
{{/unless}}

{{#with steps.data}}
  {{name}}: {{value}}
{{/with}}
```

## 完整示例

### 示例 1：简单顺序流程

```json
{
  "name": "简单内容处理",
  "description": "分析并总结用户输入",
  "steps": [
    {
      "name": "analyze",
      "agent": {
        "name": "Data Analyst"
      },
      "prompt_template": "请分析以下内容：{{input}}",
      "mode": "sequential",
      "timeout_secs": 120,
      "error_mode": "fail"
    },
    {
      "name": "summarize",
      "agent": {
        "name": "Writer"
      },
      "prompt_template": "请总结以下分析结果：{{steps.analyze.output}}",
      "mode": "sequential",
      "timeout_secs": 120,
      "error_mode": "fail"
    }
  ]
}
```

### 示例 2：条件执行流程

```json
{
  "name": "条件修复流程",
  "description": "分析内容，有错误则修复",
  "steps": [
    {
      "name": "analyze",
      "agent": {
        "name": "Data Analyst"
      },
      "prompt_template": "请分析以下内容，判断是否包含错误或需要改进的地方：{{input}}",
      "mode": "sequential",
      "timeout_secs": 120,
      "error_mode": "fail"
    },
    {
      "name": "fix-if-needed",
      "agent": {
        "name": "Writer"
      },
      "prompt_template": "根据以下分析结果，修复问题：{{steps.analyze.output}}",
      "mode": {
        "condition": "steps.analyze.output.includes('ERROR')"
      },
      "timeout_secs": 120,
      "error_mode": "fail"
    },
    {
      "name": "finalize",
      "agent": {
        "name": "Writer"
      },
      "prompt_template": "请生成最终输出。如果有修复后的内容，使用修复后的内容；否则使用原始分析：{{#if steps.fix-if-needed.output}}{{steps.fix-if-needed.output}}{{else}}{{steps.analyze.output}}{{/if}}",
      "mode": "sequential",
      "timeout_secs": 120,
      "error_mode": "fail"
    }
  ]
}
```

### 示例 3：循环执行流程

```json
{
  "name": "内容优化循环",
  "description": "反复优化直到满意",
  "steps": [
    {
      "name": "draft",
      "agent": {
        "name": "Writer"
      },
      "prompt_template": "请为以下主题写一篇文章：{{input}}",
      "mode": "sequential",
      "timeout_secs": 120,
      "error_mode": "fail"
    },
    {
      "name": "review-and-improve",
      "agent": {
        "name": "Data Analyst"
      },
      "prompt_template": "请评审以下文章，提出改进建议：{{#if steps.review-and-improve.output}}{{steps.review-and-improve.output}}{{else}}{{steps.draft.output}}{{/if}}",
      "mode": {
        "max_iterations": 3,
        "until": "steps.review-and-improve.output.includes('满意')"
      },
      "timeout_secs": 120,
      "error_mode": {
        "max_retries": 2
      }
    }
  ]
}
```

## 前端创建 Workflow

你也可以通过前端 Dashboard 的 Workflows 页面创建 Workflow：

1. 点击 "New Workflow" 按钮
2. 填写 Workflow 名称和描述
3. 添加步骤：
   - 填写步骤名称
   - 指定 Agent 名称
   - 选择执行模式
   - 选择错误处理模式
   - 填写提示模板
   - 对于 conditional 模式，填写条件表达式
   - 对于 loop 模式，填写最大迭代次数和终止条件
   - 对于 retry 错误模式，填写最大重试次数
4. 点击 "Create" 创建 Workflow

## 注意事项

1. **步骤名称**：步骤名称在 Workflow 中必须唯一，用于在提示模板中引用。
2. **条件表达式**：条件表达式使用 JavaScript 语法，请确保语法正确。
3. **超时设置**：根据任务复杂度合理设置超时时间，避免长时间等待。
4. **错误处理**：对于关键步骤建议使用 "fail" 模式，对于非关键步骤可以使用 "skip" 或 "retry" 模式。
5. **提示模板**：提示模板中引用的步骤必须在当前步骤之前执行。
