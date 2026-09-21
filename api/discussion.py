# -*- coding: utf-8 -*-
"""
讨论区浏览 + 挑帖子回复（模式 2）。

模式 1 是"任务里的主题讨论"：任务中心里 planType=14 的任务点，按分组顺序自动刷。
模式 2 就是这里：直接读课程讨论区的全部帖子，列出来让用户自己挑要回复哪一个。

接口（2026-09-21 只读取证，groupweb.chaoxing.com）：
  * 帖子列表 GET /pc/topic/topiclist/{bbsid}/getTopicList
      ?folder_uuid=&page=1&pageSize=20&kw=&last_reply_time=&searchType=&authMappId=&isSetTop=1
  * 帖子详情与发回复复用 TaskCenter.reply_topic（与模式 1 同一条链路：
      读已有回复 → 去 AI 味生成 → 提交 → 留痕 + 实时显示）。
"""

import re
import textwrap
from typing import List, Optional

from api.logger import logger

TOPIC_LIST_PATH = "/pc/topic/topiclist/{bbsid}/getTopicList"
DEFAULT_PAGE_SIZE = 20


def normalize_topic(item: dict) -> dict:
    """把接口返回的一条帖子整理成界面要用的字段"""
    item = item or {}
    title = str(item.get("title") or "").strip()
    content = re.sub(r"\s+", " ", str(item.get("content") or "")).strip()
    last = item.get("lastReply") or {}
    return {
        "uuid": str(item.get("uuid") or ""),
        "id": item.get("id"),
        "title": title or content,
        "content": content,
        "author": str(item.get("createrName") or "").strip(),
        "reply_count": int(item.get("reply_count") or 0),
        "time": str(item.get("ftime") or "").strip(),
        "last_reply_name": str(last.get("name") or "").strip(),
        "last_reply_time": str(last.get("formattime") or "").strip(),
        "tags": str(item.get("tags") or "").strip(),
        "top": bool(item.get("top")),
    }


def fetch_topics(session, bbsid: str, page: int = 1, page_size: int = DEFAULT_PAGE_SIZE,
                 keyword: str = "", base: str = "") -> List[dict]:
    """只读拉一页帖子；失败返回空列表（不抛异常）"""
    from api.task_center import DISCUSSION_BASE
    url = (base or DISCUSSION_BASE) + TOPIC_LIST_PATH.format(bbsid=bbsid)
    params = {
        "folder_uuid": "",
        "page": max(1, int(page or 1)),
        "pageSize": max(1, int(page_size or DEFAULT_PAGE_SIZE)),
        "kw": keyword or "",
        "last_reply_time": "",
        "searchType": "",
        "authMappId": "",
        "isSetTop": "1",
    }
    try:
        resp = session.get(url, params=params, timeout=20)
    except Exception as e:
        logger.warning("讨论区帖子列表请求失败: {}", e)
        return []
    if getattr(resp, "status_code", 0) != 200:
        logger.warning("讨论区帖子列表读取失败: HTTP {}", getattr(resp, "status_code", "?"))
        return []
    try:
        data = resp.json()
    except ValueError:
        logger.warning("讨论区帖子列表返回的不是 JSON（可能登录失效或页面改版）")
        return []
    if not isinstance(data, dict) or not data.get("status"):
        logger.warning("讨论区帖子列表被平台拒绝: {}", str(data)[:150])
        return []
    items = data.get("datas")
    if not isinstance(items, list):
        return []
    return [normalize_topic(it) for it in items if isinstance(it, dict) and it.get("uuid")]


def resolve_bbsid(tc, course: dict) -> str:
    """拿到课程的讨论区板块 id：优先问任务中心里的主题讨论任务点"""
    direct = str((course or {}).get("bbsid") or "").strip()
    if direct:
        return direct
    try:
        from api.task_center import PLAN_TYPE_DISCUSS
        for task in tc.get_course_tasks(course) or []:
            info = tc.open_task(task)
            if not info:
                continue
            groups = tc.get_groups(info.get("encryTaskUserId", "")) or []
            for group in groups:
                plans = tc.get_plans(info.get("encryTaskUserId", ""),
                                     group.get("encryptGroupId", "")) or []
                for plan in plans:
                    if int(plan.get("planType") or -1) != PLAN_TYPE_DISCUSS:
                        continue
                    url = tc.get_study_url(info.get("encryTaskUserId", ""),
                                           plan.get("encryptPlanId"))
                    matched = re.search(r"bbsid=([0-9a-fA-F]+)", url or "")
                    if matched:
                        return matched.group(1)
    except Exception as e:
        logger.debug("从任务中心解析讨论区板块失败: {}", e)
    return ""


