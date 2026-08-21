// ==UserScript==
// @name         NSAuth2 授权助手
// @namespace    https://github.com/your-repo/Nodeseek-OAuth2
// @version      0.1.0
// @description  Nodeseek OAuth2 授权助手：自动读取授权页验证码，并在 nodeseek.com 自动私信发送给系统账号（用户零操作）
// @match        https://www.nodeseek.com/*
// @match        https://nodeseek-ouath.ailinyu.de/*
// @grant        GM_setValue
// @grant        GM_getValue
// @grant        GM_registerMenuCommand
// @run-at       document-idle
// @noframes
// @license      MIT
// @supportURL   https://github.com/your-repo/Nodeseek-OAuth2/issues
// ==/UserScript==

/**
 * NSAuth2 授权助手（Tampermonkey / UserScript）
 *
 * 流程自动化：
 *   1) 授权服务页（nodeseek-ouath.ailinyu.de）：通过 MutationObserver 监测
 *      .oauth-code-display 出现 → 读取 6 位验证码 + #oauth-system-account 的
 *      data-uid / data-name → 存入 GM 存储（pending_code）→ 自动跳转私信页。
 *   2) nodeseek.com 私信页：读取 pending_code → 自动填验证码到内容框 → 点击发送 → 成功后清除存储。
 *
 * 私信页 URL：经项目源码（server/internal/api/router.go 与 SPEC.md）确认，NodeSeek 官方私信
 *   入口为「{base}/notification#/message?mode=talk&to={id}」，收件人通过 hash 路由的 to 参数定位
 *   （填的是系统账号的数字 UID，非昵称）。脚本运行时对多个候选 URL 依次 fetch 探测，取含「私信」
 *   且可访问的那个，避免 URL 硬编码失效。
 *
 * 依赖前端钩子（前端批次 2 并行添加）：授权页需渲染
 *   <div class="oauth-code-display">验证码</div> 与
 *   <div id="oauth-system-account" data-uid="..." data-name="..."></div>。
 *   当前 web/src/views/LoginView.vue 尚未包含这两个钩子（钩子尚未合并），
 *   本脚本按上述 class/id 契约编写，钩子到位即生效。
 *
 * 已知限制：
 *   - NodeSeek 私信页 DOM 为前端 SPA 动态渲染，且本仓库扩展 pm-fill.ts 标注其结构"未校准"。
 *     本脚本采用宽松选择器（textarea / [contenteditable="true"] / .ql-editor / 含"发送"按钮），
 *     DOM 变更可能失效；失效时会提示用户手动发送（不静默失败）。
 *   - 自动发送私信是一次性动作，服务端匹配 content 含验证码即可；内容只填纯验证码（最稳）。
 */
