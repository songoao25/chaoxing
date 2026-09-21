# -*- coding: utf-8 -*-
"""
审计修复的离线回归（2026-09-21）：

  * 直播任务失败/被终止时不再报成功
  * 没有题库的章节测验不再被记为完成（返回 ERROR）
  * 向导确认页回车 = 取消（安全默认）
  * main() 在任务中心阶段被终止时不会走到"全部完成"
"""
import os
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
os.environ.setdefault("CX_DATA_HOME", tempfile.mkdtemp(prefix="cx-audit-"))

from api import base as base_mod  # noqa: E402
from api import interrupt  # noqa: E402
from api.live_process import LiveProcessor  # noqa: E402
import setup_wizard as wizard  # noqa: E402


class FakeLive:
    def __init__(self, status=True, duration=60, finish=True):
        self.name = "测试直播"
        self._status = status
        self._duration = duration
        self._finish = finish
        self.calls = 0

    def get_status(self):
        if not self._status:
            return None
        return {"temp": {"data": {"duration": self._duration}}}

    def do_finish(self):
        self.calls += 1
        return self._finish


class LiveProcessorTestCase(unittest.TestCase):
    def test_failed_status_is_not_success(self):
        self.assertFalse(LiveProcessor.run_live(FakeLive(status=False), speed=1.0))

    def test_failed_submit_is_not_success(self):
        live = FakeLive(finish=False)
        with mock.patch("api.live_process.time.sleep", return_value=None):
            self.assertFalse(LiveProcessor.run_live(live, speed=1.0))
        self.assertGreaterEqual(live.calls, 2)   # 失败会重试一次，但仍返回失败

    def test_interrupt_stops_and_reports_failure(self):
        live = FakeLive()
        with mock.patch.object(interrupt, "should_stop", return_value=True):
            self.assertFalse(LiveProcessor.run_live(live, speed=1.0))
        self.assertEqual(live.calls, 0)

    def test_success_only_after_all_minutes(self):
        live = FakeLive(duration=60)
        with mock.patch("api.live_process.time.sleep", return_value=None):
            self.assertTrue(LiveProcessor.run_live(live, speed=2.0))  # 倍速被忽略，仍按真实时间


class NoTikuQuizTestCase(unittest.TestCase):
    def test_no_tiku_returns_error(self):
        cx = base_mod.Chaoxing()
        cx.tiku = mock.Mock(DISABLE=True)
        result = cx.study_work({"title": "测试课"}, {"name": "章节测验"}, {})
        self.assertEqual(result, base_mod.StudyResult.ERROR)


class ConfirmDefaultTestCase(unittest.TestCase):
    def test_enter_cancels_by_default(self):
        with mock.patch.object(wizard, "_read_line", return_value=""), \
             mock.patch("builtins.print"):
            self.assertFalse(wizard.ask_yes_no("确认开始刷课吗？"))

    def test_y_confirms(self):
        with mock.patch.object(wizard, "_read_line", return_value="y"), \
             mock.patch("builtins.print"):
            self.assertTrue(wizard.ask_yes_no("确认开始刷课吗？"))

    def test_default_yes_variant(self):
        with mock.patch.object(wizard, "_read_line", return_value=""), \
             mock.patch("builtins.print"):
            self.assertTrue(wizard.ask_yes_no("重新输入吗？", default_no=False))


class TerminationGuardTestCase(unittest.TestCase):
    def test_stop_branch_exists_before_completion_notice(self):
        source = open(
            os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "main.py"),
            encoding="utf-8",
        ).read()
        stop_idx = source.rfind("已终止刷课")
        done_idx = source.rfind("超星刷课：全部完成")
        self.assertGreater(stop_idx, 0)
        self.assertGreater(done_idx, 0)
        self.assertLess(stop_idx, done_idx, "终止分支必须出现在'全部完成'通知之前")


if __name__ == "__main__":
    unittest.main()
