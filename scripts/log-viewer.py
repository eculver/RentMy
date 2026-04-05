#!/usr/bin/env python3
"""
Viewport-aware log viewer for Claude Code stream-json output.

Reads newline-delimited JSON from stdin, renders a compact rolling view
that fits the terminal. The header stays fixed; logs scroll in the
remaining space.

Usage (standalone):
    tail -f session.log | python3 scripts/log-viewer.py --task "3.6: Handoff screens"

Usage (from run-agent.sh):
    claude ... --output-format stream-json | tee log.log | python3 scripts/log-viewer.py ...
"""
import argparse
import json
import os
import re
import signal
import sys
import time
from datetime import datetime

# ── Terminal helpers ──────────────────────────────────────────────────────────

def get_size():
    try:
        cols, rows = os.get_terminal_size()
    except OSError:
        rows, cols = 24, 80
    return rows, cols

def write(s):
    sys.stdout.write(s)

def flush():
    sys.stdout.flush()

def clear_screen():
    write("\033[2J")

def move(row, col=1):
    write(f"\033[{row};{col}H")

def clear_line():
    write("\033[2K")

def set_scroll_region(top, bottom):
    write(f"\033[{top};{bottom}r")

def reset_scroll_region():
    write("\033[r")

def hide_cursor():
    write("\033[?25l")

def show_cursor():
    write("\033[?25h")

def dim(s):
    return f"\033[2m{s}\033[0m"

def bold(s):
    return f"\033[1m{s}\033[0m"

def cyan(s):
    return f"\033[36m{s}\033[0m"

def green(s):
    return f"\033[32m{s}\033[0m"

def yellow(s):
    return f"\033[33m{s}\033[0m"

def truncate(s, width):
    if len(s) <= width:
        return s
    return s[: width - 1] + "\u2026"

# ── Event parsing ─────────────────────────────────────────────────────────────

def summarize_event(line, viewport=None):
    """Parse a stream-json line and return a compact one-line summary, or None to skip.

    If a Viewport is passed, updates its model and commit_count fields
    as relevant events are encountered.
    """
    try:
        event = json.loads(line)
    except (json.JSONDecodeError, TypeError):
        # Not JSON — show raw line (trimmed)
        stripped = line.strip()
        return stripped if stripped else None

    etype = event.get("type", "")

    # ── system init ──
    if etype == "system":
        subtype = event.get("subtype", "")
        if subtype == "init":
            model = event.get("model", "")
            if model and viewport is not None:
                viewport.model = model
            return dim(f"session started ({model})") if model else dim("session started")
        return None

    # ── assistant message ──
    if etype == "assistant":
        msg = event.get("message", {})
        content = msg.get("content", [])
        parts = []
        for block in content:
            btype = block.get("type", "")
            if btype == "thinking":
                parts.append(dim("thinking..."))
            elif btype == "text":
                text = block.get("text", "").strip()
                if text:
                    # Take first meaningful line
                    for tline in text.split("\n"):
                        tline = tline.strip()
                        if tline:
                            parts.append(tline)
                            break
            elif btype == "tool_use":
                parts.append(_format_tool_use(block))
        return " | ".join(parts) if parts else None

    # ── tool_use (top-level) ──
    if etype == "tool_use":
        tool = event.get("tool", event)
        return _format_tool_use(tool)

    # ── tool_result ──
    if etype == "tool_result":
        subtype = event.get("subtype", "")
        content = event.get("content", "")
        if subtype == "error":
            error = event.get("error", content)
            if isinstance(error, str):
                error = error.split("\n")[0][:120]
            return yellow(f"  error: {error}")
        # Detect successful git commits from output (e.g., "[branch hash] message")
        if isinstance(content, str) and viewport is not None:
            if re.search(r'^\[[\w/.-]+ [0-9a-f]{7,}\]', content, re.MULTILINE):
                viewport.commit_count += 1
        # Success results are usually too verbose — skip unless short
        return None

    # ── result (session end) ──
    if etype == "result":
        cost = event.get("cost_usd")
        turns = event.get("num_turns")
        duration = event.get("duration_ms")
        parts = []
        if turns is not None:
            parts.append(f"{turns} turns")
        if duration is not None:
            mins = duration / 60000
            parts.append(f"{mins:.1f}m")
        if cost is not None:
            parts.append(f"${cost:.2f}")
        summary = ", ".join(parts)
        return green(f"session complete ({summary})" if summary else "session complete")

    return None


def _format_tool_use(block):
    name = block.get("name", "?")
    inp = block.get("input", {})

    if name in ("Read",):
        path = inp.get("file_path", "?")
        return cyan(f"[Read]") + f" {_short_path(path)}"

    if name in ("Write",):
        path = inp.get("file_path", "?")
        return cyan(f"[Write]") + f" {_short_path(path)}"

    if name in ("Edit",):
        path = inp.get("file_path", "?")
        return cyan(f"[Edit]") + f" {_short_path(path)}"

    if name == "Bash":
        cmd = inp.get("command", "?")
        # Show first line (truncate() handles length)
        first_line = cmd.split("\n")[0].strip()
        return cyan("[Bash]") + f" {first_line}"

    if name in ("Grep",):
        pattern = inp.get("pattern", "?")
        path = inp.get("path", "")
        suffix = f" in {_short_path(path)}" if path else ""
        return cyan("[Grep]") + f" /{pattern}/{suffix}"

    if name in ("Glob",):
        pattern = inp.get("pattern", "?")
        return cyan("[Glob]") + f" {pattern}"

    if name == "Agent":
        desc = inp.get("description", inp.get("prompt", "?"))[:60]
        return cyan("[Agent]") + f" {desc}"

    if name == "TodoWrite":
        return dim("[TodoWrite]")

    return cyan(f"[{name}]")


