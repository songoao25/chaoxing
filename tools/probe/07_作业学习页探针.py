# -*- coding: utf-8 -*-
"""作业任务点探针：解锁后看学习地址 / 页面结构，并尝试复用 study_work 作答"""
import os, sys, re, json, configparser, time
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
os.chdir(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import probe_config  # noqa: E402
from api.base import Chaoxing, SessionManager, StudyResult
from api import cookies as cookie_mod, paths
from api.answer import Tiku
from api.task_center import TaskCenter
import main

SessionManager.get_session().cookies.update(cookie_mod.use_cookies(probe_config.account()))
cfg = configparser.ConfigParser(); cfg.read(paths.config_path(), encoding="utf-8")
tiku = Tiku.get_tiku_from_config(dict(cfg.items("tiku")), config_path=paths.config_path()); tiku.init_tiku()
cx = Chaoxing(tiku=tiku, work_max_retries=3)
tc = TaskCenter(cx, {"speed": 1.0})
C = probe_config.course()

for t in tc.get_course_tasks(C):
    info = tc.open_task(t)
    for g in tc.get_groups(info["encryTaskUserId"]):
        for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
            if p["planType"] != 4 or tc.plan_finished(p):
                continue
            print(f'作业: {p["name"]} 分组={g["taskGroup"]["name"]} 可学={g["groupAllowStudy"]}')
            if not g["groupAllowStudy"]:
                print("  （分组未解锁）")
                continue
            url = tc.get_study_url(info["encryTaskUserId"], p["encryptPlanId"])
            print("  学习地址:", url)
            if not url:
                continue
            r = tc.session.get(url, allow_redirects=True)
            print("  页面:", r.status_code, len(r.text), r.url)
            print("  title:", (re.search(r"<title>(.*?)</title>", r.text) or [None, "?"])[1])
            for kw in ("workId", "workid", "enc", "jobid", "ktoken", "api/work", "addStudentWorkNew", "answerId"):
                i = r.text.find(kw)
                if i > 0:
                    print(f"   {kw}:", re.sub(r"\s+", " ", r.text[max(0,i-100):i+160])[:260])
            open("/tmp/homework_page.html", "w", encoding="utf-8").write(r.text)
            # 尝试用 study_work 作答
            m = re.search(r'workId[=:"]+(\d+)', r.text) or re.search(r"workId[=:]'?(\d+)", url or "")
            if m:
                work_id = m.group(1)
                print("  尝试 study_work, workId =", work_id)
                course = dict(C); course["clazzId"] = C["clazzId"]
                job = {"jobid": f"work-{work_id}", "otherinfo": "", "jtoken": "", "enc": "", "type": "workid"}
                job_info = {"knowledgeid": p.get("externalDataId") or "", "ktoken": "", "cpi": C["cpi"]}
                result = cx.study_work(course, job, job_info)
                print("  study_work ->", result)
                time.sleep(3)
                print("  完成:", tc.is_plan_finished(info["encryTaskUserId"], g["encryptGroupId"], p["planId"]))
            break
