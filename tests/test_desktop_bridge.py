# -*- coding: utf-8 -*-
"""桌面桥（api/desktop_bridge.py）离线回归：协议帧、命令、设置写入、运行状态机.

约束：
* 绝不联网（courses.list 用假 Chaoxing 替身）；
* 绝不读写真实 ~/.chaoxing —— 每个用例都用 tempfile 临时 CX_DATA_HOME；
* 引擎执行函数 desktop_bridge._run_engine 用替身注入，不真的刷课。

接口以 desktop/docs/INTERFACE.md 为准。
"""

import importlib
import json
import os
import stat
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from unittest import mock

_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, _ROOT)
# 必须在 import api.* 之前给出临时数据目录（与仓库其它测试同一手法）
os.environ.setdefault("CX_DATA_HOME", tempfile.mkdtemp(prefix="cx-bridge-module-"))

from api import accounts, paths, review  # noqa: E402
from api import desktop_bridge  # noqa: E402

ACCOUNT_USERNAME = "13800001234"
ACCOUNT_PASSWORD = "pw-123456"

CONFIG_TEXT = """[common]
speed = 1
jobs = 2
notopen_action = continue
use_cookies = false
username = 13800001234
password = pw-123456
course_list = 111,222

[tiku]
provider = AI
key = sk-demo-key
endpoint = https://example.invalid/v1

[notification]
provider = ServerChan
url = https://example.invalid/send

[cx]
quiz_degraded = yes
"""


class BridgeTestCase(unittest.TestCase):
    """公共脚手架：临时数据目录 + 事件捕获 + 引擎替身注入."""

    def setUp(self):
        self.tmp = tempfile.mkdtemp(prefix="cx-bridge-case-")
        env_patch = mock.patch.dict(os.environ, {"CX_DATA_HOME": self.tmp})
        env_patch.start()
        self.addCleanup(env_patch.stop)

        accounts_dir = os.path.join(self.tmp, "accounts")
        acc_patch = mock.patch.object(accounts, "ACCOUNTS_DIR", accounts_dir)
        acc_patch.start()
        self.addCleanup(acc_patch.stop)

        self.frames = []
        self.bridge = desktop_bridge.Bridge(emit=self.frames.append)
        desktop_bridge.set_bridge(self.bridge)
        self.addCleanup(desktop_bridge.set_bridge, None)
        self._logger_configured = False

    # ---------- 数据准备 ----------

    def write_config(self, text=CONFIG_TEXT):
        path = paths.config_path()
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "w", encoding="utf-8") as fp:
            fp.write(text)
        os.chmod(path, 0o600)
        return path

    def add_account(self, username=ACCOUNT_USERNAME, password=ACCOUNT_PASSWORD, name="测试同学"):
        accounts.save_account(username, password, name=name)

    def config_text(self):
        with open(paths.config_path(), encoding="utf-8") as fp:
            return fp.read()

    # ---------- 请求/事件辅助 ----------

    def call(self, cmd, params=None, cmd_id="1"):
        request = {"id": cmd_id, "cmd": cmd}
        if params is not None:
            request["params"] = params
        response, after = self.bridge.process_line(json.dumps(request, ensure_ascii=False))
        if after is not None:
            after()
        return response

    def assert_error(self, response, code):
        self.assertIsInstance(response, dict, response)
        self.assertFalse(response.get("ok"), response)
        self.assertEqual(response["error"]["code"], code, response)
        self.assertTrue(response["error"].get("message"))
        return response["error"]

    def events(self, name):
        return [frame["data"] for frame in list(self.frames) if frame.get("event") == name]

    def wait_event(self, name, run_id=None, timeout=8.0):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            for data in self.events(name):
                if run_id is None or data.get("run_id") == run_id:
                    return data
            time.sleep(0.01)
        self.fail("等待事件超时：{}（已收到 {} 帧）".format(name, len(self.frames)))

    def wait_finished(self, run_id, timeout=8.0):
        return self.wait_event("run.finished", run_id=run_id, timeout=timeout)

    # ---------- 注入 ----------

    def use_engine(self, func):
        patcher = mock.patch.object(desktop_bridge, "_run_engine", func)
        patcher.start()
        self.addCleanup(patcher.stop)

    def install_streams(self):
        desktop_bridge.install_streams()
        self.addCleanup(desktop_bridge.restore_streams)

    def configure_logging(self):
        desktop_bridge.configure_logging()
        self._logger_configured = True
        self.addCleanup(self._restore_logger)

    def _restore_logger(self):
        # 恢复 api.logger 的默认 sink（reload 会先 logger.remove()，不会叠加）
        from api import logger as logger_module
        importlib.reload(logger_module)

    def start_run(self, **params):
        payload = {"account_id": ACCOUNT_USERNAME, "course_ids": ["111", "222"]}
        payload.update(params)
        response = self.call("run.start", payload)
        self.assertTrue(response.get("ok"), response)
        result = response["result"]
        self.assertRegex(result["run_id"], r"^r-\d{8}-\d{6}")
        self.assertRegex(result["started_at"], r"^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$")
        return result