def _short_path(path):
    """Shorten absolute paths to be relative-ish."""
    # Strip common prefixes
    for prefix in ("/Users/eculver/dev/src/github.com/Brett2thered/RentMy/", "./"):
        if path.startswith(prefix):
            return path[len(prefix):]
    return path

# ── Viewport renderer ─────────────────────────────────────────────────────────

class Viewport:
    def __init__(self, task_label, log_path, session_num, graphite_enabled):
        self.task_label = task_label
        self.log_path = log_path
        self.session_num = session_num
        self.graphite_enabled = graphite_enabled
        self.start_time = datetime.now()
        self.buffer = []
        self.event_count = 0
        self.commit_count = 0
        self.model = ""
        self.rows, self.cols = get_size()
        self.header_height = 0  # computed on first draw

    def _build_header(self):
        elapsed = datetime.now() - self.start_time
        mins, secs = divmod(int(elapsed.total_seconds()), 60)
        hours, mins = divmod(mins, 60)
        if hours:
            elapsed_str = f"{hours}h{mins:02d}m{secs:02d}s"
        else:
            elapsed_str = f"{mins}m{secs:02d}s"

        w = self.cols
        bar = dim("\u2500" * w)

        title = bold("  RentMy Autonomous Coding Agent")
        if self.model:
            title += dim(f"  ({self.model})")

        graphite_str = green("yes") if self.graphite_enabled else dim("no")
        commits_str = f"  Commits: {bold(str(self.commit_count))}" if self.commit_count else ""

        lines = [
            bar,
            title,
            bar,
            f"  Task:      {self.task_label}",
            f"  Session:   {self.session_num}  |  Elapsed: {elapsed_str}  |  Events: {self.event_count}{commits_str}",
            f"  Graphite:  {graphite_str}",
            bar,
            dim(f"  Full logs: tail -f {self.log_path}"),
            bar,
        ]
        return lines

    def draw_header(self):
        header = self._build_header()
        self.header_height = len(header)
        move(1)
        for i, line in enumerate(header):
            move(i + 1)
            clear_line()
            write(truncate(line, self.cols))
        flush()

    def resize(self):
        self.rows, self.cols = get_size()
        self.redraw()

    def redraw(self):
        clear_screen()
        self.draw_header()
        log_capacity = self.rows - self.header_height - 1  # 1 for bottom margin
        if log_capacity < 1:
            log_capacity = 1
        visible = self.buffer[-log_capacity:]
        log_start = self.header_height + 1
        for i, line in enumerate(visible):
            move(log_start + i)
            clear_line()
            write("  " + truncate(line, self.cols - 4))
        flush()

    def add_line(self, line):
        self.event_count += 1
        self.buffer.append(line)
        # Cap buffer to avoid unbounded memory (keep last 500 lines)
        if len(self.buffer) > 500:
            self.buffer = self.buffer[-500:]

        log_capacity = self.rows - self.header_height - 1
        if log_capacity < 1:
            log_capacity = 1

        # Refresh header periodically (for elapsed time)
        if self.event_count % 5 == 0:
            self.draw_header()

        # If buffer fits, just append at the right row
        visible_count = min(len(self.buffer), log_capacity)
        if len(self.buffer) <= log_capacity:
            row = self.header_height + len(self.buffer)
            move(row)
            clear_line()
            write("  " + truncate(line, self.cols - 4))
        else:
            # Need to scroll — redraw visible portion
            visible = self.buffer[-log_capacity:]
            log_start = self.header_height + 1
            for i, vline in enumerate(visible):
                move(log_start + i)
                clear_line()
                write("  " + truncate(vline, self.cols - 4))
        flush()


# ── Main ──────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="Viewport log viewer for Claude agent sessions")
    parser.add_argument("--task", default="unknown", help="Task label to display in header")
    parser.add_argument("--log-path", default="(see thoughts/agent-logs/)", help="Path to full log file")
    parser.add_argument("--session", default="1", help="Session number")
    parser.add_argument("--graphite", action="store_true", help="Graphite is enabled")
    args = parser.parse_args()

    vp = Viewport(
        task_label=args.task,
        log_path=args.log_path,
        session_num=args.session,
        graphite_enabled=args.graphite,
    )

    # Handle resize
    def on_resize(signum, frame):
        vp.resize()

    if hasattr(signal, "SIGWINCH"):
        signal.signal(signal.SIGWINCH, on_resize)

    # Graceful exit
    def on_exit(signum, frame):
        reset_scroll_region()
        show_cursor()
        move(vp.rows, 1)
        write("\n")
        flush()
        sys.exit(0)

    signal.signal(signal.SIGINT, on_exit)
    signal.signal(signal.SIGTERM, on_exit)

    hide_cursor()
    clear_screen()
    vp.draw_header()

    try:
        for raw_line in sys.stdin:
            summary = summarize_event(raw_line, viewport=vp)
            if summary is not None:
                vp.add_line(summary)
    except KeyboardInterrupt:
        pass
    finally:
        # Final header refresh with final counts
        vp.draw_header()
        reset_scroll_region()
        show_cursor()
        move(vp.rows, 1)
        write("\n")
        flush()


if __name__ == "__main__":
    main()