def render_topics(topics: List[dict], page: int = 1) -> str:
    """列表视图：编号 + 回复数 + 作者 + 时间 + 正文前两行"""
    if not topics:
        return "\n  这一页没有帖子（可能已经翻到底，或该课程还没开放讨论区）。\n"
    lines = ["", f"  第 {page} 页 · {len(topics)} 条", ""]
    for idx, topic in enumerate(topics, 1):
        meta = f"[{topic.get('reply_count', 0)} 回复]"
        who = topic.get("author") or "同学"
        when = topic.get("time") or topic.get("last_reply_time") or ""
        head = f"  {idx:>2}. {meta} {who}" + (f" · {when}" if when else "")
        lines.append(head)
        body = textwrap.fill(topic.get("title") or topic.get("content") or "",
                             width=62, initial_indent="      ", subsequent_indent="      ")
        lines.append(body)
    lines.append("")
    return "\n".join(lines)


def _ask(prompt: str) -> str:
    try:
        return input(prompt).strip()
    except (EOFError, KeyboardInterrupt):
        return "q"


def discuss_cli(chaoxing, tc, config: dict, list_only: bool = False,
                course_id: Optional[str] = None, courses: Optional[list] = None) -> int:
    """cx discuss 的入口：浏览课程讨论区，挑帖子回复。"""
    from api.task_center import TaskCenter
    if tc is None:
        tc = TaskCenter(chaoxing, config)
    if courses is None:
        try:
            courses = chaoxing.get_course_list() or []
        except Exception as e:
            print(f"  ✘ 读取课程列表失败：{e}")
            return 1
    courses = list(courses)
    wanted = str(course_id or "").strip()
    if wanted:
        courses = [c for c in courses if str(c.get("courseId")) == wanted]
    if not courses:
        print("  ✘ 没有可用的课程")
        return 1
    if len(courses) > 1 and not list_only:
        print()
        print("  选择课程")
        print("  " + "─" * 46)
        for i, c in enumerate(courses, 1):
            print(f"   {i:>2}. {c.get('title', '')}")
        print()
        raw = _ask("  ▶ 输入序号（q 退出） ")
        if raw.lower() in ("q", "quit", "exit", ""):
            return 0
        if not (raw.isdigit() and 1 <= int(raw) <= len(courses)):
            print("  ✘ 没有这个选项")
            return 1
        courses = [courses[int(raw) - 1]]
    course = courses[0]
    bbsid = resolve_bbsid(tc, course)
    if not bbsid:
        # 这门课没开讨论区就往下试，别让用户自己猜哪门课有
        for candidate in courses[1:]:
            found = resolve_bbsid(tc, candidate)
            if found:
                course, bbsid = candidate, found
                break
    if not bbsid:
        titles = "、".join(str(c.get("title") or "") for c in courses[:5])
        print(f"  ✘ 这些课程都没找到讨论区板块：{titles}")
        return 1

    page = 1
    while True:
        topics = fetch_topics(tc.session, bbsid, page=page)
        print()
        print(f"  讨论区 · {course.get('title', '')}")
        print("  " + "─" * 46)
        print(render_topics(topics, page))
        if list_only:
            return 0 if topics else 1
        if not topics and page > 1:
            page -= 1
            continue
        raw = _ask("  ▶ 输入序号回复该帖（n 下一页 · p 上一页 · q 退出） ")
        low = raw.lower()
        if low in ("q", "quit", "exit", ""):
            return 0
        if low == "n":
            page += 1
            continue
        if low == "p":
            page = max(1, page - 1)
            continue
        if not (raw.isdigit() and 1 <= int(raw) <= len(topics)):
            print("  ✘ 没有这个选项")
            continue
        topic = topics[int(raw) - 1]
        print()
        print(f"  正在为「{topic.get('title', '')[:30]}」生成回复…")
        ok = tc.reply_topic(bbsid, topic["uuid"], course=course,
                            name=topic.get("title") or "讨论区帖子")
        print()
        if ok:
            print("  ✓ 已回复（平台约 3 分钟后才显示完成，可在 ./cx review 里复核）")
        else:
            print("  ✘ 这次没有回复成功（详见运行日志），可以换一条再试")
        print()