class ProtocolTests(BridgeTestCase):
    """§1 帧格式与错误码."""

    def test_ping(self):
        response = self.call("ping")
        self.assertTrue(response["ok"])
        self.assertEqual(response["id"], "1")
        result = response["result"]
        self.assertEqual(result["protocol"], 1)
        self.assertEqual(result["bridge_version"], desktop_bridge.BRIDGE_VERSION)
        self.assertTrue(result["python"].startswith("3."))
        self.assertTrue(os.path.isdir(result["repo"]))
        self.assertTrue(os.path.exists(os.path.join(result["repo"], "main.py")))

    def test_bad_json_returns_bad_request(self):
        response, after = self.bridge.process_line("这不是 JSON")
        self.assertIsNone(after)
        self.assertFalse(response["ok"])
        self.assertEqual(response["error"]["code"], "bad_request")

    def test_unknown_command_and_missing_cmd(self):
        response = self.call("no.such.command", cmd_id="7")
        self.assertEqual(response["id"], "7")
        self.assert_error(response, "bad_request")

        no_cmd, _after = self.bridge.process_line(json.dumps({"id": "8"}))
        self.assert_error(no_cmd, "bad_request")

        not_object, _after = self.bridge.process_line(json.dumps([1, 2, 3]))
        self.assert_error(not_object, "bad_request")

        bad_params, _after = self.bridge.process_line(
            json.dumps({"id": "9", "cmd": "ping", "params": [1]})
        )
        self.assert_error(bad_params, "bad_request")

    def test_empty_line_is_ignored(self):
        self.assertEqual(self.bridge.process_line("  \n"), (None, None))

    def test_data_home_is_temporary(self):
        self.assertTrue(os.path.realpath(paths.data_dir()).startswith(
            os.path.realpath(self.tmp)))
        self.assertTrue(os.path.realpath(accounts.ACCOUNTS_DIR).startswith(
            os.path.realpath(self.tmp)))


class AccountsTests(BridgeTestCase):
    """§3.3–3.5 账号 CRUD."""

    def test_accounts_crud(self):
        response = self.call("accounts.list")
        self.assertEqual(response["result"]["accounts"], [])

        response = self.call("accounts.add", {
            "username": ACCOUNT_USERNAME, "password": ACCOUNT_PASSWORD, "name": "测试同学",
        })
        self.assertTrue(response["ok"])
        account = response["result"]["account"]
        self.assertEqual(account["id"], ACCOUNT_USERNAME)
        self.assertEqual(account["name"], "测试同学")
        self.assertEqual(account["username_masked"], "138****1234")
        self.assertTrue(account["has_password"])
        account_file = os.path.join(accounts.ACCOUNTS_DIR, ACCOUNT_USERNAME + ".ini")
        self.assertTrue(os.path.exists(account_file))
        self.assertEqual(stat.S_IMODE(os.stat(account_file).st_mode), 0o600)

        listed = self.call("accounts.list")["result"]["accounts"]
        self.assertEqual(len(listed), 1)
        self.assertEqual(listed[0]["id"], ACCOUNT_USERNAME)

        removed = self.call("accounts.remove", {"id": ACCOUNT_USERNAME})
        self.assertTrue(removed["ok"])
        self.assertEqual(removed["result"], {"removed": True})
        self.assertFalse(os.path.exists(account_file))
        self.assertEqual(self.call("accounts.list")["result"]["accounts"], [])

        self.assert_error(self.call("accounts.remove", {"id": ACCOUNT_USERNAME}), "not_found")

    def test_accounts_add_validation(self):
        self.assert_error(self.call("accounts.add", {"password": "x"}), "bad_request")
        self.assert_error(self.call("accounts.add", {"username": "138"}), "bad_request")
        self.assert_error(self.call("accounts.add", {"username": "  ", "password": "x"}),
                          "bad_request")
        self.assert_error(self.call("accounts.add", {"username": "138", "password": ""}),
                          "bad_request")
        self.assert_error(self.call("accounts.add", {"username": "138", "password": "x",
                                                     "name": 3}), "bad_request")
        self.assert_error(self.call("accounts.remove", {}), "bad_request")


