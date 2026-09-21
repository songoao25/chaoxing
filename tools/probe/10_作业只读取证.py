# -*- coding: utf-8 -*-
"""作业任务点只读取证（绝不提交任何作答）

做三件事：
1. 列出教学任务里 planType=4 的作业任务点（完成状态 / 分组是否可学 / plan 关键字段）
2. 对已解锁的作业：调 getToStudyUrl 拿学习地址，GET 页面（跟随 302）
3. 只打印字段线索（workId / enc / ktoken / knowledgeid / cpi / api 接口名），HTML 存到 /tmp

用法：
  ./.venv/bin/python tools/probe/10_作业只读取证.py [账号] [courseId]
"""
import os, sys, re, time

BASE = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, BASE)
os.chdir(BASE)

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import probe_config  # noqa: E402
from api.base import SessionManager
from api import cookies as cookie_mod, paths
from api.task_center import TaskCenter, plan_type_name, PLAN_TYPE_HOMEWORK

ACCOUNT = sys.argv[1] if len(sys.argv) > 1 else probe_config.account()
COURSE_ID = sys.argv[2] if len(sys.argv) > 2 else probe_config.get("CX_COURSE_ID")

s = SessionManager.get_session()
s.cookies.update(cookie_mod.use_cookies(ACCOUNT))
tc = TaskCenter(object(), {})
C = {{**probe_config.course(), "courseId": COURSE_ID or probe_config.get("CX_COURSE_ID")}}

KEYWORDS = ("workId", "workid", "workAnswerId", "ktoken", "knowledgeid", "cpi=",
            "jobid", "api/work", "addStudentWorkNew", "work/task", "workRelationId",
            "singleQuesId", "enc=")

found = False
# --dry-answer / --api-work 是独立的取证模式，不重复跑上面的全量页面 dump
_run_listing = not any(flag in sys.argv for flag in
                        ("--dry-answer", "--api-work", "--submit-once"))
for t in (tc.get_course_tasks(C) if _run_listing else []):
    info = tc.open_task(t)
    if not info:
        continue
    for g in tc.get_groups(info["encryTaskUserId"]):
        for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
            if int(p.get("planType", -1) or -1) != PLAN_TYPE_HOMEWORK:
                continue
            found = True
            print("=" * 70)
            print(f'作业：{t["name"]} / {g["taskGroup"]["name"]} / {p["name"]}')
            print("  isFinish =", tc.plan_finished(p), " groupAllowStudy =", g["groupAllowStudy"])
            print("  plan:", {k: p.get(k) for k in
                              ("planId", "encryptPlanId", "externalDataId", "groupSort", "sort")})
            if not g["groupAllowStudy"]:
                print("  （分组未解锁，本次不取页面）")
                continue
            url = tc.get_study_url(info["encryTaskUserId"], p["encryptPlanId"])
            print("  study_url =", url)
            if not url:
                continue
            try:
                r = tc.session.get(url, allow_redirects=True, timeout=20)
            except Exception as e:
                print("  GET 失败:", e)
                continue
            text = r.text or ""
            print("  GET ->", r.status_code, getattr(r, "url", ""), len(text))
            m = re.search(r"<title>(.*?)</title>", text, re.S)
            print("  title:", (m.group(1).strip() if m else "?"))
            path = f"/tmp/cx_homework_{p.get('planId')}.html"
            with open(path, "w", encoding="utf-8") as f:
                f.write(text)
            print("  已保存:", path)
            for kw in KEYWORDS:
                for hit in re.finditer(re.escape(kw), text):
                    i = hit.start()
                    ctx = re.sub(r"\s+", " ", text[max(0, i - 120):i + 180])
                    print(f"   [{kw}] …{ctx}…")
                    break
            time.sleep(2)

if "--submit-once" in sys.argv:
    # 真机验收：用生产代码路径提交第一个已解锁的作业，并复查任务中心状态。
    import configparser
    from api.base import Chaoxing
    from api.answer import Tiku

    cfg = configparser.ConfigParser()
    cfg.read(paths.config_path(), encoding="utf-8")
    tiku_cfg = dict(cfg.items("tiku")) if cfg.has_section("tiku") else {}
    tiku = Tiku.get_tiku_from_config(tiku_cfg, config_path=paths.config_path())
    tiku.init_tiku()
    cx = Chaoxing(tiku=tiku, work_max_retries=3)
    tc = TaskCenter(cx, {"speed": 1.0, "tiku_config": tiku_cfg,
                         "task_center_submit_mode": "auto"})

    for t in tc.get_course_tasks(C):
        info = tc.open_task(t)
        if not info:
            continue
        done = False
        for g in tc.get_groups(info["encryTaskUserId"]):
            for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
                if int(p.get("planType", -1) or -1) != PLAN_TYPE_HOMEWORK or not g["groupAllowStudy"]:
                    continue
                url = tc.get_study_url(info["encryTaskUserId"], p["encryptPlanId"])
                if not url:
                    continue
                print("=" * 70)
                print("真机提交:", t["name"], "/", p["name"])
                ok = tc.study_homework(url, p, C)
                print("  study_homework ->", ok, "outcome:", tc.last_outcome)
                time.sleep(8)
                for _ in range(3):
                    info2 = tc.open_task(t)
                    cur = None
                    for g2 in tc.get_groups(info2["encryTaskUserId"]):
                        for p2 in tc.get_plans(info2["encryTaskUserId"], g2["encryptGroupId"]):
                            if str(p2.get("planId")) == str(p.get("planId")):
                                cur = p2
                    if cur is None:
                        break
                    print("  复查 isFinish =", tc.plan_finished(cur),
                          "| jobCompleteRate =", cur.get("jobCompleteRate"),
                          "| score =", cur.get("score"))
                    if tc.plan_finished(cur):
                        break
                    time.sleep(10)
                done = True
                break
            if done:
                break
        if done:
            break
    sys.exit(0)


