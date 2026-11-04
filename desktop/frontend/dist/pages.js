/* 超星助手 · 页面模板 v3（统一设计语言，组件与数值对齐 ykt app.css） */
(function () {
  'use strict';

  var icon = UI.icon;
  var esc = UI.esc;

  var LOG_LEVELS = {
    trace: { label: '追踪', cls: 'is-debug' },
    debug: { label: '调试', cls: 'is-debug' },
    info: { label: '信息', cls: 'is-info' },
    warning: { label: '警告', cls: 'is-warn' },
    error: { label: '错误', cls: 'is-error' }
  };

  var STEPS = [
    ['login', '登录'],
    ['courses', '课程'],
    ['chapters', '章节'],
    ['task_center', '任务中心'],
    ['review', '收尾']
  ];

  var REVIEW_DAYS = [[1, '今天'], [3, '近 3 天'], [0, '全部']];

  var KIND_COLOR = {
    '测验简答': 'is-blue',
    '作业简答': 'is-indigo',
    '讨论回复': 'is-purple',
    'AI实践作答': 'is-orange'
  };

  /* ================= 基础片段 ================= */

  function appMark() {
    return '<span class="about-icon is-app" aria-hidden="true">' +
      '<svg viewBox="0 0 40 40" role="img" aria-hidden="true">' +
      '<rect width="40" height="40" rx="10" fill="#0a84ff"/>' +
      '<text x="20" y="27" text-anchor="middle" font-size="19" font-weight="600" fill="#ffffff" ' +
      'font-family="-apple-system, BlinkMacSystemFont, PingFang SC, sans-serif">超</text>' +
      '<circle cx="32.5" cy="32.5" r="6.4" fill="#34c759" stroke="#ffffff" stroke-width="1.5"/>' +
      '<polyline points="29.5 32.6 31.5 34.6 35.5 30.5" fill="none" stroke="#ffffff" ' +
      'stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>' +
      '</svg></span>';
  }

  function pageHead(title, sub, side) {
    return '<header class="page-head">' +
      '<div class="page-heading"><h1>' + esc(title) + '</h1>' +
      (sub ? '<p class="page-sub">' + sub + '</p>' : '') + '</div>' +
      (side ? '<div class="page-head-side">' + side + '</div>' : '') +
      '</header>';
  }

  function rowIcon(name, color) {
    return '<span class="row-icon ' + color + '" aria-hidden="true">' + icon(name) + '</span>';
  }

  function badge(text, tone) {
    return '<span class="badge' + (tone ? ' is-' + tone : '') + '"><span class="badge-dot" aria-hidden="true"></span>' + esc(text) + '</span>';
  }

  function statusBadge(status) {
    var s = String(status || '');
    if (s.indexOf('已提交') >= 0) return badge(s, 'success');
    if (s.indexOf('待提交') >= 0) return badge(s, 'warning');
    if (s.indexOf('失败') >= 0 || s.indexOf('出错') >= 0) return badge(s, 'danger');
    return badge(s || '未知');
  }

  function reviewExplain(status) {
    var s = String(status || '');
    if (s.indexOf('已提交') >= 0) return '已提交给平台，可以在这里回看或复制正文；需要重答时可在平台撤回后重刷。';
    if (s.indexOf('待提交') >= 0) return '答案已生成但还没提交，再刷到同一条任务时会带上这份内容。';
    if (s.indexOf('取消') >= 0) return '这条任务被取消了，内容不会再提交。';
    if (s.indexOf('失败') >= 0) return '提交失败了，可以看运行日志里的错误信息，处理好网络或登录状态后再刷。';
    return '本次生成的复核记录。';
  }

  function emptyState(opts) {
    return '<div class="empty-state">' +
      '<span class="empty-icon' + (opts.error ? ' is-error' : '') + '" aria-hidden="true">' +
      icon(opts.error ? 'alert' : (opts.icon || 'info')) + '</span>' +
      '<span class="empty-title">' + esc(opts.title) + '</span>' +
      (opts.desc ? '<span class="empty-desc">' + esc(opts.desc) + '</span>' : '') +
      (opts.action || '') +
      '</div>';
  }

  function skeletonGroup(rows) {
    var out = '<div class="skeleton-block">';
    for (var i = 0; i < (rows || 3); i++) out += '<span class="sk-line" style="width:' + (70 - i * 8) + '%"></span>';
    return out + '</div>';
  }

  function selectWrap(model, options, current, cls, label) {
    var out = '<span class="select-wrap ' + (cls || 'select-sm') + '"><select data-model="' + model + '" aria-label="' + esc(label) + '">';
    options.forEach(function (opt) {
      var selected = String(opt[0]) === String(current);
      out += '<option value="' + esc(opt[0]) + '"' + (selected ? ' selected' : '') + '>' + esc(opt[1]) + '</option>';
    });
    return out + '</select></span>';
  }

  function inputNum(model, value, min, max, step, label, extraClass) {
    return '<input class="input mono ' + (extraClass || 'input-num') + '" type="number" min="' + (min != null ? min : 0) + '"' +
      (max != null ? ' max="' + max + '"' : '') + ' step="' + (step || 1) + '" value="' + (value != null ? value : 0) + '"' +
      ' data-model="' + model + '" aria-label="' + esc(label) + '">';
  }

  function switchButton(model, checked, label) {
    return '<button class="switch" type="button" role="switch" data-action="toggle-switch" data-model="' + model + '"' +
      ' aria-checked="' + (checked ? 'true' : 'false') + '" aria-label="' + esc(label) + '">' +
      '<span class="switch-knob" aria-hidden="true"></span></button>';
  }

  function row(title, desc, control, iconHtml) {
    return '<div class="row">' + (iconHtml || '') +
      '<div class="row-copy"><div class="row-title">' + title + '</div>' +
      (desc ? '<div class="row-desc">' + desc + '</div>' : '') + '</div>' +
      (control ? '<div class="row-control">' + control + '</div>' : '') + '</div>';
  }

  function group(title, boxHtml, note) {
    return '<section class="group"><h2 class="group-title">' + esc(title) + '</h2>' +
      '<div class="group-box">' + boxHtml + '</div>' +
      (note ? '<p class="group-note">' + note + '</p>' : '') + '</section>';
  }

  /* ================= 数据辅助 ================= */

  function currentCourses(s) {
    var cached = s.run.coursesByAccount[s.run.accountId];
    return (cached && cached.courses) || null;
  }

  function filteredCourses(s) {
    var list = currentCourses(s) || [];
    var q = (s.run.search || '').trim().toLowerCase();
    if (!q) return list;
    return list.filter(function (c) {
      return String(c.title || '').toLowerCase().indexOf(q) >= 0 ||
        String(c.teacher || '').toLowerCase().indexOf(q) >= 0;
    });
  }

  function selectedCount(s) {
    var map = s.run.selected || {};
    return Object.keys(map).filter(function (k) { return map[k]; }).length;
  }

  function validationHint(s) {
    if (!s.run.accountId) return '先选一个账号';
    if (s.run.coursesLoading) return '正在拉取课程';
    if (!selectedCount(s)) return '至少选一门课程';
    if (!s.run.scope.chapters && !s.run.scope.task_center) return '章节和任务中心至少开一项';
    return '';
  }

  function scopeText(s) {
    var parts = [];
    if (s.run.scope.chapters) parts.push('章节');
    if (s.run.scope.task_center) parts.push('任务中心');
    return parts.join(' + ') || '未选择范围';
  }

  function accountLabel(s) {
    var accounts = s.accounts || [];
    for (var i = 0; i < accounts.length; i++) {
      if (accounts[i].id === s.run.accountId) return accounts[i].name || accounts[i].username_masked;
    }
    return '未选择账号';
  }

  function reviewCount(s, days) {
    var n = 0;
    var items = (s.status && s.status.reviews) || {};
    if (days === 1) n = items.today || 0;
    else if (days === 3) n = items.recent || 0;
    else n = (s.review.items || []).length;
    return n;
  }

  /* ================= 概览 ================= */

  function overview(s) {
    var st = s.status || {};
    var reviews = st.reviews || {};
    var lastRun = st.last_run || {};
    var accounts = s.accounts || [];
    var engine = s.engine || {};
    var engineStatus = (st.engine) || {};
    var ready = !!s.engineReady;
    var nickname = accounts.length ? (accounts[0].name || accounts[0].username_masked) : '';
    var subtitle = accounts.length
      ? '当前账号 ' + esc(nickname) + (accounts.length > 1 ? ' · ' + accounts.length + ' 个账号' : '')
      : '未登录';

    var engineRow = '<div class="row">' +
      rowIcon('server', ready ? 'is-green' : 'is-orange') +
      '<div class="row-copy"><div class="row-title">引擎' + badge(ready ? '已就绪' : '未就绪', ready ? 'success' : 'warning') + '</div>' +
      '<div class="row-desc mono">' + esc(engine.message || (engine.python ? 'Python ' + engine.python : '尚未检测到可用环境')) +
      (engine.repo ? ' · ' + esc(engine.repo) : '') + '</div></div>' +
      '<div class="row-control">' +
      (ready ? '' : '<button class="btn btn-sm btn-primary" data-action="choose-engine">选择目录</button>') +
      '<button class="btn btn-sm" data-action="retry-engine">重新检测</button>' +
      '</div></div>';

    var statusBox =
      '<div class="row">' + rowIcon('clock', 'is-blue') +
      '<div class="row-copy"><div class="row-title">上次运行</div>' +
      '<div class="row-desc">' + (lastRun.log_mtime ? '最近日志 ' + esc(UI.fmtClock(lastRun.log_mtime)) : '暂无运行记录') + '</div></div>' +
      '<div class="row-control"><span class="row-value mono">' + (lastRun.at ? esc(UI.fmtClock(lastRun.at)) : '—') + '</span></div></div>' +
      '<div class="row">' + rowIcon('user', 'is-indigo') +
      '<div class="row-copy"><div class="row-title">账号</div><div class="row-desc">保存在本机的学习通账号</div></div>' +
      '<div class="row-control"><span class="row-value">' + accounts.length + ' 个</span></div></div>' +
      '<div class="row">' + rowIcon('file', 'is-orange') +
      '<div class="row-copy"><div class="row-title">今日复核</div><div class="row-desc">AI 生成并已提交的文字</div></div>' +
      '<div class="row-control"><span class="row-value">' + (reviews.today || 0) + ' 条</span></div></div>' +
      engineRow;

    var quickBox =
      '<div class="row">' + rowIcon('play', 'is-blue') +
      '<div class="row-copy"><div class="row-title">开始刷课</div><div class="row-desc">选账号与课程，进度自动保存</div></div>' +
      '<div class="row-control"><button class="btn btn-primary" data-action="goto-run">开始刷课</button></div></div>' +
      '<div class="row">' + rowIcon('file', 'is-gray') +
      '<div class="row-copy"><div class="row-title">查看复核</div><div class="row-desc">' +
      ((reviews.today || 0) ? '今天有 ' + (reviews.today || 0) + ' 条记录' : '暂时没有今天的记录') + '</div></div>' +
      '<div class="row-control"><button class="btn btn-sm" data-action="goto-review">查看</button></div></div>' +
      '<div class="row">' + rowIcon('folder', 'is-gray') +
      '<div class="row-copy"><div class="row-title">打开数据目录</div><div class="row-desc mono">' +
      esc((engineStatus && engineStatus.data_dir) || '~/.chaoxing') + '</div></div>' +
      '<div class="row-control"><button class="btn btn-sm" data-action="open-data-dir">打开</button></div></div>';

    var recent = s.overviewRecent || [];
    var recentBox;
    if (s.overviewRecentLoading) {
      recentBox = skeletonGroup(3);
    } else if (s.overviewRecentError) {
      recentBox = emptyState({
        error: true, title: '读取复核记录失败',
        desc: s.overviewRecentError + '。检查引擎状态后再试。',
        action: '<button class="btn" data-action="refresh-overview">重试</button>'
      });
    } else if (!recent.length) {
      recentBox = emptyState({
        icon: 'file', title: '暂无复核记录',
        desc: '刷课中 AI 生成并提交的文字会出现在这里。'
      });
    } else {
      recentBox = recent.slice(0, 5).map(function (item) {
        return '<button class="row" data-action="open-review" data-id="' + esc(item.submission_id) + '">' +
          '<div class="row-copy"><div class="row-title">' + esc(item.task || item.kind || '未命名任务') + '</div>' +
          '<div class="row-desc">' + esc(item.kind || '') + (item.course ? ' · ' + esc(item.course) : '') + '</div></div>' +
          '<div class="row-control"><span class="row-value mono">' + esc(UI.fmtClock(item.ts)) + '</span>' +
          statusBadge(item.status) + '</div></button>';
      }).join('');
    }

    var body = '';
    if (s.overviewError) {
      body += '<section class="group"><div class="group-box">' + emptyState({
        error: true, title: '读取状态失败',
        desc: s.overviewError + '。多半是引擎还没启动完，重试；一直失败就去设置里检查引擎目录。',
        action: '<button class="btn" data-action="refresh-overview">重试</button>'
      }) + '</div></section>';
    }
    body += group('运行状态', statusBox,
      ready ? '' : '选择 chaoxing 仓库目录，或确认已安装 Python 3.13 以上版本。') +
      group('快捷操作', quickBox) +
      group('最近复核', recentBox);

    return pageHead('概览', subtitle) + body;
  }

  /* ================= 刷课：配置 ================= */

  function accountRowsHTML(s) {
    var accounts = s.accounts || [];
    if (!accounts.length) {
      return emptyState({
        icon: 'user', title: '还没有账号',
        desc: '添加学习通账号后才能拉取课程和刷课。',
        action: '<button class="btn btn-primary" data-action="add-account">添加账号</button>'
      });
    }
    return accounts.map(function (a) {
      var selected = s.run.accountId === a.id;
      return '<button class="row' + (selected ? ' is-selected' : '') + '" role="radio" aria-checked="' + selected + '" data-action="select-account" data-id="' + esc(a.id) + '">' +
        rowIcon('user', 'is-indigo') +
        '<div class="row-copy"><div class="row-title">' + esc(a.name || a.username_masked) + '</div>' +
        '<div class="row-desc">' + esc(a.name ? a.username_masked : '未设置昵称') + (a.last_used ? ' · 最近使用 ' + esc(UI.fmtClock(a.last_used)) : '') + '</div></div>' +
        '<div class="row-control"><span class="row-check">' + icon('check') + '</span></div></button>';
    }).join('');
  }

  function courseRowsHTML(s) {
    var selected = s.run.selected || {};
    return filteredCourses(s).map(function (c) {
      var checked = !!selected[c.courseId];
      var meta = [];
      if (c.teacher) meta.push(c.teacher);
      if (c.desc) meta.push(c.desc);
      return '<button class="row' + (checked ? ' is-selected' : '') + '" role="checkbox" aria-checked="' + checked + '" data-action="toggle-course" data-id="' + esc(c.courseId) + '">' +
        '<div class="row-copy"><div class="row-title">' + esc(c.title || '未命名课程') + '</div>' +
        (meta.length ? '<div class="row-desc">' + esc(meta.join(' · ')) + '</div>' : '') + '</div>' +
        '<div class="row-control"><span class="row-check is-square">' + icon('check') + '</span></div></button>';
    }).join('');
  }

  function courseListHTML(s) {
    if (!s.run.accountId) {
      return emptyState({ icon: 'book', title: '先选一个账号', desc: '选完账号会自动拉取课程列表。' });
    }
    if (s.run.coursesLoading) return skeletonGroup(4);
    if (s.run.coursesError) {
      return emptyState({
        error: true, title: '拉取课程失败',
        desc: s.run.coursesError + '。检查网络和账号密码后再试；一直失败就去设置里重新添加账号。',
        action: '<button class="btn" data-action="refresh-courses">重试</button>'
      });
    }
    var cached = currentCourses(s);
    if (!cached) {
      return emptyState({ icon: 'book', title: '还没有拉取课程', desc: '点「刷新课程」拉取这个账号的课程。' });
    }
    if (!cached.length) {
      return emptyState({ icon: 'book', title: '这个账号下没有课程', desc: '确认账号在网页端能看到课程，或者换一个账号。' });
    }
    var list = filteredCourses(s);
    if (!list.length) {
      return emptyState({
        icon: 'list', title: '没有匹配的课程', desc: '换个关键词，或者清空搜索。',
        action: '<button class="btn" data-action="clear-course-search">清空搜索</button>'
      });
    }
    return courseRowsHTML(s);
  }

  function courseMetaText(s) {
    var total = (currentCourses(s) || []).length;
    return selectedCount(s) + ' / ' + total;
  }

  function scopeNoteHTML(s) {
    if (!s.run.scope.chapters && !s.run.scope.task_center) return '章节任务和任务中心至少开一项，否则无法开始刷课。';
    if (!s.run.accountId) return '先选一个账号，再选择课程和参数。';
    if (!selectedCount(s)) return '至少选一门课程后开始。';
    return '';
  }

  function runConfigView(s) {
    var o = s.run.options;
    var hint = validationHint(s);
    var headSide =
      '<button class="btn btn-sm" data-action="refresh-courses">刷新课程</button>' +
      '<button class="btn btn-primary" id="start-run" data-action="start-run"' + (hint ? ' disabled' : '') + '>开始刷课</button>';

    var accountsBox = accountRowsHTML(s);
    var searchRow = '<div class="row course-search-row">' +
      '<input class="input" id="course-search" type="search" data-model="run.search" value="' + esc(s.run.search || '') + '" placeholder="搜索课程或老师" aria-label="搜索课程"' + (s.run.accountId ? '' : ' disabled') + '>' +
      '<div class="row-control">' +
      '<span class="badge" id="course-meta-badge"><span class="badge-dot" aria-hidden="true"></span>' +
      (s.run.accountId ? (s.run.coursesLoading ? '拉取中' : courseMetaText(s)) : '未选账号') + '</span>' +
      '<button class="btn btn-sm" data-action="toggle-all-courses"' + (s.run.accountId && !s.run.coursesLoading ? '' : ' disabled') + '>全选</button>' +
      '<button class="btn btn-sm" data-action="clear-courses"' + (selectedCount(s) ? '' : ' disabled') + '>清空</button>' +
      '</div></div>';
    var coursesBox = searchRow + '<div class="course-list" id="course-list">' + courseListHTML(s) + '</div>';

    var scopeBox =
      row('章节任务点', '视频、文档、章节测验', switchButton('run.scope.chapters', s.run.scope.chapters, '章节任务点')) +
      row('任务中心', '作业、考试、讨论回复', switchButton('run.scope.task_center', s.run.scope.task_center, '任务中心'));

    var speedOptions = [];
    for (var v = 10; v <= 20; v++) speedOptions.push([(v / 10).toFixed(1), (v / 10).toFixed(1) + '×']);
    var jobsOptions = [];
    for (var j = 1; j <= 8; j++) jobsOptions.push([String(j), String(j)]);
    var limitOptions = [['0', '全部'], ['1', '1 个'], ['2', '2 个'], ['3', '3 个'], ['5', '5 个'], ['10', '10 个']];
    var notopenOptions = [['continue', '跳过'], ['retry', '稍后重试']];

    var paramsBox =
      row('播放速度', '视频播放倍速', selectWrap('run.options.speed', speedOptions, o.speed, 'select-sm', '播放速度')) +
      row('并发任务', '同时跑几门课', selectWrap('run.options.jobs', jobsOptions, o.jobs, 'select-sm', '并发任务')) +
      row('每门课上限', '任务点数量，全部表示不限', selectWrap('run.limits.max_points_per_course', limitOptions, s.run.limits.max_points_per_course, 'select-sm', '每门课上限')) +
      row('每门课最多任务数', '0 表示不限', inputNum('run.limits.max_tasks_per_course', s.run.limits.max_tasks_per_course, 0, 1000, 1, '每门课最多任务数')) +
      row('遇到关闭的任务点', '任务点还没开放时怎么处理', selectWrap('run.options.notopen_action', notopenOptions, o.notopen_action, 'select-sm', '未开放任务处理')) +
      row('重试间隔', '任务点未开放时的重试等待', inputNum('run.options.retry_interval', o.retry_interval, 0, 600, 0.5, '重试间隔') + '<span class="input-unit">秒</span>') +
      row('串行播放视频', '播完一个再播下一个', switchButton('run.options.serial_video', o.serial_video, '串行播放视频')) +
      row('增加学习次数', '刷完任务点后轮询章节增加学习次数', switchButton('run.options.add_learning_count', o.add_learning_count, '增加学习次数')) +
      '<div class="row" id="row-run-target">' +
      '<div class="row-copy"><div class="row-title">章节学习次数目标</div>' +
      '<div class="row-desc">开启「增加学习次数」后，轮询章节直到达到该次数</div></div>' +
      '<div class="row-control"><input class="input mono input-num" id="run-target-count" type="number" min="1" max="1000" value="' + o.target_count + '" data-model="run.options.target_count" aria-label="章节学习次数目标"' + (o.add_learning_count ? '' : ' disabled') + '></div></div>' +
      row('测验答错重做', '章节检测答错后自动重新作答（仅允许作答一次的课程不要开）', switchButton('run.options.work_redo_enabled', o.work_redo_enabled, '测验答错重做')) +
      row('重做次数上限', '答错后最多重新作答几次', '<input class="input mono input-num" id="work-retries" type="number" min="1" max="10" value="' + o.work_max_retries + '" data-model="run.options.work_max_retries" aria-label="重做次数上限"' + (o.work_redo_enabled ? '' : ' disabled') + '>');

    return pageHead('刷课', '选账号与课程，然后开始', headSide) +
      group('账号', accountsBox) +
      group('课程', coursesBox) +
      group('范围', scopeBox, '<span id="scope-note">' + esc(scopeNoteHTML(s)) + '</span>') +
      group('参数', paramsBox);
  }

  /* ================= 刷课：运行 / 结果 ================= */

  function stepsHTML(s) {
    var current = s.run.stage || 'login';
    var idx = 0;
    for (var i = 0; i < STEPS.length; i++) if (STEPS[i][0] === current) idx = i;
    if (s.run.phase === 'stopping') idx = Math.max(idx, 4);
    return STEPS.map(function (st, i) {
      var done = i < idx;
      var currentCls = i === idx && !done ? ' is-current' : '';
      return '<div class="step-row">' +
        '<span class="step-num' + (done ? ' is-done' : '') + currentCls + '" aria-hidden="true">' +
        (done ? icon('check') : '<span>' + (i + 1) + '</span>') + '</span>' +
        '<div class="row-copy"><div class="row-title">' + esc(st[1]) + '</div></div>' +
        '</div>';
    }).join('');
  }

  function progressHTML(s) {
    var p = s.run.progress;
    if (!p) return '';
    var total = Number(p.total) || 0;
    var done = Number(p.done) || 0;
    var pct = total > 0 ? Math.min(100, Math.round((done / total) * 100)) : 0;
    return '<div class="row progress-row">' +
      '<div class="row-copy"><span class="row-value">' + esc(p.title || '当前课程') + '</span>' +
      '<span class="progress-track"><span class="progress-bar" style="width:' + pct + '%"></span></span>' +
      '<span class="row-value mono">' + done + '/' + total + '</span></div></div>';
  }

  function logFilteredLines(s) {
    return (s.run.logLines || []).filter(function (line) {
      if (s.run.logFilter === 'all') return true;
      return line.level === s.run.logFilter;
    });
  }

  function logLineHTML(line) {
    var level = line.level || 'info';
    var meta = LOG_LEVELS[level] || LOG_LEVELS.info;
    var time = line.ts ? UI.fmtTimeOfDay(line.ts) : '';
    return '<div class="log-row"><span class="log-time" title="' + esc(line.ts || '') + '">' + esc(time) + '</span>' +
      '<span class="log-level ' + meta.cls + '">' + meta.label + '</span>' +
      '<span class="log-msg">' + esc(line.text) + '</span></div>';
  }

  function logBodyHTML(s) {
    var lines = logFilteredLines(s);
    if (!lines.length) {
      return emptyState({
        icon: 'list', title: (s.run.logLines || []).length ? '当前级别下没有日志' : '暂无日志',
        desc: (s.run.logLines || []).length ? '换一个级别筛选看看。' : '开始刷课后，引擎输出会实时出现在这里。'
      });
    }
    return lines.map(logLineHTML).join('');
  }

  function logCountText(s) {
    return (s.run.logLines || []).length + ' 条';
  }

  function logCardHTML(s) {
    var filters = [['all', '全部'], ['info', '信息'], ['warning', '警告'], ['error', '错误']];
    return '<div class="log-card is-runtime' + (s.run.logCollapsed ? ' is-collapsed' : '') + '" id="log-card">' +
      '<div class="log-card-head has-tools">' +
      '<span class="log-live"><span class="live-dot" aria-hidden="true"></span>日志流</span>' +
      '<span class="log-count" id="log-count">' + logCountText(s) + '</span>' +
      '<div class="appearance-seg" role="group" aria-label="日志级别">' +
      filters.map(function (f) {
        return '<button type="button" class="seg-btn' + (s.run.logFilter === f[0] ? ' is-active' : '') + '" data-action="log-filter" data-level="' + f[0] + '" aria-pressed="' + (s.run.logFilter === f[0]) + '">' + f[1] + '</button>';
      }).join('') +
      '</div>' +
      '<div class="log-tools">' +
      '<span class="tool-switch"><span>自动滚动</span>' +
      '<button class="switch switch-sm" type="button" role="switch" data-action="toggle-switch" data-model="run.autoScroll" aria-checked="' + (s.run.autoScroll ? 'true' : 'false') + '" aria-label="自动滚动"><span class="switch-knob" aria-hidden="true"></span></button></span>' +
      '<button class="btn btn-sm" data-action="copy-logs">复制</button>' +
      '<button class="btn btn-sm" data-action="toggle-log">' + (s.run.logCollapsed ? '展开' : '收起') + '</button>' +
      '</div></div>' +
      '<div class="log-list" id="log-body" role="log" aria-label="运行日志" tabindex="0">' + logBodyHTML(s) + '</div>' +
      '</div>';
  }

  function runLiveView(s) {
    var stopping = s.run.phase === 'stopping';
    var stageText = stopping ? '正在停止' : (s.run.stageText || '正在启动');
    var headSide =
      '<span class="badge is-running" id="run-state-badge"><span class="badge-dot" aria-hidden="true"></span>' + (stopping ? '停止中' : '运行中') + '</span>' +
      '<span class="run-timer mono" id="run-timer">' + UI.fmtDuration(s.run.elapsed) + '</span>' +
      '<button class="btn btn-sm btn-danger-quiet" id="stop-run" data-action="stop-run"' + (stopping ? ' disabled' : '') + '>停止</button>';
    return pageHead('刷课', '<span id="run-page-sub">' + esc(stageText) + '</span> · 已用 <span class="mono" id="run-page-timer">' + UI.fmtDuration(s.run.elapsed) + '</span>', headSide) +
      group('进度', '<div class="run-steps" id="run-steps">' + stepsHTML(s) + '</div><div id="run-progress">' + progressHTML(s) + '</div>') +
      logCardHTML(s);
  }

  function resultView(s) {
    var f = s.run.finished || {};
    var stopped = f.reason === 'stopped';
    var failed = !stopped && f.ok === false;
    var tone = failed ? 'danger' : stopped ? 'warning' : 'success';
    var toneText = failed ? '中断' : stopped ? '已停止' : '已完成';
    var title = failed ? '刷课中断' : stopped ? '已停止' : '刷课完成';
    var message = f.message || (failed
      ? '运行时出了点问题，日志里有详细原因。'
      : stopped ? '已完成的进度会保留，未开始的任务不再继续。'
        : '任务都处理完了，可以到复核里查看生成的文字。');
    var duration = f.duration_s != null ? f.duration_s : s.run.elapsed;
    var summary = f.summary || {};
    var rows =
      row('状态', message, badge(toneText, tone)) +
      row('课程', '', '<span class="row-value">' + (summary.courses != null ? summary.courses : selectedCount(s)) + '</span>') +
      row('错误', '', '<span class="row-value">' + (summary.errors != null ? summary.errors : 0) + '</span>') +
      row('耗时', '', '<span class="row-value mono">' + UI.fmtDuration(duration) + '</span>') +
      '<div class="settings-foot-row">' +
      '<button class="btn btn-primary" data-action="goto-review">查看复核</button>' +
      '<button class="btn" data-action="run-again">再刷一轮</button>' +
      (failed ? '<button class="btn" data-action="toggle-log">查看日志</button>' : '') +
      '</div>';
    return pageHead('刷课', '本轮已结束') +
      group('结果', rows) +
      logCardHTML(s);
  }

  function run(s) {
    if (s.run.phase === 'config') return runConfigView(s);
    if (s.run.phase === 'finished') return resultView(s);
    return runLiveView(s);
  }

  /* ================= 复核 ================= */

  function reviewListHTML(s) {
    var r = s.review;
    if (r.loading) return skeletonGroup(4);
    if (r.error) {
      return emptyState({
        error: true, title: '读取复核记录失败',
        desc: r.error + '。检查引擎状态后再试。',
        action: '<button class="btn" data-action="refresh-reviews">重试</button>'
      });
    }
    if (!r.items.length) {
      return emptyState({
        icon: 'file',
        title: r.days === 1 ? '今天没有需要复核的文字' : '没有复核记录',
        desc: r.days === 1 ? '刷课中 AI 生成并提交的文字会出现在这里。' : '换一个时间范围看看。'
      });
    }
    return r.items.map(function (item) {
      var active = r.selectedId === item.submission_id;
      return '<button class="row' + (active ? ' is-selected' : '') + '" data-action="open-review" data-id="' + esc(item.submission_id) + '" aria-pressed="' + active + '">' +
        rowIcon('file', KIND_COLOR[item.kind] || 'is-gray') +
        '<div class="row-copy"><div class="row-title">' + esc(item.task || '未命名任务') + '</div>' +
        '<div class="row-desc">' + esc(item.course || '未知课程') + ' · ' + esc(item.kind || '') + '</div></div>' +
        '<div class="row-control"><span class="row-time">' + esc(UI.fmtClock(item.ts)) + '</span>' +
        statusBadge(item.status) + '</div></button>';
    }).join('');
  }

  function reviewDetailHTML(s) {
    var r = s.review;
    if (r.itemLoading) {
      return '<div class="review-detail-head"><div class="review-detail-copy">' + skeletonGroup(2) + '</div></div>' +
        '<div class="review-body">' + skeletonGroup(4) + '</div>';
    }
    if (r.itemError) {
      return emptyState({
        error: true, title: '读取全文失败',
        desc: r.itemError + '。重试一下，或者换一条看。',
        action: '<button class="btn" data-action="retry-review-item">重试</button>'
      });
    }
    var item = r.item;
    if (!item) {
      return emptyState({ icon: 'file', title: '从左侧选一条查看', desc: '全文可以选中复制。' });
    }
    var meta = [];
    if (item.course) meta.push('<span><span class="k">课程</span> ' + esc(item.course) + '</span>');
    if (item.kind) meta.push('<span><span class="k">类型</span> ' + esc(item.kind) + '</span>');
    if (item.ts) meta.push('<span><span class="k">时间</span> ' + esc(item.ts) + '</span>');
    meta.push('<span><span class="k">字数</span> ' + (item.chars || 0) + '</span>');
    meta.push('<span><span class="k">状态</span> ' + item.status + '</span>');
    return '<div class="review-detail-head">' +
      '<span class="review-detail-icon" aria-hidden="true">' + icon('file') + '</span>' +
      '<div class="review-detail-copy"><div class="review-detail-title">' + esc(item.task || item.kind || '复核详情') + '</div>' +
      '<div class="review-detail-meta">' + meta.join('') + '</div></div>' +
      '<div class="row-control"><button class="btn btn-sm" data-action="copy-review-text">复制正文</button></div>' +
      '</div>' +
      '<div class="review-status-note">' + icon('info') + '<span>' + esc(reviewExplain(item.status)) + '</span></div>' +
      '<div class="review-body"><div class="review-text" tabindex="0" aria-label="复核正文">' + esc(item.text || '（这条记录没有正文）') + '</div></div>' +
      '<div class="review-foot"><span class="review-path" title="' + esc(r.markdownPath || '') + '">' +
      (r.markdownPath ? esc(r.markdownPath) : '暂无记录文件路径') + '</span></div>';
  }

  function review(s) {
    var filters = '<div class="appearance-seg" role="group" aria-label="时间范围">' + REVIEW_DAYS.map(function (f) {
      var active = Number(s.review.days) === f[0];
      return '<button type="button" class="seg-btn' + (active ? ' is-active' : '') + '" data-action="review-days" data-days="' + f[0] + '" aria-pressed="' + active + '">' + f[1] + '</button>';
    }).join('') + '</div>' +
      '<button class="btn btn-sm" data-action="open-reviews-dir">打开记录文件夹</button>';
    var count = (s.review.items || []).length;
    return pageHead('复核', 'AI 生成并已提交给平台的文字，可回看与复制', filters) +
      '<div class="review-layout">' +
      '<section class="group"><div class="group-box review-list" id="review-items">' + reviewListHTML(s) + '</div>' +
      '<p class="group-note">' + (s.review.error ? '' : (s.review.loading ? '正在读取…' : '共 ' + count + ' 条记录')) + '</p></section>' +
      '<section class="group"><div class="group-box review-detail" id="review-detail">' + reviewDetailHTML(s) + '</div></section>' +
      '</div>';
  }

  /* ================= 设置 ================= */

  function settingsDraft(s) {
    return s.settingsPage.draft || s.settings || {};
  }

  function settingsView(s) {
    var d = settingsDraft(s);
    var engine = s.engine || {};
    var statusEngine = (s.status && s.status.engine) || {};
    var info = s.appInfo || {};
    var ready = !!s.engineReady;
    var dirty = !!s.settingsPage.dirty;

    var headSide =
      '<span class="dirty-chip" id="settings-dirty"' + (dirty ? '' : ' hidden') + '>有未保存的更改</span>' +
      '<button class="btn btn-sm" id="revert-settings" data-action="revert-settings"' + (dirty ? '' : ' hidden') + '>撤销</button>' +
      '<button class="btn btn-primary" id="save-settings" data-action="save-settings"' + (dirty && !s.settingsPage.saving ? '' : ' disabled') + '>' +
      (s.settingsPage.saving ? '保存中…' : '保存设置') + '</button>';

    var accountsBox = (s.accounts || []).map(function (a) {
      return '<div class="row">' + rowIcon('user', 'is-green') +
        '<div class="row-copy"><div class="row-title">' + esc(a.name || a.username_masked) + '</div>' +
        '<div class="row-desc">' + esc(a.name ? a.username_masked : '未设置昵称') +
        (a.last_used ? ' · 最近使用 ' + esc(UI.fmtClock(a.last_used)) : '') +
        (a.has_password ? '' : ' · 没有保存密码') + '</div></div>' +
        '<div class="row-control"><button class="btn btn-sm btn-danger-quiet" data-action="remove-account" data-id="' + esc(a.id) + '" data-label="' + esc(a.username_masked) + '">删除</button></div>' +
        '</div>';
    }).join('');
    if (!accountsBox) accountsBox = emptyState({ icon: 'user', title: '还没有账号', desc: '添加学习通账号后才能拉取课程。' });
    accountsBox += '<div class="row">' + rowIcon('plus', 'is-blue') +
      '<div class="row-copy"><div class="row-title">添加账号</div><div class="row-desc">账号只保存在本机</div></div>' +
      '<div class="row-control"><button class="btn btn-sm" data-action="add-account">添加账号</button></div></div>';

    var speedOptions = [];
    for (var v = 10; v <= 20; v++) speedOptions.push([(v / 10).toFixed(1), (v / 10).toFixed(1) + '×']);
    var jobsOptions = [];
    for (var j = 1; j <= 8; j++) jobsOptions.push([String(j), String(j)]);
    var notopenOptions = [['continue', '跳过'], ['retry', '稍后重试']];

    var paramsBox =
      row('播放速度', '视频播放倍速', selectWrap('settingsPage.draft.speed', speedOptions, d.speed, 'select-sm', '默认播放速度')) +
      row('并发任务', '同时跑几门课', selectWrap('settingsPage.draft.jobs', jobsOptions, d.jobs, 'select-sm', '默认并发任务')) +
      row('遇到关闭的任务点', '任务点还没开放时怎么处理', selectWrap('settingsPage.draft.notopen_action', notopenOptions, d.notopen_action, 'select-sm', '未开放任务处理')) +
      row('重试间隔', '任务点未开放时的重试等待', inputNum('settingsPage.draft.retry_interval', d.retry_interval, 0, 600, 0.5, '重试间隔') + '<span class="input-unit">秒</span>') +
      row('串行播放视频', '播完一个再播下一个', switchButton('settingsPage.draft.serial_video', d.serial_video, '串行播放视频')) +
      row('增加学习次数', '刷完任务点后轮询章节增加学习次数', switchButton('settingsPage.draft.add_learning_count', d.add_learning_count, '增加学习次数')) +
      '<div class="row' + (d.add_learning_count ? '' : ' is-disabled') + '" id="row-target-count">' +
      '<div class="row-copy"><div class="row-title">章节学习次数目标</div>' +
      '<div class="row-desc">开启「增加学习次数」后，轮询章节直到达到该次数</div></div>' +
      '<div class="row-control"><input class="input mono input-num" id="set-target" type="number" min="1" max="1000" value="' + d.target_count + '" data-model="settingsPage.draft.target_count" aria-label="章节学习次数目标"' + (d.add_learning_count ? '' : ' disabled') + '></div></div>' +
      row('测验答错重做', '章节检测答错后自动重新作答（仅允许作答一次的课程不要开）', switchButton('settingsPage.draft.work_redo_enabled', d.work_redo_enabled, '测验答错重做')) +
      '<div class="row' + (d.work_redo_enabled ? '' : ' is-disabled') + '" id="row-work-retries">' +
      '<div class="row-copy"><div class="row-title">重做次数上限</div>' +
      '<div class="row-desc">' + (d.work_redo_enabled ? '答错后最多重新作答几次' : '打开「测验答错重做」后才能设置') + '</div></div>' +
      '<div class="row-control"><input class="input mono input-num" id="set-work-retries" type="number" min="1" max="10" value="' + d.work_max_retries + '" data-model="settingsPage.draft.work_max_retries" aria-label="重做次数上限"' + (d.work_redo_enabled ? '' : ' disabled') + '></div></div>' +
      row('任务中心提交方式', '界面固定自动提交，不需要终端确认', '<span class="row-value">自动提交</span>');

    var repoPath = engine.repo || statusEngine.repo || '未设置';
    var dataDir = statusEngine.data_dir || '~/.chaoxing';
    var engineBox =
      '<div class="row"><div class="row-copy"><div class="row-title">仓库路径</div>' +
      '<div class="row-desc mono" title="' + esc(repoPath) + '" style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + esc(repoPath) + '</div></div>' +
      '<div class="row-control"><button class="btn btn-sm" data-action="choose-engine">选择目录</button></div></div>' +
      row('Python', '运行引擎使用的解释器', '<span class="row-value mono">' + esc(engine.python || statusEngine.python || '未检测到') + '</span>') +
      '<div class="row"><div class="row-copy"><div class="row-title">检测状态</div>' +
      '<div class="row-desc">' + esc(engine.message || (ready ? '仓库和 Python 都正常' : '选择目录后重新检测')) + '</div></div>' +
      '<div class="row-control">' + badge(ready ? '已就绪' : '未就绪', ready ? 'success' : 'warning') +
      '<button class="btn btn-sm" data-action="retry-engine">重新检测</button></div></div>' +
      '<div class="row"><div class="row-copy"><div class="row-title">数据目录</div>' +
      '<div class="row-desc mono" title="' + esc(dataDir) + '" style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + esc(dataDir) + '</div></div>' +
      '<div class="row-control"><button class="btn btn-sm" data-action="open-data-dir">打开</button></div></div>' +
      row('配置文件', 'config.ini，包含题库与通知设置', '<button class="btn btn-sm" data-action="open-config">打开</button>');

    var aboutBox = '<div class="about-head">' + appMark() +
      '<div class="about-copy"><div class="about-name">超星助手</div>' +
      '<div class="about-version mono">' + esc(info.version || '0.1.0') + '</div></div></div>' +
      row('协议版本', '', '<span class="row-value mono">' + esc(info.protocol != null ? info.protocol : 1) + '</span>') +
      row('许可证', '', '<span class="row-value">' + esc(info.license || 'GPL-3.0') + '</span>');

    return pageHead('设置', '账号、刷课参数与引擎环境', headSide) +
      '<div class="settings-layout">' +
      '<div class="settings-col">' +
      group('账号', accountsBox) +
      group('刷课参数', paramsBox) +
      '</div><div class="settings-col">' +
      group('引擎与数据', engineBox) +
      group('关于', aboutBox, '账号、配置和复核记录只保存在本机；请遵守学习通平台的使用规则。') +
      '</div></div>';
  }

  window.Pages = {
    overview: overview,
    run: run,
    review: review,
    settings: settingsView,
    courseListHTML: courseListHTML,
    courseMetaText: courseMetaText,
    filteredCourses: filteredCourses,
    selectedCount: selectedCount,
    validationHint: validationHint,
    scopeNoteHTML: scopeNoteHTML,
    stepsHTML: stepsHTML,
    progressHTML: progressHTML,
    logFilteredLines: logFilteredLines,
    logLineHTML: logLineHTML,
    logBodyHTML: logBodyHTML,
    logCountText: logCountText,
    reviewListHTML: reviewListHTML,
    reviewDetailHTML: reviewDetailHTML,
    LOG_LEVELS: LOG_LEVELS
  };
})();
