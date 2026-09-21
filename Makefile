# 常用命令：给人和 Agent 共用
PY := ./.venv/bin/python
.PHONY: help test lint compile status classify probes doctor

help:  ## 列出所有命令
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

# 本地默认解释器可能比 CI 新（如 3.14）：注解求值时机不同，
# 提交前用 CI 同版本再跑一遍，避免"本地绿、CI 红"。
test-313:
	@command -v python3.13 >/dev/null 2>&1 || { echo "没有 python3.13，跳过（CI 会跑）"; exit 0; }
	@python3.13 -c "import loguru, openai, bs4, lxml, requests, httpx, tqdm, tenacity" 2>/dev/null \
		|| { echo "python3.13 缺依赖，跳过（CI 会跑）。要本地对齐：python3.13 -m venv /tmp/cx313 && /tmp/cx313/bin/pip install -r requirements.txt"; exit 0; }
	python3.13 -m compileall -q api main.py setup_wizard.py tools tests
	python3.13 -m unittest discover -s tests -t .

test:  ## 跑全量单测（离线）
	$(PY) -m unittest discover -s tests -t .

compile:  ## 语法编译检查
	$(PY) -m compileall -q api main.py tools tests

lint: compile test  ## 提交前必跑：编译 + 单测

status:  ## 当前账号任务中心进度快照（只读）
	$(PY) tools/probe/02_当前任务状态快照.py

classify:  ## 平台结构分类总表（只读，会抽样请求）
	$(PY) tools/probe/01_分类总表.py

probes:  ## 编译检查所有探针脚本
	$(PY) -m py_compile tools/probe/*.py

doctor:  ## 环境自检
	@echo "python : $$($(PY) -V)"
	@echo "venv   : $$(test -x $(PY) && echo ok || echo missing)"
	@echo "data   : $$(test -d $$HOME/.chaoxing && echo $$HOME/.chaoxing || echo 'missing (~/.chaoxing)')"
	@$(PY) -c "import requests, bs4, loguru, tqdm; print('deps   : ok')"
