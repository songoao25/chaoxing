# -*- coding: utf-8 -*-
"""
复核环节（api/review.py）回归：留痕、列表、正文渲染，以及写作钩子。

留痕必须落盘可查，且写失败/空内容不能污染记录；钩子要真的把
"会被平台看到的文字"记下来（简答题/讨论/实践作答）。
"""
import os
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
os.environ.setdefault("CX_DATA_HOME", tempfile.mkdtemp(prefix="cx-review-"))

from api import review  # noqa: E402


class ReviewLogTestCase(unittest.TestCase):
    def test_record_and_load_roundtrip(self):
        item = review.record("讨论回复", "我觉得波特五力里最难判断的是替代品。",
                             course="企业战略管理", task="决策者对外部环境的洞察")
        self.assertIsNotNone(item)
        items = review.load()
        self.assertTrue(items)
        latest = items[0]
        self.assertEqual(latest["kind"], "讨论回复")
        self.assertEqual(latest["course"], "企业战略管理")
        self.assertIn("替代品", latest["text"])
        self.assertEqual(latest["chars"], len(latest["text"]))

    def test_markdown_file_keeps_human_readable_copy(self):
        review.record("作业简答", "我倾向于先把主业做扎实，再考虑多元化。",
                      course="企业战略管理", task="总体战略对比分析")
        path = review.markdown_path()
        self.assertTrue(os.path.exists(path))
        body = open(path, encoding="utf-8").read()
        self.assertIn("总体战略对比分析", body)
        self.assertIn("先把主业做扎实", body)

    def test_empty_text_is_not_recorded(self):
        before = len(review.load())
        self.assertIsNone(review.record("讨论回复", "   "))
        self.assertEqual(len(review.load()), before)

    def test_render_index_lists_items(self):
        review.record("测验简答", "企业应该根据自身资源做权变选择。",
                      course="企业战略管理", task="1.2 章节测验")
        text = review.render_index(review.load(limit=1))
        self.assertIn("测验简答", text)
        self.assertIn("企业战略管理", text)

    def test_render_item_wraps_and_shows_meta(self):
        long_text = "这是一段比较长的回答。" * 12
        review.record("AI实践作答", long_text, course="企业战略管理", task="与董明珠对话")
        text = review.render_item(review.load(limit=1)[0], "[1/1]")
        self.assertIn("与董明珠对话", text)
        self.assertIn("AI实践作答", text)
        for line in text.splitlines():
            self.assertLessEqual(len(line), 80)

    def test_empty_state_is_friendly(self):
        text = review.render_index([])
        self.assertIn("还没有可复核的内容", text)

    def test_count_today_counts_todays_records(self):
        before = review.count_today()
        review.record("讨论回复", "随便说两句，凑个字数。")
        self.assertEqual(review.count_today(), before + 1)

    def test_cli_list_mode_returns_zero(self):
        with mock.patch("builtins.print"):
            self.assertEqual(review.review_cli(["--list"]), 0)


class FakeTiku:
    DISABLE = False

    def query_all(self, questions, query_delay=0):
        return [None] * len(questions)


class FakeWriter:
    available = True

    def __init__(self):
        self.calls = []

    def answer(self, question, **kwargs):
        self.calls.append(question)
        return "我觉得专业化更稳一些，先把主业做扎实。"


class HomeworkShortAnswerHookTestCase(unittest.TestCase):
    def test_short_answer_is_recorded(self):
        from api.task_center import TaskCenter
        tc = TaskCenter(object(), {})
        tc.chaoxing = type("CX", (), {"tiku": FakeTiku()})()
        tc.writer = FakeWriter()
        questions = [{"id": 7, "type": "shortanswer", "title": "你更支持哪种战略？",
                      "options": "", "answerField": {}}]
        with mock.patch("api.task_center.review.record") as recorder:
            failure = tc._fill_homework_answers(questions, course={"title": "企业战略管理"},
                                                task_name="总体战略对比分析")
        self.assertIsNone(failure)
        self.assertTrue(recorder.called)
        args, kwargs = recorder.call_args
        self.assertEqual(args[0], "作业简答")
        self.assertIn("专业化", args[1])
        self.assertEqual(kwargs.get("course"), "企业战略管理")
        self.assertEqual(kwargs.get("task"), "总体战略对比分析")


if __name__ == "__main__":
    unittest.main()
