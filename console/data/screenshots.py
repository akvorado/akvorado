#!/usr/bin/env python3
# SPDX-FileCopyrightText: 2026 Free Mobile
# SPDX-License-Identifier: AGPL-3.0-only

"""Regenerate the screenshots used in the documentation.

It drives a headless Firefox through the Marionette protocol. By default, it
takes the screenshots on the public demo site. Use "make docs-screenshots" to
run it.

"""

import argparse
import base64
import json
import os
import shutil
import socket
import subprocess
import sys
import tempfile
import time

# The screenshots are 1536x1152: 1024x768 CSS pixels with a device pixel ratio
# of 1.5. A width of 1024 is also what puts the options panel of the "visualize"
# page on the left instead of on top.
WIDTH, HEIGHT = 1024, 768
DEVICE_PIXEL_RATIO = "1.5"
MARIONETTE_PORT = 2828

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# A graph is complete when the chart is drawn and the table below it is filled.
GRAPH_READY = (
    "document.querySelectorAll('canvas').length >= 1"
    " && document.querySelectorAll('table tbody tr').length > 2"
)
# On the home page, the widgets are fetched one by one and the number of flows
# starts as "???".
HOME_READY = (
    "document.querySelectorAll('canvas').length >= 6"
    " && !document.body.innerText.includes('???')"
)
# A documentation page is complete when the text and the table of contents on
# the left are both rendered.
DOCS_READY = (
    "document.querySelector('.prose h1')"
    " && document.querySelectorAll('nav a').length > 5"
)

# The screenshots to take, written to "<name>.png". A shot with a "visualize"
# key targets the "visualize" page: it holds the options to select there, which
# are turned into a compressed URL below, and the options panel is opened before
# taking the shot.
SHOTS = {
    "home": {"path": "/", "ready": HOME_READY},
    "documentation": {"path": "/docs", "ready": DOCS_READY},
    "timeseries": {"visualize": {}, "ready": GRAPH_READY},
    "sankey": {
        "ready": GRAPH_READY,
        "visualize": {
            "graphType": "sankey",
            "dimensions": ["SrcAS", "InIfConnectivity", "InIfProvider", "ExporterName"],
            "limit": 20,
        },
    },
}


def encode_states(states):
    """Build the URL fragment holding the options of the "visualize" page.

    The console compresses the options with lz-string. Instead of implementing
    the algorithm again, use the copy shipped with the frontend.
    """
    if not states:
        return {}
    script = """
const LZString = require("lz-string");
const now = new Date();
const start = new Date(now.getTime() - 24 * 3600 * 1000);
const states = JSON.parse(process.argv[1]);
const result = {};
for (const [name, extra] of Object.entries(states)) {
  const state = {
    graphType: "stacked",
    start: start.toISOString(),
    end: now.toISOString(),
    humanStart: "24 hours ago",
    humanEnd: "now",
    dimensions: ["SrcAS"],
    limit: 10,
    limitType: "avg",
    "truncate-v4": 32,
    "truncate-v6": 128,
    filter: "InIfBoundary = external",
    units: "l3bps",
    bidirectional: false,
    previousPeriod: false,
    ...extra,
  };
  result[name] =
    "v1-" +
    LZString.compressToEncodedURIComponent(
      JSON.stringify(state, Object.keys(state).sort()),
    );
}
console.log(JSON.stringify(result));
"""
    frontend = os.path.join(ROOT, "console", "frontend")
    if not os.path.isdir(os.path.join(frontend, "node_modules", "lz-string")):
        sys.exit("lz-string is missing, run 'make console/frontend/node_modules'")
    out = subprocess.run(
        ["node", "-e", script, "--", json.dumps(states)],
        cwd=frontend,
        check=True,
        capture_output=True,
        text=True,
    )
    return json.loads(out.stdout)


class Marionette:
    """A minimal client for the Marionette protocol of Firefox.

    Messages are JSON arrays prefixed by their length in bytes.
    """

    def __init__(self, port):
        self.sock = socket.create_connection(("127.0.0.1", port), timeout=60)
        self.sock.settimeout(180)
        self.buf = b""
        self.msgid = 0
        self.receive()  # the server starts by sending its capabilities

    def receive(self):
        while b":" not in self.buf:
            self.buf += self.sock.recv(65536)
        length, _, rest = self.buf.partition(b":")
        length = int(length)
        while len(rest) < length:
            rest += self.sock.recv(65536)
        self.buf = rest[length:]
        return json.loads(rest[:length])

    def call(self, command, params=None):
        self.msgid += 1
        payload = json.dumps([0, self.msgid, command, params or {}]).encode()
        self.sock.sendall(b"%d:%s" % (len(payload), payload))
        while True:
            message = self.receive()
            if message[0] == 1 and message[1] == self.msgid:
                if message[2] is not None:
                    raise RuntimeError(f"{command}: {message[2]}")
                return message[3]

    def script(self, js):
        return self.call("WebDriver:ExecuteScript", {"script": js, "args": []})["value"]

    def wait_for(self, condition, timeout=90):
        deadline = time.time() + timeout
        while time.time() < deadline:
            try:
                if self.script(f"return !!({condition});"):
                    return True
            except RuntimeError:
                pass  # the page may still be loading
            time.sleep(1)
        print(f"  timeout waiting for: {condition}", file=sys.stderr)
        return False

    def fit_viewport(self):
        """Resize the window until the viewport has the expected size."""
        self.call("WebDriver:SetWindowRect", {"width": WIDTH, "height": HEIGHT})
        for _ in range(5):
            inner = self.script("return [window.innerWidth, window.innerHeight];")
            missing = (WIDTH - inner[0], HEIGHT - inner[1])
            if missing == (0, 0):
                return inner
            rect = self.call("WebDriver:GetWindowRect")
            self.call(
                "WebDriver:SetWindowRect",
                {
                    "width": int(rect["width"] + missing[0]),
                    "height": int(rect["height"] + missing[1]),
                },
            )
        return self.script("return [window.innerWidth, window.innerHeight];")

    def screenshot(self, path):
        data = self.call("WebDriver:TakeScreenshot", {"full": False, "hash": False})
        with open(path, "wb") as f:
            f.write(base64.b64decode(data["value"]))


