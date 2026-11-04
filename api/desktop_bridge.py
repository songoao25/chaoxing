# -*- coding: utf-8 -*-
"""桌面端 NDJSON 桥（Go ⇄ Python）.

由桌面应用（Wails 后端）以 ``python -u -m api.desktop_bridge`` 启动，
stdin / stdout 只走 NDJSON 协议（见 desktop/docs/INTERFACE.md）：

* stdout **只允许**出现协议行：启动时先把原始 stdout 的 fd dup 出来专门写协议，
  再把进程的 stdout 替换成「引擎输出 → run.log 事件」的包装流；
  任何诊断信息只写 stderr。
* 普通命令同步应答（``{"id","ok","result"}``），run 相关事件随时异步下发。
* 复用现有引擎（api/*、main.py），不改变 CLI 行为；数据目录仍由 api.paths 管理
  （支持 CX_DATA_HOME，测试用临时目录，绝不碰真实 ~/.chaoxing）。

命令与字段严格对齐 INTERFACE.md §2–§3；事件对齐 §4。
"""

import builtins
import io
import json
import os
import platform
import re
import sys
import threading
import time
import traceback
from dataclasses import dataclass
from typing import Any, Callable, Dict, List, Optional, Tuple

BRIDGE_VERSION = "0.1.0"
PROTOCOL = 1
_REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# ---------------------------------------------------------------------------
# 协议输出：原始 stdout 的 dup
# ---------------------------------------------------------------------------

_PROTO_LOCK = threading.Lock()
_PROTO_OUT = None          # type: Optional[io.TextIOBase]
_ORIG_STDOUT = None        # type: Optional[io.TextIOBase]
_ORIG_STDERR = None        # type: Optional[io.TextIOBase]
_STREAMS_INSTALLED = False
_bridge = None             # type: Optional["Bridge"]


def _write_protocol(frame: Dict[str, Any]) -> None:
    """写一行 NDJSON 到原始 stdout（协议通道）；失败静默（Go 侧会重启桥）."""
    if _PROTO_OUT is None:
        return
    try:
        line = json.dumps(frame, ensure_ascii=False, separators=(",", ":"))
    except (TypeError, ValueError):
        line = json.dumps(
            {"id": frame.get("id", ""), "ok": False,
             "error": {"code": "internal", "message": "响应序列化失败"}},
            ensure_ascii=False,
        )
    with _PROTO_LOCK:
        try:
            _PROTO_OUT.write(line + "\n")
            _PROTO_OUT.flush()
        except (BrokenPipeError, OSError, ValueError):
            pass


def _install_protocol_stdout() -> None:
    """把 fd 1 dup 出来当协议通道，然后把 fd 1 重定向到 stderr."""
    global _ORIG_STDOUT, _ORIG_STDERR, _PROTO_OUT
    _ORIG_STDOUT = sys.stdout
    _ORIG_STDERR = sys.stderr
    try:
        fd = os.dup(1)
        _PROTO_OUT = os.fdopen(fd, "w", encoding="utf-8", newline="\n", buffering=1)
    except OSError:
        _PROTO_OUT = None
    # 兜底：任何绕过 sys.stdout、直接写 fd 1 的 C 扩展输出都进 stderr，不污染协议
    try:
        os.dup2(2, 1)
    except OSError:
        pass


def _diag(message: str) -> None:
    """诊断信息只写 stderr（Go 收集日志，不解析）."""
    stream = _ORIG_STDERR or sys.stderr
    try:
        stream.write("[desktop_bridge] " + _redact(message) + "\n")
        stream.flush()
    except Exception:
        pass


def _redact(text: Any) -> str:
    try:
        from api.privacy import redact
        return redact(str(text))
    except Exception:
        return str(text)


def _now() -> str:
    return time.strftime("%Y-%m-%d %H:%M:%S")


# ---------------------------------------------------------------------------
# 错误码（INTERFACE.md §1）
# ---------------------------------------------------------------------------

class BridgeError(Exception):
    def __init__(self, code: str, message: str, detail: str = ""):
        super().__init__(message)
        self.code = code
        self.message = message
        self.detail = detail


def _bad_request(message: str, detail: str = "") -> BridgeError:
    return BridgeError("bad_request", message, detail)


# ---------------------------------------------------------------------------
# 引擎 stdout → run.log 事件
# ---------------------------------------------------------------------------

_TEXT_LIMIT = 4000


class _EngineStdout:
    """替换 sys.stdout：逐行把引擎 print() 输出转成 run.log 事件.

    非运行期间直接丢弃（桌面桥平时不刷日志；也避免把内容漏到协议 stdout）。
    """

    encoding = "utf-8"
    errors = "replace"

    def __init__(self, real_stream):
        self._real = real_stream
        self._buffer = ""
        self._lock = threading.Lock()

    # --- io 接口（尽量兼容第三方库对 stdout 的少量假设） ---
    def write(self, text) -> int:
        if not isinstance(text, str):
            text = str(text)
        lines = []
        with self._lock:
            self._buffer += text
            while "\n" in self._buffer:
                line, self._buffer = self._buffer.split("\n", 1)
                line = line.rstrip("\r")
                if line.strip():
                    lines.append(line)
        # 派发放在锁外，避免桥内部 emit 时反过来阻塞引擎线程的 print
        for line in lines:
            _dispatch_engine_stdout(line)
        return len(text)

    def writelines(self, lines) -> None:
        for line in lines:
            self.write(line)

    def flush(self) -> None:
        with self._lock:
            rest, self._buffer = self._buffer, ""
        if rest.strip():
            _dispatch_engine_stdout(rest.rstrip("\r"))

    def isatty(self) -> bool:
        return False

    def readable(self) -> bool:
        return False

    def writable(self) -> bool:
        return True

    def fileno(self) -> int:
        try:
            return self._real.fileno()
        except (AttributeError, OSError, ValueError):
            raise io.UnsupportedOperation("fileno")

    def reconfigure(self, **_kwargs):
        # main.py 的 safe_console() 会对 sys.stdout/sys.stderr 调用 reconfigure
        return None


