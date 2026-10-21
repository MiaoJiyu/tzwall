#!/usr/bin/env python3
"""AI 审核联调用 mock 服务：OpenAI 兼容 /chat/completions，返回 {"pass":bool,"reason":...}。

仅用于阶段三联调与回归验证，不属于生产组件。
用法：python3 deploy/mock_ai.py [端口]
"""
import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

PASS_REPLY = {"pass": True, "reason": "内容正常"}
BLOCK_REPLY = {"pass": False, "reason": "包含广告导流"}


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):  # 静音默认日志
        pass

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length).decode("utf-8", "ignore")
        if "/chat/completions" not in self.path:
            self.send_response(404)
            self.end_headers()
            return
        body = json.loads(raw) if raw else {}
        user_text = ""
        for msg in body.get("messages", []):
            if msg.get("role") == "user":
                user_text = msg.get("content", "")
        # 命中关键字时判不通过，便于联调验证驳回分支
        if "广告" in user_text or "博彩" in user_text:
            reply = BLOCK_REPLY
        else:
            reply = PASS_REPLY
        payload = {
            "choices": [{"message": {"role": "assistant", "content": json.dumps(reply, ensure_ascii=False)}}],
        }
        data = json.dumps(payload, ensure_ascii=False).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 9099
    print(f"mock AI 审核服务已启动：http://127.0.0.1:{port}/v1")
    HTTPServer(("127.0.0.1", port), Handler).serve_forever()
