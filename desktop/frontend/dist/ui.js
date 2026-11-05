/* 超星助手 · 通用 UI 基础（图标 / Toast / 弹层 / 格式化）
   设计语言与雨课助手一致：图标为内联 SVG sprite，class 对齐 ykt app.css。 */
(function () {
  'use strict';

  var SPRITE = [
    '<svg class="icon-sprite" aria-hidden="true" focusable="false" xmlns="http://www.w3.org/2000/svg">',
    '<symbol id="i-grid" viewBox="0 0 24 24"><rect x="3.5" y="3.5" width="7.2" height="7.2" rx="1.7"/><rect x="13.3" y="3.5" width="7.2" height="7.2" rx="1.7"/><rect x="3.5" y="13.3" width="7.2" height="7.2" rx="1.7"/><rect x="13.3" y="13.3" width="7.2" height="7.2" rx="1.7"/></symbol>',
    '<symbol id="i-play" viewBox="0 0 24 24"><path d="M8.4 5.8a.9.9 0 0 1 1.37-.77l9.1 6.2a.9.9 0 0 1 0 1.54l-9.1 6.2a.9.9 0 0 1-1.37-.77V5.8Z" fill="currentColor" stroke="none"/></symbol>',
    '<symbol id="i-play-line" viewBox="0 0 24 24"><path d="M8.4 5.8a.9.9 0 0 1 1.37-.77l9.1 6.2a.9.9 0 0 1 0 1.54l-9.1 6.2a.9.9 0 0 1-1.37-.77V5.8Z"/></symbol>',
    '<symbol id="i-stop" viewBox="0 0 24 24"><rect x="6.6" y="6.6" width="10.8" height="10.8" rx="2" fill="currentColor" stroke="none"/></symbol>',
    '<symbol id="i-file" viewBox="0 0 24 24"><path d="M6.2 3.8h7.4l5 5.2v10.2a1.7 1.7 0 0 1-1.7 1.7H6.2a1.7 1.7 0 0 1-1.7-1.7V5.5a1.7 1.7 0 0 1 1.7-1.7Z"/><polyline points="13.4 3.8 13.4 9.2 18.8 9.2"/><line x1="8.4" y1="12.9" x2="15.4" y2="12.9"/><line x1="8.4" y1="16.4" x2="13.2" y2="16.4"/></symbol>',
    '<symbol id="i-gear" viewBox="0 0 24 24"><circle cx="12" cy="12" r="3.1"/><path d="M19.3 14.9a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.7 1.7 0 0 0-1.87-.34 1.7 1.7 0 0 0-1.03 1.56V21a2 2 0 1 1-4 0v-.09a1.7 1.7 0 0 0-1.11-1.56 1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.7 1.7 0 0 0 .34-1.87 1.7 1.7 0 0 0-1.56-1.03H3a2 2 0 1 1 0-4h.09a1.7 1.7 0 0 0 1.56-1.11 1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.7 1.7 0 0 0 1.87.34h.09a1.7 1.7 0 0 0 1.03-1.56V3a2 2 0 1 1 4 0v.09a1.7 1.7 0 0 0 1.03 1.56 1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.7 1.7 0 0 0-.34 1.87v.09a1.7 1.7 0 0 0 1.56 1.03H21a2 2 0 1 1 0 4h-.09a1.7 1.7 0 0 0-1.56 1.03Z"/></symbol>',
    '<symbol id="i-list" viewBox="0 0 24 24"><line x1="8.5" y1="6.5" x2="20" y2="6.5"/><line x1="8.5" y1="12" x2="20" y2="12"/><line x1="8.5" y1="17.5" x2="20" y2="17.5"/><circle cx="4.6" cy="6.5" r="1.05" fill="currentColor" stroke="none"/><circle cx="4.6" cy="12" r="1.05" fill="currentColor" stroke="none"/><circle cx="4.6" cy="17.5" r="1.05" fill="currentColor" stroke="none"/></symbol>',
    '<symbol id="i-server" viewBox="0 0 24 24"><rect x="3.5" y="4.3" width="17" height="6.1" rx="2"/><rect x="3.5" y="13.6" width="17" height="6.1" rx="2"/><circle cx="7.6" cy="7.35" r=".95" fill="currentColor" stroke="none"/><circle cx="7.6" cy="16.65" r=".95" fill="currentColor" stroke="none"/><line x1="12" y1="7.35" x2="16.6" y2="7.35"/><line x1="12" y1="16.65" x2="16.6" y2="16.65"/></symbol>',
    '<symbol id="i-clock" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.4"/><polyline points="12 7.4 12 12.2 15.4 14"/></symbol>',
    '<symbol id="i-user" viewBox="0 0 24 24"><circle cx="12" cy="8.2" r="3.4"/><path d="M5.4 19.2c.6-3.4 3.2-5.2 6.6-5.2s6 1.8 6.6 5.2"/></symbol>',
    '<symbol id="i-check" viewBox="0 0 24 24"><polyline points="4.8 12.4 9.6 17.2 19.2 6.8"/></symbol>',
    '<symbol id="i-check-circle" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.4"/><polyline points="8.2 12.3 11 15.1 15.9 9.6"/></symbol>',
    '<symbol id="i-alert" viewBox="0 0 24 24"><path d="M10.3 4.2a1.96 1.96 0 0 1 3.4 0l7.3 12.7a1.96 1.96 0 0 1-1.7 2.94H4.7A1.96 1.96 0 0 1 3 16.9l7.3-12.7Z"/><line x1="12" y1="9.2" x2="12" y2="13.4"/><circle cx="12" cy="16.5" r=".95" fill="currentColor" stroke="none"/></symbol>',
    '<symbol id="i-info" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8.4"/><line x1="12" y1="11" x2="12" y2="16.2"/><circle cx="12" cy="8.1" r=".95" fill="currentColor" stroke="none"/></symbol>',
    '<symbol id="i-x" viewBox="0 0 24 24"><line x1="5.6" y1="5.6" x2="18.4" y2="18.4"/><line x1="18.4" y1="5.6" x2="5.6" y2="18.4"/></symbol>',
    '<symbol id="i-eye" viewBox="0 0 24 24"><path d="M2.6 12S6.1 5.7 12 5.7 21.4 12 21.4 12 17.9 18.3 12 18.3 2.6 12 2.6 12Z"/><circle cx="12" cy="12" r="3.1"/></symbol>',
    '<symbol id="i-eye-off" viewBox="0 0 24 24"><path d="M4.3 7.3C3.1 8.7 2.6 10.4 2.6 12c0 0 3.5 6.3 9.4 6.3 1.6 0 3-.5 4.2-1.2"/><path d="M9.5 6.1c.8-.2 1.6-.4 2.5-.4 5.9 0 9.4 6.3 9.4 6.3s-1.1 2-3.1 3.7"/><path d="M9.9 9.9a3.1 3.1 0 0 0 4.3 4.3"/><line x1="4.3" y1="4.3" x2="19.7" y2="19.7"/></symbol>',
    '<symbol id="i-folder" viewBox="0 0 24 24"><path d="M3.6 7.4a1.9 1.9 0 0 1 1.9-1.9h3.1l2 2.5h7.9a1.9 1.9 0 0 1 1.9 1.9v7.7a1.9 1.9 0 0 1-1.9 1.9H5.5a1.9 1.9 0 0 1-1.9-1.9V7.4Z"/></symbol>',
    '<symbol id="i-trash" viewBox="0 0 24 24"><line x1="4.4" y1="6.9" x2="19.6" y2="6.9"/><path d="M9 6.9V4.6h6v2.3"/><path d="M6.4 6.9l.85 11.9a1.7 1.7 0 0 0 1.7 1.6h6.1a1.7 1.7 0 0 0 1.7-1.6l.85-11.9"/><line x1="10.1" y1="10.9" x2="10.1" y2="16.3"/><line x1="13.9" y1="10.9" x2="13.9" y2="16.3"/></symbol>',
    '<symbol id="i-plus" viewBox="0 0 24 24"><line x1="12" y1="5.4" x2="12" y2="18.6"/><line x1="5.4" y1="12" x2="18.6" y2="12"/></symbol>',
    '<symbol id="i-refresh" viewBox="0 0 24 24"><path d="M20.2 12.3a8.2 8.2 0 1 1-2.3-6.05"/><polyline points="20.1 4.3 20.1 11.2 13.4 11.2"/></symbol>',
    '<symbol id="i-external" viewBox="0 0 24 24"><path d="M13.8 4.6h5.6v5.6"/><line x1="19.4" y1="4.6" x2="11.4" y2="12.6"/><path d="M19.4 14.2v3.9a1.9 1.9 0 0 1-1.9 1.9H5.9A1.9 1.9 0 0 1 4 18.1V6.5a1.9 1.9 0 0 1 1.9-1.9h3.9"/></symbol>',
    '<symbol id="i-copy" viewBox="0 0 24 24"><rect x="8.8" y="8.8" width="11.2" height="11.2" rx="2.1"/><path d="M5.6 15.2H5a1.9 1.9 0 0 1-1.9-1.9v-8A1.9 1.9 0 0 1 5 3.4h8a1.9 1.9 0 0 1 1.9 1.9v.6"/></symbol>',
    '<symbol id="i-key" viewBox="0 0 24 24"><circle cx="15.4" cy="8.6" r="4.1"/><path d="M12.2 11.8 4 20"/><path d="M6.5 17.5 8.6 19.6"/><path d="M4 20l1.7 1.7"/></symbol>',
    '<symbol id="i-book" viewBox="0 0 24 24"><path d="M4.5 5.6A1.6 1.6 0 0 1 6.1 4h4.4A1.5 1.5 0 0 1 12 5.5v13.8a2 2 0 0 0-2-1.8H4.5V5.6Z"/><path d="M19.5 5.6A1.6 1.6 0 0 0 17.9 4h-4.4A1.5 1.5 0 0 0 12 5.5v13.8a2 2 0 0 1 2-1.8h5.5V5.6Z"/></symbol>',
    '<symbol id="i-activity" viewBox="0 0 24 24"><polyline points="3.4 12.6 7.6 12.6 10.1 6 13.9 18 16.4 12.6 20.6 12.6"/></symbol>',
    '<symbol id="i-sparkle" viewBox="0 0 24 24"><path d="M11 3.6c.34 3.05 1.86 4.57 4.9 4.9-3.04.34-4.56 1.86-4.9 4.9-.34-3.04-1.86-4.56-4.9-4.9 3.04-.33 4.56-1.85 4.9-4.9Z"/><path d="M17.6 13.9c.2 1.9.97 2.67 2.9 2.9-1.93.2-2.7.97-2.9 2.9-.2-1.93-.97-2.7-2.9-2.9 1.93-.23 2.7-1 2.9-2.9Z"/></symbol>',
    '<symbol id="i-terminal" viewBox="0 0 24 24"><rect x="3.5" y="4.8" width="17" height="14.4" rx="2"/><polyline points="7.4 9.5 10.3 12 7.4 14.5"/><line x1="12.6" y1="14.8" x2="16.8" y2="14.8"/></symbol>',
    '<symbol id="i-chevron-right" viewBox="0 0 24 24"><polyline points="9.4 5.8 15.7 12 9.4 18.2"/></symbol>',
    '</svg>'
  ].join('');

  function mountSprite() {
    var host = document.createElement('div');
    host.innerHTML = SPRITE;
    document.body.insertBefore(host.firstElementChild, document.body.firstChild);
  }

  var FILLED = { play: 1, stop: 1, sparkle: 1 };

  function icon(name) {
    var cls = 'icon' + (FILLED[name] ? ' icon-fill' : '');
    return '<svg class="' + cls + '" aria-hidden="true" focusable="false"><use href="#i-' + name + '"></use></svg>';
  }

  function esc(value) {
    if (value === null || value === undefined) return '';
    return String(value)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function pad2(n) { return (n < 10 ? '0' : '') + n; }

  function parseDateTime(text) {
    if (!text) return null;
    var d = new Date(String(text).replace(' ', 'T'));
    return isNaN(d.getTime()) ? null : d;
  }

  function isSameDay(a, b) {
    return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
  }

  function fmtClock(ts) {
    var d = parseDateTime(ts);
    if (!d) return ts || '';
    var now = new Date();
    if (isSameDay(d, now)) return pad2(d.getHours()) + ':' + pad2(d.getMinutes());
    return pad2(d.getMonth() + 1) + '-' + pad2(d.getDate()) + ' ' + pad2(d.getHours()) + ':' + pad2(d.getMinutes());
  }

  // 中文时间：今天 08:42 / 昨天 21:10 / 11月3日 21:10 / 2025年11月3日
  function fmtClockCn(ts) {
    var d = parseDateTime(ts);
    if (!d) return ts || '';
    var now = new Date();
    var hm = pad2(d.getHours()) + ':' + pad2(d.getMinutes());
    if (isSameDay(d, now)) return '今天 ' + hm;
    if (isSameDay(d, new Date(now.getTime() - 86400000))) return '昨天 ' + hm;
    if (d.getFullYear() === now.getFullYear()) return (d.getMonth() + 1) + '月' + d.getDate() + '日 ' + hm;
    return d.getFullYear() + '年' + (d.getMonth() + 1) + '月' + d.getDate() + '日';
  }

  // 相对时间：刚刚 / 8 分钟前 / 3 小时前 / 44 天前
  function fmtRelative(ts) {
    var d = parseDateTime(ts);
    if (!d) return '';
    var diff = Math.max(0, Date.now() - d.getTime());
    var min = Math.floor(diff / 60000);
    if (min < 1) return '刚刚';
    if (min < 60) return min + ' 分钟前';
    var hours = Math.floor(min / 60);
    if (hours < 24) return hours + ' 小时前';
    var days = Math.floor(hours / 24);
    if (days < 30) return days + ' 天前';
    var months = Math.floor(days / 30);
    if (months < 12) return months + ' 个月前';
    return Math.floor(months / 12) + ' 年前';
  }

  function fmtTimeOfDay(ts) {
    var d = parseDateTime(ts);
    if (!d) return '';
    return pad2(d.getHours()) + ':' + pad2(d.getMinutes()) + ':' + pad2(d.getSeconds());
  }

  function fmtDuration(seconds) {
    var s = Math.max(0, Math.floor(Number(seconds) || 0));
    var h = Math.floor(s / 3600);
    var m = Math.floor((s % 3600) / 60);
    var sec = s % 60;
    if (h > 0) return h + ':' + pad2(m) + ':' + pad2(sec);
    return pad2(m) + ':' + pad2(sec);
  }

  function fmtDurationCn(seconds) {
    var s = Math.max(0, Math.floor(Number(seconds) || 0));
    var h = Math.floor(s / 3600);
    var m = Math.floor((s % 3600) / 60);
    var sec = s % 60;
    if (h > 0) return h + ' 小时 ' + pad2(m) + ' 分';
    if (m > 0) return m + ' 分 ' + pad2(sec) + ' 秒';
    return sec + ' 秒';
  }

  function todayCn() {
    var d = new Date();
    var week = ['日', '一', '二', '三', '四', '五', '六'][d.getDay()];
    return (d.getMonth() + 1) + ' 月 ' + d.getDate() + ' 日 · 周' + week;
  }

  function debounce(fn, wait) {
    var t = null;
    return function () {
      var args = arguments, self = this;
      clearTimeout(t);
      t = setTimeout(function () { fn.apply(self, args); }, wait);
    };
  }

  function clamp(n, min, max) { return Math.min(max, Math.max(min, n)); }

  /* ---------- Toast ---------- */
  var TOAST_ICON = { success: 'check-circle', error: 'alert', warning: 'alert', info: 'info' };

  function toast(message, tone) {
    var root = document.getElementById('toast-root');
    if (!root) return;
    var kind = tone === 'error' ? 'error' : tone === 'warn' || tone === 'warning' ? 'warning' : tone === 'info' ? 'info' : 'success';
    var el = document.createElement('div');
    el.className = 'toast is-' + kind;
    el.innerHTML = '<span class="toast-icon">' + icon(TOAST_ICON[kind]) + '</span>' +
      '<span class="toast-text">' + esc(message) + '</span>';
    root.appendChild(el);
    while (root.children.length > 3) root.removeChild(root.firstChild);
    setTimeout(function () {
      el.classList.add('is-leaving');
      setTimeout(function () { if (el.parentNode) el.parentNode.removeChild(el); }, 220);
    }, 3000);
  }

  /* ---------- 复制 ---------- */
  function copyText(text) {
    var value = String(text == null ? '' : text);
    return new Promise(function (resolve) {
      if (!value) { resolve(false); return; }
      var fallback = function () {
        try {
          var ta = document.createElement('textarea');
          ta.value = value;
          ta.setAttribute('readonly', 'readonly');
          ta.style.position = 'fixed';
          ta.style.top = '-1000px';
          document.body.appendChild(ta);
          ta.select();
          var ok = document.execCommand('copy');
          document.body.removeChild(ta);
          resolve(!!ok);
        } catch (e) { resolve(false); }
      };
      if (window.runtime && typeof window.runtime.ClipboardSetText === 'function') {
        try {
          Promise.resolve(window.runtime.ClipboardSetText(value)).then(function () { resolve(true); }, fallback);
        } catch (e) { fallback(); }
        return;
      }
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(value).then(function () { resolve(true); }, fallback);
        return;
      }
      fallback();
    });
  }

  /* ---------- 弹层 ---------- */
  var modalStack = [];
  var modalSeq = 0;

  function focusables(root) {
    return Array.prototype.slice.call(root.querySelectorAll(
      'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
    )).filter(function (el) { return el.offsetParent !== null || el === document.activeElement; });
  }

  function onDocKeydown(e) {
    if (!modalStack.length) return;
    var top = modalStack[modalStack.length - 1];
    if (e.key === 'Escape') {
      e.preventDefault();
      if (!top.locked) top.close(top.escapeResult);
      return;
    }
    if (e.key === 'Tab') {
      var items = focusables(top.el);
      if (!items.length) { e.preventDefault(); return; }
      var first = items[0], last = items[items.length - 1];
      if (e.shiftKey && (document.activeElement === first || !top.el.contains(document.activeElement))) {
        e.preventDefault(); last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault(); first.focus();
      }
    }
  }
  document.addEventListener('keydown', onDocKeydown, true);

  function openModal(opts) {
    opts = opts || {};
    var root = document.getElementById('modal-root');
    var id = 'modal-title-' + (++modalSeq);
    var tone = opts.tone || 'danger';
    var layer = document.createElement('div');
    layer.className = 'modal-layer';
    layer.innerHTML =
      '<div class="modal-backdrop" data-role="backdrop"></div>' +
      '<div class="modal' + (opts.wide ? ' is-form' : '') + '" role="dialog" aria-modal="true" aria-labelledby="' + id + '">' +
      (opts.tone === 'none' ? '' :
        '<div class="modal-icon' + (tone === 'info' ? ' is-info' : '') + '">' + icon(tone === 'info' ? 'info' : 'alert') + '</div>') +
      '<div class="modal-copy"><h2 class="modal-title" id="' + id + '">' + esc(opts.title || '') + '</h2>' +
      (opts.subtitle ? '<p class="modal-desc">' + esc(opts.subtitle) + '</p>' : '') + '</div>' +
      (opts.body ? '<div class="modal-body">' + opts.body + '</div>' : '') +
      (opts.footer ? '<div class="modal-actions">' + opts.footer + '</div>' : '') +
      '</div>';
    root.appendChild(layer);

    var previousFocus = document.activeElement;
    var api = {
      el: layer,
      modal: layer.querySelector('.modal'),
      body: layer.querySelector('.modal-body'),
      locked: false,
      escapeResult: opts.escapeResult,
      close: function (result) {
        if (layer.dataset.closed) return;
        layer.dataset.closed = '1';
        var idx = modalStack.indexOf(api);
        if (idx >= 0) modalStack.splice(idx, 1);
        if (layer.parentNode) layer.parentNode.removeChild(layer);
        if (!modalStack.length && previousFocus && typeof previousFocus.focus === 'function') {
          try { previousFocus.focus(); } catch (e) {}
        }
        if (typeof opts.onClose === 'function') opts.onClose(result);
      }
    };
    layer.querySelector('[data-role="backdrop"]').addEventListener('mousedown', function () {
      if (!api.locked) api.close(opts.escapeResult);
    });
    modalStack.push(api);
    if (typeof opts.onMount === 'function') opts.onMount(api);
    var focusTarget = layer.querySelector('[data-autofocus]') || focusables(layer)[0];
    if (focusTarget) setTimeout(function () { try { focusTarget.focus(); } catch (e) {} }, 30);
    return api;
  }

  function confirmDialog(opts) {
    opts = opts || {};
    return new Promise(function (resolve) {
      var settled = false;
      function finish(ok) {
        if (settled) return;
        settled = true;
        resolve(ok);
        api.close(ok);
      }
      var footer =
        '<button type="button" class="btn" data-role="cancel">' + esc(opts.cancelLabel || '取消') + '</button>' +
        '<button type="button" class="btn ' + (opts.danger ? 'btn-danger' : 'btn-primary') + '" data-role="confirm">' +
        esc(opts.confirmLabel || '确定') + '</button>';
      var api = openModal({
        title: opts.title || '确认操作',
        subtitle: opts.message || '',
        body: opts.body || '',
        footer: footer,
        tone: opts.danger ? 'danger' : 'info',
        escapeResult: false,
        onMount: function (m) {
          m.el.querySelector('[data-role="cancel"]').addEventListener('click', function () { finish(false); });
          m.el.querySelector('[data-role="confirm"]').addEventListener('click', function () { finish(true); });
          m.el.querySelector('[data-role="confirm"]').focus();
        }
      });
    });
  }

  window.UI = {
    icon: icon,
    esc: esc,
    mountSprite: mountSprite,
    toast: toast,
    copyText: copyText,
    openModal: openModal,
    confirm: confirmDialog,
    fmtClock: fmtClock,
    fmtClockCn: fmtClockCn,
    fmtRelative: fmtRelative,
    fmtTimeOfDay: fmtTimeOfDay,
    fmtDuration: fmtDuration,
    fmtDurationCn: fmtDurationCn,
    todayCn: todayCn,
    parseDateTime: parseDateTime,
    debounce: debounce,
    clamp: clamp
  };
})();
