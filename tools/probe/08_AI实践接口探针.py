# -*- coding: utf-8 -*-
"""AI实践（思维阶梯）接口探测：init / load-data"""
import os, sys, re, json, time
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
task = next(t for t in tc.get_course_tasks(C) if t["id"] == 5026538)
info = tc.open_task(task)
plan = None
for g in tc.get_groups(info["encryTaskUserId"]):
    for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
        if p["planType"] == 15:
            plan = p
url = tc.get_study_url(info["encryTaskUserId"], plan["encryptPlanId"])
r = s.get(url, allow_redirects=True)
print("final:", r.url)
params = {}
for _, k, v in re.findall(r"([?&])([A-Za-z]+)=([^&]*)", r.url):
    params[k] = v
print("SPA params:", params)
base = "https://mooc2-ans.chaoxing.com"
for ep in ("/mooc2-ans/ai-evaluate/v2/answer/init",
           "/mooc2-ans/ai-evaluate/v2/answer/load-data",
           "/mooc2-ans/ai-evaluate/v2/answer/load-review-data"):
    for method in ("post", "get"):
        try:
            resp = getattr(s, method)(base + ep, params=params if method == "get" else None,
                                      data=params if method == "post" else None, timeout=20)
            body = resp.text
            print(f"--- {ep} {method.upper()}: {resp.status_code} len={len(body)}")
            print("   ", body[:900].replace("\n", " "))
        except Exception as e:
            print(ep, method, "ERR", e)
