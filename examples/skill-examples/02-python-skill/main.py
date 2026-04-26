#!/usr/bin/env python3
"""文件处理工具主入口"""

import sys
import json


def read_file(path):
    """读取文件内容"""
    try:
        with open(path, "r", encoding="utf-8") as f:
            return {"output": f.read(), "is_error": False}
    except Exception as e:
        return {"output": str(e), "is_error": True}


def write_file(path, content):
    """写入文件内容"""
    try:
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)
        return {"output": "File written successfully", "is_error": False}
    except Exception as e:
        return {"output": str(e), "is_error": True}


def main():
    if len(sys.argv) < 2:
        print(json.dumps({"output": "Usage: python main.py <tool_name> [params_json]", "is_error": True}))
        return

    tool_name = sys.argv[1]
    params = {}

    if len(sys.argv) > 2:
        try:
            params = json.loads(sys.argv[2])
        except json.JSONDecodeError:
            print(json.dumps({"output": "Invalid JSON params", "is_error": True}))
            return

    if tool_name == "read_file":
        result = read_file(params.get("path", ""))
    elif tool_name == "write_file":
        result = write_file(params.get("path", ""), params.get("content", ""))
    else:
        result = {"output": f"Unknown tool: {tool_name}", "is_error": True}

    print(json.dumps(result))


if __name__ == "__main__":
    main()