class StatusTests(BridgeTestCase):
    """§3.2 status 字段契约."""

    def test_status_contract(self):
        self.write_config()
        self.add_account()
        review.record("测验简答", "合成的复核正文" * 8, course="示例课程", task="任务A")

        result = self.call("status")["result"]
        engine = result["engine"]
        self.assertEqual(engine["repo"], desktop_bridge._REPO)
        self.assertEqual(engine["data_dir"], self.tmp)
        self.assertTrue(engine["config_exists"])   # 已写入 config.ini
        self.assertTrue(engine["python"].startswith("3."))

        self.assertEqual(len(result["accounts"]), 1)
        self.assertEqual(result["accounts"][0]["username_masked"], "138****1234")

        settings = result["settings"]
        for key in desktop_bridge.SETTINGS_FIELDS:
            self.assertIn(key, settings)
        self.assertEqual(settings["speed"], 1.0)
        self.assertEqual(settings["jobs"], 2)
        self.assertEqual(settings["notopen_action"], "continue")
        self.assertEqual(settings["retry_interval"], 1.0)
        self.assertEqual(settings["target_count"], 100)
        self.assertEqual(settings["work_max_retries"], 3)
        self.assertEqual(settings["task_center_submit_mode"], "auto")

        self.assertEqual(result["reviews"]["today"], 1)
        self.assertGreaterEqual(result["reviews"]["recent"], 1)
        self.assertEqual(result["last_run"]["at"], "")
        self.assertIn("log_mtime", result["last_run"])

        caps = result["capabilities"]
        self.assertEqual(caps["tiku_provider"], "AI")
        self.assertEqual(caps["notification"], "ServerChan")
        self.assertTrue(caps["llm_configured"])

    def test_status_defaults_without_config(self):
        result = self.call("status")["result"]
        self.assertFalse(result["engine"]["config_exists"])
        self.assertEqual(result["accounts"], [])
        self.assertEqual(result["settings"]["speed"], 1.0)
        self.assertEqual(result["capabilities"]["llm_configured"], False)


