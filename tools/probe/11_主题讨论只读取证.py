# -*- coding: utf-8 -*-
"""主题讨论（planType=14）只读取证：拿学习地址、看页面/接口，不发布任何回复

用法：
  ./.venv/bin/python tools/probe/11_主题讨论只读取证.py [账号] [courseId]
  ./.venv/bin/python tools/probe/11_主题讨论只读取证.py --raw-url   # 额外打印 getToStudyUrl 原始响应
"""
import os, sys, re, json, time

BASE = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
sys.path.insert(0, BASE)
os.chdir(BASE)

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import probe_config  # noqa: E402
from api.base import SessionManager
from api import cookies as cookie_mod, paths
from api.task_center import TaskCenter, PLAN_TYPE_DISCUSS, TASK_BASE

ACCOUNT = sys.argv[1] if len(sys.argv) > 1 and not sys.argv[1].startswith("--") else probe_config.account()
COURSE_ID = sys.argv[2] if len(sys.argv) > 2 and not sys.argv[2].startswith("--") else probe_config.get("CX_COURSE_ID")

s = SessionManager.get_session()
s.cookies.update(cookie_mod.use_cookies(ACCOUNT))
tc = TaskCenter(object(), {})
C = {{**probe_config.course(), "courseId": COURSE_ID or probe_config.get("CX_COURSE_ID")}}

KEYWORDS = ("bbs", "discuss", "reply", "circle", "topic", "postId", "topicId",
            "insertPost", "addPost", "api/", "enc=", "activeList", "viewTopic")

if "--check-finish" in sys.argv:
    # 只检查主题讨论任务点的完成状态（供轮询验证用）
    for t in tc.get_course_tasks(C):
        info = tc.open_task(t)
        if not info:
            continue
        for g in tc.get_groups(info["encryTaskUserId"]):
            for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
                if int(p.get("planType", -1) or -1) == PLAN_TYPE_DISCUSS:
                    print(f'{p["name"]}: isFinish={tc.plan_finished(p)}')
    sys.exit(0)


if "--replace-reply-file" in sys.argv:
    # 把本账号在这个话题下已发的回复替换成文件里的文本（用于修正风格）
    import configparser
    from urllib.parse import urlparse, parse_qs, quote
    from api.task_center import _extract_discussion_topic

    path_arg = sys.argv[sys.argv.index("--replace-reply-file") + 1]
    new_text = open(path_arg, encoding="utf-8").read().strip()
    print("新文本（%d 字）：%s" % (len(new_text), new_text))

    for t in tc.get_course_tasks(C):
        info = tc.open_task(t)
        if not info:
            continue
        done = False
        for g in tc.get_groups(info["encryTaskUserId"]):
            for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
                if int(p.get("planType", -1) or -1) != PLAN_TYPE_DISCUSS or not g["groupAllowStudy"]:
                    continue
                url = tc.get_study_url(info["encryTaskUserId"], p["encryptPlanId"])
                qs = parse_qs(urlparse(url).query)
                bbsid = qs.get("bbsid", [""])[0]
                topic_uuid = qs.get("uuid", [""])[0]
                page = tc.session.get(url, allow_redirects=True, timeout=20).text or ""
                info_page = _extract_discussion_topic(page)
                mine_uuid = ""
                r = tc.session.get(
                    "https://groupweb.chaoxing.com/pc/invitation/getReplyList",
                    params={"bbsid": bbsid, "uuid": topic_uuid, "tag": "", "order": 2,
                            "lastValue": "", "lastAuxValue": ""},
                    timeout=20,
                )
                for d in ((r.json() or {}).get("datas") or []):
                    if str(d.get("createrPuid") or "") == str(info_page.get("user_puid") or ""):
                        mine_uuid = str(d.get("uuid") or "")
                        break
                if not mine_uuid:
                    print("  没找到自己的回复，跳过:", p["name"])
                    done = True
                    break
                resp = tc.session.post(
                    "https://groupweb.chaoxing.com/pc/invitation/%s/updateReply" % topic_uuid,
                    data={"uuid": mine_uuid,
                          "reply_content": quote(new_text, safe="!'()*-._~"),
                          "reply_files_url": "", "reply_files_attr": ""},
                    timeout=20,
                    headers={"X-Requested-With": "XMLHttpRequest",
                             "Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
                             "Origin": "https://groupweb.chaoxing.com", "Referer": url},
                )
                print("  updateReply ->", resp.status_code, str(resp.text or "")[:200])
                time.sleep(3)
                r2 = tc.session.get(
                    "https://groupweb.chaoxing.com/pc/invitation/getReplyList",
                    params={"bbsid": bbsid, "uuid": topic_uuid, "tag": "", "order": 2,
                            "lastValue": "", "lastAuxValue": ""},
                    timeout=20,
                )
                for d in ((r2.json() or {}).get("datas") or []):
                    if str(d.get("uuid")) == mine_uuid:
                        print("  读回：", str(d.get("content"))[:300])
                print("  任务点 isFinish =", tc.plan_finished(p))
                done = True
                break
            if done:
                break
        if done:
            break
    sys.exit(0)


