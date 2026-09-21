# -*- coding: utf-8 -*-
"""严格分类探明（轻量版）：入口 / 类型 / 规则 / 接口，抽样章节避免打爆接口"""
import os, sys, json, re, collections
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
os.chdir(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import probe_config  # noqa: E402
from api.base import Chaoxing, SessionManager
from api import cookies as cookie_mod
from api.task_center import TaskCenter, plan_type_name

s = SessionManager.get_session(); s.cookies.update(cookie_mod.use_cookies(probe_config.account()))
cx = Chaoxing()
tc = TaskCenter(cx, {"speed": 1.0})
C = probe_config.course()
out = {}
PATH = "/tmp/cx_classification.json"

def save():
    json.dump(out, open(PATH, "w", encoding="utf-8"), ensure_ascii=False, indent=1)

# 1) 教学任务：任务 -> 分组 -> 任务点 + 完成规则
tasks = tc.get_course_tasks(C)
out["teaching_tasks"] = []
for t in tasks:
    info = tc.open_task(t)
    groups = tc.get_groups(info["encryTaskUserId"])
    g_out = []
    for g in groups:
        p_out = []
        for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
            bt = p.get("planBreakthroughSet") or {}
            rules = {k: v for k, v in bt.items()
                     if (k.startswith("enable") and v) or k in ("videoWatchDuration", "documentWatchDuration")}
            p_out.append({"planId": p["planId"], "type": p["planType"], "typeName": plan_type_name(p["planType"]),
                          "name": p["name"], "finish": tc.plan_finished(p),
                          "externalDataId": p.get("externalDataId"), "rules": rules,
                          "allowStudy": p.get("planAllowStudy")})
        g_out.append({"group": g["taskGroup"]["name"], "allow": g["groupAllowStudy"],
                      "condition": g["groupAllowStudyCondition"], "plans": p_out})
    out["teaching_tasks"].append({"id": t["id"], "name": t["name"], "progress": t["taskStudyProgress"],
                                  "planFinish": t["planFinishCount"], "planCount": t["planCount"], "groups": g_out})
save(); print("教学任务 ok", flush=True)

# 2) 课堂活动
try:
    acts = (s.get("https://mooc2-ans.chaoxing.com/mooc2-ans/fanya3/s/activityList",
                   params={"courseId": C["courseId"], "clazzId": C["clazzId"], "cpi": C["cpi"]}).json().get("data") or {}).get("activeList") or []
except Exception as e:
    acts = [{"error": str(e)}]
out["activities"] = [{"id": a.get("id"), "type": a.get("type"), "nameOne": a.get("nameOne"),
                      "nameFour": a.get("nameFour"), "url": a.get("url"), "status": a.get("status")} for a in acts]

# 3) 作业 / 考试
def jget(url, params):
    try:
        return s.get(url, params=params).json()
    except Exception as e:
        return {"error": str(e)}

out["homework"] = jget("https://mooc1-api.chaoxing.com/mooc-ans/work/api/task",
                       {"courseId": C["courseId"], "clazzId": C["clazzId"], "cpi": C["cpi"], "pageNum": 1, "pageSize": 50})
out["exam"] = jget("https://mooc1-api.chaoxing.com/exam-ans/mooc2/exam/task",
                   {"courseId": C["courseId"], "clazzId": C["clazzId"], "cpi": C["cpi"]})
out["study_data"] = {name: jget(f"https://mooc2-ans.chaoxing.com/stat2/study-data/v3/{name}",
                                {"courseid": C["courseId"], "clazzid": C["clazzId"], "cpi": C["cpi"], "ut": "s"})
                     for name in ("overview", "job", "work", "test", "exam", "ai-evaluate")}
save(); print("课堂活动/作业/考试/学情 ok", flush=True)

# 4) 章节 + 抽样任务点类型
points = (cx.get_course_point(C["courseId"], C["clazzId"], C["cpi"]) or {}).get("points") or []
sample = points[:3] + points[::10]
counter = collections.Counter()
detail = []
for p in sample:
    jobs, _info = cx.get_job_list(C, p)
    types = [j.get("type") for j in (jobs or [])]
    counter.update(types)
    detail.append({"id": p["id"], "title": p["title"], "jobCount": p["jobCount"],
                   "finished": p["has_finished"], "need_unlock": p.get("need_unlock"),
                   "jobTypes": types})
out["chapters_total"] = len(points)
out["chapters_sample"] = detail
out["chapter_job_type_counter"] = dict(counter)
save(); print("章节抽样 ok", flush=True)
print("=== 摘要 ===")
print("教学任务:", len(out["teaching_tasks"]))
print("课堂活动:", [(a.get("type"), a.get("nameOne") or a.get("nameFour")) for a in out["activities"]])
print("作业:", json.dumps(out["homework"], ensure_ascii=False)[:400])
print("考试:", json.dumps(out["exam"], ensure_ascii=False)[:400])
print("学情概览:", json.dumps(out["study_data"]["overview"], ensure_ascii=False)[:700])
print("章节抽样类型:", dict(counter))
print("已保存", PATH)