class SettingsTests(BridgeTestCase):
    """§3.6–3.7 设置读写：白名单、范围、禁止部分写入、保留其它节、备份."""

    def setUp(self):
        super().setUp()
        self.write_config()

    def test_settings_get(self):
        response = self.call("settings.get")
        self.assertTrue(response["ok"])
        settings = response["result"]["settings"]
        self.assertEqual(settings["speed"], 1.0)
        self.assertEqual(settings["jobs"], 2)

    def test_settings_set_writes_and_preserves_sections(self):
        response = self.call("settings.set", {"settings": {
            "speed": 1.5,
            "jobs": 3,
            "serial_video": True,
            "target_count": 50,
            "task_center_submit_mode": "auto",
        }})
        self.assertTrue(response["ok"], response)
        settings = response["result"]["settings"]
        self.assertEqual(settings["speed"], 1.5)
        self.assertEqual(settings["jobs"], 3)
        self.assertTrue(settings["serial_video"])
        self.assertEqual(settings["target_count"], 50)

        text = self.config_text()
        self.assertIn("speed = 1.5", text)
        self.assertIn("jobs = 3", text)
        self.assertIn("serial_video = true", text)
        # 其它节必须原样保留（不能 configparser 整体重写）
        self.assertIn("[tiku]", text)
        self.assertIn("provider = AI", text)
        self.assertIn("key = sk-demo-key", text)
        self.assertIn("[notification]", text)
        self.assertIn("provider = ServerChan", text)
        self.assertIn("[cx]", text)
        self.assertIn("quiz_degraded = yes", text)
        # 注释/顺序：原文件开头就是 [common]
        self.assertTrue(text.startswith("[common]"))
        # 备份与权限
        backups = os.listdir(paths.backups_dir())
        self.assertTrue(any(name.startswith("config-") for name in backups), backups)
        self.assertEqual(stat.S_IMODE(os.stat(paths.config_path()).st_mode), 0o600)

        # 再读一次，确认写入后回读一致
        self.assertEqual(self.call("settings.get")["result"]["settings"]["speed"], 1.5)

    def test_invalid_values_rejected_without_partial_write(self):
        before = self.config_text()
        bad_values = {
            "speed": 3.0,
            "jobs": 0,
            "notopen_action": "ask",
            "retry_interval": -1,
            "target_count": 0,
            "work_max_retries": 11,
            "serial_video": "yes",
            "task_center_submit_mode": "confirm",
        }
        for key, value in bad_values.items():
            with self.subTest(key=key, value=value):
                self.assert_error(self.call("settings.set", {"settings": {key: value}}),
                                  "bad_request")
                self.assertEqual(self.config_text(), before)

        # 合法 + 非法混合：整体拒绝，不允许部分写入
        mixed = self.call("settings.set", {"settings": {"speed": 1.8, "jobs": 99}})
        self.assert_error(mixed, "bad_request")
        self.assertEqual(self.config_text(), before)

        unknown = self.call("settings.set", {"settings": {"no_such_key": 1}})
        self.assert_error(unknown, "bad_request")
        self.assertEqual(self.config_text(), before)

        self.assert_error(self.call("settings.set", {}), "bad_request")
        self.assert_error(self.call("settings.set", {"settings": {}}), "bad_request")
        self.assert_error(self.call("settings.set", {"settings": []}), "bad_request")


class ReviewTests(BridgeTestCase):
    """§3.12–3.13 复核列表与全文."""

    def setUp(self):
        super().setUp()
        self.write_config()

    def _seed(self):
        review.record("测验简答", "A" * 200, course="示例课程", task="任务A",
                      extra={"submission_id": "sub-1"})
        review.record("讨论回复", "这是讨论回复的合成正文。", course="示例课程",
                      task="任务B", extra={"submission_id": "sub-2"})

    def test_reviews_list_summary_fields(self):
        self._seed()
        response = self.call("reviews.list", {"days": 1, "limit": 50})
        self.assertTrue(response["ok"], response)
        result = response["result"]
        self.assertTrue(result["markdown_path"].startswith(self.tmp))
        self.assertTrue(result["markdown_path"].endswith(".md"))
        items = result["items"]
        self.assertEqual(len(items), 2)
        expected_keys = {"submission_id", "ts", "day", "kind", "course", "task",
                         "status", "chars", "text_preview"}
        by_id = {item["submission_id"]: item for item in items}
        self.assertEqual(set(by_id), {"sub-1", "sub-2"})
        for item in items:
            self.assertEqual(set(item), expected_keys)
            self.assertNotIn("text", item)
            self.assertLessEqual(len(item["text_preview"]), 120)
        self.assertEqual(by_id["sub-1"]["chars"], 200)
        self.assertEqual(by_id["sub-1"]["text_preview"], "A" * 120)
        self.assertEqual(by_id["sub-1"]["kind"], "测验简答")
        self.assertEqual(by_id["sub-1"]["course"], "示例课程")
        self.assertTrue(by_id["sub-1"]["ts"])
        self.assertTrue(by_id["sub-1"]["day"])

    def test_reviews_list_days_and_limit(self):
        self._seed()
        self.assertEqual(len(self.call("reviews.list", {"days": 1})["result"]["items"]), 2)
        self.assertEqual(len(self.call("reviews.list", {"days": 0})["result"]["items"]), 2)
        self.assertEqual(len(self.call("reviews.list", {"days": 1, "limit": 1})["result"]["items"]), 1)
        self.assert_error(self.call("reviews.list", {"days": -1}), "bad_request")
        self.assert_error(self.call("reviews.list", {"limit": -2}), "bad_request")

    def test_reviews_get_full_text(self):
        self._seed()
        response = self.call("reviews.get", {"submission_id": "sub-1"})
        self.assertTrue(response["ok"], response)
        item = response["result"]["item"]
        self.assertEqual(item["submission_id"], "sub-1")
        self.assertEqual(item["text"], "A" * 200)
        self.assertEqual(item["chars"], 200)
        self.assert_error(self.call("reviews.get", {"submission_id": "missing"}), "not_found")
        self.assert_error(self.call("reviews.get", {}), "bad_request")

    def test_reviews_empty_state(self):
        result = self.call("reviews.list")["result"]
        self.assertEqual(result["items"], [])
        self.assertTrue(result["markdown_path"].endswith(".md"))