if "--submit-reply" in sys.argv:
    # 真机验收：走生产 study_discussion 提交一条回复，再读回核对
    from api.ai_writer import HumanLikeWriter
    import configparser
    from urllib.parse import urlparse, parse_qs

    cfg = configparser.ConfigParser()
    cfg.read(paths.config_path(), encoding="utf-8")
    tiku_cfg = dict(cfg.items("tiku")) if cfg.has_section("tiku") else {}
    tc2 = TaskCenter(object(), {"task_center_submit_mode": "auto", "tiku_config": tiku_cfg})

    for t in tc2.get_course_tasks(C):
        info = tc2.open_task(t)
        if not info:
            continue
        done = False
        for g in tc2.get_groups(info["encryTaskUserId"]):
            for p in tc2.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
                if int(p.get("planType", -1) or -1) != PLAN_TYPE_DISCUSS or not g["groupAllowStudy"]:
                    continue
                if tc2.plan_finished(p):
                    print("已完成，跳过:", p["name"])
                    done = True
                    break
                url = tc2.get_study_url(info["encryTaskUserId"], p["encryptPlanId"])
                qs = parse_qs(urlparse(url).query)
                print("=" * 74)
                print("真机提交主题讨论:", p["name"])
                ok = tc2.study_discussion(url, p, C)
                print("  study_discussion ->", ok, "outcome:", tc2.last_outcome)
                time.sleep(5)
                r = tc2.session.get(
                    "https://groupweb.chaoxing.com/pc/invitation/getReplyList",
                    params={"bbsid": qs.get("bbsid", [""])[0], "uuid": qs.get("uuid", [""])[0],
                            "tag": "", "order": 2, "lastValue": "", "lastAuxValue": ""},
                    timeout=20,
                )
                try:
                    datas = (r.json() or {}).get("datas") or []
                except Exception:
                    datas = []
                print("  读回：共", len(datas), "条回复")
                for item in datas[:2]:
                    print("    最新:", item.get("creater_name"), "|", item.get("ftime"), "|",
                          str(item.get("content"))[:200])
                info2 = tc2.open_task(t)
                for g2 in tc2.get_groups(info2["encryTaskUserId"]):
                    for p2 in tc2.get_plans(info2["encryTaskUserId"], g2["encryptGroupId"]):
                        if str(p2.get("planId")) == str(p.get("planId")):
                            print("  任务点 isFinish =", tc2.plan_finished(p2))
                done = True
                break
            if done:
                break
        if done:
            break
    sys.exit(0)


