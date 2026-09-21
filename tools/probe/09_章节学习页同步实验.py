# -*- coding: utf-8 -*-
"""验证：章节类任务点打开任务引擎给出的学习地址（courseapi/stustudy-page-transfer）后是否会被同步为完成"""
import os, sys, re, time, json
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
os.chdir(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import probe_config  # noqa: E402
from api.base import SessionManager
from api import cookies as cookie_mod
from api.task_center import TaskCenter
s = SessionManager.get_session(); s.cookies.update(cookie_mod.use_cookies(probe_config.account()))
tc = TaskCenter(object(), {})
C = probe_config.course()
task = next(t for t in tc.get_course_tasks(C) if t["id"] == 5026526)
info = tc.open_task(task)
targets = []
for g in tc.get_groups(info["encryTaskUserId"]):
    if not g["groupAllowStudy"]:
        continue
    for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
        if p["planType"] == 8 and not tc.plan_finished(p):
            targets.append((g, p))
print("待同步章节类任务点:", [p["name"] for _, p in targets], flush=True)
for g, p in targets:
    url = tc.get_study_url(info["encryTaskUserId"], p["encryptPlanId"])
    print("打开:", p["name"], url, flush=True)
    try:
        r = s.get(url, allow_redirects=True, timeout=20)
        print("   ->", r.status_code, len(r.text), r.url[:160], flush=True)
    except Exception as e:
        print("   ERR", e, flush=True)
    for i in range(4):
        time.sleep(4)
        if tc.is_plan_finished(info["encryTaskUserId"], g["encryptGroupId"], p["planId"]):
            print("   任务中心显示完成 ✓", flush=True)
            break
    else:
        print("   任务中心仍未完成", flush=True)
t2 = next(x for x in tc.get_course_tasks(C) if x["id"] == 5026526)
print("第1章进度:", t2["taskStudyProgress"], t2["planFinishCount"], "/", t2["planCount"], flush=True)
