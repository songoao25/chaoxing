/* 超星助手 · 浏览器预览用 Mock（只在 dev/index-dev.html 加载，生产不加载） */
/* 模拟 window.go.main.App.* 与 window.runtime 事件总线，数据全部为合成假数据 */
(function () {
  'use strict';

  window.__DEV_PREVIEW__ = true;
  document.documentElement.setAttribute('data-preview', '1');
  document.documentElement.classList.add('mock');

  // 浏览器预览：禁用过渡与动画，保证无头截图 / 普通浏览器看到的是稳定终态
  (function () {
    var style = document.createElement('style');
    style.textContent = '*,*::before,*::after{transition:none!important;animation:none!important;scroll-behavior:auto!important}';
    (document.head || document.documentElement).appendChild(style);
  })();

  /* dev-only 调试：把 JS 错误直接画在页面上，方便无头截图排查 */
  function showDebug(text) {
    var el = document.getElementById('mock-debug');
    if (!el) {
      el = document.createElement('pre');
      el.id = 'mock-debug';
      el.style.cssText = 'position:fixed;left:8px;bottom:8px;right:8px;max-height:40vh;overflow:auto;z-index:9999;margin:0;padding:10px;background:#7a0010;color:#fff;font:11px/1.5 ui-monospace,monospace;white-space:pre-wrap;border-radius:8px';
      document.body.appendChild(el);
    }
    el.textContent += text + '\n';
  }
  window.addEventListener('error', function (e) {
    showDebug('[JS 错误] ' + (e.message || '') + '\n' + (e.error && e.error.stack ? e.error.stack : ''));
  });
  window.addEventListener('unhandledrejection', function (e) {
    var r = e.reason;
    showDebug('[Promise 拒绝] ' + (r && (r.stack || r.message) ? (r.stack || r.message) : String(r)));
  });

  var params = new URLSearchParams(window.location.search);
  var scenario = params.get('scenario') || '';
  var theme = params.get('theme');
  if (theme === 'dark' || theme === 'light') {
    document.documentElement.setAttribute('data-theme', theme);
  }

  function clone(value) {
    if (value === null || value === undefined) return value;
    return JSON.parse(JSON.stringify(value));
  }
  function pad2(n) { return (n < 10 ? '0' : '') + n; }
  function ymd(d) { return d.getFullYear() + '-' + pad2(d.getMonth() + 1) + '-' + pad2(d.getDate()); }
  function hms(d) { return pad2(d.getHours()) + ':' + pad2(d.getMinutes()) + ':' + pad2(d.getSeconds()); }
  function fullTs(d) { return ymd(d) + ' ' + hms(d); }
  function minuteTs(d) { return ymd(d) + ' ' + pad2(d.getHours()) + ':' + pad2(d.getMinutes()); }
  function dayOffset(offset, hour, minute, second) {
    var d = new Date();
    d.setDate(d.getDate() + offset);
    if (hour != null) d.setHours(hour, minute || 0, second || 0, 0);
    return d;
  }
  function delay(ms, value) {
    return new Promise(function (resolve) {
      setTimeout(function () { resolve(clone(value)); }, ms);
    });
  }
  function reject(ms, message) {
    return new Promise(function (resolve, rejectFn) {
      setTimeout(function () { rejectFn(new Error(message)); }, ms);
    });
  }
  function maskPhone(phone) {
    var s = String(phone || '');
    if (s.length >= 7) return s.slice(0, 3) + '****' + s.slice(-4);
    return s;
  }

  /* ---------- 假数据 ---------- */
  var accounts = [
    { id: '13800001111', name: '林晓', username_masked: '138****1111', last_used: minuteTs(dayOffset(0, 8, 42)), has_password: true },
    { id: '13900002222', name: '陈默', username_masked: '139****2222', last_used: minuteTs(dayOffset(-1, 21, 10)), has_password: true },
    { id: '13700003333', name: '', username_masked: '137****3333', last_used: '', has_password: true }
  ];

  var courses = [
    { courseId: '2151141', clazzId: 'c-2151141', title: '大学英语（二）', teacher: '王敏', desc: '2025–2026 学年第二学期', cpi: 'x' },
    { courseId: '189191', clazzId: 'c-189191', title: '高等数学 A', teacher: '李国华', desc: '必修 · 4 学分', cpi: 'x' },
    { courseId: '200302', clazzId: 'c-200302', title: '线性代数', teacher: '张伟', desc: '', cpi: 'x' },
    { courseId: '210988', clazzId: 'c-210988', title: '中国近现代史纲要', teacher: '刘芳', desc: '专题教学', cpi: 'x' },
    { courseId: '198765', clazzId: 'c-198765', title: '计算机基础', teacher: '陈晨', desc: '含实验课', cpi: 'x' },
    { courseId: '220011', clazzId: 'c-220011', title: '大学物理', teacher: '赵磊', desc: '', cpi: 'x' },
    { courseId: '230145', clazzId: 'c-230145', title: '马克思主义基本原理', teacher: '孙静', desc: '', cpi: 'x' },
    { courseId: '240677', clazzId: 'c-240677', title: '体育与健康', teacher: '周强', desc: '第 3–16 周', cpi: 'x' }
  ];

  var writerText = [
    '第 1 题：阅读下面材料，回答“作者为什么说‘慢下来’是一种能力”。',
    '',
    '参考答案：',
    '作者把“慢下来”看成一种能力，是因为在节奏越来越快的当下，慢需要主动选择和自我约束。首先，慢下来意味着能判断事情的优先级，把时间留给真正重要的内容；其次，慢下来让人有机会复述、检查和修正自己的理解，而不是被信息推着走；最后，慢是一种对注意力的管理能力，它需要练习，也需要抵抗即时反馈的诱惑。',
    '',
    '第 2 题：结合生活实际谈谈你的理解（不少于 80 字）。',
    '',
    '我认为刷题和读书都需要“慢”的能力。以前我习惯把视频调到倍速、直接跳到结论，结果做题时经常卡在第一步。后来我改成先完整看一遍、随手记下关键概念，再回头做练习，正确率反而更高。慢不是拖延，而是把理解放在速度前面。'
  ].join('\n');

  var mathText = [
    '题目：求函数 f(x)=x³−3x²+2 在区间 [−1,3] 上的最大值和最小值。',
    '',
    '解：',
    'f′(x)=3x²−6x=3x(x−2)，令 f′(x)=0 得 x=0 或 x=2。',
    '计算端点与驻点处的函数值：',
    'f(−1)=−1−3+2=−2；',
    'f(0)=2；',
    'f(2)=8−12+2=−2；',
    'f(3)=27−27+2=2。',
    '比较可得：最大值为 2（在 x=0 与 x=3 处取得），最小值为 −2（在 x=−1 与 x=2 处取得）。',
    '',
    '所以 f(x) 在 [−1,3] 上的最大值是 2，最小值是 −2。'
  ].join('\n');

  var historyText = [
    '题目：结合史实，简述近代中国先进分子探索国家出路的过程。',
    '',
    '答：鸦片战争后，中国逐步沦为半殖民地半封建社会，先进分子开始寻找救国道路。',
    '第一，洋务派以“自强”“求富”为口号兴办近代军事和民用工业，但只学技术不改制度，甲午战败宣告其破产。',
    '第二，维新派主张君主立宪，推动戊戌变法，因缺乏群众基础而失败。',
    '第三，革命派发动辛亥革命，推翻清王朝、建立中华民国，但未能改变半殖民地半封建的社会性质。',
    '第四，新文化运动高举民主与科学旗帜，马克思主义传入中国，中国共产党应运而生，中国革命从此有了新的领导力量和正确方向。',
    '结论：近代中国的出路探索从器物到制度再到思想文化，最终证明只有把马克思主义基本原理同中国具体实际相结合，才能找到民族独立和人民解放的道路。'
  ].join('\n');

  var discussionText = [
    '回复：同学你好，我同意你对“技术改变教育”的判断，但想补充两点。',
    '一是技术只是工具，决定效果的还是使用方式。同样的在线课程，有人用来系统学习，有人只是挂着刷时长，结果差别很大。',
    '二是技术会带来新的不平等。设备、网络、数字素养的差异，可能让一部分人更容易被落下。所以在推广技术的同时，也要考虑如何补齐这些基础条件。',
    '总的来说，技术能扩大优质资源的覆盖面，但需要配套的规则和投入，才能真正让更多人受益。'
  ].join('\n');

  var reviews = [
    {
      submission_id: 'r-1001',
      ts: fullTs(dayOffset(0, 9, 14, 22)),
      day: ymd(dayOffset(0)),
      kind: '测验简答',
      course: '大学英语（二）',
      task: 'Unit 1 阅读理解简答题',
      status: '已提交',
      chars: writerText.length,
      text: writerText
    },
    {
      submission_id: 'r-1002',
      ts: fullTs(dayOffset(0, 9, 16, 5)),
      day: ymd(dayOffset(0)),
      kind: '作业简答',
      course: '高等数学 A',
      task: '第二章 导数与极值 课后作业',
      status: '已提交',
      chars: mathText.length,
      text: mathText
    },
    {
      submission_id: 'r-1003',
      ts: fullTs(dayOffset(0, 10, 2, 41)),
      day: ymd(dayOffset(0)),
      kind: '讨论回复',
      course: '线性代数',
      task: '讨论 3：矩阵在实际生活中的应用',
      status: '待提交',
      chars: discussionText.length,
      text: discussionText
    },
    {
      submission_id: 'r-1004',
      ts: fullTs(dayOffset(0, 10, 21, 9)),
      day: ymd(dayOffset(0)),
      kind: 'AI实践作答',
      course: '中国近现代史纲要',
      task: '专题二 近代国家出路的探索',
      status: '失败',
      chars: historyText.length,
      text: historyText
    },
    {
      submission_id: 'r-0901',
      ts: fullTs(dayOffset(-1, 20, 3, 12)),
      day: ymd(dayOffset(-1)),
      kind: '测验简答',
      course: '高等数学 A',
      task: '第一章 函数与极限 小测',
      status: '已提交',
      chars: mathText.length,
      text: mathText
    },
    {
      submission_id: 'r-0902',
      ts: fullTs(dayOffset(-1, 20, 8, 44)),
      day: ymd(dayOffset(-1)),
      kind: '讨论回复',
      course: '大学英语（二）',
      task: 'Discussion: How to learn a language',
      status: '取消',
      chars: discussionText.length,
      text: discussionText
    },
    {
      submission_id: 'r-0801',
      ts: fullTs(dayOffset(-2, 19, 41, 30)),
      day: ymd(dayOffset(-2)),
      kind: '作业简答',
      course: '计算机基础',
      task: '第 3 章 算法与程序 作业',
      status: '已提交',
      chars: 268,
      text: '1. 算法是解决问题的步骤描述，具有有穷性、确定性、可行性等特征。\n\n2. 流程图用矩形表示处理步骤，菱形表示判断，平行四边形表示输入输出。\n\n3. 本题用冒泡排序实现数组升序：外层循环控制轮数，内层循环比较相邻元素并交换，时间复杂度 O(n²)，空间复杂度 O(1)。'
    },
    {
      submission_id: 'r-0701',
      ts: fullTs(dayOffset(-10, 18, 22, 10)),
      day: ymd(dayOffset(-10)),
      kind: '测验简答',
      course: '马克思主义基本原理',
      task: '第一章 世界的物质性 简答',
      status: '已提交',
      chars: 210,
      text: '物质是不依赖于人的意识并能为人的意识所反映的客观实在。世界的本原是物质，意识是物质世界长期发展的产物，是人脑的机能和属性。坚持物质的决定作用，同时重视意识的能动作用，是辩证唯物主义的基本立场。'
    }
  ];
  reviews.forEach(function (r) {
    r.text_preview = r.text.slice(0, 120);
  });

  var settings = {
    speed: 1.5,
    jobs: 2,
    notopen_action: 'continue',
    retry_interval: 1,
    serial_video: false,
    add_learning_count: false,
    target_count: 100,
    work_redo_enabled: false,
    work_max_retries: 3,
    task_center_submit_mode: 'auto'
  };

  var engineState = {
    repo: '/Users/demo/code/chaoxing',
    python: '3.14.7',
    data_dir: '/Users/demo/.chaoxing',
    config_exists: true
  };
  var engineReady = scenario !== 'engine-down';
  var bridgeState = scenario === 'engine-down' ? 'failed' : 'ready';

  var runState = { active: false, run_id: '', started_at: '', course_ids: [] };
  var runTimers = [];
  var runToken = 0;

  function countReviews(days) {
    if (!days) return reviews.length;
    var cutoff = dayOffset(-(days - 1));
    cutoff.setHours(0, 0, 0, 0);
    return reviews.filter(function (r) {
      var d = new Date(r.day + 'T00:00:00');
      return d.getTime() >= cutoff.getTime();
    }).length;
  }

  function buildStatus() {
    return {
      engine: clone(engineState),
      accounts: clone(accounts),
      settings: clone(settings),
      reviews: { today: countReviews(1), recent: countReviews(3) },
      last_run: {
        at: minuteTs(dayOffset(-1, 20, 10)),
        log_mtime: minuteTs(dayOffset(-1, 21, 35))
      },
      capabilities: { tiku_provider: 'AI', notification: 'ServerChan', llm_configured: true },
      engine_ready: engineReady,
      bridge_state: bridgeState
    };
  }

  function engineStatus() {
    return {
      repo: engineState.repo,
      python: engineState.python,
      ready: engineReady,
      message: engineReady ? '引擎可用' : '找不到 main.py，请选择引擎仓库目录，或检查 Python 环境'
    };
  }

  function filteredReviews(days) {
    if (scenario === 'review-empty') return [];
    var list = reviews.slice();
    if (days > 0) {
      var cutoff = dayOffset(-(days - 1));
      cutoff.setHours(0, 0, 0, 0);
      list = list.filter(function (r) {
        var d = new Date(r.day + 'T00:00:00');
        return d.getTime() >= cutoff.getTime();
      });
    }
    list.sort(function (a, b) { return a.ts < b.ts ? 1 : -1; });
    return list;
  }

  /* ---------- 事件总线 ---------- */
  var handlers = {};
  function emit(name, data) {
    var list = (handlers[name] || []).slice();
    list.forEach(function (cb) {
      setTimeout(function () {
        try { cb(clone(data)); } catch (e) { setTimeout(function () { throw e; }, 0); }
      }, 0);
    });
  }
  function EventsOn(name, cb) {
    (handlers[name] = handlers[name] || []).push(cb);
    return function () { EventsOff(name, cb); };
  }
  function EventsOnMultiple(name, cb, maxCallbacks) {
    var remaining = maxCallbacks;
    function wrapped(data) {
      if (remaining === 0) return;
      if (remaining > 0) remaining -= 1;
      cb(data);
    }
    return EventsOn(name, wrapped);
  }
  function EventsOff(name) {
    if (!name) { handlers = {}; return; }
    delete handlers[name];
    for (var i = 1; i < arguments.length; i++) delete handlers[arguments[i]];
  }
  window.runtime = {
    EventsOn: EventsOn,
    EventsOnMultiple: EventsOnMultiple,
    EventsOff: EventsOff,
    EventsOffAll: function () { handlers = {}; },
    EventsEmit: function (name) {
      var args = Array.prototype.slice.call(arguments, 1);
      emit(name, args.length > 1 ? args : args[0]);
    },
    ClipboardSetText: function (text) {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        return navigator.clipboard.writeText(String(text));
      }
      return Promise.resolve();
    },
    BrowserOpenURL: function () { return Promise.resolve(); },
    WindowSetTitle: function () { return Promise.resolve(); }
  };

  /* ---------- 刷课事件流 ---------- */
  function clearRunTimers() {
    runTimers.forEach(function (t) { clearTimeout(t); });
    runTimers = [];
  }
  function later(fn, ms) {
    var t = setTimeout(fn, ms);
    runTimers.push(t);
    return t;
  }

  function startMockRun(opts) {
    if (runState.active) return reject(120, 'busy: 已有刷课任务在运行');
    if (!opts || !opts.course_ids || !opts.course_ids.length) return reject(120, 'bad_request: 至少选择一门课程');
    if (!opts.scope || (!opts.scope.chapters && !opts.scope.task_center)) return reject(120, 'bad_request: 章节任务和任务中心至少选一项');
    var token = ++runToken;
    var started = new Date();
    var runId = 'r-' + started.getFullYear() + pad2(started.getMonth() + 1) + pad2(started.getDate()) + '-' +
      pad2(started.getHours()) + pad2(started.getMinutes()) + pad2(started.getSeconds());
    var selectedCourses = courses.filter(function (c) { return opts.course_ids.indexOf(c.courseId) >= 0; });
    runState = {
      active: true,
      run_id: runId,
      started_at: fullTs(started),
      course_ids: opts.course_ids.slice()
    };
    var account = accounts.filter(function (a) { return a.id === opts.account_id; })[0] || accounts[0];
    var firstTitle = selectedCourses.length ? selectedCourses[0].title : '课程';
    var secondTitle = selectedCourses.length > 1 ? selectedCourses[1].title : firstTitle;
    var errors = 0;

    function emitLog(ms, level, text) {
      later(function () {
        if (token !== runToken) return;
        emit('run:log', { run_id: runId, level: level, text: text, ts: fullTs(new Date()) });
      }, ms);
    }
    function emitStage(ms, stage, text) {
      later(function () {
        if (token !== runToken) return;
        emit('run:stage', { run_id: runId, stage: stage, text: text });
      }, ms);
    }
    function emitProgress(ms, course, done, total) {
      later(function () {
        if (token !== runToken) return;
        emit('run:progress', {
          run_id: runId, kind: 'course', course_id: course.courseId,
          title: course.title, done: done, total: total
        });
      }, ms);
    }

    later(function () {
      if (token !== runToken) return;
      emit('run:started', {
        run_id: runId,
        started_at: fullTs(started),
        account_id: opts.account_id,
        username_masked: account ? account.username_masked : '',
        courses: selectedCourses.map(function (c) { return { courseId: c.courseId, title: c.title }; }),
        scope: opts.scope
      });
    }, 160);

    emitStage(420, 'login', '正在登录学习通…');
    emitLog(900, 'info', '已登录：' + (account ? (account.name || account.username_masked) : '演示账号'));
    emitStage(1150, 'courses', '正在读取课程列表…');
    emitLog(1500, 'info', '共加载 ' + selectedCourses.length + ' 门课程');
    if (selectedCourses.length > 2) emitLog(1750, 'warning', '《' + selectedCourses[2].title + '》暂未开始，本轮跳过');
    emitStage(2000, 'chapters', '正在处理章节任务…');
    emitProgress(2300, selectedCourses[0] || courses[0], 1, 5);
    emitLog(2500, 'info', '[' + firstTitle + '] 开始播放第 1 个视频（12:35）');
    emitLog(2850, 'debug', '播放速度 1.5×，已开启自动下一节');
    emitLog(3150, 'info', '[' + firstTitle + '] 视频播放完成，进度已保存');
    emitLog(3500, 'info', '[' + secondTitle + '] 开始完成章节测验');
    emitLog(3850, 'warning', '[' + (selectedCourses[3] || courses[3]).title + '] 任务点暂未开放，1 秒后重试');
    emitLog(4200, 'info', '[' + (selectedCourses[3] || courses[3]).title + '] 任务点已开放，继续处理');
    emitProgress(4500, selectedCourses[1] || courses[1], 3, 5);
    emitStage(4800, 'task_center', '正在处理任务中心…');
    emitLog(5100, 'info', '[' + secondTitle + '] 讨论回复已生成，已写入复核');
    emitLog(5450, 'info', '[' + firstTitle + '] 作业已提交');
    emitLog(5800, 'info', '[' + (selectedCourses[2] || courses[2]).title + '] 简答题答案已生成，等待复核');
    emitLog(6150, 'debug', '本轮已处理 12 个任务点，还剩 3 个');
    emitProgress(6400, selectedCourses[1] || courses[1], 5, 5);
    emitStage(6800, 'review', '正在整理复核记录…');
    emitLog(7100, 'info', '新生成 4 条复核记录，可在「复核」里查看');
    emitStage(7450, 'done', '正在收尾');
    emitLog(7800, errors ? 'warning' : 'info', '刷课任务结束：' + selectedCourses.length + ' 门课程，' + errors + ' 个错误');
    later(function () {
      if (token !== runToken) return;
      runState = { active: false, run_id: '', started_at: '', course_ids: [] };
      clearRunTimers();
      emit('run:finished', {
        run_id: runId,
        ok: true,
        reason: 'completed',
        duration_s: Math.max(1, Math.round((Date.now() - started.getTime()) / 1000)),
        summary: { courses: selectedCourses.length, errors: errors },
        message: ''
      });
    }, 8300);

    return delay(180, { run_id: runId, started_at: fullTs(started) });
  }

  function stopMockRun() {
    if (!runState.active) return reject(120, 'not_found: 没有正在运行的任务');
    var runId = runState.run_id;
    runState.active = false;
    clearRunTimers();
    emit('run:stopping', { run_id: runId });
    later(function () {
      emit('run:finished', {
        run_id: runId,
        ok: true,
        reason: 'stopped',
        duration_s: 6,
        summary: { courses: 1, errors: 0 },
        message: ''
      });
    }, 900);
    return delay(150, { stopping: true });
  }

  /* ---------- window.go ---------- */
  var App = {
    GetStatus: function () { return delay(260, buildStatus()); },

    ListAccounts: function () { return delay(180, accounts); },

    AddAccount: function (username, password, name) {
      return new Promise(function (resolve, rejectFn) {
        setTimeout(function () {
          if (!username || !password) { rejectFn(new Error('bad_request: 手机号和密码不能为空')); return; }
          if (accounts.some(function (a) { return a.id === username; })) {
            rejectFn(new Error('bad_request: 这个账号已经添加过了'));
            return;
          }
          var account = {
            id: username,
            name: name || '',
            username_masked: maskPhone(username),
            last_used: '',
            has_password: true
          };
          accounts.push(account);
          resolve(clone(account));
        }, 520);
      });
    },

    RemoveAccount: function (id) {
      return new Promise(function (resolve, rejectFn) {
        setTimeout(function () {
          var idx = -1;
          for (var i = 0; i < accounts.length; i++) if (accounts[i].id === id) idx = i;
          if (idx < 0) { rejectFn(new Error('not_found: 账号不存在')); return; }
          accounts.splice(idx, 1);
          resolve(null);
        }, 380);
      });
    },

    GetSettings: function () { return delay(180, settings); },

    SaveSettings: function (partial) {
      return new Promise(function (resolve, rejectFn) {
        setTimeout(function () {
          var next = clone(settings);
          var src = partial || {};
          if (src.speed != null) {
            var speed = Number(src.speed);
            if (!(speed >= 1 && speed <= 2)) { rejectFn(new Error('bad_request: 播放速度需要在 1.0–2.0 之间')); return; }
            next.speed = Math.round(speed * 10) / 10;
          }
          if (src.jobs != null) {
            var jobs = Math.round(Number(src.jobs));
            if (!(jobs >= 1 && jobs <= 8)) { rejectFn(new Error('bad_request: 并发任务需要在 1–8 之间')); return; }
            next.jobs = jobs;
          }
          if (src.notopen_action != null) {
            if (src.notopen_action !== 'retry' && src.notopen_action !== 'continue') {
              rejectFn(new Error('bad_request: 未开放任务处理方式不合法'));
              return;
            }
            next.notopen_action = src.notopen_action;
          }
          if (src.retry_interval != null) next.retry_interval = Math.max(0, Number(src.retry_interval) || 0);
          if (src.target_count != null) {
            var target = Math.round(Number(src.target_count));
            if (!(target >= 1 && target <= 1000)) { rejectFn(new Error('bad_request: 目标完成数需要在 1–1000 之间')); return; }
            next.target_count = target;
          }
          if (src.work_max_retries != null) {
            var retries = Math.round(Number(src.work_max_retries));
            if (!(retries >= 1 && retries <= 10)) { rejectFn(new Error('bad_request: 作业重试次数需要在 1–10 之间')); return; }
            next.work_max_retries = retries;
          }
          if (src.serial_video != null) next.serial_video = !!src.serial_video;
          if (src.add_learning_count != null) next.add_learning_count = !!src.add_learning_count;
          if (src.work_redo_enabled != null) next.work_redo_enabled = !!src.work_redo_enabled;
          next.task_center_submit_mode = 'auto';
          settings = next;
          resolve(clone(settings));
        }, 420);
      });
    },

    ListCourses: function (accountID) {
      return new Promise(function (resolve, rejectFn) {
        setTimeout(function () {
          if (scenario === 'courses-error' && accountID === accounts[0].id) {
            rejectFn(new Error('network_error: 网络连接失败，请检查网络后重试'));
            return;
          }
          if (accountID === '13700003333') {
            resolve({ realname: '', courses: [] });
            return;
          }
          resolve({ realname: '林晓', courses: clone(courses) });
        }, 900);
      });
    },

    StartRun: function (opts) { return startMockRun(opts); },

    StopRun: function () { return stopMockRun(); },

    GetRunState: function () {
      return delay(120, {
        active: runState.active,
        run_id: runState.run_id,
        started_at: runState.started_at,
        course_ids: runState.course_ids
      });
    },

    ListReviews: function (days) {
      return delay(380, {
        markdown_path: '/Users/demo/.chaoxing/reviews/' + ymd(new Date()) + '.md',
        items: filteredReviews(Number(days) || 0).map(function (r) {
          return {
            submission_id: r.submission_id,
            ts: r.ts,
            day: r.day,
            kind: r.kind,
            course: r.course,
            task: r.task,
            status: r.status,
            chars: r.chars,
            text_preview: r.text_preview
          };
        })
      });
    },

    GetReview: function (submissionID) {
      return new Promise(function (resolve, rejectFn) {
        setTimeout(function () {
          var found = null;
          for (var i = 0; i < reviews.length; i++) if (reviews[i].submission_id === submissionID) found = reviews[i];
          if (!found) { rejectFn(new Error('not_found: 复核条目不存在')); return; }
          resolve(clone(found));
        }, 280);
      });
    },

    ChooseEngineFolder: function () {
      return new Promise(function (resolve) {
        setTimeout(function () { resolve('/Users/demo/code/chaoxing'); }, 400);
      });
    },

    SetEnginePath: function (repoPath) {
      return new Promise(function (resolve, rejectFn) {
        setTimeout(function () {
          if (!repoPath) { rejectFn(new Error('bad_request: 目录不能为空')); return; }
          engineState.repo = repoPath;
          engineReady = true;
          bridgeState = 'ready';
          resolve(engineStatus());
        }, 500);
      });
    },

    GetEngineStatus: function () {
      return new Promise(function (resolve) {
        setTimeout(function () { resolve(clone(engineStatus())); }, 220);
      });
    },

    OpenDataDir: function () { return delay(120, null); },
    OpenConfigFile: function () { return delay(120, null); },

    GetAppInfo: function () {
      return delay(120, { version: '0.1.0', protocol: 1, license: 'GPL-3.0' });
    }
  };

  window.go = { main: { App: App } };

  /* ---------- 场景驱动（截图/演示用） ---------- */
  function waitFor(selector, timeout) {
    return new Promise(function (resolve) {
      var started = Date.now();
      (function poll() {
        var el = document.querySelector(selector);
        if (el) { resolve(el); return; }
        if (Date.now() - started > (timeout || 8000)) { resolve(null); return; }
        setTimeout(poll, 80);
      })();
    });
  }

  async function driveScenario() {
    if (!scenario) return;
    var navSel = function (page) { return '[data-action="nav"][data-page="' + page + '"]'; };
    await waitFor(navSel('run'), 9000);
    if (!document.querySelector(navSel('run'))) return;

    if (scenario === 'review' || scenario === 'review-empty') {
      document.querySelector(navSel('review')).click();
      await waitFor('#review-items .row, #review-items .empty-state', 7000);
      return;
    }
    if (scenario === 'run') {
      document.querySelector(navSel('run')).click();
      await waitFor('.row[data-action="select-account"]', 8000);
      return;
    }
    if (scenario === 'run-ready') {
      document.querySelector(navSel('run')).click();
      var acc = await waitFor('.row[data-action="select-account"]', 8000);
      if (acc) acc.click();
      await waitFor('.row[data-action="toggle-course"]', 9000);
      var selectAll = document.querySelector('[data-action="toggle-all-courses"]');
      if (selectAll) selectAll.click();
      return;
    }
    if (scenario === 'modal-add') {
      document.querySelector(navSel('settings')).click();
      await waitFor('[data-action="add-account"]', 8000);
      var addBtn = document.querySelector('[data-action="add-account"]');
      if (addBtn) addBtn.click();
      await waitFor('.modal', 4000);
      return;
    }
    if (scenario === 'settings') {
      document.querySelector(navSel('settings')).click();
      await waitFor('.settings-layout', 7000);
      return;
    }
    if (scenario === 'result') {
      document.querySelector(navSel('run')).click();
      await new Promise(function (r) { setTimeout(r, 700); });
      var app = window.__APP;
      if (app) {
        app.state.run.phase = 'finished';
        app.state.run.finished = {
          run_id: 'r-demo', ok: true, reason: 'completed', duration_s: 312,
          summary: { courses: 2, errors: 0 }, message: ''
        };
        app.state.run.elapsed = 312;
        app.state.run.logLines = [
          { level: 'info', text: '已登录：林晓', ts: '2026-11-04 21:00:01' },
          { level: 'info', text: '共加载 2 门课程', ts: '2026-11-04 21:00:02' },
          { level: 'info', text: '本轮已处理 12 个任务点，0 个错误', ts: '2026-11-04 21:05:12' }
        ];
        app.render();
      }
      return;
    }
    if (scenario === 'running' || scenario === 'finished' || scenario === 'courses-error') {
      document.querySelector(navSel('run')).click();
      var chip = await waitFor('.row[data-action="select-account"]', 8000);
      if (chip) chip.click();
      var selectAll = await waitFor('[data-action="toggle-all-courses"]:not([disabled])', 9000);
      if (selectAll) selectAll.click();
      var startBtn = await waitFor('#start-run:not([disabled])', 9000);
      if (startBtn) startBtn.click();
      var confirmBtn = await waitFor('[data-role="confirm"]', 4000);
      if (confirmBtn) confirmBtn.click();
      if (scenario === 'finished') {
        await waitFor('[data-action="run-again"]', 16000);
      } else {
        await waitFor('.log-row', 6000);
      }
    }
  }

  window.addEventListener('load', function () {
    setTimeout(function () {
      driveScenario().catch(function (e) {
        setTimeout(function () { throw e; }, 0);
      });
    }, 80);
  });
})();