class _StderrTap:
    """透传 stderr（诊断仍可见），同时尽力从进度条里解析 run.progress."""

    encoding = "utf-8"
    errors = "replace"

    def __init__(self, real_stream):
        self._real = real_stream
        self._buffer = ""
        self._lock = threading.Lock()

    def write(self, text) -> int:
        try:
            self._real.write(text)
        except Exception:
            pass
        if not isinstance(text, str):
            return 0
        pending = []
        with self._lock:
            self._buffer += text
            while True:
                positions = [pos for pos in (self._buffer.find("\n"), self._buffer.find("\r")) if pos >= 0]
                if not positions:
                    break
                pos = min(positions)
                line, self._buffer = self._buffer[:pos], self._buffer[pos + 1:]
                if line.strip():
                    pending.append(line)
            if len(self._buffer) > 65536:      # 防御：异常输出不让缓冲区无限涨
                self._buffer = ""
        for line in pending:
            _dispatch_engine_progress(line)
        return len(text)

    def flush(self) -> None:
        try:
            self._real.flush()
        except Exception:
            pass

    def isatty(self) -> bool:
        return False

    def readable(self) -> bool:
        return False

    def writable(self) -> bool:
        return True

    def fileno(self) -> int:
        try:
            return self._real.fileno()
        except (AttributeError, OSError, ValueError):
            raise io.UnsupportedOperation("fileno")

    def reconfigure(self, **_kwargs):
        return None


def _dispatch_engine_stdout(line: str) -> None:
    bridge = _bridge
    if bridge is not None:
        bridge.on_engine_stdout(line)


def _dispatch_engine_progress(line: str) -> None:
    bridge = _bridge
    if bridge is not None:
        bridge.on_engine_progress(line)


def set_bridge(bridge: Optional["Bridge"]) -> None:
    """注册当前桥实例（测试里也用它接管事件）."""
    global _bridge
    _bridge = bridge


def install_streams() -> None:
    """把 sys.stdout / sys.stderr 换成引擎适配流（不动 fd 1，测试也安全）."""
    global _STREAMS_INSTALLED
    if _STREAMS_INSTALLED:
        return
    sys.stdout = _EngineStdout(sys.stdout)
    sys.stderr = _StderrTap(sys.stderr)
    _STREAMS_INSTALLED = True


def restore_streams() -> None:
    """恢复系统流（主要给离线单测收尾用）."""
    global _STREAMS_INSTALLED
    if not _STREAMS_INSTALLED:
        return
    sys.stdout = getattr(sys.stdout, "_real", sys.stdout)
    sys.stderr = getattr(sys.stderr, "_real", sys.stderr)
    _STREAMS_INSTALLED = False


def _loguru_sink(message) -> None:
    try:
        record = message.record
        level_name = str(record["level"].name).upper()
        level = _LEVEL_MAP.get(level_name, "info")
        text = str(record["message"]).strip()
        if not text:
            return
        bridge = _bridge
        if bridge is not None:
            bridge.on_engine_log(text, level)
    except Exception:
        # 日志 sink 出错绝不能影响引擎/协议
        return


_LEVEL_MAP = {
    "TRACE": "trace",
    "DEBUG": "debug",
    "INFO": "info",
    "SUCCESS": "info",
    "WARNING": "warning",
    "ERROR": "error",
    "CRITICAL": "error",
}


def configure_logging() -> None:
    """接管 loguru：INFO 及以上 → run.log 事件，同时保留文件日志.

    api.logger 在 import 时已经 logger.remove() 并 logger.configure(patcher=redact)，
    这里保留 patcher（脱敏），只把 sink 换成「事件 + 文件」。
    """
    from api.logger import logger
    from api import paths as _paths

    logger.remove()
    logger.add(_loguru_sink, level="INFO", format="{message}", diagnose=False,
               backtrace=False, enqueue=False)
    try:
        log_file = _paths.log_path()
        logger.add(log_file, rotation="10 MB",
                   level=(os.environ.get("CX_LOG_LEVEL") or "DEBUG").strip().upper(),
                   diagnose=False, backtrace=False)
        try:
            os.chmod(log_file, 0o600)
        except OSError:
            pass
    except Exception as exc:
        _diag("文件日志 sink 不可用：{}".format(type(exc).__name__))


# ---------------------------------------------------------------------------
# 配置 / 设置读取
# ---------------------------------------------------------------------------

SETTINGS_FIELDS = (
    "speed",
    "jobs",
    "notopen_action",
    "retry_interval",
    "serial_video",
    "add_learning_count",
    "target_count",
    "work_redo_enabled",
    "work_max_retries",
    "task_center_submit_mode",
)

RUN_OPTION_FIELDS = (
    "speed",
    "jobs",
    "notopen_action",
    "retry_interval",
    "serial_video",
    "add_learning_count",
    "target_count",
    "work_redo_enabled",
    "work_max_retries",
)

SETTINGS_DEFAULTS = {
    "speed": 1.0,
    "jobs": 2,
    "notopen_action": "retry",
    "retry_interval": 1.0,
    "serial_video": False,
    "add_learning_count": False,
    "target_count": 100,
    "work_redo_enabled": False,
    "work_max_retries": 3,
    "task_center_submit_mode": "auto",
}

# 字段 -> (类型, 最小值, 最大值/候选值, 界面提示)
_SPEED_RANGE = (1.0, 2.0)
_JOBS_RANGE = (1, 8)
_TARGET_COUNT_RANGE = (1, 1000)
_WORK_RETRIES_RANGE = (1, 10)
_RETRY_INTERVAL_RANGE = (0.0, 3600.0)
_NOTOPEN_CHOICES = ("retry", "continue")


def _load_config_parser():
    from api import paths
    from api.configfile import read_config_file
    try:
        config, _broken = read_config_file(paths.config_path())
        return config
    except Exception:
        import configparser
        return configparser.ConfigParser()


def _cfg_get(config, section: str, key: str, default: Any = "") -> Any:
    try:
        return config.get(section, key, fallback=default)
    except Exception:
        return default


def _to_bool(value: Any, default: bool = False) -> bool:
    if isinstance(value, bool):
        return value
    try:
        return str(value).strip().lower() in ("1", "true", "yes", "on", "是")
    except Exception:
        return default


def _display_path(path: str) -> str:
    """~ 开头的展示路径（契约示例 data_dir: "~/.chaoxing"）."""
    try:
        home = os.path.expanduser("~")
        if home and path == home:
            return "~"
        if home and path.startswith(home + os.sep):
            return "~" + path[len(home):]
    except Exception:
        pass
    return path


