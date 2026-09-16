# -*- coding: utf-8 -*-
"""
离线回归测试：平台文本 / 填空题提交 / 大模型连接检查 / 视频并发开关

对应上游 issue：
  #602 课程名含 &nbsp;(\xa0) 时控制台编码崩溃
  #615 #575 填空题答案保存后为空
  #603 思考模型连接检查误判失败
  #588 视频并发导致进度回退（提供串行开关）
全部测试不联网、不读写用户真实数据。
"""
import os
import sys
import tempfile
import threading
import time
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# 必须在导入 api 之前指定数据目录，避免碰到用户真实配置
os.environ.setdefault("CX_DATA_HOME", tempfile.mkdtemp(prefix="cx-test-"))

from api import answer as answer_mod  # noqa: E402
from api.base import (  # noqa: E402
    Chaoxing,
    StudyResult,
    build_completion_fields,
    split_completion_answer,
)
from api.decode import _get_question_type, clean_text, decode_course_list  # noqa: E402


class CleanTextTestCase(unittest.TestCase):
    """#602：特殊空白字符在 GBK 控制台会直接把程序打崩"""

    def test_nbsp_becomes_space(self):
        self.assertEqual(clean_text("数据\xa0结构"), "数据 结构")

    def test_exotic_spaces_and_zero_width(self):
        self.assertEqual(clean_text("a\u3000b"), "a b")
        self.assertEqual(clean_text("a\u200bb"), "ab")

    def test_none_and_blank(self):
        self.assertEqual(clean_text(None), "")
        self.assertEqual(clean_text("   "), "")

    def test_normal_text_kept(self):
        self.assertEqual(clean_text("第一章 绪论"), "第一章 绪论")

    def test_course_title_is_cleaned(self):
        html = (
            '<div class="course" id="1" info="i" roleid="2">'
            '<input class="clazzId" value="9"><input class="courseId" value="8">'
            '<a href="x?cpi=123&y=1"></a>'
            '<span class="course-name" title="数据结构\xa0与算法"></span>'
            '<p class="margint10" title="描述"></p><p class="color3" title="张老师"></p></div>'
        )
        courses = decode_course_list(html)
        self.assertEqual(courses[0]["title"], "数据结构 与算法")


class CompletionAnswerTestCase(unittest.TestCase):
    """#615 / #575：填空题必须按空提交，否则网页端显示答案为空"""

    def test_split_two_blanks(self):
        self.assertEqual(split_completion_answer("并发#线程", 2), ["并发", "线程"])

    def test_single_blank_keeps_hash(self):
        # 只有一个空时不能按 # 拆，否则 "C# 语言" 会被切坏
        self.assertEqual(split_completion_answer("C# 语言", 1), ["C# 语言"])

    def test_unknown_count_splits_by_hash(self):
        self.assertEqual(split_completion_answer("a#b", 0), ["a", "b"])

    def test_more_expected_than_answers(self):
        self.assertEqual(split_completion_answer("a#b#c", 2), ["a", "b#c"])

    def test_list_answer(self):
        self.assertEqual(split_completion_answer(["a", "b"], 2), ["a", "b"])

    def test_empty_inputs(self):
        self.assertEqual(split_completion_answer(None, 2), [])
        self.assertEqual(split_completion_answer("", 2), [])

    @staticmethod
    def _question(qid="42", qtype="completion", atype="2", source="cover"):
        return {
            "id": qid,
            "type": qtype,
            "answerField": {f"answer{qid}": "", f"answertype{qid}": atype},
            f"answerSource{qid}": source,
        }

    def test_completion_fields_built(self):
        form = {"workId": "1", "tiankongsize42": "2", "answer42": "并发#线程"}
        build_completion_fields(form, self._question())
        self.assertEqual(form["answerEditor421"], "并发")
        self.assertEqual(form["answerEditor422"], "线程")
        self.assertEqual(form["tiankongsize42"], 2)
        self.assertNotIn("answer42", form)

    def test_single_blank_payload(self):
        form = {"tiankongsize42": "1", "answer42": "C# 语言"}
        build_completion_fields(form, self._question())
        self.assertEqual(form["answerEditor421"], "C# 语言")
        self.assertEqual(form["tiankongsize42"], 1)

    def test_missing_declared_count(self):
        form = {"answer42": "a#b"}
        build_completion_fields(form, self._question())
        self.assertEqual(form["answerEditor421"], "a")
        self.assertEqual(form["answerEditor422"], "b")
        self.assertEqual(form["tiankongsize42"], 2)

    def test_empty_answer_still_reports_blanks(self):
        form = {"tiankongsize42": "2", "answer42": ""}
        build_completion_fields(form, self._question(source="random"))
        self.assertEqual(form["answerEditor421"], "")
        self.assertEqual(form["answerEditor422"], "")
        self.assertEqual(form["tiankongsize42"], 2)

    def test_type_code_10_is_completion(self):
        self.assertEqual(_get_question_type("10"), "completion")
        form = {"tiankongsize42": "1", "answer42": "x"}
        build_completion_fields(form, self._question(atype="10"))
        self.assertEqual(form["answerEditor421"], "x")

    def test_other_types_untouched(self):
        form = {"answer42": "A", "answertype42": "0"}
        build_completion_fields(form, self._question(qtype="single", atype="0"))
        self.assertEqual(form["answer42"], "A")
        self.assertNotIn("tiankongsize42", form)


