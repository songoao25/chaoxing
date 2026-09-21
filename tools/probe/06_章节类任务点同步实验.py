# -*- coding: utf-8 -*-
"""章节类任务点（planType=8）同步实验

背景（2026-09-17 抓包确认）：
  任务引擎的"章节"任务点，网页端是打开 mooc 章节页
  （studentstudy?...&isTaskEngineNode=true，页面注入 window.isEngineNode="1"），
  视频打点 / mooc-ans/job/document 的响应里会带 stuJobInfo；
  章节页把它 postMessage 给任务中心父页面，父页面再 POST
  /userStudyPlan/autoPullChapterScore。enc 由平台签发，**不能自己拼**。

本探针：
  1) 默认只读：列出当前教学任务里未完成的章节类任务点；
  2) 加 --kid <knowledgeId> 时，对那一个任务点跑完整链路
     （复用 main._complete_teaching_plan：章节刷课 engine_info=True → 同步 → 复查）。

用法：
  python tools/probe/06_章节类任务点同步实验.py
  python tools/probe/06_章节类任务点同步实验.py --kid 1199477654
"""
import argparse
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
os.chdir(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import probe_config  # noqa: E402

from api.base import SessionManager, Chaoxing  # noqa: E402
from api import cookies as cookie_mod  # noqa: E402
import main  # noqa: E402


def main_probe():
    parser = argparse.ArgumentParser()
    parser.add_argument("--account", default=probe_config.get("CX_ACCOUNT"), help="cookie 账号名")
    parser.add_argument("--course-id", default=probe_config.get("CX_COURSE_ID"))
    parser.add_argument("--clazz-id", default=probe_config.get("CX_CLAZZ_ID"))
    parser.add_argument("--cpi", default=probe_config.get("CX_CPI"))
    parser.add_argument("--task-id", type=int, default=5026526,
                        help="教学任务 id（默认第1章）")
    parser.add_argument("--kid", default=None,
                        help="要实验的 knowledgeId；不给则只列出未完成的章节任务点")
    args = parser.parse_args()

    session = SessionManager.get_session()
    session.cookies.update(cookie_mod.use_cookies(args.account))
    chaoxing = Chaoxing()
    course = {
        "title": probe_config.get("CX_COURSE_TITLE", "示例课程"),
        "courseId": args.course_id,
        "clazzId": args.clazz_id,
        "cpi": args.cpi,
    }
    config = {"speed": 1.0, "task_center_submit_mode": "confirm"}
    tc = main.TaskCenter(chaoxing, config)
    task = next(t for t in tc.get_course_tasks(course) if t["id"] == args.task_id)
    info = tc.open_task(task)
    point_map = main._load_course_point_map(chaoxing, course)

    targets = []
    for group in tc.get_groups(info["encryTaskUserId"]):
        if not group.get("groupAllowStudy"):
            continue
        for plan in tc.get_plans(info["encryTaskUserId"], group["encryptGroupId"]):
            if plan.get("planType") != 8:
                continue
            done = tc.plan_finished(plan)
            print("章节任务点: {:<28} knowledgeId={} finished={}".format(
                plan.get("name", "?"), plan.get("externalDataId"), done))
            if not done:
                plan["encryptGroupId"] = group["encryptGroupId"]
                targets.append(plan)

    if not args.kid:
        print()
        print("未完成的章节任务点 %d 个；加 --kid <knowledgeId> 跑其中一个（会真实播放）" % len(targets))
        return 0

    target = next((p for p in targets if str(p.get("externalDataId")) == str(args.kid)), None)
    if target is None:
        print("没有找到未完成的章节任务点 knowledgeId=%s" % args.kid)
        return 1
    print()
    print("开始处理:", target.get("name"))
    ok = main._complete_teaching_plan(tc, chaoxing, course, target, info, config, point_map)
    print("完成:", ok, "| outcome:", getattr(tc, "last_outcome", None))
    return 0 if ok else 2


if __name__ == "__main__":
    sys.exit(main_probe())