class CoursesTests(BridgeTestCase):
    """§3.8 courses.list：真实流程用假 Chaoxing 替身，绝不联网."""

    def setUp(self):
        super().setUp()
        self.write_config()
        self.add_account()

    def test_courses_list_success(self):
        import api.base

        class FakeChaoxing:
            def __init__(self, account=None, tiku=None, **kwargs):
                self.account = account

            def login(self, login_with_cookies=False):
                return {"status": True, "msg": "登录成功"}

            def get_course_list(self):
                return [{
                    "id": "raw-1", "info": "meta", "roleid": "0",
                    "clazzId": "9001", "courseId": "111", "cpi": "abc123",
                    "title": "示例课程", "desc": "课程简介", "teacher": "王老师",
                }]

            def get_name(self):
                return "张三"

        with mock.patch.object(api.base, "Chaoxing", FakeChaoxing):
            response = self.call("courses.list", {"account_id": ACCOUNT_USERNAME})
        self.assertTrue(response["ok"], response)
        result = response["result"]
        self.assertEqual(result["account_id"], ACCOUNT_USERNAME)
        self.assertEqual(result["realname"], "张三")
        self.assertEqual(len(result["courses"]), 1)
        self.assertEqual(set(result["courses"][0]),
                         {"courseId", "clazzId", "title", "teacher", "desc", "cpi"})
        self.assertEqual(result["courses"][0]["courseId"], "111")
        self.assertEqual(result["courses"][0]["teacher"], "王老师")
        self.assertEqual(result["courses"][0]["desc"], "课程简介")

        # courses.list 之后 run.started 能带上课程标题（桥内缓存）
        self.use_engine(lambda argv: None)
        run = self.start_run()
        started = self.wait_event("run.started", run_id=run["run_id"])
        self.assertEqual(started["courses"], [{"courseId": "111", "title": "示例课程"},
                                              {"courseId": "222", "title": ""}])
        self.wait_finished(run["run_id"])

    def test_courses_list_login_failed_and_errors(self):
        import api.base

        class BadLogin:
            def __init__(self, account=None, tiku=None, **kwargs):
                pass

            def login(self, login_with_cookies=False):
                return {"status": False, "msg": "密码错误"}

        class NetworkLogin(BadLogin):
            def login(self, login_with_cookies=False):
                raise ConnectionError("网络不可用")

        with mock.patch.object(api.base, "Chaoxing", BadLogin):
            error = self.assert_error(
                self.call("courses.list", {"account_id": ACCOUNT_USERNAME}), "login_failed")
        self.assertEqual(error["message"], "密码错误")

        with mock.patch.object(api.base, "Chaoxing", NetworkLogin):
            self.assert_error(self.call("courses.list", {"account_id": ACCOUNT_USERNAME}),
                              "network_error")

        self.assert_error(self.call("courses.list", {"account_id": "not-exists"}),
                          "not_found")
        self.assert_error(self.call("courses.list", {}), "bad_request")


