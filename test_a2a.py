#!/usr/bin/env python3
"""
A2A 功能完整测试脚本
包含：
1. HTTP API 测试
2. WebSocket 实时推送测试
3. 外部 Agent 通讯测试
"""

import requests
import json
import time
import threading
import sys
import websocket
from typing import Optional, Dict, Any

BASE_URL = "http://localhost:8080"
WS_BASE_URL = "ws://localhost:8080"

class A2ATester:
    def __init__(self, base_url: str = BASE_URL, ws_url: str = WS_BASE_URL, interactive: bool = True, skip_ws: bool = False):
        self.base_url = base_url
        self.ws_url = ws_url
        self.interactive = interactive
        self.skip_ws = skip_ws
        self.task_id: Optional[str] = None
        self.ws_messages = []
        self.ws_connected = False

    def print_section(self, title: str):
        print("\n" + "=" * 60)
        print(f"  {title}")
        print("=" * 60)

    def wait_for_input(self, message: str = "按回车继续..."):
        if self.interactive:
            input(f"\n{message}")

    def test_get_agent_card(self) -> bool:
        """测试获取 Agent Card"""
        self.print_section("1. 测试获取 Agent Card")
        try:
            response = requests.get(f"{self.base_url}/.well-known/agent.json", timeout=5)
            if response.status_code == 200:
                data = response.json()
                print(f"✓ 成功获取 Agent Card")
                print(f"  名称: {data.get('name', 'N/A')}")
                print(f"  描述: {data.get('description', 'N/A')}")
                print(f"  URL: {data.get('url', 'N/A')}")
                return True
            else:
                print(f"✗ 获取 Agent Card 失败，状态码: {response.status_code}")
                return False
        except Exception as e:
            print(f"✗ 获取 Agent Card 异常: {e}")
            return False

    def test_list_agents(self) -> bool:
        """测试列出发现的外部Agents"""
        self.print_section("2. 测试列出发现的外部Agents")
        try:
            response = requests.get(f"{self.base_url}/a2a/agents", timeout=5)
            if response.status_code == 200:
                data = response.json()
                print(f"✓ 成功列出 Agents")
                print(f"  总数: {data.get('total', 0)}")
                for i, agent in enumerate(data.get('agents', [])):
                    print(f"  Agent {i+1}: {agent.get('name', 'N/A')}")
                return True
            else:
                print(f"✗ 列出 Agents 失败，状态码: {response.status_code}")
                return False
        except Exception as e:
            print(f"✗ 列出 Agents 异常: {e}")
            return False

    def test_send_task(self, message: str = "你好，请介绍一下自己。") -> Optional[str]:
        """测试发送任务"""
        self.print_section("3. 测试发送任务")
        try:
            payload = {
                "params": {
                    "message": {
                        "role": "user",
                        "parts": [
                            {
                                "type": "text",
                                "text": message
                            }
                        ]
                    },
                    "sessionId": "test-session-001"
                }
            }
            response = requests.post(
                f"{self.base_url}/a2a/tasks/send",
                json=payload,
                timeout=10
            )
            if response.status_code == 200:
                data = response.json()
                self.task_id = data.get("id")
                print(f"✓ 任务发送成功")
                print(f"  任务 ID: {self.task_id}")
                
                status = data.get('status')
                if isinstance(status, dict):
                    state = status.get('state', 'N/A')
                else:
                    state = status
                print(f"  状态: {state}")
                return self.task_id
            else:
                print(f"✗ 发送任务失败，状态码: {response.status_code}")
                print(f"  响应: {response.text}")
                return None
        except Exception as e:
            print(f"✗ 发送任务异常: {e}")
            return None

    def test_get_task_status(self, task_id: Optional[str] = None) -> Optional[Dict]:
        """测试获取任务状态"""
        self.print_section("4. 测试查询任务状态")
        task_id = task_id or self.task_id
        if not task_id:
            print("✗ 没有任务 ID")
            return None

        try:
            print("  等待任务处理...")
            time.sleep(3)
            
            response = requests.get(
                f"{self.base_url}/a2a/tasks/{task_id}",
                timeout=5
            )
            if response.status_code == 200:
                data = response.json()
                print(f"✓ 成功获取任务状态")
                
                status = data.get('status')
                if isinstance(status, dict):
                    state = status.get('state', 'N/A')
                else:
                    state = status
                print(f"  状态: {state}")
                
                messages = data.get('messages', [])
                if messages:
                    print("  消息:")
                    for msg in messages:
                        role = msg.get('role', 'N/A')
                        parts = msg.get('parts', [])
                        for part in parts:
                            if part.get('type') == 'text':
                                text = part.get('text', '')[:200]
                                print(f"    {role}: {text}...")
                return data
            else:
                print(f"✗ 获取任务状态失败，状态码: {response.status_code}")
                return None
        except Exception as e:
            print(f"✗ 获取任务状态异常: {e}")
            return None

    def test_cancel_task(self, task_id: Optional[str] = None) -> bool:
        """测试取消任务"""
        self.print_section("5. 测试取消任务")
        task_id = task_id or self.task_id
        if not task_id:
            print("✗ 没有任务 ID")
            return False

        try:
            response = requests.post(
                f"{self.base_url}/a2a/tasks/{task_id}/cancel",
                timeout=5
            )
            if response.status_code == 200:
                data = response.json()
                print(f"✓ 任务取消请求已发送")
                print(f"  新状态: {data.get('status', {}).get('state', 'N/A')}")
                return True
            else:
                print(f"✗ 取消任务失败，状态码: {response.status_code}")
                print(f"  响应: {response.text}")
                return False
        except Exception as e:
            print(f"✗ 取消任务异常: {e}")
            return False

    def test_discover_external_agent(self, external_url: str) -> Optional[Dict]:
        """测试发现外部 Agent"""
        self.print_section("6. 测试发现外部 Agent")
        try:
            payload = {"url": external_url}
            response = requests.post(
                f"{self.base_url}/api/a2a/discover",
                json=payload,
                timeout=10
            )
            if response.status_code == 200:
                data = response.json()
                print(f"✓ 外部 Agent 发现请求成功")
                print(f"  响应: {json.dumps(data, indent=2, ensure_ascii=False)}")
                return data
            else:
                print(f"✗ 发现外部 Agent 失败，状态码: {response.status_code}")
                print(f"  响应: {response.text}")
                return None
        except Exception as e:
            print(f"✗ 发现外部 Agent 异常: {e}")
            return None

    def test_send_external_task(self, external_url: str, message: str = "你好，外部 Agent！") -> Optional[str]:
        """测试向外部 Agent 发送任务"""
        self.print_section("7. 测试向外部 Agent 发送任务")
        try:
            payload = {
                "url": external_url,
                "message": message
            }
            response = requests.post(
                f"{self.base_url}/api/a2a/send",
                json=payload,
                timeout=10
            )
            if response.status_code == 200:
                data = response.json()
                external_task_id = data.get("id")
                print(f"✓ 外部任务发送成功")
                print(f"  外部任务 ID: {external_task_id}")
                return external_task_id
            else:
                print(f"✗ 发送外部任务失败，状态码: {response.status_code}")
                print(f"  响应: {response.text}")
                return None
        except Exception as e:
            print(f"✗ 发送外部任务异常: {e}")
            return None

    def test_external_task_status(self, external_task_id: str, external_url: str) -> Optional[Dict]:
        """测试查询外部任务状态"""
        self.print_section("8. 测试查询外部任务状态")
        try:
            print("  等待外部任务处理...")
            time.sleep(3)
            
            response = requests.get(
                f"{self.base_url}/api/a2a/tasks/{external_task_id}/status",
                params={"url": external_url},
                timeout=5
            )
            if response.status_code == 200:
                data = response.json()
                print(f"✓ 成功获取外部任务状态")
                print(f"  响应: {json.dumps(data, indent=2, ensure_ascii=False)}")
                return data
            else:
                print(f"✗ 获取外部任务状态失败，状态码: {response.status_code}")
                return None
        except Exception as e:
            print(f"✗ 获取外部任务状态异常: {e}")
            return None

    def on_ws_message(self, ws, message):
        """WebSocket 消息处理"""
        print(f"  [WebSocket 消息] {message}")
        try:
            data = json.loads(message)
            self.ws_messages.append(data)
        except:
            self.ws_messages.append(message)

    def on_ws_error(self, ws, error):
        """WebSocket 错误处理"""
        print(f"  [WebSocket 错误] {error}")

    def on_ws_close(self, ws, close_status_code, close_msg):
        """WebSocket 关闭处理"""
        print(f"  [WebSocket 关闭] 状态码: {close_status_code}, 消息: {close_msg}")
        self.ws_connected = False

    def on_ws_open(self, ws):
        """WebSocket 打开处理"""
        print("  [WebSocket 连接已建立]")
        self.ws_connected = True

    def test_websocket(self, task_id: Optional[str] = None):
        """测试 WebSocket 实时推送"""
        self.print_section("9. 测试 WebSocket 实时推送")
        
        task_id = task_id or self.task_id
        ws_url = f"{self.ws_url}/ws/a2a/tasks"
        
        if task_id:
            ws_url += f"?taskId={task_id}"
        
        print(f"  连接地址: {ws_url}")
        print("  等待消息... (按 Ctrl+C 停止)")
        
        try:
            ws = websocket.WebSocketApp(
                ws_url,
                on_open=self.on_ws_open,
                on_message=self.on_ws_message,
                on_error=self.on_ws_error,
                on_close=self.on_ws_close
            )
            
            ws_thread = threading.Thread(target=ws.run_forever)
            ws_thread.daemon = True
            ws_thread.start()
            
            time.sleep(10)
            
            ws.close()
            ws_thread.join(timeout=2)
            
            print(f"\n  共收到 {len(self.ws_messages)} 条消息")
            return True
            
        except Exception as e:
            print(f"✗ WebSocket 测试异常: {e}")
            return False

    def run_all_tests(self):
        """运行所有测试"""
        print("=" * 60)
        print("  A2A 功能完整测试")
        print("=" * 60)
        
        results = []
        
        results.append(self.test_get_agent_card())
        self.wait_for_input()
        
        results.append(self.test_list_agents())
        self.wait_for_input()
        
        task_id = self.test_send_task("请介绍一下 FangClaw-Go 这个项目。")
        results.append(task_id is not None)
        self.wait_for_input()
        
        if task_id:
            self.test_get_task_status(task_id)
            self.wait_for_input()
        else:
            print("跳过任务状态测试")
        
        if not self.skip_ws:
            results.append(self.test_websocket(task_id))
        
        print("\n" + "=" * 60)
        print("  测试总结")
        print("=" * 60)
        passed = sum(1 for r in results if r)
        total = len(results)
        print(f"  通过: {passed}/{total}")
        print("=" * 60)


