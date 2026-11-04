/* 超星助手 · 应用主逻辑（数据结构与事件绑定不变，视觉层对齐 ykt 统一设计语言） */
(function () {
  'use strict';

  var UI = window.UI;
  var Pages = window.Pages;

  var GO = null;
  try { GO = window.go && window.go.main && window.go.main.App; } catch (e) { GO = null; }
  var RUNTIME = null;
  try { RUNTIME = window.runtime; } catch (e) { RUNTIME = null; }

  var isDev = !!window.__DEV_PREVIEW__;

  /* ---------- 无 Wails 运行时 ---------- */
  function renderFallback() {
    UI.mountSprite();
    var app = document.getElementById('app');
    app.setAttribute('aria-busy', 'false');
    app.innerHTML =
      '<div class="fallback-wrap"><div class="fallback-card">' +
      '<div class="fallback-icon">' + UI.icon('book') + '</div>' +
      '<div class="fallback-title">请在「超星助手」应用里打开</div>' +
      '<p class="fallback-text">这个界面需要由桌面应用加载。直接双击 HTML 或普通浏览器打开时，读不到本地引擎和数据。</p>' +
      '<div class="fallback-note">开发者要做界面预览，请用 <strong>dev/index-dev.html</strong>，它带有模拟数据。</div>' +
      '</div></div>';
  }

  if (!GO) {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', renderFallback);
    } else {
      renderFallback();
    }
    return;
  }

  /* ---------- 基础工具 ---------- */
  function clone(value) {
    if (value === null || value === undefined) return value;
    return JSON.parse(JSON.stringify(value));
  }

  function num(value, fallback) {
    var n = Number(value);
    return isFinite(n) ? n : fallback;
  }

  function getPath(obj, path) {
    var parts = path.split('.');
    var cur = obj;
    for (var i = 0; i < parts.length; i++) {
      if (cur === null || typeof cur !== 'object') return undefined;
      cur = cur[parts[i]];
    }
    return cur;
  }

  function setPath(obj, path, value) {
    var parts = path.split('.');
    var cur = obj;
    for (var i = 0; i < parts.length - 1; i++) {
      var key = parts[i];
      if (cur[key] === null || typeof cur[key] !== 'object') cur[key] = {};
      cur = cur[key];
    }
    cur[parts[parts.length - 1]] = value;
  }

  function errText(err) {
    if (err === null || err === undefined || err === '') return '未知错误';
    if (typeof err === 'string') return err;
    if (err && err.message) return String(err.message);
    try { return String(err); } catch (e) { return '未知错误'; }
  }

  function friendlyError(message) {
    var m = String(message || '').trim();
    var rules = [
      [/busy|已有刷课任务/, '已经有刷课任务在跑了，先停止它，或者等这一轮结束。'],
      [/login_failed|登录失败|密码错误|账号或密码/, '登录失败：账号或密码不对，或者需要先在网页端完成验证。'],
      [/network_error|网络|timeout|超时|timed out/, '网络请求失败：检查一下网络，然后再试。'],
      [/not_found|不存在/, '没有找到对应的数据，刷新一下再试。'],
      [/config_error|配置/, '配置读写失败：检查数据目录权限，或者重新选择引擎目录。'],
      [/not_ready|尚未就绪|还没准备好/, '引擎还没准备好，等几秒再试一次。'],
      [/engine_error|引擎内部/, '引擎出错了：' + m],
      [/bad_request|参数/, '请求参数不对：' + m]
    ];
    for (var i = 0; i < rules.length; i++) {
      if (rules[i][0].test(m)) return rules[i][1];
    }
    return m || '未知错误';
  }

  function call(method) {
    var args = Array.prototype.slice.call(arguments, 1);
    return Promise.resolve().then(function () {
      var fn = GO[method];
      if (typeof fn !== 'function') throw new Error('缺少接口 ' + method + '，请确认桌面后端已更新');
      return fn.apply(GO, args);
    });
  }

  function normalizePayload(data) {
    if (typeof data === 'string') {
      try { return JSON.parse(data); } catch (e) { return {}; }
    }
    return data || {};
  }

  var SETTINGS_DEFAULTS = {
    speed: 1,
    jobs: 1,
    notopen_action: 'continue',
    retry_interval: 1,
    serial_video: false,
    add_learning_count: false,
    target_count: 100,
    work_redo_enabled: false,
    work_max_retries: 3,
    task_center_submit_mode: 'auto'
  };
  var SETTINGS_KEYS = ['speed', 'jobs', 'notopen_action', 'retry_interval', 'serial_video',
    'add_learning_count', 'target_count', 'work_redo_enabled', 'work_max_retries', 'task_center_submit_mode'];

  function normalizeSettings(raw) {
    var out = {};
    var src = raw || {};
    for (var i = 0; i < SETTINGS_KEYS.length; i++) out[SETTINGS_KEYS[i]] = src[SETTINGS_KEYS[i]];
    if (out.speed === undefined || out.speed === null) out.speed = SETTINGS_DEFAULTS.speed;
    if (out.jobs === undefined || out.jobs === null) out.jobs = SETTINGS_DEFAULTS.jobs;
    if (!out.notopen_action) out.notopen_action = SETTINGS_DEFAULTS.notopen_action;
    if (out.retry_interval === undefined) out.retry_interval = SETTINGS_DEFAULTS.retry_interval;
    if (out.serial_video === undefined) out.serial_video = SETTINGS_DEFAULTS.serial_video;
    if (out.add_learning_count === undefined) out.add_learning_count = SETTINGS_DEFAULTS.add_learning_count;
    if (out.target_count === undefined) out.target_count = SETTINGS_DEFAULTS.target_count;
    if (out.work_redo_enabled === undefined) out.work_redo_enabled = SETTINGS_DEFAULTS.work_redo_enabled;
    if (out.work_max_retries === undefined) out.work_max_retries = SETTINGS_DEFAULTS.work_max_retries;
    out.task_center_submit_mode = 'auto';
    return out;
  }

  function pickSettings(source) {
    var src = source || {};
    var out = {};
    for (var i = 0; i < SETTINGS_KEYS.length; i++) out[SETTINGS_KEYS[i]] = src[SETTINGS_KEYS[i]];
    out.task_center_submit_mode = 'auto';
    return out;
  }

  /* ---------- 状态 ---------- */
  var S = {
    page: 'overview',
    engineReady: false,
    bridge: { state: 'starting', message: '' },
    status: null,
    accounts: [],
    settings: normalizeSettings(null),
    appInfo: null,
    engine: null,
    overviewError: '',
    overviewRecent: [],
    overviewRecentLoading: true,
    overviewRecentError: '',
    run: {
      phase: 'config',
      accountId: '',
      coursesByAccount: {},
      coursesLoading: false,
      coursesError: '',
      search: '',
      selected: {},
      scope: { chapters: true, task_center: true },
      limits: { max_points_per_course: 0, max_tasks_per_course: 0 },
      options: {
        speed: 1, jobs: 1, notopen_action: 'continue', retry_interval: 1,
        serial_video: false, add_learning_count: false, target_count: 100,
        work_redo_enabled: false, work_max_retries: 3
      },
      runId: '',
      startedAt: '',
      localStart: 0,
      stage: '',
      stageText: '',
      logLines: [],
      logFilter: 'all',
      autoScroll: true,
      logCollapsed: false,
      elapsed: 0,
      progress: null,
      finished: null,
      startError: '',
      stopping: false
    },
    review: {
      days: 1,
      loading: false,
      error: '',
      items: [],
      markdownPath: '',
      selectedId: '',
      item: null,
      itemLoading: false,
      itemError: ''
    },
    settingsPage: { draft: null, dirty: false, saving: false }
  };

  function currentCourses() {
    var cached = S.run.coursesByAccount[S.run.accountId];
    return (cached && cached.courses) || null;
  }

  function selectedCount() { return Pages.selectedCount(S); }

  function seedRunOptionsFromSettings() {
    var st = S.settings || SETTINGS_DEFAULTS;
    var o = S.run.options;
    o.speed = num(st.speed, 1);
    o.jobs = num(st.jobs, 1);
    o.notopen_action = st.notopen_action === 'retry' ? 'retry' : 'continue';
    o.retry_interval = num(st.retry_interval, 1);
    o.serial_video = !!st.serial_video;
    o.add_learning_count = !!st.add_learning_count;
    o.target_count = num(st.target_count, 100);
    o.work_redo_enabled = !!st.work_redo_enabled;
    o.work_max_retries = num(st.work_max_retries, 3);
  }

  function applyStatus(status) {
    var st = status || {};
    S.status = st;
    if (st.accounts && st.accounts.length !== undefined) S.accounts = st.accounts;
    if (st.engine) {
      var merged = {};
      var keys = ['repo', 'python', 'data_dir', 'config_exists'];
      for (var i = 0; i < keys.length; i++) merged[keys[i]] = st.engine[keys[i]];
      if (S.engine) {
        for (var k in S.engine) if (Object.prototype.hasOwnProperty.call(S.engine, k)) merged[k] = S.engine[k];
      }
      S.engine = merged;
    }
    if (st.bridge_state) S.bridge.state = st.bridge_state;
    if (typeof st.engine_ready === 'boolean' && st.engine_ready) S.engineReady = true;
    if (st.settings && !S.settingsPage.dirty) S.settings = normalizeSettings(st.settings);
  }

  /* ---------- 外壳 ---------- */
  var mounted = false;

  function shellHTML() {
    var navItems = [
      ['工作台', null],
      ['overview', '概览', 'grid', '⌘1'],
      ['run', '刷课', 'play', '⌘2'],
      ['review', '复核', 'file', '⌘3'],
      ['系统', null],
      ['settings', '设置', 'gear', '⌘4']
    ];
    var nav = navItems.map(function (item) {
      if (item[1] === null) {
        return '<div class="nav-label" role="presentation">' + item[0] + '</div>';
      }
      return '<button class="nav-item" type="button" role="tab" data-action="nav" data-page="' + item[0] + '" aria-selected="false" aria-label="' + item[1] + '">' +
        '<svg class="icon nav-icon" aria-hidden="true"><use href="#i-' + item[2] + '"></use></svg>' +
        '<span class="nav-text">' + item[1] + '</span>' +
        '<span class="nav-kbd" aria-hidden="true">' + item[3] + '</span></button>';
    }).join('');

    return '<div class="app-shell" id="app-shell">' +
      '<header class="toolbar" id="toolbar">' +
      (isDev ? '<div class="mock-lights" aria-hidden="true"><span class="mock-light is-red"></span><span class="mock-light is-yellow"></span><span class="mock-light is-green"></span></div>' : '') +
      '<div class="toolbar-actions">' +
      '<span class="toolbar-sync" id="toolbar-sync">' +
      '<span class="sync-dot" id="sync-dot" aria-hidden="true"></span>' +
      '<span id="toolbar-sync-text">正在连接引擎…</span></span>' +
      '<button class="btn btn-sm" type="button" id="btn-refresh-page" data-action="refresh-page" aria-label="刷新当前页">' +
      '<svg class="icon" aria-hidden="true"><use href="#i-refresh"></use></svg><span>刷新</span></button>' +
      '</div></header>' +
      '<aside class="sidebar" aria-label="应用侧栏"><div class="sidebar-inner">' +
      '<nav class="nav" role="tablist" aria-label="主导航" aria-orientation="vertical">' + nav + '</nav>' +
      '<div class="sidebar-footer">' +
      '<div class="mini-status" id="mini-status" aria-live="polite">' +
      '<span class="mini-dot" id="mini-dot" aria-hidden="true"></span>' +
      '<span class="mini-text" id="mini-text">正在连接…</span></div>' +
      '<div class="sidebar-version mono" id="sidebar-version">v0.1.0</div>' +
      '</div></div></aside>' +
      '<main class="main" id="main"><div class="main-scroll" id="main-scroll">' +
      '<div class="page" id="page"></div></div></main></div>';
  }

  function mountShell() {
    if (mounted) return;
    mounted = true;
    var app = document.getElementById('app');
    app.innerHTML = shellHTML();
    app.setAttribute('aria-busy', 'false');
    bindEvents();
    render();
    updateSidebar();
  }

  function render() {
    if (!mounted) return;
    var pageEl = document.getElementById('page');
    if (!pageEl) return;
    var wide = S.page === 'review' || (S.page === 'run' && S.run.phase !== 'config');
    pageEl.className = 'page' + (wide ? ' page-wide' : '');
    pageEl.innerHTML = Pages[S.page](S);
    updateSidebar();
    if (S.page === 'run') {
      if (S.run.phase !== 'config' && S.run.phase !== 'finished') scrollLogToBottom(true);
    }
  }

  function updateSidebar() {
    var navs = document.querySelectorAll('.nav-item');
    for (var i = 0; i < navs.length; i++) {
      var active = navs[i].getAttribute('data-page') === S.page;
      navs[i].classList.toggle('is-active', active);
      navs[i].setAttribute('aria-selected', active ? 'true' : 'false');
    }
    var state = S.bridge.state;
    var ready = !!S.engineReady;
    var dotCls = ready ? 'is-running' : (state === 'failed' ? 'is-error' : '');
    var dot = document.getElementById('sync-dot');
    if (dot) dot.className = 'sync-dot' + (dotCls ? ' ' + dotCls : '');
    var miniDot = document.getElementById('mini-dot');
    if (miniDot) miniDot.className = 'mini-dot' + (dotCls ? ' ' + dotCls : '');
    var syncText = document.getElementById('toolbar-sync-text');
    var miniText = document.getElementById('mini-text');
    var label = ready ? '引擎已就绪'
      : state === 'starting' ? '引擎启动中'
        : state === 'restarting' ? '引擎重连中'
          : state === 'failed' ? '引擎连接失败'
            : '引擎未就绪';
    if (syncText) syncText.textContent = label;
    if (miniText) miniText.textContent = ready ? '引擎已就绪' : label;
    var versionEl = document.getElementById('sidebar-version');
    if (versionEl && S.appInfo && S.appInfo.version) versionEl.textContent = 'v' + S.appInfo.version;
  }

  function applyPlatformClass() {
    var platform = (S.appInfo && S.appInfo.platform) || '';
    var darwin = /darwin/i.test(platform) ||
      (!isDev && typeof navigator !== 'undefined' && /Mac/i.test(navigator.platform || ''));
    document.documentElement.classList.toggle('mock', isDev);
    document.documentElement.classList.toggle('platform-darwin', !!darwin && !isDev);
  }

  /* ---------- 局部更新 ---------- */
  function patchStartState() {
    var btn = document.getElementById('start-run');
    var hint = Pages.validationHint(S);
    if (btn) btn.disabled = !!hint;
    var note = document.getElementById('scope-note');
    if (note) note.textContent = Pages.scopeNoteHTML(S);
  }

  function patchCourseArea() {
    var list = document.getElementById('course-list');
    if (list) list.innerHTML = Pages.courseListHTML(S);
    var meta = document.getElementById('course-meta-badge');
    if (meta) {
      meta.innerHTML = '<span class="badge-dot" aria-hidden="true"></span>' +
        (S.run.accountId ? (S.run.coursesLoading ? '拉取中' : Pages.courseMetaText(S)) : '未选账号');
    }
    var search = document.getElementById('course-search');
    if (search) search.disabled = !S.run.accountId;
    var selAll = document.querySelector('[data-action="toggle-all-courses"]');
    if (selAll) selAll.disabled = !S.run.accountId || S.run.coursesLoading;
    var clear = document.querySelector('[data-action="clear-courses"]');
    if (clear) clear.disabled = !selectedCount();
    patchStartState();
  }

  function patchSteps() {
    var steps = document.getElementById('run-steps');
    if (steps) steps.innerHTML = Pages.stepsHTML(S);
  }

  function patchProgress() {
    var host = document.getElementById('run-progress');
    if (host) host.innerHTML = Pages.progressHTML(S);
  }

  function patchLiveStatus() {
    var sub = document.getElementById('run-page-sub');
    if (sub) sub.textContent = S.run.phase === 'stopping' ? '正在停止' : (S.run.stageText || '正在启动');
    var timer = document.getElementById('run-timer');
    if (timer) timer.textContent = UI.fmtDuration(S.run.elapsed);
    var pageTimer = document.getElementById('run-page-timer');
    if (pageTimer) pageTimer.textContent = UI.fmtDuration(S.run.elapsed);
    var stop = document.getElementById('stop-run');
    if (stop) stop.disabled = S.run.phase === 'stopping';
  }

  function patchLogFilterButtons() {
    var buttons = document.querySelectorAll('[data-action="log-filter"]');
    for (var i = 0; i < buttons.length; i++) {
      var active = buttons[i].getAttribute('data-level') === S.run.logFilter;
      buttons[i].classList.toggle('is-active', active);
      buttons[i].setAttribute('aria-pressed', active ? 'true' : 'false');
    }
  }

  function patchLogBody() {
    var body = document.getElementById('log-body');
    if (!body) return;
    body.innerHTML = Pages.logBodyHTML(S);
    var count = document.getElementById('log-count');
    if (count) count.textContent = Pages.logCountText(S);
    if (S.run.autoScroll) scrollLogToBottom(true);
  }

  function scrollLogToBottom(force) {
    var body = document.getElementById('log-body');
    if (!body) return;
    var nearBottom = body.scrollHeight - body.scrollTop - body.clientHeight < 48;
    if (force || (S.run.autoScroll && nearBottom)) body.scrollTop = body.scrollHeight;
  }

  function appendLogLine(line) {
    if (!line) return;
    S.run.logLines.push(line);
    if (S.run.logLines.length > 2000) S.run.logLines.splice(0, S.run.logLines.length - 2000);
    if (S.page !== 'run') return;
    var visible = S.run.logFilter === 'all' || S.run.logFilter === line.level;
    var body = document.getElementById('log-body');
    if (!body) return;
    if (body.querySelector('.empty-state')) body.innerHTML = '';
    if (visible) body.insertAdjacentHTML('beforeend', Pages.logLineHTML(line));
    while (body.children.length > 2000) body.removeChild(body.firstChild);
    var count = document.getElementById('log-count');
    if (count) count.textContent = Pages.logCountText(S);
    scrollLogToBottom(false);
  }

  /* ---------- 导航 ---------- */
  function navigate(page) {
    if (!page) return;
    S.page = page;
    render();
    var scroller = document.getElementById('main-scroll');
    if (scroller) scroller.scrollTop = 0;
    if (page === 'review') loadReviews({});
    if (page === 'run' && (S.run.phase === 'running' || S.run.phase === 'stopping')) startTimer();
    if (page === 'overview') refreshStatusQuiet();
  }

  function refreshPage() {
    if (S.page === 'overview') refreshOverview();
    else if (S.page === 'run') {
      if (S.run.phase === 'config') loadCourses(S.run.accountId);
      else refreshStatusQuiet();
    } else if (S.page === 'review') loadReviews({});
    else if (S.page === 'settings') refreshEngine(false);
  }

  /* ---------- 数据加载 ---------- */
  async function loadData() {
    try {
      applyStatus(await call('GetStatus'));
    } catch (e) {
      S.overviewError = friendlyError(errText(e));
    }
    try {
      var accounts = await call('ListAccounts');
      if (accounts && accounts.length !== undefined) S.accounts = accounts;
    } catch (e) {
      if (!S.accounts.length) S.overviewError = S.overviewError || friendlyError(errText(e));
    }
    try {
      S.settings = normalizeSettings(await call('GetSettings'));
      seedRunOptionsFromSettings();
    } catch (e) { /* 设置读不到时用默认值 */ }
    try {
      var engine = await call('GetEngineStatus');
      if (engine && typeof engine === 'object') S.engine = Object.assign({}, S.engine || {}, engine);
      if (engine && typeof engine.ready === 'boolean') S.engineReady = engine.ready;
    } catch (e) { /* 老版本没有这个接口时忽略 */ }
    try { S.appInfo = await call('GetAppInfo'); } catch (e) { S.appInfo = null; }
    if (S.status && S.status.engine_ready) S.engineReady = true;
    try {
      var runState = await call('GetRunState');
      if (runState && runState.active) {
        S.run.phase = 'running';
        S.run.runId = runState.run_id || '';
        S.run.startedAt = runState.started_at || '';
        S.run.localStart = Date.now();
        S.run.stageText = '运行中';
      }
    } catch (e) { /* 没有活动任务或接口不可用 */ }
    applyPlatformClass();
  }

  async function refreshStatusQuiet() {
    try {
      applyStatus(await call('GetStatus'));
      updateSidebar();
      if (S.page === 'overview' || S.page === 'settings') render();
    } catch (e) { /* 静默 */ }
  }

  async function refreshOverview() {
    S.overviewError = '';
    S.overviewRecentLoading = true;
    S.overviewRecentError = '';
    if (S.page === 'overview') render();
    try {
      applyStatus(await call('GetStatus'));
    } catch (e) {
      S.overviewError = friendlyError(errText(e));
    }
    await loadOverviewRecentData();
    S.overviewRecentLoading = false;
    if (S.page === 'overview') render();
    updateSidebar();
  }

  async function loadOverviewRecentData() {
    try {
      var res = await call('ListReviews', 3);
      S.overviewRecent = (res && res.items ? res.items : []).slice(0, 5);
      S.overviewRecentError = '';
    } catch (e) {
      S.overviewRecentError = friendlyError(errText(e));
    }
  }

  async function loadOverviewRecent() {
    await loadOverviewRecentData();
    S.overviewRecentLoading = false;
    if (S.page === 'overview') render();
  }

  async function loadAccounts() {
    try {
      S.accounts = await call('ListAccounts');
    } catch (e) {
      UI.toast('读取账号失败：' + friendlyError(errText(e)), 'error');
    }
    try {
      applyStatus(await call('GetStatus'));
    } catch (e) { /* 静默 */ }
    if (mounted) render();
    updateSidebar();
  }

  /* ---------- 刷课：课程 ---------- */
  async function loadCourses(accountId) {
    if (!accountId) return;
    S.run.coursesLoading = true;
    S.run.coursesError = '';
    if (S.page === 'run') patchCourseArea();
    try {
      var res = await call('ListCourses', accountId);
      var courses = (res && res.courses) || [];
      S.run.coursesByAccount[accountId] = { courses: courses, realname: (res && res.realname) || '' };
      if (S.run.accountId === accountId) {
        var valid = {};
        for (var i = 0; i < courses.length; i++) {
          if (S.run.selected[courses[i].courseId]) valid[courses[i].courseId] = true;
        }
        S.run.selected = valid;
      }
    } catch (e) {
      S.run.coursesError = friendlyError(errText(e));
      delete S.run.coursesByAccount[accountId];
    } finally {
      S.run.coursesLoading = false;
      if (S.page === 'run') patchCourseArea();
    }
  }

  function toggleCourse(id) {
    if (!id) return;
    if (S.run.selected[id]) delete S.run.selected[id];
    else S.run.selected[id] = true;
    var row = document.querySelector('.row[data-action="toggle-course"][data-id="' + (window.CSS && CSS.escape ? CSS.escape(id) : id) + '"]');
    if (row) {
      var checked = !!S.run.selected[id];
      row.classList.toggle('is-selected', checked);
      row.setAttribute('aria-checked', checked ? 'true' : 'false');
    }
    var meta = document.getElementById('course-meta-badge');
    if (meta) {
      meta.innerHTML = '<span class="badge-dot" aria-hidden="true"></span>' + Pages.courseMetaText(S);
    }
    patchStartState();
    var clear = document.querySelector('[data-action="clear-courses"]');
    if (clear) clear.disabled = !selectedCount();
  }

  function replaceCourseList() {
    var list = document.getElementById('course-list');
    if (list) list.innerHTML = Pages.courseListHTML(S);
    var meta = document.getElementById('course-meta-badge');
    if (meta) {
      meta.innerHTML = '<span class="badge-dot" aria-hidden="true"></span>' +
        (S.run.accountId ? Pages.courseMetaText(S) : '未选账号');
    }
    var clear = document.querySelector('[data-action="clear-courses"]');
    if (clear) clear.disabled = !selectedCount();
    patchStartState();
  }

  function buildRunOptions() {
    var o = S.run.options;
    return {
      account_id: S.run.accountId,
      course_ids: Object.keys(S.run.selected).filter(function (k) { return S.run.selected[k]; }),
      scope: {
        chapters: !!S.run.scope.chapters,
        task_center: !!S.run.scope.task_center,
        only_discussion: false
      },
      limits: {
        max_points_per_course: Math.max(0, Math.floor(num(S.run.limits.max_points_per_course, 0))),
        max_tasks_per_course: Math.max(0, Math.floor(num(S.run.limits.max_tasks_per_course, 0)))
      },
      options: {
        speed: num(o.speed, 1),
        jobs: Math.round(num(o.jobs, 1)),
        notopen_action: o.notopen_action === 'retry' ? 'retry' : 'continue',
        retry_interval: num(o.retry_interval, 1),
        serial_video: !!o.serial_video,
        add_learning_count: !!o.add_learning_count,
        target_count: Math.round(num(o.target_count, 100)),
        work_redo_enabled: !!o.work_redo_enabled,
        work_max_retries: Math.round(num(o.work_max_retries, 3))
      }
    };
  }

  /* ---------- 刷课：开始 / 停止 / 计时 ---------- */
  var timerHandle = null;

  function startTimer() {
    stopTimer();
    timerHandle = setInterval(tickTimer, 1000);
    tickTimer();
  }

  function stopTimer() {
    if (timerHandle) {
      clearInterval(timerHandle);
      timerHandle = null;
    }
  }

  function tickTimer() {
    if (S.run.phase !== 'running' && S.run.phase !== 'stopping') return;
    var started = S.run.startedAt ? UI.parseDateTime(S.run.startedAt) : null;
    var base = started ? started.getTime() : S.run.localStart;
    if (base) S.run.elapsed = Math.max(0, Math.round((Date.now() - base) / 1000));
    if (S.page === 'run') patchLiveStatus();
  }

  function confirmStartRun() {
    var accounts = S.accounts || [];
    var account = null;
    for (var i = 0; i < accounts.length; i++) if (accounts[i].id === S.run.accountId) account = accounts[i];
    var o = S.run.options;
    var limit = Number(S.run.limits.max_points_per_course) > 0
      ? '每门课 ' + S.run.limits.max_points_per_course + ' 个任务点'
      : '全部任务点';
    var scopeParts = [];
    if (S.run.scope.chapters) scopeParts.push('章节');
    if (S.run.scope.task_center) scopeParts.push('任务中心');
    var rows = [
      ['账号', account ? (account.name || account.username_masked) : ''],
      ['课程', selectedCount() + ' 门'],
      ['范围', scopeParts.join(' + ') || '未选择'],
      ['参数', Number(o.speed).toFixed(1) + '× · 并发 ' + o.jobs],
      ['上限', limit]
    ];
    var body = '<div class="confirm-list">' + rows.map(function (r) {
      return '<div class="confirm-item"><span>' + r[0] + '</span><span class="v">' + UI.esc(r[1]) + '</span></div>';
    }).join('') + '</div>' +
      '<p class="confirm-note">按平台真实时长播放，运行中不要关闭电脑。</p>';
    return UI.confirm({
      title: '开始刷课',
      message: '确认后立即开始，运行中可以随时停止。',
      body: body,
      confirmLabel: '开始',
      cancelLabel: '取消'
    });
  }

  async function startRun() {
    var hint = Pages.validationHint(S);
    if (hint) {
      UI.toast(hint, 'warning');
      return;
    }
    var confirmed = await confirmStartRun();
    if (!confirmed) return;
    S.run.startError = '';
    S.run.finished = null;
    S.run.progress = null;
    S.run.logLines = [];
    S.run.logFilter = 'all';
    S.run.logCollapsed = false;
    S.run.stage = 'login';
    S.run.stageText = '正在启动';
    S.run.elapsed = 0;
    S.run.localStart = Date.now();
    S.run.phase = 'starting';
    render();
    try {
      var res = await call('StartRun', buildRunOptions());
      S.run.runId = (res && res.run_id) || '';
      S.run.startedAt = (res && res.started_at) || '';
      S.run.localStart = Date.now();
      S.run.phase = 'running';
      S.run.stage = 'login';
      S.run.stageText = '正在登录';
      if (S.page === 'run') render();
      startTimer();
    } catch (e) {
      S.run.phase = 'config';
      if (S.page === 'run') render();
      UI.toast('没能开始刷课：' + friendlyError(errText(e)), 'error');
    }
  }

  async function stopRun() {
    var ok = await UI.confirm({
      title: '停止本次刷课？',
      message: '已完成的进度会保留，未开始的任务不再继续。',
      confirmLabel: '停止',
      cancelLabel: '取消',
      danger: true
    });
    if (!ok) return;
    S.run.phase = 'stopping';
    if (S.page === 'run') render();
    try {
      await call('StopRun');
    } catch (e) {
      UI.toast('停止请求失败：' + friendlyError(errText(e)), 'error');
      refreshStatusQuiet();
    }
  }

  function runAgain() {
    stopTimer();
    S.run.phase = 'config';
    S.run.finished = null;
    S.run.runId = '';
    S.run.startedAt = '';
    S.run.stage = '';
    S.run.stageText = '';
    S.run.logLines = [];
    S.run.progress = null;
    S.run.elapsed = 0;
    S.run.stopping = false;
    render();
    var scroller = document.getElementById('main-scroll');
    if (scroller) scroller.scrollTop = 0;
  }

  /* ---------- 账号切换 ---------- */
  function selectAccount(id) {
    if (!id || S.run.accountId === id) return;
    S.run.accountId = id;
    S.run.selected = {};
    S.run.search = '';
    S.run.coursesError = '';
    S.run.phase = 'config';
    render();
    if (!S.run.coursesByAccount[id]) loadCourses(id);
  }

  /* ---------- 复核 ---------- */
  function patchReviewPane() {
    var detail = document.getElementById('review-detail');
    if (detail) detail.innerHTML = Pages.reviewDetailHTML(S);
    var rows = document.querySelectorAll('#review-items .row');
    for (var i = 0; i < rows.length; i++) {
      var active = rows[i].getAttribute('data-id') === S.review.selectedId;
      rows[i].classList.toggle('is-selected', active);
      rows[i].setAttribute('aria-pressed', active ? 'true' : 'false');
    }
  }

  async function loadReviews(options) {
    options = options || {};
    S.review.error = '';
    if (!S.review.items.length) S.review.loading = true;
    if (S.page === 'review') render();
    try {
      var res = await call('ListReviews', S.review.days);
      S.review.items = (res && res.items) || [];
      S.review.markdownPath = (res && res.markdown_path) || '';
      var exists = false;
      for (var i = 0; i < S.review.items.length; i++) {
        if (S.review.items[i].submission_id === S.review.selectedId) { exists = true; break; }
      }
      if (!exists) {
        S.review.selectedId = '';
        S.review.item = null;
        S.review.itemError = '';
      }
    } catch (e) {
      S.review.error = friendlyError(errText(e));
      S.review.items = [];
    } finally {
      S.review.loading = false;
    }
    if (S.page === 'review') render();
    if (!S.review.selectedId && S.review.items.length) {
      S.review.selectedId = S.review.items[0].submission_id;
      patchReviewPane();
      await loadReviewItem(S.review.selectedId, true);
    }
  }

  async function loadReviewItem(id, quiet) {
    if (!id) return;
    S.review.itemLoading = true;
    S.review.itemError = '';
    if (!quiet) patchReviewPane();
    try {
      var res = await call('GetReview', id);
      S.review.item = (res && res.item) ? res.item : res;
    } catch (e) {
      S.review.itemError = friendlyError(errText(e));
      S.review.item = null;
    } finally {
      S.review.itemLoading = false;
      if (S.review.selectedId === id) patchReviewPane();
    }
  }

  async function openReviewFromAnywhere(id) {
    S.review.selectedId = id;
    S.review.item = null;
    S.review.itemLoading = true;
    S.review.itemError = '';
    if (S.page !== 'review') {
      S.page = 'review';
      if (S.review.days === 1) S.review.days = 3;
    }
    render();
    await Promise.all([loadReviewItem(id, true), loadReviews({})]);
    patchReviewPane();
  }

  /* ---------- 设置 ---------- */
  function ensureSettingsDraft() {
    if (!S.settingsPage.draft) S.settingsPage.draft = clone(S.settings || SETTINGS_DEFAULTS);
    return S.settingsPage.draft;
  }

  function settingsDirty() {
    if (!S.settingsPage.draft) return false;
    var a = pickSettings(S.settingsPage.draft);
    var b = pickSettings(S.settings);
    return JSON.stringify(a) !== JSON.stringify(b);
  }

  function patchSettingsHead() {
    var dirty = S.settingsPage.dirty;
    var chip = document.getElementById('settings-dirty');
    if (chip) chip.hidden = !dirty;
    var revert = document.getElementById('revert-settings');
    if (revert) revert.hidden = !dirty;
    var save = document.getElementById('save-settings');
    if (save) {
      save.disabled = !(dirty && !S.settingsPage.saving);
      save.textContent = S.settingsPage.saving ? '保存中…' : '保存设置';
    }
  }

  function markSettingsDirty() {
    S.settingsPage.dirty = settingsDirty();
    patchSettingsHead();
  }

  function patchWorkRetries() {
    var row = document.getElementById('row-work-retries');
    var input = document.getElementById('set-work-retries');
    var enabled = !!ensureSettingsDraft().work_redo_enabled;
    if (row) {
      row.classList.toggle('is-disabled', !enabled);
      var desc = row.querySelector('.row-desc');
      if (desc) desc.textContent = enabled ? '答错后最多重新作答几次' : '打开「测验答错重做」后才能设置';
    }
    if (input) input.disabled = !enabled;
  }

  function patchTargetCount() {
    var row = document.getElementById('row-target-count');
    var input = document.getElementById('set-target');
    var enabled = !!ensureSettingsDraft().add_learning_count;
    if (row) row.classList.toggle('is-disabled', !enabled);
    if (input) input.disabled = !enabled;
    var runRow = document.getElementById('row-run-target');
    var runInput = document.getElementById('run-target-count');
    if (runRow) runRow.classList.toggle('is-disabled', !S.run.options.add_learning_count);
    if (runInput) runInput.disabled = !S.run.options.add_learning_count;
  }

  async function saveSettings() {
    if (S.settingsPage.saving) return;
    var draft = ensureSettingsDraft();
    S.settingsPage.saving = true;
    patchSettingsHead();
    try {
      var saved = await call('SaveSettings', pickSettings(draft));
      S.settings = normalizeSettings(saved);
      seedRunOptionsFromSettings();
      S.settingsPage.draft = null;
      S.settingsPage.dirty = false;
      S.settingsPage.saving = false;
      if (S.page === 'settings') render();
      UI.toast('设置已保存');
      refreshStatusQuiet();
    } catch (e) {
      S.settingsPage.saving = false;
      patchSettingsHead();
      UI.toast('保存失败：' + friendlyError(errText(e)), 'error');
    }
  }

  function revertSettings() {
    S.settingsPage.draft = null;
    S.settingsPage.dirty = false;
    S.settingsPage.saving = false;
    render();
  }

  /* ---------- 引擎 ---------- */
  async function refreshEngine(showToast) {
    try {
      var engine = await call('GetEngineStatus');
      if (engine && typeof engine === 'object') S.engine = Object.assign({}, S.engine || {}, engine);
      if (engine && typeof engine.ready === 'boolean') S.engineReady = engine.ready;
    } catch (e) {
      if (showToast) UI.toast('检测失败：' + friendlyError(errText(e)), 'error');
    }
    try {
      applyStatus(await call('GetStatus'));
    } catch (e) { /* 静默 */ }
    if (mounted) render();
    updateSidebar();
    if (showToast) UI.toast(S.engineReady ? '引擎已就绪' : '引擎还没就绪，检查仓库目录和 Python 环境', S.engineReady ? 'success' : 'warning');
  }

  async function chooseEngine() {
    try {
      var folder = await call('ChooseEngineFolder');
      if (!folder) return;
      try { await call('SetEnginePath', folder); } catch (e) { /* 有的实现由 ChooseEngineFolder 直接保存 */ }
      await refreshEngine(false);
      UI.toast(S.engineReady ? '引擎目录已更新' : '已选择目录，但引擎还没就绪，检查 Python 环境', S.engineReady ? 'success' : 'warning');
    } catch (e) {
      UI.toast('选择目录失败：' + friendlyError(errText(e)), 'error');
    }
  }

  function openDataDir(doneText) {
    call('OpenDataDir').then(function () {
      UI.toast(doneText || '已打开数据目录');
    }).catch(function (e) {
      UI.toast('打开失败：' + friendlyError(errText(e)), 'error');
    });
  }

  function openConfigFile() {
    call('OpenConfigFile').then(function () {
      UI.toast('已打开配置文件');
    }).catch(function (e) {
      UI.toast('打开失败：' + friendlyError(errText(e)), 'error');
    });
  }

  /* ---------- 复制 ---------- */
  function copyLogs() {
    var lines = Pages.logFilteredLines(S);
    if (!lines.length) {
      UI.toast('没有可复制的日志', 'warning');
      return;
    }
    var text = lines.map(function (line) {
      return '[' + (line.ts || '') + '] [' + (line.level || 'info') + '] ' + (line.text || '');
    }).join('\n');
    UI.copyText(text).then(function (ok) {
      UI.toast(ok ? '日志已复制' : '复制失败，可以手动选中复制', ok ? 'success' : 'error');
    });
  }

  function copyReviewText() {
    var item = S.review.item;
    if (!item || !item.text) {
      UI.toast('这条还没有全文可复制', 'warning');
      return;
    }
    UI.copyText(item.text).then(function (ok) {
      UI.toast(ok ? '全文已复制' : '复制失败，可以手动选中复制', ok ? 'success' : 'error');
    });
  }

  /* ---------- 事件订阅 ---------- */
  function subscribeEvents() {
    if (!RUNTIME || typeof RUNTIME.EventsOn !== 'function') return;
    var on = function (name, handler) {
      try {
        RUNTIME.EventsOn(name, function (payload) {
          try { handler(normalizePayload(payload)); } catch (e) { /* 不让单个事件打断界面 */ }
        });
      } catch (e) { /* 运行时事件不可用时忽略 */ }
    };

    on('bridge:state', function (data) {
      var prev = S.bridge.state;
      S.bridge = { state: data.state || 'starting', message: data.message || '' };
      if (data.state === 'ready') {
        if (prev === 'failed' || prev === 'restarting') UI.toast('引擎已重新连接');
        refreshEngine(false);
      } else if (data.state === 'failed') {
        UI.toast('引擎连接失败，正在重试…', 'error');
        updateSidebar();
      } else {
        updateSidebar();
      }
      if (S.page === 'overview') render();
    });

    on('run:started', function (data) {
      S.run.phase = 'running';
      S.run.runId = data.run_id || S.run.runId;
      if (data.started_at) S.run.startedAt = data.started_at;
      S.run.localStart = S.run.localStart || Date.now();
      S.run.stage = 'login';
      S.run.stageText = '已开始';
      S.run.finished = null;
      if (S.page === 'run') render();
      startTimer();
    });

    on('run:log', function (data) {
      appendLogLine({
        level: data.level || 'info',
        text: data.text || '',
        ts: data.ts || ''
      });
    });

    on('run:stage', function (data) {
      S.run.stage = data.stage || S.run.stage;
      S.run.stageText = data.text || S.run.stageText;
      if (S.page === 'run' && S.run.phase !== 'finished' && S.run.phase !== 'config') {
        S.run.phase = S.run.phase === 'starting' ? 'running' : S.run.phase;
        patchLiveStatus();
        patchSteps();
      }
    });

    on('run:progress', function (data) {
      S.run.progress = data || null;
      if (S.page === 'run') patchProgress();
    });

    on('run:stopping', function () {
      S.run.phase = 'stopping';
      if (S.page === 'run') render();
    });

    on('run:finished', function (data) {
      S.run.phase = 'finished';
      S.run.finished = data || {};
      S.run.stopping = false;
      S.run.progress = null;
      stopTimer();
      if (data && data.duration_s != null) S.run.elapsed = data.duration_s;
      if (S.page === 'run') render();
      var ok = data && data.ok;
      var reason = data && data.reason;
      if (ok && reason !== 'stopped') UI.toast('刷课完成');
      else if (reason === 'stopped') UI.toast('已停止本次刷课', 'warning');
      else UI.toast((data && data.message) || '刷课中断了，日志里有详细原因', 'error');
      refreshStatusQuiet();
      loadOverviewRecentData().then(function () {
        if (S.page === 'overview') render();
      });
    });
  }

  /* ---------- 模型绑定 ---------- */
  function handleModelValue(el) {
    var model = el.getAttribute('data-model');
    if (!model) return;
    if (model === 'run.search') {
      S.run.search = el.value;
      var list = document.getElementById('course-list');
      if (list) list.innerHTML = Pages.courseListHTML(S);
      return;
    }
    var value = el.type === 'number' ? num(el.value, 0) : el.value;
    if (model.indexOf('run.') === 0) {
      setPath(S, model, value);
      return;
    }
    if (model.indexOf('settingsPage.draft.') === 0) {
      ensureSettingsDraft();
      setPath(S, model, value);
      markSettingsDirty();
    }
  }

  /* ---------- 账户弹层 ---------- */
  function openAddAccountModal() {
    UI.openModal({
      title: '添加账号',
      subtitle: '账号只保存在本机，不会上传。',
      tone: 'info',
      wide: true,
      body: '<form class="form" id="add-account-form" novalidate>' +
        '<label class="form-field"><span class="form-field-label">手机号 / 账号</span>' +
        '<input class="input" name="username" autocomplete="username" placeholder="学习通登录手机号" required></label>' +
        '<label class="form-field"><span class="form-field-label">密码</span>' +
        '<span class="password-row"><input class="input" id="add-account-password" type="password" name="password" autocomplete="current-password" placeholder="学习通登录密码" required>' +
        '<button type="button" class="password-toggle" data-action="toggle-password" aria-label="显示密码">' + UI.icon('eye') + '</button></span></label>' +
        '<label class="form-field"><span class="form-field-label">昵称 <span class="optional">可选</span></span>' +
        '<input class="input" name="name" placeholder="方便区分多个账号"></label>' +
        '<div class="form-error" id="add-account-error" role="alert"></div>' +
        '<p class="form-note">首次登录可能需要先在网页端完成验证码或安全验证。</p>' +
        '</form>',
      footer: '<button type="button" class="btn" data-role="cancel">取消</button>' +
        '<button type="button" class="btn btn-primary" data-role="submit">添加</button>',
      onMount: function (m) {
        var form = m.el.querySelector('#add-account-form');
        var errorEl = m.el.querySelector('#add-account-error');
        var submitBtn = m.el.querySelector('[data-role="submit"]');
        var cancelBtn = m.el.querySelector('[data-role="cancel"]');
        var usernameInput = form.elements.username;
        setTimeout(function () { try { usernameInput.focus(); } catch (e) {} }, 40);
        form.addEventListener('submit', function (e) { e.preventDefault(); submit(); });
        cancelBtn.addEventListener('click', function () { m.close(); });
        submitBtn.addEventListener('click', submit);

        function showError(text) { errorEl.textContent = text; }

        async function submit() {
          if (submitBtn.disabled) return;
          var username = String(form.elements.username.value || '').trim();
          var password = String(form.elements.password.value || '');
          var name = String(form.elements.name.value || '').trim();
          errorEl.textContent = '';
          if (!username || !password) { showError('手机号和密码都要填'); return; }
          submitBtn.disabled = true;
          submitBtn.textContent = '添加中…';
          m.locked = true;
          try {
            await call('AddAccount', username, password, name);
            m.close();
            UI.toast('账号已添加');
            await loadAccounts();
            if (S.page === 'run' && !S.run.accountId) {
              var newest = S.accounts[S.accounts.length - 1];
              if (newest) {
                S.run.accountId = newest.id;
                render();
                loadCourses(newest.id);
              }
            }
          } catch (e) {
            submitBtn.disabled = false;
            submitBtn.textContent = '添加';
            m.locked = false;
            showError('添加失败：' + friendlyError(errText(e)));
            try { usernameInput.focus(); } catch (err) {}
          }
        }
      }
    });
  }

  async function removeAccount(id, label) {
    var ok = await UI.confirm({
      title: '删除账号？',
      message: '删除 ' + (label || '这个账号') + ' 后，本地保存的密码也会一起删掉，不影响学习通上的数据。',
      confirmLabel: '删除',
      cancelLabel: '取消',
      danger: true
    });
    if (!ok) return;
    try {
      await call('RemoveAccount', id);
      UI.toast('账号已删除');
      if (S.run.accountId === id) {
        S.run.accountId = '';
        S.run.selected = {};
        S.run.search = '';
      }
      await loadAccounts();
    } catch (e) {
      UI.toast('删除失败：' + friendlyError(errText(e)), 'error');
    }
  }

  /* ---------- 操作表 ---------- */
  var actions = {
    nav: function (el) { navigate(el.getAttribute('data-page')); },
    'goto-run': function () { navigate('run'); },
    'goto-review': function () { if (S.page !== 'review') S.review.days = S.review.days || 1; navigate('review'); },
    'goto-review-today': function () { S.review.days = 1; S.review.selectedId = ''; S.review.item = null; navigate('review'); },
    'goto-settings': function () { navigate('settings'); },
    'refresh-page': function () { refreshPage(); },

    'refresh-overview': function () { refreshOverview(); },
    'choose-engine': function () { chooseEngine(); },
    'retry-engine': function () { refreshEngine(true); },
    'open-data-dir': function () { openDataDir(); },
    'open-config': function () { openConfigFile(); },
    'open-reviews-dir': function () { openDataDir('已打开记录文件夹'); },
    'add-account': function () { openAddAccountModal(); },
    'toggle-password': function (el) {
      var input = document.getElementById('add-account-password');
      if (!input) return;
      var show = input.type === 'password';
      input.type = show ? 'text' : 'password';
      el.innerHTML = UI.icon(show ? 'eye-off' : 'eye');
      el.setAttribute('aria-label', show ? '隐藏密码' : '显示密码');
    },
    'remove-account': function (el) {
      removeAccount(el.getAttribute('data-id'), el.getAttribute('data-label'));
    },

    'select-account': function (el) { selectAccount(el.getAttribute('data-id')); },
    'toggle-course': function (el) { toggleCourse(el.getAttribute('data-id')); },
    'toggle-all-courses': function () {
      var visible = Pages.filteredCourses(S);
      if (!visible.length) return;
      var allSelected = visible.every(function (c) { return !!S.run.selected[c.courseId]; });
      visible.forEach(function (c) {
        if (allSelected) delete S.run.selected[c.courseId];
        else S.run.selected[c.courseId] = true;
      });
      replaceCourseList();
    },
    'clear-courses': function () {
      S.run.selected = {};
      replaceCourseList();
    },
    'clear-course-search': function () {
      S.run.search = '';
      var input = document.getElementById('course-search');
      if (input) input.value = '';
      replaceCourseList();
    },
    'refresh-courses': function () { loadCourses(S.run.accountId); },
    'toggle-switch': function (el) {
      var model = el.getAttribute('data-model');
      if (!model) return;
      var next = el.getAttribute('aria-checked') !== 'true';
      el.setAttribute('aria-checked', next ? 'true' : 'false');
      if (model.indexOf('settingsPage.draft.') === 0) {
        ensureSettingsDraft();
        setPath(S, model, next);
        if (model === 'settingsPage.draft.work_redo_enabled') patchWorkRetries();
        if (model === 'settingsPage.draft.add_learning_count') patchTargetCount();
        markSettingsDirty();
      } else if (model.indexOf('run.') === 0) {
        setPath(S, model, next);
        if (model === 'run.autoScroll') {
          if (next) scrollLogToBottom(true);
        } else if (model.indexOf('run.scope.') === 0) {
          patchStartState();
        } else if (model === 'run.options.work_redo_enabled') {
          patchStartState();
          var wr = document.getElementById('work-retries');
          if (wr) wr.disabled = !next;
        } else if (model === 'run.options.add_learning_count') {
          var tr = document.getElementById('run-target-count');
          if (tr) tr.disabled = !next;
          var trRow = document.getElementById('row-run-target');
          if (trRow) trRow.classList.toggle('is-disabled', !next);
        }
      }
    },

    'log-filter': function (el) {
      S.run.logFilter = el.getAttribute('data-level') || 'all';
      patchLogFilterButtons();
      patchLogBody();
    },
    'toggle-log': function () {
      S.run.logCollapsed = !S.run.logCollapsed;
      var card = document.getElementById('log-card');
      if (card) card.classList.toggle('is-collapsed', S.run.logCollapsed);
      var btn = document.querySelector('[data-action="toggle-log"]');
      if (btn) btn.textContent = S.run.logCollapsed ? '展开' : '收起';
    },
    'copy-logs': function () { copyLogs(); },
    'start-run': function () { startRun(); },
    'stop-run': function () { stopRun(); },
    'run-again': function () { runAgain(); },

    'review-days': function (el) {
      var days = Number(el.getAttribute('data-days'));
      if (!isFinite(days)) days = 1;
      S.review.days = days;
      S.review.selectedId = '';
      S.review.item = null;
      S.review.itemError = '';
      loadReviews({});
    },
    'refresh-reviews': function () { loadReviews({}); },
    'open-review': function (el) {
      var id = el.getAttribute('data-id');
      if (!id) return;
      if (S.page !== 'review') { openReviewFromAnywhere(id); return; }
      S.review.selectedId = id;
      S.review.item = null;
      S.review.itemError = '';
      patchReviewPane();
      loadReviewItem(id);
    },
    'retry-review-item': function () { loadReviewItem(S.review.selectedId); },
    'copy-review-text': function () { copyReviewText(); },

    'save-settings': function () { saveSettings(); },
    'revert-settings': function () { revertSettings(); }
  };

  /* ---------- 事件绑定 ---------- */
  var bound = false;

  function bindEvents() {
    if (bound) return;
    bound = true;
    document.addEventListener('click', function (e) {
      var el = e.target && e.target.closest ? e.target.closest('[data-action]') : null;
      if (!el || el.disabled) return;
      var name = el.getAttribute('data-action');
      var fn = actions[name];
      if (!fn) return;
      e.preventDefault();
      fn(el, e);
    });
    document.addEventListener('input', function (e) {
      var el = e.target;
      if (!el || !el.getAttribute || !el.getAttribute('data-model')) return;
      if (el.tagName === 'SELECT' || el.tagName === 'BUTTON') return;
      handleModelValue(el);
    });
    document.addEventListener('change', function (e) {
      var el = e.target;
      if (!el || !el.getAttribute || !el.getAttribute('data-model')) return;
      if (el.tagName === 'SELECT') handleModelValue(el);
    });
    document.addEventListener('keydown', function (e) {
      if (!(e.metaKey || e.ctrlKey) || e.altKey) return;
      var map = { '1': 'overview', '2': 'run', '3': 'review', '4': 'settings' };
      var target = map[e.key];
      if (target) {
        e.preventDefault();
        navigate(target);
      }
    });
    window.addEventListener('beforeunload', function () {
      try { if (RUNTIME && typeof RUNTIME.EventsOff === 'function') RUNTIME.EventsOff(); } catch (e) {}
    });
  }

  /* ---------- 启动 ---------- */
  function boot() {
    UI.mountSprite();
    subscribeEvents();
    var mountedGuard = setTimeout(function () {
      if (mounted) return;
      mountShell();
      S.overviewError = '引擎响应超时：后端还没返回状态，可以点「重试」或检查引擎目录。';
      S.overviewRecentLoading = false;
      render();
    }, 15000);
    loadData().then(function () {
      clearTimeout(mountedGuard);
      applyPlatformClass();
      mountShell();
      loadOverviewRecent();
    }).catch(function (e) {
      clearTimeout(mountedGuard);
      applyPlatformClass();
      mountShell();
      S.overviewError = friendlyError(errText(e));
      S.overviewRecentLoading = false;
      render();
    });
  }

  if (isDev) {
    window.__APP = { state: S, navigate: navigate, render: render, actions: actions };
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', boot);
  } else {
    boot();
  }
})();
