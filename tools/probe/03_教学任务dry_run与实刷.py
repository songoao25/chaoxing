# -*- coding: utf-8 -*-
"""
真机验证：用已保存的 cookie（账号（本机 local.env） / 示例课程）跑一遍任务中心逻辑。

    python /tmp/cx_live_task_center.py dry     # 只读，列任务/分组/任务点/学习地址
    python /tmp/cx_live_task_center.py apply   # 真刷一个教学任务（开学第一课）
"""
import os
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
os.chdir(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import probe_config  # noqa: E402

from api.base import Chaoxing, SessionManager  # noqa: E402
from api import cookies as cookie_mod  # noqa: E402
from api.task_center import TaskCenter, plan_type_name  # noqa: E402
import main  # noqa: E402

ACCOUNT = probe_config.account()
COURSE = probe_config.course()
TARGET_TASK_ID = 5026538  # 开学第一课


class DisabledTiku:
    DISABLE = True

    def get_answer(self, *args, **kwargs):
        return None


def main_run():
    mode = sys.argv[1] if len(sys.argv) > 1 else "dry"
    SessionManager.get_session().cookies.update(cookie_mod.use_cookies(ACCOUNT))
    cx = Chaoxing(tiku=DisabledTiku())
    tc = TaskCenter(cx, {"speed": 2.0})

    tasks = tc.get_course_tasks(COURSE)
    print(f"教学任务 {len(tasks)} 个：")
    for t in tasks:
        print(f'  - {t["name"]}  进度={t["taskStudyProgress"]} 计划数={t["planCount"]} 完成数={t["planFinishCount"]}')

    if mode == "dry":
        for t in tasks:
            info = tc.open_task(t)
            if not info:
                print(f'  !! 打不开：{t["name"]}')
                continue
            print(f'\n== 任务「{t["name"]}」')
            for g in tc.get_groups(info["encryTaskUserId"]):
                plans = tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"])
                print(f'   分组 {g["taskGroup"]["name"]} 可学={g["groupAllowStudy"]} 条件={g["groupAllowStudyCondition"]}')
                for p in plans:
                    finished = tc.plan_finished(p)
                    url = ""
                    if not finished and g["groupAllowStudy"]:
                        url = tc.get_study_url(info["encryTaskUserId"], p["encryptPlanId"]) or "(未解锁)"
                    print(f'     - [{plan_type_name(p["planType"])}] {p["name"][:36]} 完成={finished} {url[:80]}')
        return

    # apply：只刷「开学第一课」这一个教学任务
    task = next((t for t in tasks if t["id"] == TARGET_TASK_ID), None)
    if not task:
        print("找不到目标教学任务")
        return
    point_map = main._load_course_point_map(cx, COURSE)
    print(f'\n开始刷教学任务：{task["name"]}（章节映射 {len(point_map)} 个）')
    started = time.time()
    ok, unsupported = main._process_teaching_task(tc, cx, COURSE, task, {"speed": 2.0}, point_map)
    print(f'结果：完成={ok} 未支持类型={sorted(unsupported)} 耗时={int(time.time()-started)}秒')

    # 复查
    time.sleep(3)
    tasks2 = tc.get_course_tasks(COURSE)
    t2 = next((x for x in tasks2 if x["id"] == TARGET_TASK_ID), None)
    print("复查：", {k: t2.get(k) for k in ("name", "taskStudyProgress", "planFinishCount", "planCount")})
    info = tc.open_task(task)
    for g in tc.get_groups(info["encryTaskUserId"]):
        plans = tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"])
        print(f'   分组 {g["taskGroup"]["name"]} 可学={g["groupAllowStudy"]}')
        for p in plans:
            print(f'     - [{plan_type_name(p["planType"])}] {p["name"][:30]} 完成={tc.plan_finished(p)}')


main_run()