class _FakeResponse:
    status_code = 200
    text = "ok"

    def __init__(self, payload):
        self._payload = payload

    def json(self):
        return self._payload


class LlmConnectionTestCase(unittest.TestCase):
    """#603：思考模型只返回 reasoning_content 时不能误判为连接失败"""

    def setUp(self):
        self._orig_post = answer_mod.requests.post

    def tearDown(self):
        answer_mod.requests.post = self._orig_post

    def _provider(self):
        provider = answer_mod.SiliconFlow()
        provider.config_set({
            "siliconflow_key": "k",
            "siliconflow_model": "m",
            "siliconflow_endpoint": "http://example.invalid",
        })
        provider._init_tiku()
        provider.min_interval = 0
        return provider

    def test_reasoning_content_counts_as_success(self):
        answer_mod.requests.post = lambda *a, **kw: _FakeResponse(
            {"choices": [{"message": {"content": "", "reasoning_content": "1+1=2"}}]}
        )
        self.assertTrue(self._provider().check_llm_connection())

    def test_malformed_response_does_not_raise(self):
        answer_mod.requests.post = lambda *a, **kw: _FakeResponse({})
        self.assertFalse(self._provider().check_llm_connection())

    def test_400_has_readable_message(self):
        message = answer_mod.brief_error(Exception("Error code: 400 - bad request"))
        self.assertIn("400", message)


class VideoSerialTestCase(unittest.TestCase):
    """#588：serial_video = true 时同一个进程一次只播一个视频"""

    def setUp(self):
        self._orig = Chaoxing._study_video
        self.counter = {"now": 0, "max": 0}
        self._lock = threading.Lock()

        counter = self.counter
        lock = self._lock

        # 注意：第一个参数是被替换方法里的 Chaoxing 实例，别用 self 命名，免得遮住测试用例
        def fake_play(chaoxing_self, _course, _job, _job_info, _speed=1.0, _type="Video"):
            with lock:
                counter["now"] += 1
                counter["max"] = max(counter["max"], counter["now"])
            time.sleep(0.05)
            with lock:
                counter["now"] -= 1
            return StudyResult.SUCCESS

        Chaoxing._study_video = fake_play

    def tearDown(self):
        Chaoxing._study_video = self._orig

    def _run(self, serial, workers=4):
        cx = Chaoxing(account=None, tiku=None, serial_video=serial)
        threads = [
            threading.Thread(target=cx.study_video, args=({}, {}, {}))
            for _ in range(workers)
        ]
        for t in threads:
            t.start()
        for t in threads:
            t.join()

    def test_serial_mode_runs_one_video(self):
        self._run(True)
        self.assertEqual(self.counter["max"], 1)

    def test_default_mode_allows_concurrency(self):
        self._run(False)
        self.assertGreater(self.counter["max"], 1)


if __name__ == "__main__":
    unittest.main()