if "--dry-reply" in sys.argv:
    # 只读演练：取话题页 + 已有回复 → 生成一条草稿，不提交
    import configparser
    from urllib.parse import urlparse, parse_qs
    from api.ai_writer import HumanLikeWriter

    cfg = configparser.ConfigParser()
    cfg.read(paths.config_path(), encoding="utf-8")
    tiku_cfg = dict(cfg.items("tiku")) if cfg.has_section("tiku") else {}
    writer = HumanLikeWriter(tiku_cfg)
    print("writer.available =", writer.available)

    for t in tc.get_course_tasks(C):
        info = tc.open_task(t)
        if not info:
            continue
        done = False
        for g in tc.get_groups(info["encryTaskUserId"]):
            for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
                if int(p.get("planType", -1) or -1) != PLAN_TYPE_DISCUSS or not g["groupAllowStudy"]:
                    continue
                url = tc.get_study_url(info["encryTaskUserId"], p["encryptPlanId"])
                print("=" * 74)
                print("演练:", p["name"], "\n  study_url =", url)
                qs = parse_qs(urlparse(url).query)
                bbsid = qs.get("bbsid", [""])[0]
                topic_uuid = qs.get("uuid", [""])[0]
                page = tc.session.get(url, allow_redirects=True, timeout=20).text or ""
                idx = page.find("topic:{")
                blob = page[idx:idx + 4000]
                m = re.search(r'"title":"((?:[^"\\]|\\.)*)"', blob)
                title = json.loads('"' + m.group(1) + '"') if m else ""
                m = re.search(r'"content":"((?:[^"\\]|\\.)*)"', blob)
                content = json.loads('"' + m.group(1) + '"') if m else ""
                m = re.search(r"urlToken:'([^']+)'", page) or re.search(r'urlToken:"([^"]+)"', page)
                token = m.group(1) if m else ""
                print("  bbsid =", bbsid, "| topic_uuid =", topic_uuid, "| urlToken =", token)
                print("  标题:", title)
                print("  内容:", content[:200])

                r = tc.session.get(
                    "https://groupweb.chaoxing.com/pc/invitation/getReplyList",
                    params={"bbsid": bbsid, "uuid": topic_uuid, "tag": "", "order": 2,
                            "lastValue": "", "lastAuxValue": ""},
                    timeout=20,
                )
                try:
                    data = r.json()
                except Exception:
                    data = {}
                items = data.get("datas") or []
                print("  getReplyList ->", r.status_code, "status =", data.get("status"),
                      "| 已有回复", len(items), "条")
                for item in items[:3]:
                    print("    keys:", sorted(item.keys()))
                    print("    ", item.get("creater_name"), "=>", str(item.get("content"))[:70])
                refs = [str(i.get("content") or "") for i in items][:8]
                draft = writer.discussion(title + "\n" + content, existing_posts=refs)
                print("  草稿:", draft)
                print("  （dry-reply 未提交）")
                done = True
                break
            if done:
                break
        if done:
            break
    sys.exit(0)


for t in ([] if any(f in sys.argv for f in
                        ("--dry-reply", "--submit-reply", "--check-finish", "--replace-reply-file"))
          else tc.get_course_tasks(C)):
    info = tc.open_task(t)
    if not info:
        continue
    for g in tc.get_groups(info["encryTaskUserId"]):
        for p in tc.get_plans(info["encryTaskUserId"], g["encryptGroupId"]):
            if int(p.get("planType", -1) or -1) != PLAN_TYPE_DISCUSS:
                continue
            print("=" * 74)
            print(f'主题讨论：{t["name"]} / {g["taskGroup"]["name"]} / {p["name"]}')
            print("  可学 =", g["groupAllowStudy"], "| isFinish =", tc.plan_finished(p))
            print("  plan:", json.dumps({k: p.get(k) for k in
                  ("planId", "encryptPlanId", "externalDataId", "planIntroduce",
                   "planBreakthroughSet", "taskClassRelationId", "sort", "groupSort")},
                  ensure_ascii=False)[:600])
            if not g["groupAllowStudy"]:
                print("  （分组未解锁，跳过）")
                continue

            # 原始 getToStudyUrl 响应（只读）
            raw = tc.session.post(
                f"{TASK_BASE}/userStudyPlan/getToStudyUrl",
                params={"encryptPlanId": p.get("encryptPlanId", ""),
                        "encryTaskUserId": info["encryTaskUserId"],
                        "studyJumpType": 0, "isInterface": "false"},
                timeout=20,
            )
            print("  getToStudyUrl ->", raw.status_code, str(raw.text or "")[:400])

            url = tc.get_study_url(info["encryTaskUserId"], p.get("encryptPlanId", ""))
            print("  study_url =", url)
            if not url:
                time.sleep(2)
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
            path = f"/tmp/cx_discuss_{p.get('planId')}.html"
            with open(path, "w", encoding="utf-8") as f:
                f.write(text)
            print("  已保存:", path)
            for kw in KEYWORDS:
                hit = text.find(kw)
                if hit >= 0:
                    ctx = re.sub(r"\s+", " ", text[max(0, hit - 120):hit + 180])
                    print(f"   [{kw}] …{ctx}…")
            time.sleep(2)