def start_firefox(profile):
    """Start a headless Firefox and wait for Marionette to accept connections."""
    if shutil.which("firefox") is None:
        sys.exit("firefox is not installed")
    with open(os.path.join(profile, "user.js"), "w") as f:
        f.write(f"""
user_pref("layout.css.devPixelsPerPx", "{DEVICE_PIXEL_RATIO}");
user_pref("ui.useOverlayScrollbars", 1);
user_pref("ui.scrollbarFadeBeginDelay", 0);
user_pref("ui.scrollbarFadeDuration", 0);
user_pref("browser.shell.checkDefaultBrowser", false);
user_pref("browser.startup.homepage_override.mstone", "ignore");
user_pref("datareporting.policy.dataSubmissionEnabled", false);
user_pref("toolkit.telemetry.enabled", false);
user_pref("app.update.enabled", false);
""")
    process = subprocess.Popen(
        [
            "firefox",
            "--headless",
            "--marionette",
            "--no-remote",
            "-profile",
            profile,
            "--window-size",
            f"{WIDTH},{HEIGHT}",
            "about:blank",
        ],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        env={**os.environ, "MOZ_MARIONETTE": "1"},
    )
    for _ in range(60):
        try:
            socket.create_connection(("127.0.0.1", MARIONETTE_PORT), timeout=2).close()
            return process
        except OSError:
            time.sleep(1)
    process.kill()
    sys.exit("Firefox did not start")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--url",
        default="https://demo.akvorado.net",
        help="console to take the screenshots from",
    )
    parser.add_argument(
        "--output",
        default=os.path.join(ROOT, "console", "data", "docs"),
        help="where to write the PNG files",
    )
    parser.add_argument(
        "screenshot",
        nargs="*",
        help="screenshots to generate, among {} (default: all of them)".format(
            ", ".join(SHOTS)
        ),
    )
    options = parser.parse_args()
    base = options.url.rstrip("/")

    selected = options.screenshot or list(SHOTS)
    for name in selected:
        if name not in SHOTS:
            parser.error(
                "unknown screenshot {!r}, choose from {}".format(
                    name, ", ".join(SHOTS)
                )
            )
    states = encode_states(
        {name: SHOTS[name]["visualize"] for name in selected if "visualize" in SHOTS[name]}
    )

    with tempfile.TemporaryDirectory() as profile:
        process = start_firefox(profile)
        try:
            marionette = Marionette(MARIONETTE_PORT)
            marionette.call("WebDriver:NewSession", {"capabilities": {}})
            marionette.call("WebDriver:Navigate", {"url": f"{base}/"})
            width, height = marionette.fit_viewport()
            if (width, height) != (WIDTH, HEIGHT):
                print(f"warning: viewport is {width}x{height}", file=sys.stderr)
            for name in selected:
                shot = SHOTS[name]
                print(f"▶ {name}.png")
                if name in states:
                    url = f"{base}/visualize/{states[name]}"
                else:
                    url = f"{base}{shot['path']}"
                marionette.call("WebDriver:Navigate", {"url": url})
                marionette.wait_for(shot["ready"])
                if name in states:
                    # The options panel starts collapsed. The documentation
                    # describes the options it holds, so open it.
                    marionette.script(
                        "const b = document.querySelector('aside button');"
                        "if (b && !document.querySelector('aside form div'))"
                        " b.click();"
                    )
                    marionette.wait_for(
                        "document.querySelector('aside form div')", timeout=20
                    )
                time.sleep(6)  # let the animations of the charts finish
                target = os.path.join(options.output, f"{name}.png")
                marionette.screenshot(target)
                subprocess.run(
                    ["pngquant", "--ext", ".png", "--force", "--strip", "--quiet", target],
                )
                print(f"  {os.path.getsize(target) // 1024} KB")
        finally:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()


if __name__ == "__main__":
    main()
