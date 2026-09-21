# -*- coding: utf-8 -*-
"""实验：用 readPoint 打点把文档读到要求时长（第3章需要 5 分钟）"""
import os, sys, re, json, time
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
os.chdir(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import probe_config  # noqa: E402
from api.base import SessionManager
from api import cookies as cookie_mod
from api.task_center import (
    READ_POINT_HEADERS,
    TaskCenter,
    _browser_timestamp,
    _extract_json_object,
    _read_point_enc,
)
s = SessionManager.get_session(); s.cookies.update(cookie_mod.use_cookies(probe_config.account()))
tc = TaskCenter(object(), {})
C = probe_config.course()
task = next(t for t in tc.get_course_tasks(C) if t["id"] == 5026528)
info = tc.open_task(task)
plan = group = None
for g in tc.get_groups(info["encryTaskUserId"]):
    for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
        if p["planType"] == 11 and not tc.plan_finished(p):
            plan, group = p, g
print("文档:", plan["name"], flush=True)
page = s.get(tc.get_study_url(info["encryTaskUserId"], plan["encryptPlanId"])).text
vo = _extract_json_object(page, "docStudyUrlInfo")
pan = vo["yunPanUrl"]
t = s.get(pan).text
m = re.search(r'id="markDataStr"[^>]*>(.*?)</div>', t, re.S)
md = json.loads(m.group(1).strip())
md["curPage"] = 1
mark_for_log = dict(md)
if "token" in mark_for_log:
    mark_for_log["token"] = "<REDACTED>"
print("markData:", json.dumps(mark_for_log, ensure_ascii=False)[:200], flush=True)
encry_plan_user_id = re.search(r'encryPlanUserId\s*=\s*"([^"]+)"', page).group(1)
required_seconds = max(1, int(tc.need_seconds(plan, "enableDocumentWatchDuration", "documentWatchDuration")))
max_points = (required_seconds + 29) // 30 + 1

def ping(seq):
    d = {
        "r": md["resourceID"], "t": md["resourceType"], "l": md.get("location", 1), "f": md.get("from", 4),
        "p": md["curPage"], "tp": md["totalPage"], "wc": 0, "ic": 2, "v": 2,
        "s": 1 if seq == 1 else 2, "h": 0,
        "ext": md["ext"],
    }
    params = {
        "f": "readPoint", "u": md["passportUID"], "d": json.dumps(d, ensure_ascii=False, separators=(",", ":")),
        "t": _browser_timestamp(),
    }
    params["enc"] = _read_point_enc(params)
    r = s.get(
        "https://data-xxt.aichaoxing.com/analysis/ac_mark",
        params=params,
        headers=READ_POINT_HEADERS,
        timeout=15,
    )
    return r.status_code

started = time.time()
for seq in range(1, max_points + 1):
    code = ping(seq)
    waited = int(time.time() - started)
    print(f"[{waited:>3}s] 打点 #{seq} HTTP {code}", flush=True)
    if seq % 2 == 0 or seq == max_points:
        ok = tc.is_plan_finished(info["encryTaskUserId"], group["encryptGroupId"], plan["planId"])
        print(f"[{int(time.time()-started):>3}s] 完成={ok}", flush=True)
        if ok:
            break
    if seq < max_points:
        time.sleep(30)
else:
    ok = False
print("打点结束，调用 readEnd", flush=True)
r = s.get("https://task.chaoxing.com/documentStudy/readEnd", params={"encryPlanUserId": encry_plan_user_id})
print("readEnd:", r.status_code, r.text[:120], flush=True)
time.sleep(4)
print("最终完成:", tc.is_plan_finished(info["encryTaskUserId"], group["encryptGroupId"], plan["planId"]), flush=True)
t2 = next(x for x in tc.get_course_tasks(C) if x["id"] == 5026528)
print("任务进度:", t2["taskStudyProgress"], t2["planFinishCount"], "/", t2["planCount"], flush=True)
