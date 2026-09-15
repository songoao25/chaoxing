# -*- coding: utf-8 -*-
"""
刷课进度显示

目标：刷课时不再刷屏，每完成一个章节只输出一行，清晰可读。

显示形式：
  ██████░░░░░░░░░░░░░░░░░░░░   23/111  21%   剩 88 节 · 预计 12 分 30 秒   ✓ 1.2 什么是马克思主义
  ██████████░░░░░░░░░░░░░░░░   24/111  22%   剩 87 节 · 预计 12 分 10 秒   ⤼ 3.1 社会形态（未开放）
  ████████████░░░░░░░░░░░░░░   25/111  23%   剩 86 节 · 预计 11 分 50 秒   ✗ 4.2.6 社会形态

  ✓ 完成   ⤼ 跳过   ✗ 失败

通过 tqdm.write 输出，与视频进度条共存不会互相破坏。
"""

import sys
import threading
import time
import unicodedata

from tqdm import tqdm


def _disp_width(text) -> int:
    """
    字符串在终端里的显示宽度。
    中文/全角字符占 2 列，ASCII 占 1 列 —— 直接用 len() 会导致中文对不齐。
    """
    width = 0
    for ch in str(text):
        if unicodedata.east_asian_width(ch) in ("W", "F"):
            width += 2
        else:
            width += 1
    return width


def _pad_right(text, width) -> str:
    """按显示宽度右侧补空格"""
    text = str(text)
    gap = width - _disp_width(text)
    return text + (" " * gap if gap > 0 else "")


def _truncate(text, max_width) -> str:
    """按显示宽度截断，超出部分用省略号"""
    text = str(text)
    if _disp_width(text) <= max_width:
        return text
    out = ""
    used = 0
    for ch in text:
        w = 2 if unicodedata.east_asian_width(ch) in ("W", "F") else 1
        if used + w > max_width - 1:
            break
        out += ch
        used += w
    return out + "…"


def _fmt_duration(seconds, ceil_min=False):
    """秒 -> 可读时长；ceil_min=True 时不足 1 秒也显示 1 秒"""
    try:
        seconds = float(seconds)
    except Exception:
        return "--"
    if seconds < 0:
        return "--"
    if ceil_min:
        import math
        seconds = max(1, math.ceil(seconds))
    seconds = int(seconds)
    if seconds < 60:
        return str(seconds) + " 秒"
    if seconds < 3600:
        return str(seconds // 60) + " 分 " + str(seconds % 60) + " 秒"
    return str(seconds // 3600) + " 小时 " + str((seconds % 3600) // 60) + " 分"


def _fmt_bar(done, total, width=20):
    """文本进度条（用方块字符，比 # 更清晰）"""
    if total <= 0:
        return "░" * width
    filled = int(width * done / total)
    filled = max(0, min(width, filled))
    return "█" * filled + "░" * (width - filled)


class ChapterProgress:
    """章节级进度：每完成一个章节输出一行整体进度"""

    def __init__(self, total, enabled=True, title_width=38):
        self.total = int(total or 0)
        self.done = 0
        self.failed = 0
        self.skipped = 0
        self.start_time = time.time()
        self.title_width = title_width
        self._lock = threading.Lock()
        self._enabled = bool(enabled) and self.total > 0

    # ---------- 内部 ----------
    def _elapsed(self):
        return max(0.0, time.time() - self.start_time)

    def _eta(self):
        """基于平均每章节耗时估算剩余时间"""
        processed = self.done + self.failed + self.skipped
        if processed <= 0:
            return None
        avg = self._elapsed() / processed
        remain = self.total - processed
        if remain <= 0:
            return 0
        return avg * remain

    def _render(self, mark, title, extra=""):
        processed = self.done + self.failed + self.skipped
        total = self.total
        bar = _fmt_bar(processed, total)
        remain = max(0, total - processed)

        # 只要还有未完成的章节就显示 ETA；全部完成时改为显示耗时
        if remain > 0:
            eta = self._eta()
            when_text = "预计 " + (_fmt_duration(eta, ceil_min=True) if eta is not None else "--")
        else:
            when_text = "耗时 " + _fmt_duration(self._elapsed())

        name = _truncate(str(title).strip(), self.title_width)

        # 固定列宽（按显示宽度计算），保证多行对齐、一目了然。
        # 百分比不单独列一栏：进度条本身已经直观表达了比例。
        counter = (str(processed) + "/" + str(total)).rjust(9)
        info = _pad_right("剩 " + str(remain) + " 节 · " + when_text, 26)

        return (
            "  " + bar + "  " + counter + "   " + info + " " + mark + " " + name + extra
        )

    def _emit(self, line):
        if not self._enabled:
            return
        try:
            tqdm.write(line, file=sys.stderr)
            sys.stderr.flush()
        except Exception:
            try:
                print(line, file=sys.stderr, flush=True)
            except Exception:
                pass

    def _bump(self, kind, title, extra=""):
        with self._lock:
            if kind == "done":
                self.done += 1
                mark = "✓"
            elif kind == "skip":
                self.skipped += 1
                mark = "⤼"
            else:
                self.failed += 1
                mark = "✗"
            self._emit(self._render(mark, title, extra))

    # ---------- 对外 ----------
    def chapter_done(self, title):
        self._bump("done", title)

    def chapter_skipped(self, title, reason=""):
        self._bump("skip", title, ("（" + reason + "）") if reason else "")

    def chapter_failed(self, title):
        self._bump("fail", title)

    def summary(self):
        """刷课结束时的总结"""
        if not self._enabled:
            return
        elapsed = self._elapsed()
        parts = []
        if self.done:
            parts.append("完成 " + str(self.done))
        if self.skipped:
            parts.append("跳过 " + str(self.skipped))
        if self.failed:
            parts.append("失败 " + str(self.failed))
        self._emit("  " + "─" * 46)
        self._emit("  刷课结束：共 " + str(self.total) + " 节 · "
                   + " · ".join(parts) + " · 耗时 " + _fmt_duration(elapsed))
