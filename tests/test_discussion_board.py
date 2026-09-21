# -*- coding: utf-8 -*-
"""
讨论区浏览（模式 2）回归：列表解析、渲染、板块解析、只列不回复。

模式 1（任务里的主题讨论）在 tests/test_discussion.py 里；两者共用
TaskCenter.reply_topic，所以这里的重点是"能不能把讨论区的帖子列出来并选中"。
"""

import contextlib
import io
import os
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
os.environ.setdefault("CX_DATA_HOME", tempfile.mkdtemp(prefix="cx-board-"))

from api import discussion  # noqa: E402


class FakeResp:
    def __init__(self, payload, status=200):
        self.status_code = status
        self._payload = payload
        self.text = "{}"

    def json(self):
        if isinstance(self._payload, Exception):
            raise self._payload
        return self._payload


class FakeSession:
    def __init__(self, payload):
        self.payload = payload
        self.calls = []

    def get(self, url, **kwargs):
        self.calls.append((url, kwargs))
        return FakeResp(self.payload)


def _topic_item(**over):
    item = {
        "uuid": "uuid-1",
        "id": 711682418,
        "title": "",
        "content": "企业外部环境存在机会，是否就代表企业一定可以抓住机会？",
        "createrName": "徐飞阳",
        "reply_count": 1,
        "ftime": "2小时前",
        "lastReply": {"name": "徐飞阳", "formattime": "2小时前"},
    }
    item.update(over)
    return item


class NormalizeTopicTestCase(unittest.TestCase):
    def test_title_falls_back_to_content(self):
        topic = discussion.normalize_topic(_topic_item())
        self.assertIn("企业外部环境", topic["title"])
        self.assertEqual(topic["author"], "徐飞阳")
        self.assertEqual(topic["reply_count"], 1)

    def test_real_title_wins(self):
        topic = discussion.normalize_topic(_topic_item(title="这是标题"))
        self.assertEqual(topic["title"], "这是标题")


class FetchTopicsTestCase(unittest.TestCase):
    def test_parses_topic_list(self):
        payload = {"status": True, "datas": [_topic_item(), _topic_item(uuid="uuid-2")]}
        session = FakeSession(payload)
        topics = discussion.fetch_topics(session, "bbs-1", page=1)
        self.assertEqual(len(topics), 2)
        self.assertEqual(topics[0]["uuid"], "uuid-1")
        url, kwargs = session.calls[0]
        self.assertIn("/pc/topic/topiclist/bbs-1/getTopicList", url)
        self.assertEqual(kwargs["params"]["page"], 1)

    def test_platform_rejection_returns_empty(self):
        topics = discussion.fetch_topics(FakeSession({"status": False, "msg": "无权访问"}), "bbs-1")
        self.assertEqual(topics, [])

    def test_non_json_returns_empty(self):
        topics = discussion.fetch_topics(FakeSession(ValueError("not json")), "bbs-1")
        self.assertEqual(topics, [])

    def test_items_without_uuid_are_skipped(self):
        payload = {"status": True, "datas": [{"content": "没有 uuid"}, _topic_item()]}
        topics = discussion.fetch_topics(FakeSession(payload), "bbs-1")
        self.assertEqual([t["uuid"] for t in topics], ["uuid-1"])


class RenderTopicsTestCase(unittest.TestCase):
    def test_list_view_has_numbers_and_body(self):
        topics = [discussion.normalize_topic(_topic_item()),
                  discussion.normalize_topic(_topic_item(uuid="u2", createrName="范钟贤"))]
        out = discussion.render_topics(topics, page=1)
        self.assertIn("第 1 页", out)
        self.assertIn("1.", out)
        self.assertIn("徐飞阳", out)
        self.assertIn("企业外部环境", out)

    def test_empty_page_is_friendly(self):
        self.assertIn("没有帖子", discussion.render_topics([], page=3))


class FakeTaskCenter:
    def __init__(self, session, bbsid="2a4b2fff0f67dd5b88099bcd5c2a941e"):
        self.session = session
        self._bbsid = bbsid

    def get_course_tasks(self, course):
        return [{"name": "第1章"}]

    def open_task(self, task):
        return {"encryTaskUserId": "u1"}

    def get_groups(self, encry):
        return [{"encryptGroupId": "g1"}]

    def get_plans(self, encry, group):
        return [{"planType": 14, "encryptPlanId": "p1"}]

    def get_study_url(self, encry, plan_id):
        return ("https://groupweb.chaoxing.com/pc/topic/jumpToTopicDetail?"
                "bbsid=" + self._bbsid + "&uuid=topic-uuid")


class ResolveBbsidTestCase(unittest.TestCase):
    def test_bbsid_from_task_center(self):
        tc = FakeTaskCenter(FakeSession({"status": True, "datas": []}))
        self.assertEqual(discussion.resolve_bbsid(tc, {"title": "课"}),
                         "2a4b2fff0f67dd5b88099bcd5c2a941e")

    def test_direct_bbsid_wins(self):
        tc = FakeTaskCenter(FakeSession({"status": True, "datas": []}))
        self.assertEqual(discussion.resolve_bbsid(tc, {"bbsid": "direct"}), "direct")

    def test_missing_board_returns_empty(self):
        class EmptyTC:
            def get_course_tasks(self, course):
                return []
        self.assertEqual(discussion.resolve_bbsid(EmptyTC(), {"title": "课"}), "")


class DiscussCliListOnlyTestCase(unittest.TestCase):
    def test_list_only_prints_topics_and_returns_zero(self):
        payload = {"status": True, "datas": [_topic_item()]}
        session = FakeSession(payload)
        tc = FakeTaskCenter(session)
        chaoxing = mock.Mock()
        chaoxing.get_course_list.return_value = [{"courseId": "1", "title": "企业战略管理"}]
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            code = discussion.discuss_cli(chaoxing, tc, {}, list_only=True)
        self.assertEqual(code, 0)
        out = buf.getvalue()
        self.assertIn("讨论区 · 企业战略管理", out)
        self.assertIn("企业外部环境", out)

    def test_course_without_board_falls_through_to_next(self):
        payload = {"status": True, "datas": [_topic_item()]}
        session = FakeSession(payload)
        tc = FakeTaskCenter(session)

        class Chaoxing:
            def get_course_list(self):
                return [{"courseId": "1", "title": "没讨论区的课"},
                        {"courseId": "2", "title": "企业战略管理"}]

        real_resolve = discussion.resolve_bbsid
        calls = []

        def fake_resolve(tc_, course):
            calls.append(course.get("title"))
            return "2a4b2fff0f67dd5b88099bcd5c2a941e" if course.get("courseId") == "2" else ""

        with mock.patch.object(discussion, "resolve_bbsid", side_effect=fake_resolve):
            buf = io.StringIO()
            with contextlib.redirect_stdout(buf):
                code = discussion.discuss_cli(Chaoxing(), tc, {}, list_only=True)
        self.assertEqual(code, 0)
        self.assertEqual(calls[:2], ["没讨论区的课", "企业战略管理"])
        self.assertIn("企业战略管理", buf.getvalue())


if __name__ == "__main__":
    unittest.main()