class RunTests(BridgeTestCase):
    """§3.9–3.11 运行状态机 + §4 事件."""

    def setUp(self):
        super().setUp()
        self.write_config()
        self.add_account()

    # ---------- 参数校验 ----------

    def test_run_start_validation(self):
        self.assert_error(self.call("run.start", {"course_ids": ["111"]}), "bad_request")
        self.assert_error(self.call("run.start", {
            "account_id": "not-exists", "course_ids": ["111"]}), "not_found")
        self.assert_error(self.call("run.start", {
            "account_id": ACCOUNT_USERNAME, "course_ids": []}), "bad_request")
        self.assert_error(self.call("run.start", {
            "account_id": ACCOUNT_USERNAME}), "bad_request")
        self.assert_error(self.call("run.start", {
            "account_id": ACCOUNT_USERNAME, "course_ids": ["111"],
            "scope": {"chapters": False, "task_center": False}}), "bad_request")
        self.assert_error(self.call("run.start", {
            "account_id": ACCOUNT_USERNAME, "course_ids": ["111"],
            "options": {"speed": 9}}), "bad_request")
        self.assert_error(self.call("run.start", {
            "account_id": ACCOUNT_USERNAME, "course_ids": ["111"],
            "options": {"no_such_option": 1}}), "bad_request")
        self.assert_error(self.call("run.start", {
            "account_id": ACCOUNT_USERNAME, "course_ids": ["111"],
            "limits": {"max_points_per_course": -1}}), "bad_request")
        # 校验失败不应留下运行中的任务
        self.assertFalse(self.call("run.status")["result"]["active"])

    def test_run_start_without_saved_password(self):
        accounts.save_account("13900009999", "", name="无密码")
        self.assert_error(self.call("run.start", {
            "account_id": "13900009999", "course_ids": ["111"]}), "bad_request")

    def test_run_stop_without_active(self):
        self.assert_error(self.call("run.stop"), "not_found")

    # ---------- 完整状态机 ----------

    def test_run_lifecycle_start_busy_stop(self):
        engine_started = threading.Event()

        def fake_engine(argv):
            from api import interrupt
            print("开始刷课 · 合成课程")
            engine_started.set()
            while not interrupt.should_stop():
                time.sleep(0.01)

        self.use_engine(fake_engine)
        self.install_streams()

        run = self.start_run()
        run_id = run["run_id"]
        self.assertTrue(engine_started.wait(5), "引擎替身没有启动")

        started = self.wait_event("run.started", run_id=run_id)
        self.assertEqual(started["account_id"], ACCOUNT_USERNAME)
        self.assertEqual(started["username_masked"], "138****1234")
        self.assertEqual(started["courses"], [{"courseId": "111", "title": ""},
                                              {"courseId": "222", "title": ""}])
        self.assertEqual(started["scope"], {"chapters": True, "task_center": True,
                                            "only_discussion": False})

        # 引擎 print 变成 run.log 事件
        logs = self.events("run.log")
        self.assertTrue(any("开始刷课" in log["text"] for log in logs), logs)
        for log in logs:
            self.assertIn(log["level"], ("trace", "debug", "info", "warning", "error"))
            self.assertTrue(log["text"])
            self.assertTrue(log["ts"])
            self.assertEqual(log["run_id"], run_id)

        status = self.call("run.status")["result"]
        self.assertTrue(status["active"])
        self.assertEqual(status["run_id"], run_id)
        self.assertEqual(status["course_ids"], ["111", "222"])

        # 重复启动 → busy
        self.assert_error(self.call("run.start", {
            "account_id": ACCOUNT_USERNAME, "course_ids": ["111"]}), "busy")

        # 停止
        stop = self.call("run.stop")
        self.assertTrue(stop["ok"])
        self.assertEqual(stop["result"], {"stopping": True})
        self.wait_event("run.stopping", run_id=run_id)
        finished = self.wait_finished(run_id)
        self.assertFalse(finished["ok"])
        self.assertEqual(finished["reason"], "stopped")
        self.assertTrue(finished["message"])
        self.assertGreaterEqual(finished["duration_s"], 0)
        self.assertEqual(finished["summary"]["courses"], 2)

        # 终态之后 run.status 复位
        status = self.call("run.status")["result"]
        self.assertFalse(status["active"])
        self.assertEqual(status["run_id"], "")
        self.assertEqual(status["course_ids"], [])
        self.assert_error(self.call("run.stop"), "not_found")

        # 结束后可以立刻再开一轮（替身立即完成）
        self.use_engine(lambda argv: None)
        second = self.start_run()
        self.assertNotEqual(second["run_id"], run_id)
        second_finished = self.wait_finished(second["run_id"])
        self.assertTrue(second_finished["ok"])
        self.assertEqual(second_finished["reason"], "completed")

    def test_run_completed(self):
        self.use_engine(lambda argv: None)
        run = self.start_run()
        finished = self.wait_finished(run["run_id"])
        self.assertTrue(finished["ok"])
        self.assertEqual(finished["reason"], "completed")
        self.assertEqual(finished["message"], "")
        self.assertEqual(finished["summary"], {"courses": 2, "errors": 0})
        self.assertFalse(self.call("run.status")["result"]["active"])

    def test_run_error_exit_code_and_exception(self):
        self.use_engine(lambda argv: (_ for _ in ()).throw(SystemExit(2)))
        run = self.start_run()
        finished = self.wait_finished(run["run_id"])
        self.assertFalse(finished["ok"])
        self.assertEqual(finished["reason"], "error")
        self.assertIn("登录", finished["message"])

        def explode(_argv):
            raise RuntimeError("合成引擎异常")

        self.use_engine(explode)
        run2 = self.start_run()
        finished2 = self.wait_finished(run2["run_id"])
        self.assertFalse(finished2["ok"])
        self.assertEqual(finished2["reason"], "error")
        self.assertIn("合成引擎异常", finished2["message"])

    def test_engine_logger_events_and_redaction(self):
        self.configure_logging()

        def fake_engine(_argv):
            from api.logger import logger
            logger.info("正在读取课程列表")
            logger.warning("合成警告 password = secret123")

        self.use_engine(fake_engine)
        run = self.start_run()
        finished = self.wait_finished(run["run_id"])
        self.assertTrue(finished["ok"], finished)
        logs = self.events("run.log")
        self.assertTrue(any(log["level"] == "info" and "读取课程列表" in log["text"]
                            for log in logs), logs)
        self.assertTrue(any(log["level"] == "warning" for log in logs), logs)
        # api.logger 的 patcher 脱敏必须仍然生效
        self.assertFalse(any("secret123" in log["text"] for log in logs), logs)

    # ---------- 每轮配置 ----------

    def _capture_engine(self):
        captured = {}

        def fake_engine(argv):
            captured["argv"] = list(argv)
            captured["config_path"] = argv[2]
            with open(argv[2], encoding="utf-8") as fp:
                captured["text"] = fp.read()
            captured["mode"] = stat.S_IMODE(os.stat(argv[2]).st_mode)
            captured["exists"] = os.path.exists(argv[2])

        self.use_engine(fake_engine)
        return captured

    def test_run_config_written_with_forced_safety_values(self):
        captured = self._capture_engine()
        run = self.start_run(
            scope={"chapters": False, "task_center": True, "only_discussion": False},
            limits={"max_points_per_course": 5, "max_tasks_per_course": 2},
            options={"speed": 1.5, "jobs": 3, "notopen_action": "retry",
                     "retry_interval": 2.0, "serial_video": True,
                     "add_learning_count": True, "target_count": 50,
                     "work_redo_enabled": True, "work_max_retries": 5},
        )
        self.wait_finished(run["run_id"])
        self.assertTrue(captured["exists"])
        self.assertEqual(captured["argv"][0], "main.py")
        self.assertEqual(captured["argv"][1], "-c")
        self.assertTrue(captured["argv"][2].endswith("run_13800001234.ini"))
        self.assertEqual(captured["argv"][3], "--yes")
        self.assertEqual(captured["mode"], 0o600)

        text = captured["text"]
        for expected in (
            "username = 13800001234",
            "password = pw-123456",
            "course_list = 111,222",
            "speed = 1.5",
            "jobs = 3",
            "notopen_action = retry",
            "retry_interval = 2",
            "serial_video = true",
            "add_learning_count = true",
            "target_count = 50",
            "work_redo_enabled = true",
            "work_max_retries = 5",
            "chapter_study = false",
            "task_center = true",
            "only_discussion = false",
            "max_points_per_course = 5",
            "max_tasks_per_course = 2",
        ):
            self.assertIn(expected, text)
        # GUI 交互安全：这几项必须强制写死
        self.assertIn("task_center_submit_mode = auto", text)
        self.assertIn("discussion_mode = task", text)
        self.assertIn("use_cookies = false", text)
        # 其它节原样保留
        self.assertIn("[tiku]", text)
        self.assertIn("provider = AI", text)
        self.assertIn("[notification]", text)

    def test_only_discussion_forces_scope_in_config(self):
        captured = self._capture_engine()
        run = self.start_run(scope={"only_discussion": True})
        self.wait_finished(run["run_id"])
        text = captured["text"]
        self.assertIn("only_discussion = true", text)
        self.assertIn("chapter_study = false", text)
        self.assertIn("task_center = true", text)
        self.assertIn("discussion_mode = task", text)

    def test_run_config_uses_template_when_global_config_missing(self):
        os.remove(paths.config_path())
        captured = self._capture_engine()
        run = self.start_run()
        self.wait_finished(run["run_id"])
        self.assertTrue(captured["text"].startswith("[common]"))
        self.assertIn("task_center_submit_mode = auto", captured["text"])
        self.assertIn("[tiku]", captured["text"])