def _read_settings() -> Dict[str, Any]:
    """把全局 config.ini 的 [common] 白名单字段整理成 Settings（越界值静默收敛）."""
    config = _load_config_parser()
    settings = dict(SETTINGS_DEFAULTS)

    raw = _cfg_get(config, "common", "speed", "")
    settings["speed"] = _normalize_float(raw, _SPEED_RANGE, SETTINGS_DEFAULTS["speed"])

    raw = _cfg_get(config, "common", "jobs", "")
    settings["jobs"] = _normalize_int(raw, _JOBS_RANGE, SETTINGS_DEFAULTS["jobs"])

    raw = str(_cfg_get(config, "common", "notopen_action", "") or "").strip().lower()
    settings["notopen_action"] = raw if raw in _NOTOPEN_CHOICES else "continue"
    if not raw:
        settings["notopen_action"] = SETTINGS_DEFAULTS["notopen_action"]

    raw = _cfg_get(config, "common", "retry_interval", "")
    settings["retry_interval"] = _normalize_float(
        raw, _RETRY_INTERVAL_RANGE, SETTINGS_DEFAULTS["retry_interval"]
    )

    for key in ("serial_video", "add_learning_count", "work_redo_enabled"):
        settings[key] = _to_bool(_cfg_get(config, "common", key, ""),
                                 SETTINGS_DEFAULTS[key])

    raw = _cfg_get(config, "common", "target_count", "")
    settings["target_count"] = _normalize_int(
        raw, _TARGET_COUNT_RANGE, SETTINGS_DEFAULTS["target_count"]
    )

    raw = _cfg_get(config, "common", "work_max_retries", "")
    settings["work_max_retries"] = _normalize_int(
        raw, _WORK_RETRIES_RANGE, SETTINGS_DEFAULTS["work_max_retries"]
    )

    # GUI 固定 auto（confirm 需要终端）；即使配置文件里是 confirm，下发前也归一
    settings["task_center_submit_mode"] = "auto"
    return settings


def _normalize_float(raw: Any, bounds, default: float) -> float:
    try:
        value = float(str(raw).strip())
    except (TypeError, ValueError):
        return float(default)
    low, high = bounds
    return min(float(high), max(float(low), value))


def _normalize_int(raw: Any, bounds, default: int) -> int:
    try:
        value = int(float(str(raw).strip()))
    except (TypeError, ValueError):
        return int(default)
    low, high = bounds
    return min(int(high), max(int(low), value))


def _validate_float(key: str, value: Any, bounds) -> float:
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise _bad_request("设置项 {} 必须是数字".format(key))
    number = float(value)
    low, high = bounds
    if number < low or number > high:
        raise _bad_request(
            "设置项 {} 超出范围（{}–{}）".format(key, _fmt_num(low), _fmt_num(high))
        )
    return number


def _validate_int(key: str, value: Any, bounds) -> int:
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise _bad_request("设置项 {} 必须是整数".format(key))
    if isinstance(value, float):
        if not value.is_integer():
            raise _bad_request("设置项 {} 必须是整数".format(key))
        value = int(value)
    number = int(value)
    low, high = bounds
    if number < low or number > high:
        raise _bad_request("设置项 {} 超出范围（{}–{}）".format(key, low, high))
    return number


def _validate_bool(key: str, value: Any) -> bool:
    if not isinstance(value, bool):
        raise _bad_request("设置项 {} 必须是布尔值".format(key))
    return value


def _validate_enum(key: str, value: Any, choices) -> str:
    if not isinstance(value, str) or value not in choices:
        raise _bad_request("设置项 {} 只能是：{}".format(key, " / ".join(choices)))
    return value


def _fmt_num(value: float) -> str:
    text = ("%.2f" % float(value)).rstrip("0").rstrip(".")
    return text or "0"


def _validate_setting(key: str, value: Any) -> Any:
    """按白名单校验单个设置项，返回规范值；坏值一律 bad_request."""
    if key == "speed":
        return _validate_float(key, value, _SPEED_RANGE)
    if key == "jobs":
        return _validate_int(key, value, _JOBS_RANGE)
    if key == "notopen_action":
        return _validate_enum(key, value, _NOTOPEN_CHOICES)
    if key == "retry_interval":
        return _validate_float(key, value, _RETRY_INTERVAL_RANGE)
    if key in ("serial_video", "add_learning_count", "work_redo_enabled"):
        return _validate_bool(key, value)
    if key == "target_count":
        return _validate_int(key, value, _TARGET_COUNT_RANGE)
    if key == "work_max_retries":
        return _validate_int(key, value, _WORK_RETRIES_RANGE)
    if key == "task_center_submit_mode":
        # GUI 只允许 auto（confirm 需要终端交互）
        return _validate_enum(key, value, ("auto",))
    raise _bad_request("不支持的设置项：{}".format(key))


def _settings_to_ini(key: str, value: Any) -> str:
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, float):
        return _fmt_num(value)
    return str(value)


# ---------------------------------------------------------------------------
# config.ini 文本级修改（保留注释、其它节原样；不能用 configparser 整体重写）
# ---------------------------------------------------------------------------

def _read_config_text() -> str:
    from api import paths
    path = paths.config_path()
    if os.path.exists(path):
        with open(path, encoding="utf8", errors="replace") as fp:
            return fp.read()
    template = os.path.join(_REPO, "config_template.ini")
    if os.path.exists(template):
        with open(template, encoding="utf8", errors="replace") as fp:
            return fp.read()
    return "[common]\n"


def replace_ini_value(content: str, section: str, key: str, value: str) -> str:
    """在 ini 文本里替换 key=value（保留注释与排版）；没有就补一行/补一节."""
    out = []
    current = None
    done = False
    for line in content.splitlines():
        stripped = line.strip()
        if stripped.startswith("[") and stripped.endswith("]"):
            current = stripped[1:-1]
        if current == section and not done:
            core = stripped.split(";")[0].strip()
            if "=" in core and core.split("=")[0].strip() == key:
                out.append(key + " = " + value)
                done = True
                continue
        out.append(line)
    if done:
        return "\n".join(out)

    result = []
    inserted = False
    current = None
    for line in out:
        stripped = line.strip()
        if stripped.startswith("[") and stripped.endswith("]"):
            if current == section and not inserted:
                result.append(key + " = " + value)
                inserted = True
            current = stripped[1:-1]
        result.append(line)
    if current == section and not inserted:
        result.append(key + " = " + value)
        inserted = True
    if not inserted:
        result.append("")
        result.append("[" + section + "]")
        result.append(key + " = " + value)
    return "\n".join(result)


def _sanitize_ini_value(value: Any) -> str:
    text = str(value)
    for bad in ("\r\n", "\n", "\r"):
        text = text.replace(bad, " ")
    return text.strip()


def _write_text_file(path: str, content: str) -> None:
    """写用户数据文件：先写再 chmod 600（失败抛 config_error）."""
    try:
        directory = os.path.dirname(path)
        if directory:
            os.makedirs(directory, exist_ok=True)
        with open(path, "w", encoding="utf8") as fp:
            fp.write(content)
        try:
            os.chmod(path, 0o600)
        except OSError:
            if os.name != "nt":
                raise
    except OSError as exc:
        raise BridgeError("config_error", "写入配置失败（{}）".format(type(exc).__name__))


