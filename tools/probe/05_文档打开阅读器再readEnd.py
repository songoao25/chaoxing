# -*- coding: utf-8 -*-
"""测试：先打开 pan-yz 阅读页，再调 readEnd，看文档任务点是否完成"""
import os, sys, re, json, time
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
os.chdir(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import probe_config  # noqa: E402
from api.base import SessionManager
from api import cookies as cookie_mod
from api.task_center import TaskCenter, _extract_json_object
s = SessionManager.get_session(); s.cookies.update(cookie_mod.use_cookies(probe_config.account()))
tc = TaskCenter(object(), {"speed": 1.0})
COURSE = probe_config.course()
task = next(t for t in tc.get_course_tasks(COURSE) if t["id"] == 5026528)
info = tc.open_task(task)
plan = group = None
for g in tc.get_groups(info["encryTaskUserId"]):
    for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
        if p["planType"] == 11 and not tc.plan_finished(p):
            plan, group = p, g
print("文档:", plan["name"], flush=True)
print("完成条件:", {k: v for k, v in (plan.get("planBreakthroughSet") or {}).items()
                    if "ocument" in k or "ead" in k}, flush=True)
print("planUser:", json.dumps(plan.get("planUser"), ensure_ascii=False)[:300], flush=True)

study_url = tc.get_study_url(info["encryTaskUserId"], plan["encryptPlanId"])
page = s.get(study_url).text
vo = _extract_json_object(page, "docStudyUrlInfo")
pan = vo["yunPanUrl"]
print("打开阅读页:", flush=True)
r1 = s.get(pan)
print("  pan:", r1.status_code, len(r1.text), flush=True)
m = re.search(r'encryPlanUserId\s*=\s*"([^"]+)"', page)
print("readEnd:", flush=True)
r2 = s.get("https://task.chaoxing.com/documentStudy/readEnd", params={"encryPlanUserId": m.group(1)})
print("  ", r2.status_code, r2.text[:300], flush=True)
for i in range(6):
    time.sleep(4)
    ok = tc.is_plan_finished(info["encryTaskUserId"], group["encryptGroupId"], plan["planId"])
    print(f"  第{i+1}次检查 完成={ok}", flush=True)
    if ok:
        break