def main():
    import argparse
    
    parser = argparse.ArgumentParser(description="A2A 功能测试工具")
    parser.add_argument("--url", default=BASE_URL, help="服务地址 (默认: http://localhost:8080)")
    parser.add_argument("--ws-url", default=WS_BASE_URL, help="WebSocket 地址 (默认: ws://localhost:8080)")
    parser.add_argument("--external-url", help="外部 Agent URL (用于测试外部通讯)")
    parser.add_argument("--skip-ws", action="store_true", help="跳过 WebSocket 测试")
    parser.add_argument("--no-interactive", action="store_true", help="非交互模式（不等待用户输入）")
    
    args = parser.parse_args()
    
    tester = A2ATester(args.url, args.ws_url, interactive=not args.no_interactive, skip_ws=args.skip_ws)
    
    if args.external_url:
        print("\n" + "=" * 60)
        print("  外部 Agent 测试模式")
        print("=" * 60)
        tester.test_discover_external_agent(args.external_url)
        tester.wait_for_input()
        external_task_id = tester.test_send_external_task(args.external_url, "你好，请做个自我介绍。")
        if external_task_id:
            tester.wait_for_input()
            tester.test_external_task_status(external_task_id, args.external_url)
    else:
        tester.run_all_tests()


if __name__ == "__main__":
    main()
