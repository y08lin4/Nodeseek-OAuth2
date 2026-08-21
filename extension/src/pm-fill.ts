// pm-fill.ts —— 私信页自动填充（注入 nodeseek.com https://*.nodeseek.com/*）
//
// 职责：收到后台的 fill-pm 消息 → 轮询等待私信输入框出现（500ms 间隔，最长 10s）
// → 填入验证码并派发 input 事件（兼容 Vue v-model 双向绑定）→ 高亮「发送」按钮
// → 通过 sendResponse 返回 {ok, message}。
(() => {
  // 私信输入框候选选择器：真实 DOM 结构部署后需校准（TODO）
  const INPUT_SELECTORS = ['textarea', '[contenteditable="true"]', '.ql-editor'];
  const POLL_INTERVAL_MS = 500; // 轮询间隔
  const POLL_TIMEOUT_MS = 10_000; // 最长等待时间

  // 只认可见元素（SPA 可能隐藏了未激活的输入框）
  function isVisible(el: Element): boolean {
    const rect = el.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0;
  }

  // 按候选选择器顺序查找可见输入容器
  function findInput(): HTMLElement | null {
    for (const selector of INPUT_SELECTORS) {
      const els = document.querySelectorAll<HTMLElement>(selector);
      for (const el of els) {
        if (isVisible(el)) return el;
      }
    }
    return null;
  }

  // 轮询等待输入框出现
  async function waitForInput(): Promise<HTMLElement | null> {
    const deadline = Date.now() + POLL_TIMEOUT_MS;
    while (Date.now() < deadline) {
      const input = findInput();
      if (input) return input;
      await new Promise((resolve) => setTimeout(resolve, POLL_INTERVAL_MS));
    }
    return null;
  }

  // 填入验证码：textarea/input 直接赋值；contenteditable 设文本。
  // 赋值后派发 input/change 事件（bubbles），Vue 的 v-model 依赖 input 事件同步。
  // TODO：若 NodeSeek 私信框用 Quill 等富文本，可能还需派发 Quill 内部事件，部署后校准。
  function fillValue(el: HTMLElement, code: string): void {
    if (el instanceof HTMLTextAreaElement || el instanceof HTMLInputElement) {
      const prev = el.value;
      el.value = code;
      if (prev !== code) {
        el.dispatchEvent(new Event('input', { bubbles: true }));
        el.dispatchEvent(new Event('change', { bubbles: true }));
      }
    } else if (el.isContentEditable) {
      el.textContent = code;
      el.dispatchEvent(new Event('input', { bubbles: true }));
    }
  }

  // 高亮包含「发送」文本的按钮（TODO：按钮文案/结构部署后校准）
  function highlightSendButton(): boolean {
    const buttons = Array.from(document.querySelectorAll('button'));
    const sendBtn = buttons.find((b) => (b.textContent ?? '').includes('发送'));
    if (sendBtn) {
      sendBtn.style.outline = '3px solid #f59e0b';
      sendBtn.style.outlineOffset = '2px';
      return true;
    }
    return false;
  }

  // 处理后台发来的填充请求
  async function handleFillPm(code: string): Promise<{ ok: boolean; message: string }> {
    const input = await waitForInput();
    if (!input) {
      return { ok: false, message: '10s 内未找到私信输入框（页面结构可能已变化）' };
    }
    fillValue(input, code);
    const btnFound = highlightSendButton();
    return {
      ok: true,
      message: btnFound ? '已填入验证码并高亮发送按钮' : '已填入验证码（未找到发送按钮）',
    };
  }

  chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
    if (!msg || typeof msg !== 'object') return false;
    const m = msg as { type?: string; code?: string };
    if (m.type !== 'fill-pm') return false;

    const code = m.code ?? '';
    if (!code) {
      sendResponse({ ok: false, message: '缺少验证码 code' });
      return true;
    }
    // 异步轮询：必须返回 true 保持消息通道，等待 sendResponse
    void handleFillPm(code).then(sendResponse);
    return true;
  });
})();