def _safe_username(username: str) -> str:
    from api import accounts
    return accounts._safe_name(username)


# ---------------------------------------------------------------------------
# 账号 / 复核的契约视图
# ---------------------------------------------------------------------------

def mask_username(username: Any) -> str:
    text = str(username or "")
    if len(text) >= 8:
        return text[:3] + "****" + text[-4:]
    if len(text) >= 4:
        return text[0] + "****" + text[-1]
    return "****"


def _account_view(item: Dict[str, Any]) -> Dict[str, Any]:
    username = str(item.get("username") or "")
    password = str(item.get("password") or "")
    return {
        "id": username,
        "name": str(item.get("name") or ""),
        "username_masked": mask_username(username),
        "last_used": str(item.get("last_used") or ""),
        "has_password": bool(password),
    }


def _review_summary(item: Dict[str, Any]) -> Dict[str, Any]:
    text = str(item.get("text") or "")
    try:
        chars = int(item.get("chars") or len(text))
    except (TypeError, ValueError):
        chars = len(text)
    return {
        "submission_id": str(item.get("submission_id") or ""),
        "ts": str(item.get("ts") or ""),
        "day": str(item.get("day") or ""),
        "kind": str(item.get("kind") or ""),
        "course": str(item.get("course") or ""),
        "task": str(item.get("task") or ""),
        "status": str(item.get("status") or ""),
        "chars": chars,
        "text_preview": text[:120],
    }


def _review_item(item: Dict[str, Any]) -> Dict[str, Any]:
    data = _review_summary(item)
    data["text"] = str(item.get("text") or "")
    return data


# ---------------------------------------------------------------------------
# 运行状态
# ---------------------------------------------------------------------------

@dataclass
class _RunState:
    run_id: str
    started_at: str
    account_id: str
    username_masked: str
    course_ids: List[str]
    courses: List[Dict[str, str]]
    scope: Dict[str, Any]
    argv: List[str]
    config_path: str
    stop_requested: bool = False
    errors: int = 0
    last_error: str = ""
    last_stage: str = ""
    last_progress: Optional[Tuple] = None
    started_monotonic: float = 0.0
    exit_code: int = 0


_STAGES = (
    ("done", ("全部完成", "无需刷课", "刷课完成", "增加完成", "已完成", "全部刷完")),
    ("review", ("复核",)),
    ("task_center", ("任务中心", "教学任务")),
    ("chapters", ("章节", "任务点")),
    ("courses", ("课程列表", "读取课程", "选课")),
    ("login", ("登录",)),
)

_PROGRESS_PATTERN = re.compile(r"(\d+)\s*/\s*(\d+)")


# ---------------------------------------------------------------------------
# 引擎执行（测试可注入替身）
# ---------------------------------------------------------------------------

def _run_engine(argv: List[str]) -> None:
    """真正执行一轮刷课：设置 sys.argv，惰性 import main，调用 main.main()."""
    sys.argv = list(argv)
    import main as engine  # 惰性导入：桥启动时不必加载整个引擎
    engine.main()


_INPUT_GUARD_MESSAGE = "GUI 模式不支持终端交互"


def _reject_input(*_args, **_kwargs):
    raise RuntimeError(_INPUT_GUARD_MESSAGE)


class _UnreadableStdin:
    """运行期间替身 stdin：不可读、非 tty（引擎的交互检查会得到 False）."""

    encoding = "utf-8"
    errors = "replace"

    def isatty(self) -> bool:
        return False

    def readable(self) -> bool:
        return False

    def writable(self) -> bool:
        return False

    def fileno(self) -> int:
        raise io.UnsupportedOperation("fileno")

    def read(self, *_args, **_kwargs):
        raise RuntimeError(_INPUT_GUARD_MESSAGE)

    def readline(self, *_args, **_kwargs):
        raise RuntimeError(_INPUT_GUARD_MESSAGE)

    def __iter__(self):
        return iter(())


def _is_network_error(exc: BaseException) -> bool:
    try:
        import requests
        if isinstance(exc, requests.exceptions.RequestException):
            return True
    except Exception:
        pass
    names = " ".join(cls.__name__ for cls in type(exc).__mro__)
    return any(marker in names for marker in ("Connection", "Timeout", "Request", "HTTP"))


# ---------------------------------------------------------------------------
# 桥
# ---------------------------------------------------------------------------

