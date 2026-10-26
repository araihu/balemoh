"""Manual real-browser regression for bulk admission behind a held refresh.

Use disposable local API/UI fixtures only. Open the proxy and the selected action,
create STATE/hold, click Refresh notifications, then submit repeatedly. Verify
STATE/held exists and STATE/reads or STATE/clears is absent before creating
STATE/release. After release, the counter must equal 1 and the post-action message
must remain unread. A duplicate write is counted and rejected by this harness.
No production endpoints are allowed. --action defaults to clear for compatibility.
"""
import argparse
import http.server
import json
import pathlib
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from datetime import datetime, timezone

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--action",choices=("clear","read-all"),default="clear")
parser.add_argument("--ui", required=True)
parser.add_argument("--api", required=True)
parser.add_argument("--publisher-token-file", type=pathlib.Path, required=True)
parser.add_argument("--state", type=pathlib.Path, required=True)
parser.add_argument("--port", type=int, default=51935)
args = parser.parse_args()
for address in (args.ui, args.api):
    parsed = urllib.parse.urlparse(address)
    if parsed.scheme != "http" or parsed.hostname != "127.0.0.1" or parsed.path:
        parser.error("only loopback HTTP fixture origins are allowed")
args.state.mkdir(parents=True, exist_ok=True)
lock = threading.Lock()
clears = 0


class Proxy(http.server.BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def do_GET(self):
        if self.path.startswith(("/notifications/summary", "/notifications/tray")) and (args.state / "hold").exists():
            (args.state / "held").touch()
            deadline = time.monotonic() + 300
            while not (args.state / "release").exists() and time.monotonic() < deadline:
                time.sleep(0.02)
        self.forward()

    def do_POST(self):
        global clears
        if self.path == "/notifications/" + args.action:
            with lock:
                clears += 1
                (args.state / ("clears" if args.action=="clear" else "reads")).write_text(str(clears))
                if clears > 1:
                    self.send_error(409, "Duplicate clear regression")
                    return
        self.forward()

    def forward(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", "0"))) if self.command == "POST" else None
        headers = {k: v for k, v in self.headers.items() if k.lower() not in ("host", "connection", "origin", "content-length")}
        # Preserve same-origin protection while proxying to the local fixture.
        if "Origin" in self.headers:
            headers["Origin"] = args.ui
        request = urllib.request.Request(args.ui + self.path, data=body, headers=headers, method=self.command)
        try:
            upstream = urllib.request.urlopen(request, timeout=360)
        except urllib.error.HTTPError as error:
            upstream = error
        content = upstream.read()
        if self.command == "POST" and self.path == "/notifications/" + args.action and upstream.status == 200:
            message = {"eventId": str(uuid.uuid4()), "title": "Published after " + args.action, "severity": "warning", "occurredAt": datetime.now(timezone.utc).isoformat()}
            publish = urllib.request.Request(args.api + "/api/v1/notifications", data=json.dumps(message).encode(), headers={"Authorization": "Bearer " + args.publisher_token_file.read_text().strip(), "Content-Type": "application/json"})
            with urllib.request.urlopen(publish, timeout=5) as response:
                (args.state / "published").write_text(json.dumps(json.load(response)))
        # Test-only longer timeout permits deliberate queue holds across tool calls.
        if upstream.headers.get("Content-Type", "").startswith("text/html"):
            content = content.replace(b"timeout:4s", b"timeout:360s")
        self.send_response(upstream.status)
        for key, value in upstream.headers.items():
            if key.lower() not in ("content-length", "transfer-encoding", "connection", "server", "date"):
                self.send_header(key, value)
        self.end_headers()
        self.wfile.write(content)


http.server.ThreadingHTTPServer(("127.0.0.1", args.port), Proxy).serve_forever()
