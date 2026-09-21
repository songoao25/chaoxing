# -*- coding: utf-8 -*-
"""
探针共用配置：真实账号 / 课程参数只放在本机 tools/probe/local.env（已被 .gitignore 忽略）。

首次使用：
  1. cp tools/probe/local.env.example tools/probe/local.env
  2. 按注释填写自己的手机号与课程参数
  3. 之后所有探针都可以不带参数直接运行

也可以直接用环境变量覆盖：CX_ACCOUNT / CX_COURSE_ID / CX_CLAZZ_ID / CX_CPI / CX_COURSE_TITLE
"""
import os

_LOCAL_ENV = os.path.join(os.path.dirname(os.path.abspath(__file__)), "local.env")


def _load_local_env(path=_LOCAL_ENV):
    values = {}
    if not os.path.exists(path):
        return values
    try:
        with open(path, encoding="utf-8") as fp:
            for line in fp:
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                key, value = line.split("=", 1)
                values[key.strip()] = value.strip()
    except OSError:
        pass
    return values


_LOCAL = _load_local_env()


def get(key, default=""):
    return (os.environ.get(key) or _LOCAL.get(key) or default).strip()


def account():
    value = get("CX_ACCOUNT")
    if not value:
        raise SystemExit(
            "缺少账号：请复制 tools/probe/local.env.example 为 tools/probe/local.env 并填写 "
            "CX_ACCOUNT，或设置同名环境变量（仓库里不保存真实账号）"
        )
    return value


def course(title_default="示例课程"):
    course_id = get("CX_COURSE_ID")
    clazz_id = get("CX_CLAZZ_ID")
    cpi = get("CX_CPI")
    if not (course_id and clazz_id and cpi):
        raise SystemExit(
            "缺少课程参数：请在 tools/probe/local.env 填写 "
            "CX_COURSE_ID / CX_CLAZZ_ID / CX_CPI（或设置同名环境变量）"
        )
    return {
        "title": get("CX_COURSE_TITLE", title_default) or title_default,
        "courseId": course_id,
        "clazzId": clazz_id,
        "cpi": cpi,
    }
