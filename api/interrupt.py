# -*- coding: utf-8 -*-
"""
刷课过程中的一键终止

刷课时后台起一个线程监听键盘，用户按 q 或 Ctrl+C，立即终止程序。

实现方式：termios + select，不依赖任何第三方库。
只有在真正的终端里才启用；非交互环境（定时任务/管道）自动跳过。
"""

import os
import sys
import threading

# 全局终止标志
_stop_event = threading.Event()
_watcher_started = False
_lock = threading.Lock()
# 轮次编号：避免上一轮的监听线程影响新一轮
_generation = 0


def should_stop():
    """外部查询：是否已被要求终止"""
    return _stop_event.is_set()


def request_stop(reason=""):
    """请求终止（也可由别的模块调用）"""
    _stop_event.set()


def reset():
    """
    开始新一轮刷课前调用：清除终止标志、允许重新启动监听。

    没有这一步的话，用户上一次按 q 终止后，标志一直是 set 状态，
    下一次刷课会立刻停止。
    """
    global _watcher_started, _generation
    with _lock:
        _stop_event.clear()
        _generation += 1
        _watcher_started = False


def _watch_stdin(my_generation):
    """后台线程：读键盘，遇到 q 或 Ctrl+C 就请求终止"""
    import select
    import termios
    import tty

    fd = sys.stdin.fileno()
    try:
        old = termios.tcgetattr(fd)
    except Exception:
        return

    try:
        tty.setcbreak(fd)   # 按键立即到达程序，不用等回车
        while not _stop_event.is_set() and my_generation == _generation:
            try:
                r, _, _ = select.select([fd], [], [], 0.3)
            except Exception:
                break
            if not r:
                continue
            try:
                ch = os.read(fd, 1)
            except Exception:
                break
            if not ch:
                break
            try:
                c = ch.decode("utf-8", "ignore").lower()
            except Exception:
                continue
            if c == "q" or c == chr(3):
                _stop_event.set()
                print()
                print()
                print("  >>> 收到退出指令，正在终止刷课...")
                print()
                break
    finally:
        try:
            termios.tcsetattr(fd, termios.TCSADRAIN, old)
        except Exception:
            pass


def start_watcher():
    """启动键盘监听（只在交互终端里生效，重复调用无副作用）"""
    global _watcher_started
    with _lock:
        if _watcher_started:
            return False
        _watcher_started = True

    try:
        if not sys.stdin.isatty():
            return False
    except Exception:
        return False

    if os.name == "nt":
        return False

    t = threading.Thread(target=_watch_stdin, args=(_generation,), daemon=True)
    t.start()
    return True


def wait_stop(timeout=None):
    """等待终止信号（给需要阻塞的地方用）"""
    return _stop_event.wait(timeout)


def print_hint():
    """刷课前提示怎么退出"""
    print()
    print("=" * 62)
    print("  开始刷课。想中途停止，按 q 键（立即生效，不用回车）。")
    print("  也可以按 Ctrl + C（Mac 上是 Control 键，不是 Command 键）。")
    print("=" * 62)
    print()