if "--dry-answer" in sys.argv:
    # 真机演练：解析真实作业页 + 题库/AI 出答案，但**不提交**。
    import configparser
    from api.base import Chaoxing
    from api.answer import Tiku
    from api.decode import decode_homework_page

    cfg = configparser.ConfigParser()
    cfg.read(paths.config_path(), encoding="utf-8")
    tiku_cfg = dict(cfg.items("tiku")) if cfg.has_section("tiku") else {}
    tiku = Tiku.get_tiku_from_config(tiku_cfg, config_path=paths.config_path())
    tiku.init_tiku()
    cx = Chaoxing(tiku=tiku, work_max_retries=3)
    tc = TaskCenter(cx, {"speed": 1.0, "tiku_config": tiku_cfg})

    for t in tc.get_course_tasks(C):
        info = tc.open_task(t)
        if not info:
            continue
        done = False
        for g in tc.get_groups(info["encryTaskUserId"]):
            for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
                if int(p.get("planType", -1) or -1) != PLAN_TYPE_HOMEWORK or not g["groupAllowStudy"]:
                    continue
                url = tc.get_study_url(info["encryTaskUserId"], p["encryptPlanId"])
                if not url:
                    continue
                r = tc.session.get(url, allow_redirects=True, timeout=20)
                page = decode_homework_page(r.text or "")
                qs = page.get("questions") or []
                print("=" * 70)
                print(f'演练：{t["name"]} / {p["name"]}  共 {len(qs)} 题（不提交）')
                for q in qs:
                    print(f'  [{q["type"]}] {q["title"][:60]}')
                    if q["options"]:
                        print("      选项:", q["options"].replace("\n", " | ")[:150])
                failure = tc._fill_homework_answers(qs)
                print("  出答案:", failure or "OK")
                for q in qs:
                    print("   →", q["id"], q["type"], "=>", repr(q["answerField"].get(f'answer{q["id"]}'))[:120])
                print("  （dry-run 结束，未提交）")
                done = True
                break
            if done:
                break
        if done:
            break
    sys.exit(0)


if "--api-work" in sys.argv:
    # 只读取证：对第一个解锁的作业试 api/work（章节测验用的取题接口），
    # 看返回的是不是旧结构（带 TiMu），以判断能否复用 study_work。
    from api.decode import decode_questions_info
    for t in tc.get_course_tasks(C):
        info = tc.open_task(t)
        if not info:
            continue
        done = False
        for g in tc.get_groups(info["encryTaskUserId"]):
            for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
                if int(p.get("planType", -1) or -1) != PLAN_TYPE_HOMEWORK or not g["groupAllowStudy"]:
                    continue
                url = tc.get_study_url(info["encryTaskUserId"], p["encryptPlanId"])
                if not url:
                    continue
                r = tc.session.get(url, allow_redirects=True, timeout=20)
                final = getattr(r, "url", url)
                qs = {}
                from urllib.parse import urlparse, parse_qs
                for k, v in parse_qs(urlparse(final).query).items():
                    qs[k] = v[0] if v else ""
                work_id = qs.get("workId") or ""
                enc = qs.get("enc") or ""
                api_url = "https://mooc1.chaoxing.com/mooc-ans/api/work"
                params = {
                    "api": "1", "workId": work_id, "jobid": f"work-{work_id}",
                    "originJobId": f"work-{work_id}", "needRedirect": "true",
                    "skipHeader": "true", "knowledgeid": qs.get("knowledgeid", "0") or "0",
                    "ktoken": "", "cpi": qs.get("cpi", C["cpi"]), "ut": "s",
                    "clazzId": C["clazzId"], "type": "", "enc": enc, "mooc2": "1",
                    "courseid": C["courseId"],
                }
                print("=" * 70)
                print("api/work 试取题:", p["name"], "workId=", work_id, "enc=", enc)
                try:
                    ra = tc.session.get(api_url, params=params, timeout=20)
                    text = ra.text or ""
                    print("  api/work ->", ra.status_code, len(text), getattr(ra, "url", ""))
                    title = re.search(r"<title>(.*?)</title>", text, re.S)
                    print("  title:", title.group(1).strip() if title else "?")
                    with open("/tmp/cx_homework_api_%s.html" % work_id, "w", encoding="utf-8") as f:
                        f.write(text)
                    parsed = decode_questions_info(text)
                    qlist = parsed.get("questions") or []
                    print("  decode_questions_info ->", len(qlist), "题")
                    for q in qlist[:5]:
                        print("    ", q.get("type"), q.get("id"), (q.get("title") or "")[:40])
                    print("  表单字段:", sorted(k for k in parsed.keys() if k not in ("questions",)))
                except Exception as e:
                    print("  api/work 失败:", e)
                done = True
                break
            if done:
                break
        if done:
            break
    sys.exit(0)

if not found:
    print("没有找到 planType=4 的作业任务点")
