#!/usr/bin/env python3
"""
Simple mock A2A agent server for testing.
"""

from http.server import HTTPServer, BaseHTTPRequestHandler
from datetime import datetime
import json

MOCK_AGENT_CARD = {
    "name": "Mock-Agent",
    "description": "Mock Agent for testing A2A protocol",
    "url": "http://127.0.0.1:9000/a2a",
    "version": "0.1.0",
    "capabilities": {
        "streaming": False,
        "pushNotifications": False,
        "stateTransitionHistory": True
    },
    "skills": [
        {
            "id": "test",
            "name": "Test",
            "description": "Test skill that responds with a greeting",
            "tags": ["test"],
            "examples": ["Hello", "Test me"]
        }
    ],
    "defaultInputModes": ["text"],
    "defaultOutputModes": ["text"]
}

class MockA2AHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/.well-known/agent.json":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps(MOCK_AGENT_CARD).encode("utf-8"))
        elif self.path == "/a2a/agents":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            response = {
                "agents": [MOCK_AGENT_CARD],
                "total": 1
            }
            self.wfile.write(json.dumps(response).encode("utf-8"))
        else:
            self.send_response(404)
            self.end_headers()
            self.wfile.write(b"Not Found")
    
    def do_POST(self):
        content_length = int(self.headers.get("Content-Length", 0))
        post_data = self.rfile.read(content_length)
        
        now = datetime.utcnow().isoformat() + "Z"
        
        if self.path == "/a2a/tasks/send":
            try:
                data = json.loads(post_data)
                message_text = "Hello from Mock Agent!"
                
                if "params" in data:
                    params = data["params"]
                    if "message" in params:
                        message = params["message"]
                        if "parts" in message:
                            for part in message["parts"]:
                                if part.get("type") == "text":
                                    user_message = part.get("text", "")
                                    message_text = f"Mock Agent received: '{user_message}'. Here's a response!"
                
                task_id = "mock-task-" + str(hash(str(data)) % 100000)
                task = {
                    "id": task_id,
                    "agentId": "mock-agent-id",
                    "status": "completed",
                    "messages": [
                        {
                            "role": "user",
                            "parts": [{"type": "text", "text": "Test message"}]
                        },
                        {
                            "role": "agent",
                            "parts": [{"type": "text", "text": message_text}]
                        }
                    ],
                    "artifacts": [],
                    "createdAt": now,
                    "updatedAt": now
                }
                response = {
                    "jsonrpc": "2.0",
                    "id": data.get("id", 1),
                    "result": task
                }
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps(response).encode("utf-8"))
            except Exception as e:
                self.send_response(500)
                self.end_headers()
                self.wfile.write(f"Error: {e}".encode("utf-8"))
        elif self.path.startswith("/a2a/tasks/"):
            try:
                data = json.loads(post_data)
                task_id = self.path.split("/")[-1]
                task = {
                    "id": task_id,
                    "agentId": "mock-agent-id",
                    "status": "completed",
                    "messages": [
                        {
                            "role": "user",
                            "parts": [{"type": "text", "text": "Test message"}]
                        },
                        {
                            "role": "agent",
                            "parts": [{"type": "text", "text": "Mock Agent response for task status query"}]
                        }
                    ],
                    "artifacts": [],
                    "createdAt": now,
                    "updatedAt": now
                }
                response = {
                    "jsonrpc": "2.0",
                    "id": data.get("id", 1),
                    "result": task
                }
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps(response).encode("utf-8"))
            except Exception as e:
                self.send_response(500)
                self.end_headers()
                self.wfile.write(f"Error: {e}".encode("utf-8"))
        else:
            self.send_response(404)
            self.end_headers()
            self.wfile.write(b"Not Found")
    
    def log_message(self, format, *args):
        print(f"[Mock A2A Server] {format % args}")

def run_mock_server(port=9000):
    server_address = ("", port)
    httpd = HTTPServer(server_address, MockA2AHandler)
    print(f"Mock A2A Agent Server running on http://127.0.0.1:{port}")
    print(f"Agent Card URL: http://127.0.0.1:{port}/.well-known/agent.json")
    print("Press Ctrl+C to stop")
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        print("\nStopping server...")
        httpd.shutdown()

if __name__ == "__main__":
    run_mock_server()