class Bridge:
    """NDJSON 桥核心：解析请求、派发命令、异步发事件."""

    def __init__(self, emit: Optional[Callable[[Dict[str, Any]], None]] = None):
        self._emit_impl = emit or _write_protocol
        self._lock = threading.RLock()
        self._run = None            # type: Optional[_RunState]
        self._run_thread = None     # type: Optional[threading.Thread]
        self._course_titles = {}    # {account_id: {course_id: title}}
        self._last_run_id = ""
        self._run_seq = 0
        self._handlers = {
            "ping": self._cmd_ping,
            "status": self._cmd_status,
            "accounts.list": self._cmd_accounts_list,
            "accounts.add": self._cmd_accounts_add,
            "accounts.remove": self._cmd_accounts_remove,
            "settings.get": self._cmd_settings_get,
            "settings.set": self._cmd_settings_set,
            "courses.list": self._cmd_courses_list,
            "run.start": self._cmd_run_start,
            "run.stop": self._cmd_run_stop,
            "run.status": self._cmd_run_status,
            "reviews.list": self._cmd_reviews_list,
            "reviews.get": self._cmd_reviews_get,
        }

    # ---------------- 协议 ----------------

    def emit(self, frame: Dict[str, Any]) -> None:
        try:
            self._emit_impl(frame)
        except Exception:
            _diag("事件写出失败：\n" + traceback.format_exc())

    def _emit_event(self, event: str, data: Dict[str, Any]) -> None:
        self.emit({"event": event, "data": data})

    def process_line(self, raw: str) -> Tuple[Optional[Dict[str, Any]], Optional[Callable[[], None]]]:
        """处理一行请求，返回 (响应帧, 响应后要执行的动作)."""
        line = str(raw or "").strip()
        if not line:
            return None, None
        try:
            request = json.loads(line)
        except ValueError:
            return _error_response("", "bad_request", "请求不是合法的 JSON"), None
        if not isinstance(request, dict):
            return _error_response("", "bad_request", "请求必须是 JSON 对象"), None

        request_id = request.get("id", "")
        if request_id is None:
            request_id = ""
        params = request.get("params")
        if params is None:
            params = {}
        if not isinstance(params, dict):
            return _error_response(request_id, "bad_request", "params 必须是对象"), None

        cmd = request.get("cmd")
        if not isinstance(cmd, str) or not cmd:
            return _error_response(request_id, "bad_request", "缺少 cmd"), None
        handler = self._handlers.get(cmd)
        if handler is None:
            return _error_response(request_id, "bad_request", "未知命令：{}".format(cmd)), None

        try:
            outcome = handler(params)
        except BridgeError as exc:
            return _error_response(request_id, exc.code, exc.message, exc.detail), None
        except Exception as exc:
            _diag("命令 {} 未处理异常：\n{}".format(cmd, traceback.format_exc()))
            return (_error_response(request_id, "internal",
                                    "桥内部错误（{}）".format(type(exc).__name__)), None)

        # 说明：run.start 的副作用（run.started 事件 + 引擎线程）放在响应之后，
        # 保证 Go 先拿到响应帧再收到事件流。
        after = None
        if isinstance(outcome, tuple) and len(outcome) == 2:
            result, after = outcome
        else:
            result = outcome
        return {"id": request_id, "ok": True, "result": result}, after

    # ---------------- 命令：基础 ----------------

    def _cmd_ping(self, _params: Dict[str, Any]) -> Dict[str, Any]:
        return {
            "protocol": PROTOCOL,
            "bridge_version": BRIDGE_VERSION,
            "python": platform.python_version(),
            "repo": _REPO,
        }

    def _cmd_status(self, _params: Dict[str, Any]) -> Dict[str, Any]:
        from api import accounts as accounts_mod
        from api import paths
        from api import review as review_mod

        try:
            data_dir = paths.data_dir()
        except OSError:
            data_dir = os.path.expanduser("~/.chaoxing")
        try:
            config_exists = os.path.exists(paths.config_path())
        except OSError:
            config_exists = False

        config = _load_config_parser()
        tiku_provider = str(_cfg_get(config, "tiku", "provider", "") or "").strip()
        notification = str(_cfg_get(config, "notification", "provider", "") or "").strip()
        llm_configured = bool(
            (str(_cfg_get(config, "tiku", "key", "") or "").strip()
             and str(_cfg_get(config, "tiku", "endpoint", "") or "").strip())
            or str(_cfg_get(config, "tiku", "siliconflow_key", "") or "").strip()
        )

        try:
            review_list = review_mod.load()
            today_count = len([i for i in review_list if i.get("day") == time.strftime("%Y-%m-%d")])
            recent_count = len([i for i in review_list
                                if i.get("day") >= time.strftime(
                                    "%Y-%m-%d", time.localtime(time.time() - 2 * 86400))])
        except Exception:
            today_count = recent_count = 0

        return {
            "engine": {
                "repo": _REPO,
                "python": platform.python_version(),
                "data_dir": _display_path(data_dir),
                "config_exists": config_exists,
            },
            "accounts": [_account_view(a) for a in accounts_mod.list_accounts()],
            "settings": _read_settings(),
            "reviews": {"today": today_count, "recent": recent_count},
            "last_run": {"at": _last_run_mtime(), "log_mtime": _file_mtime_text(paths.log_path())},
            "capabilities": {
                "tiku_provider": tiku_provider,
                "notification": notification,
                "llm_configured": llm_configured,
            },
        }

    # ---------------- 命令：账号 ----------------

    def _cmd_accounts_list(self, _params: Dict[str, Any]) -> Dict[str, Any]:
        from api import accounts as accounts_mod
        return {"accounts": [_account_view(a) for a in accounts_mod.list_accounts()]}

    def _cmd_accounts_add(self, params: Dict[str, Any]) -> Dict[str, Any]:
        from api import accounts as accounts_mod
        from api.privacy import register_secret

        username = params.get("username")
        password = params.get("password")
        name = params.get("name", "")
        if not isinstance(username, str) or not username.strip():
            raise _bad_request("username 不能为空")
        if not isinstance(password, str) or not password:
            raise _bad_request("password 不能为空")
        if name is None:
            name = ""
        if not isinstance(name, str):
            raise _bad_request("name 必须是字符串")
        username = username.strip()
        register_secret(username)
        register_secret(password)
        try:
            accounts_mod.save_account(username, password, name.strip())
        except OSError as exc:
            raise BridgeError("config_error", "保存账号失败（{}）".format(type(exc).__name__))
        saved = accounts_mod.load_account(username) or {"username": username, "password": password}
        return {"account": _account_view(saved)}

    def _cmd_accounts_remove(self, params: Dict[str, Any]) -> Dict[str, Any]:
        from api import accounts as accounts_mod
        account_id = params.get("id")
        if not isinstance(account_id, str) or not account_id.strip():
            raise _bad_request("id 不能为空")
        account_id = account_id.strip()
        if not accounts_mod.load_account(account_id):
            raise BridgeError("not_found", "账号不存在")
        try:
            removed = accounts_mod.delete_account(account_id)
        except OSError as exc:
            raise BridgeError("config_error", "删除账号失败（{}）".format(type(exc).__name__))
        if not removed:
            raise BridgeError("not_found", "账号不存在")
        return {"removed": True}

    # ---------------- 命令：设置 ----------------

    def _cmd_settings_get(self, _params: Dict[str, Any]) -> Dict[str, Any]:
        return {"settings": _read_settings()}

    def _cmd_settings_set(self, params: Dict[str, Any]) -> Dict[str, Any]:
        from api import paths

        values = params.get("settings")
        if not isinstance(values, dict):
            raise _bad_request("settings 必须是对象")
        if not values:
            raise _bad_request("settings 不能为空")

        # 先全部校验，任何一个不合法都不落盘（不允许部分写入）
        normalized: Dict[str, Any] = {}
        for key, value in values.items():
            if not isinstance(key, str) or key not in SETTINGS_FIELDS:
                raise _bad_request("不支持的设置项：{}".format(key))
            normalized[key] = _validate_setting(key, value)

        text = _read_config_text()
        for key, value in normalized.items():
            text = replace_ini_value(text, "common", key, _settings_to_ini(key, value))

        try:
            paths.backup_config()
        except Exception:
            pass  # 备份失败不阻止保存（备份本身有容错），但不能写坏
        _write_text_file(paths.config_path(), text)
        return {"settings": _read_settings()}

    # ---------------- 命令：课程 ----------------

    def _cmd_courses_list(self, params: Dict[str, Any]) -> Dict[str, Any]:
        from api import accounts as accounts_mod
        account_id = params.get("account_id")
        if not isinstance(account_id, str) or not account_id.strip():
            raise _bad_request("account_id 不能为空")
        account_id = account_id.strip()
        account = accounts_mod.load_account(account_id)
        if not account:
            raise BridgeError("not_found", "账号不存在")
        username = str(account.get("username") or "")
        password = str(account.get("password") or "")
        if not username or not password:
            raise _bad_request("账号没有保存完整的用户名/密码")

        # 懒加载：课程命令是唯一会真实联网登录的命令
        try:
            from api.base import Account, Chaoxing
        except Exception as exc:
            raise BridgeError("engine_error", "加载引擎失败（{}）".format(type(exc).__name__))

        try:
            chaoxing = Chaoxing(account=Account(username, password))
            login_state = chaoxing.login(login_with_cookies=False)
        except Exception as exc:
            if _is_network_error(exc):
                raise BridgeError("network_error", "登录时网络请求失败", _redact(exc))
            raise BridgeError("engine_error", "登录失败（{}）".format(type(exc).__name__),
                              _redact(exc))
        if not isinstance(login_state, dict) or not login_state.get("status"):
            message = ""
            if isinstance(login_state, dict):
                message = str(login_state.get("msg") or "").strip()
            raise BridgeError("login_failed", message or "登录失败，请检查手机号 / 密码")

        try:
            raw_courses = chaoxing.get_course_list()
        except Exception as exc:
            if _is_network_error(exc):
                raise BridgeError("network_error", "读取课程列表时网络请求失败", _redact(exc))
            raise BridgeError("engine_error", "读取课程列表失败（{}）".format(
                type(exc).__name__), _redact(exc))
        if not isinstance(raw_courses, list):
            raw_courses = []

        courses = []
        title_cache: Dict[str, str] = {}
        for raw in raw_courses:
            if not isinstance(raw, dict):
                continue
            course = {
                "courseId": str(raw.get("courseId") or ""),
                "clazzId": str(raw.get("clazzId") or ""),
                "title": str(raw.get("title") or ""),
                "teacher": str(raw.get("teacher") or ""),
                "desc": str(raw.get("desc") or ""),
                "cpi": str(raw.get("cpi") or ""),
            }
            courses.append(course)
            if course["courseId"]:
                title_cache[course["courseId"]] = course["title"]
        with self._lock:
            self._course_titles[account_id] = title_cache

        realname = ""
        try:
            realname = str(chaoxing.get_name() or "").strip()
        except Exception:
            realname = ""
        return {"account_id": account_id, "realname": realname, "courses": courses}

    # ---------------- 命令：运行 ----------------

    def _cmd_run_start(self, params: Dict[str, Any]) -> Tuple[Dict[str, Any], Callable[[], None]]:
        from api import accounts as accounts_mod
        from api import paths

        with self._lock:
            if self._run is not None:
                raise BridgeError("busy", "已有刷课任务在运行")
            state = self._prepare_run(params, accounts_mod, paths)
            self._run = state

        def after() -> None:
            self._emit_event("run.started", {
                "run_id": state.run_id,
                "started_at": state.started_at,
                "account_id": state.account_id,
                "username_masked": state.username_masked,
                "courses": state.courses,
                "scope": state.scope,
            })
            self._start_run_thread(state)

        return {"run_id": state.run_id, "started_at": state.started_at}, after

    def _prepare_run(self, params: Dict[str, Any], accounts_mod, paths) -> _RunState:
        # --- 账号 ---
        account_id = params.get("account_id")
        if not isinstance(account_id, str) or not account_id.strip():
            raise _bad_request("account_id 不能为空")
        account_id = account_id.strip()
        account = accounts_mod.load_account(account_id)
        if not account:
            raise BridgeError("not_found", "账号不存在")
        username = str(account.get("username") or account_id)
        password = str(account.get("password") or "")
        if not password:
            raise _bad_request("账号没有保存密码，无法开始刷课")

        # --- 课程 ---
        course_ids_raw = params.get("course_ids")
        if not isinstance(course_ids_raw, list) or not course_ids_raw:
            raise _bad_request("course_ids 不能为空")
        course_ids: List[str] = []
        for item in course_ids_raw:
            if isinstance(item, bool) or not isinstance(item, (str, int)):
                raise _bad_request("course_ids 必须是字符串数组")
            text = str(item).strip()
            if not text:
                raise _bad_request("course_ids 不能包含空值")
            if text not in course_ids:
                course_ids.append(text)
        if not course_ids:
            raise _bad_request("course_ids 不能为空")

        # --- 范围 ---
        scope_raw = params.get("scope")
        if scope_raw is None:
            scope_raw = {}
        if not isinstance(scope_raw, dict):
            raise _bad_request("scope 必须是对象")
        chapters = scope_raw.get("chapters", True)
        task_center = scope_raw.get("task_center", True)
        only_discussion = scope_raw.get("only_discussion", False)
        if not isinstance(chapters, bool):
            raise _bad_request("scope.chapters 必须是布尔值")
        if not isinstance(task_center, bool):
            raise _bad_request("scope.task_center 必须是布尔值")
        if not isinstance(only_discussion, bool):
            raise _bad_request("scope.only_discussion 必须是布尔值")
        if only_discussion:
            # 只刷讨论走任务中心链路；章节必须关掉（与 main.py 一致）
            chapters, task_center = False, True
        if not chapters and not task_center:
            raise _bad_request("scope.chapters 与 scope.task_center 不能同时为 false")
        scope = {"chapters": chapters, "task_center": task_center,
                 "only_discussion": only_discussion}

        # --- 数量上限 ---
        limits_raw = params.get("limits")
        if limits_raw is None:
            limits_raw = {}
        if not isinstance(limits_raw, dict):
            raise _bad_request("limits 必须是对象")
        max_points = limits_raw.get("max_points_per_course", 0)
        max_tasks = limits_raw.get("max_tasks_per_course", 0)
        max_points = _validate_limit("limits.max_points_per_course", max_points)
        max_tasks = _validate_limit("limits.max_tasks_per_course", max_tasks)

        # --- 高级选项：白名单 + 校验，缺省取全局设置 ---
        options_raw = params.get("options")
        if options_raw is None:
            options_raw = {}
        if not isinstance(options_raw, dict):
            raise _bad_request("options 必须是对象")
        options = dict(_read_settings())
        for key, value in options_raw.items():
            if not isinstance(key, str) or key not in RUN_OPTION_FIELDS:
                raise _bad_request("不支持的参数：options.{}".format(key))
            options[key] = _validate_setting(key, value)

        # --- 写本轮配置（先备份全局配置，保留其它节原样） ---
        try:
            paths.backup_config()
        except Exception:
            pass
        text = _read_config_text()
        common_values = {
            "username": username,
            "password": password,
            "course_list": ",".join(course_ids),
            "speed": _settings_to_ini("speed", options["speed"]),
            "jobs": str(int(options["jobs"])),
            "notopen_action": options["notopen_action"],
            "retry_interval": _settings_to_ini("retry_interval", options["retry_interval"]),
            "work_redo_enabled": _settings_to_ini("work_redo_enabled", options["work_redo_enabled"]),
            "work_max_retries": str(int(options["work_max_retries"])),
            "add_learning_count": _settings_to_ini("add_learning_count", options["add_learning_count"]),
            "target_count": str(int(options["target_count"])),
            "serial_video": _settings_to_ini("serial_video", options["serial_video"]),
            "max_points_per_course": str(int(max_points)),
            "max_tasks_per_course": str(int(max_tasks)),
            "chapter_study": "true" if chapters else "false",
            "task_center": "true" if task_center else "false",
            "only_discussion": "true" if only_discussion else "false",
            # 交互安全：GUI 不允许任何终端交互，这几项强制写死
            "discussion_mode": "task",
            "task_center_submit_mode": "auto",
            "use_cookies": "false",
        }
        for key, value in common_values.items():
            text = replace_ini_value(text, "common", key, _sanitize_ini_value(value))

        run_config = os.path.join(
            paths.accounts_dir(), "run_" + _safe_username(username) + ".ini"
        )
        _write_text_file(run_config, text)

        # --- 运行标识 ---
        run_id = time.strftime("r-%Y%m%d-%H%M%S")
        if run_id == self._last_run_id:
            self._run_seq += 1
            run_id = run_id + "-" + str(self._run_seq)
        else:
            self._run_seq = 0
        self._last_run_id = run_id
        started_at = _now()

        with self._lock:
            titles = dict(self._course_titles.get(account_id) or {})
        courses = [{"courseId": cid, "title": titles.get(cid, "")} for cid in course_ids]

        return _RunState(
            run_id=run_id,
            started_at=started_at,
            account_id=account_id,
            username_masked=mask_username(username),
            course_ids=course_ids,
            courses=courses,
            scope=scope,
            argv=["main.py", "-c", run_config, "--yes"],
            config_path=run_config,
        )

    def _start_run_thread(self, state: _RunState) -> None:
        from api import interrupt
        try:
            interrupt.reset()
        except Exception:
            pass
        state.started_monotonic = time.monotonic()
        thread = threading.Thread(target=self._run_worker, args=(state,),
                                  name="chaoxing-run", daemon=True)
        self._run_thread = thread
        thread.start()

    def _run_worker(self, state: _RunState) -> None:
        saved_input = builtins.input
        saved_stdin = sys.stdin
        exit_code = 0
        error_text = ""
        try:
            builtins.input = _reject_input
            sys.stdin = _UnreadableStdin()
            _run_engine(state.argv)
        except SystemExit as exc:
            code = exc.code
            if code is None:
                exit_code = 0
            elif isinstance(code, int):
                exit_code = code
            else:
                exit_code = 1
                error_text = str(code)
        except KeyboardInterrupt:
            exit_code = 130
            error_text = "刷课被中断"
        except BaseException as exc:
            exit_code = 1
            error_text = "{}: {}".format(type(exc).__name__, exc)
            _diag("引擎异常退出：\n{}".format(traceback.format_exc()))
        finally:
            builtins.input = saved_input
            sys.stdin = saved_stdin

        state.exit_code = exit_code
        self._finish_run(state, error_text)

    def _finish_run(self, state: _RunState, error_text: str) -> None:
        from api import interrupt
        try:
            stopped = state.stop_requested or interrupt.should_stop()
        except Exception:
            stopped = state.stop_requested

        duration = int(max(0, round(time.monotonic() - (state.started_monotonic or time.monotonic()))))
        if stopped:
            ok, reason = False, "stopped"
            message = "已手动停止刷课"
        elif state.exit_code == 0:
            ok, reason = True, "completed"
            message = ""
        else:
            ok, reason = False, "error"
            message = self._error_message(state, error_text)

        data = {
            "run_id": state.run_id,
            "ok": ok,
            "reason": reason,
            "duration_s": duration,
            "summary": {"courses": len(state.course_ids), "errors": state.errors},
            "message": _redact(message)[:_TEXT_LIMIT] if message else "",
        }

        with self._lock:
            if self._run is state:
                self._run = None
            # 事件终止帧必须最后发：持有锁发完，之后到达的日志会被丢弃
            self._emit_event("run.finished", data)
        _diag("run {} finished: reason={} code={} duration={}s".format(
            state.run_id, reason, state.exit_code, duration))

    @staticmethod
    def _error_message(state: _RunState, error_text: str) -> str:
        if state.last_error:
            return state.last_error
        if error_text:
            return "刷课出错：{}".format(error_text)
        codes = {
            2: "登录或网络失败，请检查账号密码和网络后重试",
            3: "启动前检查未通过，请检查账号 / 课程 / 题库设置后重试",
            4: "有课程没能读到章节，请稍后重试",
            130: "刷课被中断",
        }
        return codes.get(state.exit_code, "刷课异常结束（引擎返回码 {}）".format(state.exit_code))

    def _cmd_run_stop(self, _params: Dict[str, Any]) -> Dict[str, Any]:
        from api import interrupt

        with self._lock:
            state = self._run
            if state is None:
                raise BridgeError("not_found", "当前没有正在运行的刷课任务")
            first_stop = not state.stop_requested
            state.stop_requested = True
            if first_stop:
                # 在锁内发 stopping：保证它一定早于 run.finished（后者同样持锁发）
                self._emit_event("run.stopping", {"run_id": state.run_id})

        if first_stop:
            # 契约要求停止在另一个线程里请求，避免阻塞命令应答
            threading.Thread(target=_request_engine_stop, args=(interrupt,),
                             name="chaoxing-stop", daemon=True).start()
        return {"stopping": True}

    def _cmd_run_status(self, _params: Dict[str, Any]) -> Dict[str, Any]:
        with self._lock:
            state = self._run
            if state is None:
                return {"active": False, "run_id": "", "started_at": "", "course_ids": []}
            return {
                "active": True,
                "run_id": state.run_id,
                "started_at": state.started_at,
                "course_ids": list(state.course_ids),
            }

    # ---------------- 命令：复核 ----------------

    def _cmd_reviews_list(self, params: Dict[str, Any]) -> Dict[str, Any]:
        from api import review as review_mod

        days = _validate_limit("days", params.get("days", 1), maximum=3650)
        limit = _validate_limit("limit", params.get("limit", 50), maximum=10000)
        items = review_mod.load(days=(int(days) or None), limit=(int(limit) or None))
        return {
            "markdown_path": review_mod.markdown_path(),
            "items": [_review_summary(item) for item in items],
        }

    def _cmd_reviews_get(self, params: Dict[str, Any]) -> Dict[str, Any]:
        from api import review as review_mod

        submission_id = params.get("submission_id")
        if not isinstance(submission_id, str) or not submission_id.strip():
            raise _bad_request("submission_id 不能为空")
        submission_id = submission_id.strip()
        for item in review_mod.load():
            if str(item.get("submission_id") or "") == submission_id:
                return {"item": _review_item(item)}
        raise BridgeError("not_found", "复核条目不存在")

    # ---------------- 引擎事件入口 ----------------

    def on_engine_stdout(self, line: str) -> None:
        text = str(line).strip()
        if not text:
            return
        level = "info"
        if text.startswith("✘") or "✘" in text[:6]:
            level = "error"
        elif text.startswith("⚠") or "⚠" in text[:6]:
            level = "warning"
        self._emit_log(level, text)

    def on_engine_log(self, text: str, level: str) -> None:
        text = str(text).strip()
        if not text:
            return
        self._emit_log(level, text)

    def on_engine_progress(self, line: str) -> None:
        """从章节进度条（stderr）尽力解析 run.progress."""
        match = _PROGRESS_PATTERN.search(str(line))
        if not match:
            return
        try:
            done, total = int(match.group(1)), int(match.group(2))
        except (TypeError, ValueError):
            return
        if total <= 0 or done < 0 or done > total or total > 100000:
            return
        title = ""
        mark = re.search(r"[✓⤼✗]\s*(\S.*)$", str(line))
        if mark:
            title = mark.group(1).strip()[:120]
        with self._lock:
            state = self._run
            if state is None:
                return
            key = (done, total, title)
            if state.last_progress == key:
                return
            state.last_progress = key
            payload = {
                "run_id": state.run_id,
                "kind": "course",
                "course_id": "",
                "title": title,
                "done": done,
                "total": total,
            }
            self._emit_event("run.progress", payload)

    def _emit_log(self, level: str, text: str) -> None:
        text = str(text)[:_TEXT_LIMIT]
        with self._lock:
            state = self._run
            if state is None:
                return
            if level == "error":
                state.errors += 1
                state.last_error = text
            payload = {
                "run_id": state.run_id,
                "level": level if level in _LEVEL_MAP.values() else "info",
                "text": text,
                "ts": _now(),
            }
            self._emit_event("run.log", payload)
            self._detect_stage(state, text)

    def _detect_stage(self, state: _RunState, text: str) -> None:
        for stage, keywords in _STAGES:
            if any(keyword in text for keyword in keywords):
                if state.last_stage != stage:
                    state.last_stage = stage
                    self._emit_event("run.stage", {
                        "run_id": state.run_id,
                        "stage": stage,
                        "text": text[:120],
                    })
                return

    # ---------------- 收尾 ----------------

    def shutdown(self, join_timeout: float = 2.0) -> None:
        """stdin EOF / 进程退出前：请求停止并尽量等引擎线程收尾."""
        with self._lock:
            state = self._run
        if state is not None:
            try:
                from api import interrupt
                interrupt.request_stop()
            except Exception:
                pass
        thread = self._run_thread
        if thread is not None and thread.is_alive():
            thread.join(timeout=join_timeout)