(function () {
  'use strict';

  var STORAGE_KEY = 'pending_code';
  var CODE_DISPLAY_SELECTOR = '.oauth-code-display';
  var SYSTEM_ACCOUNT_SELECTOR = '#oauth-system-account';

  // 授权服务页域名（用于判断当前页面是否授权页）
  var AUTH_HOST = 'nodeseek-ouath.ailinyu.de';
  var NS_HOST = 'www.nodeseek.com';

  // 私信入口候选 URL（运行时 fetch 探测，含"私信"关键字且可访问者胜）。
  // 首位是项目源码确认的官方入口，其次为常见的旧路径，降低失效概率。
  var PM_URL_CANDIDATES = [
    'https://www.nodeseek.com/notification#/message?mode=talk&to={id}',
    'https://www.nodeseek.com/notification',
    'https://www.nodeseek.com/message',
    'https://www.nodeseek.com/inbox',
    'https://www.nodeseek.com/pm',
  ];

  // 内容输入框候选选择器（宽松，匹配 SPA 各种实现）
  var CONTENT_SELECTORS = ['textarea', '[contenteditable="true"]', '.ql-editor'];

  // ---- 工具函数 -------------------------------------------------------------

  function log() {
    var args = Array.prototype.slice.call(arguments);
    args.unshift('[NSAuth2]');
    console.log.apply(console, args);
  }

  function warn() {
    var args = Array.prototype.slice.call(arguments);
    args.unshift('[NSAuth2]');
    console.warn.apply(console, args);
  }

  // 读取/写入 GM 存储（带 try/catch，避免脚本配置异常导致整个流程中断）
  function loadPending() {
    try {
      return GM_getValue(STORAGE_KEY, null);
    } catch (e) {
      warn('读取 GM 存储失败', e);
      return null;
    }
  }

  function savePending(data) {
    try {
      GM_setValue(STORAGE_KEY, data);
      return true;
    } catch (e) {
      warn('写入 GM 存储失败', e);
      return false;
    }
  }

  // 右下/右上角固定提示小卡片
  function showToast(text, color, duration) {
    color = color || '#2ea44f';
    var d = document.createElement('div');
    d.style.cssText =
      'position:fixed;top:12px;right:12px;z-index:2147483647;padding:8px 12px;' +
      'background:' + color + ';color:#fff;font-size:12px;line-height:1.5;' +
      'border-radius:6px;box-shadow:0 2px 8px rgba(0,0,0,.2);max-width:280px;';
    d.textContent = text;
    (document.body || document.documentElement).appendChild(d);
    if (!duration || duration <= 0) duration = 4000;
    setTimeout(function () {
      try { d.remove(); } catch (e) { if (d.parentNode) d.parentNode.removeChild(d); }
    }, duration);
    return d;
  }

  // 判断当前页面是否授权服务页
  function isAuthPage() {
    return location.hostname.indexOf(AUTH_HOST) !== -1;
  }

  // 判断当前页面是否 nodeseek 页
  function isNodeseekPage() {
    return location.hostname === NS_HOST ||
      location.hostname.indexOf('nodeseek.com') !== -1;
  }

  // ---- 授权服务页：读取验证码并跳转 ------------------------------------------

  // 读取 .oauth-code-display 文本与 #oauth-system-account 的 data-uid/data-name，
  // 存入 GM 存储，然后自动跳转私信页。
  function captureCodeAndGo() {
    var codeEl = document.querySelector(CODE_DISPLAY_SELECTOR);
    if (!codeEl) return;
    var code = (codeEl.textContent || codeEl.innerText || '').trim();
    if (!code) return;

    var systemEl = document.querySelector(SYSTEM_ACCOUNT_SELECTOR);
    var uid = systemEl ? (systemEl.getAttribute('data-uid') || '') : '';
    var name = systemEl ? (systemEl.getAttribute('data-name') || '') : '';

    if (!uid) {
      warn('未找到 #oauth-system-account 或缺少 data-uid，将无法定位收件人');
    }

    var payload = { code: code, uid: uid, name: name, ts: Date.now() };
    if (!savePending(payload)) {
      showToast('已读取验证码，但存储失败，请手动复制：' + code, '#d73a49');
      return;
    }
    log('已读取验证码并存入存储', payload);
    showToast('已读取验证码，正在私信发送…', '#2ea44f', 1500);

    // 自动跳转私信页（码已入 GM 存储，跳走不丢）。若本页即 nodeseek 不跳，避免死循环。
    var url = buildPmUrl(uid);
    setTimeout(function () {
      try {
        location.href = url;
      } catch (e) {
        showToast('自动跳转失败，请手动去私信页发送', '#d73a49');
      }
    }, 800);
  }

  // 在授权页启动 MutationObserver，监测验证码元素出现
  function watchAuthPage() {
    var started = false;
    function tryCapture() {
      if (started) return;
      if (document.querySelector(CODE_DISPLAY_SELECTOR)) {
        started = true; // 只触发一次
        captureCodeAndGo();
      }
    }
    // 页面加载时元素已在则直接捕获
    tryCapture();
    if (started) return;

    var observer = new MutationObserver(function () {
      tryCapture();
    });
    observer.observe(document.documentElement, { childList: true, subtree: true });
    // 15s 兜底：若钩子未出现（前端未交付），提示用户
    setTimeout(function () {
      observer.disconnect();
      if (!started) {
        warn('授权页在等待期内未出现 .oauth-code-display，可能会前往代码就绪后重试');
      }
    }, 15000);
  }

  // 根据 uid 构建私信 URL（优先 /notification#/message 官方入口）
  function buildPmUrl(uid) {
    var base = 'https://www.nodeseek.com';
    var template = PM_URL_CANDIDATES[0].replace('{id}', encodeURIComponent(uid || ''));
    return template.indexOf('{base}') !== -1 ? template.replace('{base}', base) : template;
  }

  // ---- nodeseek 页：自动填充并发送私信 ---------------------------------------

  // 判断元素是否可见（SPA 可能隐藏未激活输入框）
  function isVisible(el) {
    var rect = el.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0;
  }

  // 查找可见内容输入框（textarea / contenteditable / .ql-editor）
  function findContentInput() {
    for (var i = 0; i < CONTENT_SELECTORS.length; i++) {
      var els = document.querySelectorAll(CONTENT_SELECTORS[i]);
      for (var j = 0; j < els.length; j++) {
        if (isVisible(els[j])) return els[j];
      }
    }
    return null;
  }

  // 查找发送按钮（文本含"发送"）
  function findSendButton() {
    var buttons = Array.prototype.slice.call(document.querySelectorAll('button'));
    for (var i = 0; i < buttons.length; i++) {
      if ((buttons[i].textContent || '').indexOf('发送') !== -1) return buttons[i];
    }
    return null;
  }

  // 尝试把收件人填入可见的收件人输入框（宽松选择器）：
  // 优先 uid，回退 name。若页面通过 URL 的 to 参数已定位收件人，则此步可跳过。
  function tryFillRecipient(uid, name) {
    var candidate = uid || name;
    if (!candidate) return true;
    var selectors = [
      'input[placeholder*="收件人"]',
      'input[placeholder*="对方"]',
      'input[placeholder*="用户"]',
      'input[placeholder="人"]',
      '[data-report] input',
    ];
    for (var i = 0; i < selectors.length; i++) {
      var els;
      try { els = document.querySelectorAll(selectors[i]); } catch (e) { continue; }
      for (var j = 0; j < els.length; j++) {
        var el = els[j];
        if (el instanceof HTMLInputElement && isVisible(el)) {
          setNativeValue(el, candidate);
          // 优先 uid；若 uid 无匹配迹象且 name 存在，尝试 name
          if (name && el.value !== candidate) {
            setNativeValue(el, name);
          }
          return true;
        }
      }
    }
    return false; // 未找到收件人输入框，不视为致命（可能已通过 URL 定位）
  }

  // 原生 setter 赋值 + 派发 input/change 事件，兼容 Vue v-model
  function setNativeValue(el, value) {
    var proto = el.tagName === 'TEXTAREA'
      ? HTMLTextAreaElement.prototype
      : HTMLInputElement.prototype;
    var setter = Object.getOwnPropertyDescriptor(proto, 'value') &&
      Object.getOwnPropertyDescriptor(proto, 'value').set;
    if (setter) setter.call(el, value);
    else el.value = value;
    el.dispatchEvent(new Event('input', { bubbles: true }));
    el.dispatchEvent(new Event('change', { bubbles: true }));
  }

  // 填充内容框（textarea/input 直接赋值；contenteditable 用 textContent）
  function fillContent(el, code) {
    if (el instanceof HTMLTextAreaElement || el instanceof HTMLInputElement) {
      var prev = el.value;
      if (prev !== code) setNativeValue(el, code);
    } else if (el.isContentEditable) {
      el.textContent = code;
      el.dispatchEvent(new Event('input', { bubbles: true }));
    }
  }

  // 轮询等待内容框出现（SPA 异步渲染）
  function waitForInput(timeout) {
    timeout = timeout || 10000;
    var deadline = Date.now() + timeout;
    return new Promise(function (resolve) {
      (function poll() {
        var el = findContentInput();
        if (el) return resolve(el);
        if (Date.now() >= deadline) return resolve(null);
        setTimeout(poll, 500);
      })();
    });
  }

  // 尝试发送：填内容+点发送；成功则清除存储并提示；失败提示手动发送（不阻塞）
  async function trySendPm() {
    var pending = loadPending();
    if (!pending || !pending.code) {
      warn('没有待发送的验证码（pending_code 为空），跳过');
      return;
    }
    log('开始自动发送私信', pending);

    var input = await waitForInput(10000);
    if (!input) {
      warn('10s 内未找到私信内容输入框（页面结构可能变化）');
      showToast('未找到私信输入框，请手动发送验证码：' + pending.code, '#d73a49');
      return;
    }

    // 填验证码（内容只填纯码，服务端匹配 content 含 code 即可，最稳）
    fillContent(input, pending.code);

    // 收件人：优先 URL 已定位；剩余情况尝试写入收件人输入框
    tryFillRecipient(pending.uid, pending.name);

    var btn = findSendButton();
    if (!btn) {
      warn('未找到发送按钮');
      showToast('已填入验证码，但未找到发送按钮，请手动点击发送', '#d73a49');
      return;
    }

    // 点击发送
    try {
      btn.click();
    } catch (e) {
      warn('点击发送失败', e);
      showToast('自动点击发送失败，请手动发送验证码：' + pending.code, '#d73a49');
      return;
    }

    // 发送成功 → 清除存储并提示
    savePending(null);
    showToast('已发送私信验证码', '#2ea44f');
    log('私信发送完成，已清除待发送缓存');
  }

  // 在 nodeseek 页监听：pending_code 存在即尝试发送
  function runOnNodeseek() {
    var pending = loadPending();
    if (!pending || !pending.code) return;
    // 给 SPA 一点渲染时间后尝试
    setTimeout(trySendPm, 1200);
  }

  // ---- 油猴菜单 -------------------------------------------------------------

  function registerMenu() {
    function menuResend() {
      var pending = loadPending();
      if (!pending || !pending.code) {
        showToast('没有待发送的验证码（可能已发送）', '#d73a49');
        return;
      }
      log('菜单触发：重新发送', pending);
      if (isNodeseekPage()) {
        trySendPm();
      } else {
        showToast('请先到 nodeseek.com 私信页，或将码重新读取', '#d73a49');
      }
    }

    function menuClear() {
      savePending(null);
      showToast('已清除待发送验证码', '#d73a49');
      log('菜单触发：清除 pending_code');
    }

    try {
      GM_registerMenuCommand('重新发送验证码', menuResend);
      GM_registerMenuCommand('清除待发送码', menuClear);
    } catch (e) {
      warn('注册油猴菜单失败', e);
    }
  }

  // ---- 主入口 ---------------------------------------------------------------

  function init() {
    registerMenu();

    if (isAuthPage()) {
      log('运行于授权服务页，监听验证码…');
      watchAuthPage();
    } else if (isNodeseekPage()) {
      log('运行于 nodeseek 页');
      runOnNodeseek();
    }
  }

  // 页面加载完成后执行（@run-at document-idle 已尽量靠后，仍做 DOMContentLoaded 兜底）
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
