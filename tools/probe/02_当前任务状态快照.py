# -*- coding: utf-8 -*-
"""当前任务状态快照（只读），并把已解锁的作业学习地址 dump 出来"""
import os, sys, re, json
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
os.chdir(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import probe_config  # noqa: E402
from api.base import SessionManager
from api import cookies as cookie_mod
from api.task_center import TaskCenter, plan_type_name
s = SessionManager.get_session(); s.cookies.update(cookie_mod.use_cookies(probe_config.account()))
tc = TaskCenter(object(), {})
C = probe_config.course()
for t in tc.get_course_tasks(C):
    print(f'== {t["name"]} {t["planFinishCount"]}/{t["planCount"]} 进度={t["taskStudyProgress"]}')
    info = tc.open_task(t)
    for g in tc.get_groups(info["encryTaskUserId"]):
        flags = "可学" if g["groupAllowStudy"] else "锁"
        for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
            if tc.plan_finished(p):
                continue
            extra = ""
            if g["groupAllowStudy"]:
                url = tc.get_study_url(info["encryTaskUserId"], p["encryptPlanId"])
                extra = (url or "(取不到)")[:110]
            print(f'   [{flags}] [{p["planType"]}{plan_type_name(p["planType"])}] {p["name"][:26]} {extra}')