def _request_engine_stop(interrupt_module) -> None:
    try:
        interrupt_module.request_stop()
    except Exception:
        _diag("请求停止失败：\n" + traceback.format_exc())


def _validate_limit(key: str, value: Any, maximum: int = 100000) -> int:
    if isinstance(value, bool) or not isinstance(value, (int, float)):
        raise _bad_request("{} 必须是整数".format(key))
    if isinstance(value, float):
        if not value.is_integer():
            raise _bad_request("{} 必须是整数".format(key))
        value = int(value)
    number = int(value)
    if number < 0 or number > maximum:
        raise _bad_request("{} 超出范围（0–{}）".format(key, maximum))
    return number


def _error_response(request_id: Any, code: str, message: str, detail: str = "") -> Dict[str, Any]:
    error = {"code": code, "message": _redact(message)}
    if detail:
        error["detail"] = _redact(detail)
    return {"id": request_id, "ok": False, "error": error}


def _last_run_mtime() -> str:
    from api import paths
    best = 0.0
    try:
        names = os.listdir(paths.accounts_dir())
    except OSError:
        return ""
    for name in names:
        if not (name.startswith("run_") and name.endswith(".ini")):
            continue
        try:
            best = max(best, os.path.getmtime(os.path.join(paths.accounts_dir(), name)))
        except OSError:
            continue
    return time.strftime("%Y-%m-%d %H:%M", time.localtime(best)) if best else ""