class StdoutPurityTests(BridgeTestCase):
    """子进程回声测试：stdout 每一行都必须是合法 NDJSON（引擎输出/诊断不能混进来）."""

    def test_subprocess_stdout_is_pure_ndjson(self):
        env = dict(os.environ)
        env["CX_DATA_HOME"] = self.tmp
        env["PYTHONPATH"] = _ROOT + (
            os.pathsep + env["PYTHONPATH"] if env.get("PYTHONPATH") else ""
        )
        requests = [
            json.dumps({"id": "1", "cmd": "ping"}),
            json.dumps({"id": "2", "cmd": "status"}),
            json.dumps({"id": "3", "cmd": "unknown.command"}),
            "这不是 JSON",
        ]
        proc = subprocess.run(
            [sys.executable, "-m", "api.desktop_bridge"],
            input="\n".join(requests) + "\n",
            capture_output=True, text=True, cwd=_ROOT, env=env, timeout=180,
        )
        self.assertEqual(proc.returncode, 0, proc.stderr[-3000:])
        output_lines = [line for line in proc.stdout.splitlines() if line.strip()]
        self.assertEqual(len(output_lines), len(requests), proc.stdout)
        frames = []
        for line in output_lines:
            frames.append(json.loads(line))       # 解析失败会直接让用例失败
        self.assertEqual([frame["id"] for frame in frames], ["1", "2", "3", ""])
        self.assertTrue(frames[0]["ok"])
        self.assertEqual(frames[0]["result"]["protocol"], 1)
        self.assertTrue(frames[1]["ok"])
        self.assertEqual(frames[2]["error"]["code"], "bad_request")
        self.assertEqual(frames[3]["error"]["code"], "bad_request")

    def test_subprocess_ping_only_one_line(self):
        env = dict(os.environ)
        env["CX_DATA_HOME"] = self.tmp
        env["PYTHONPATH"] = _ROOT + (
            os.pathsep + env["PYTHONPATH"] if env.get("PYTHONPATH") else ""
        )
        proc = subprocess.run(
            [sys.executable, "-m", "api.desktop_bridge"],
            input='{"id":"1","cmd":"ping"}\n',
            capture_output=True, text=True, cwd=_ROOT, env=env, timeout=180,
        )
        self.assertEqual(proc.returncode, 0, proc.stderr[-3000:])
        lines = [line for line in proc.stdout.splitlines() if line.strip()]
        self.assertEqual(len(lines), 1, proc.stdout)
        frame = json.loads(lines[0])
        self.assertTrue(frame["ok"])
        self.assertEqual(frame["id"], "1")


if __name__ == "__main__":
    unittest.main()