def _file_mtime_text(path: str) -> str:
    try:
        if not os.path.exists(path):
            return ""
        return time.strftime("%Y-%m-%d %H:%M", time.localtime(os.path.getmtime(path)))
    except OSError:
        return ""


# ---------------------------------------------------------------------------
# 主循环
# ---------------------------------------------------------------------------

def main() -> int:
    """桥进程入口：stdin 逐行读请求，stdout 只写 NDJSON."""
    _diag("starting python bridge v{} (python {})".format(
        BRIDGE_VERSION, platform.python_version()))
    _install_protocol_stdout()
    bridge = Bridge()
    set_bridge(bridge)
    install_streams()
    try:
        configure_logging()
    except Exception:
        _diag("日志接管失败（不影响协议）：\n" + traceback.format_exc())

    stdin = sys.stdin
    try:
        while True:
            try:
                raw = stdin.readline()
            except (OSError, ValueError):
                break
            if raw is None or raw == "":
                break
            if len(raw) > 2 * 1024 * 1024:
                _write_protocol(_error_response("", "bad_request", "请求过大"))
                continue
            response, after = bridge.process_line(raw)
            if response is not None:
                _write_protocol(response)
            if after is not None:
                try:
                    after()
                except Exception:
                    _diag("启动 run 失败：\n" + traceback.format_exc())
    except KeyboardInterrupt:
        pass
    finally:
        bridge.shutdown()
        try:
            sys.stdout.flush()
            sys.stderr.flush()
        except Exception:
            pass
        try:
            if _PROTO_OUT is not None:
                _PROTO_OUT.flush()
        except Exception:
            pass
    return 0


if __name__ == "__main__":
    sys.exit(main())
