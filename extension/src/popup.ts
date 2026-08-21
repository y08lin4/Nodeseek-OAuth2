// popup.ts —— 弹窗逻辑（多槽位）
// 展示：各槽位（名称、serverUrl、成功/失败状态点、上次推送时间与结果摘要）；
// 按钮：立即推送全部（消息通知后台）、打开设置；无槽位时提示去设置页添加。
(() => {
  const CONFIG_DEFAULTS = { slots: [] };
  const LOCAL_DEFAULTS = { slotResults: [] };

  function el(id: string): HTMLElement {
    const node = document.getElementById(id);
    if (!node) throw new Error(`缺少元素 #${id}`);
    return node;
  }

  // HTML 转义（槽位名称/URL/结果可能来自用户输入）
  function escapeHtml(v: string): string {
    return v.replace(/[&<>"']/g, (c) => {
      const map: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
      return map[c] ?? c;
    });
  }

  // 状态点：停用灰 / 上次结果 ok 绿 / 其他（失败、未配置等）红
  function dotClass(enabled: boolean, lastResult: string): string {
    if (!enabled) return 'dot off';
    if (lastResult === 'ok') return 'dot ok';
    return 'dot bad';
  }

  function fmtTime(ts: number): string {
    if (!ts) return '从未推送';
    return new Date(ts).toLocaleString('zh-CN');
  }

  async function refresh(): Promise<void> {
    const cfg = await chrome.storage.sync.get(CONFIG_DEFAULTS);
    const local = await chrome.storage.local.get(LOCAL_DEFAULTS);

    const slots = (cfg.slots ?? []) as Array<Record<string, unknown>>;
    const results = new Map<string, Record<string, unknown>>();
    for (const r of (local.slotResults ?? []) as Array<Record<string, unknown>>) {
      if (r && typeof r.id === 'string') results.set(r.id, r);
    }

    const listEl = el('slot-list');
    const emptyHint = el('empty-hint');
    listEl.innerHTML = '';

    if (slots.length === 0) {
      emptyHint.hidden = false;
      return;
    }
    emptyHint.hidden = true;

    for (const s of slots) {
      const id = String(s.id ?? '');
      const name = String(s.name ?? '未命名');
      const serverUrl = String(s.serverUrl ?? '');
      const enabled = s.enabled !== false;
      const result = results.get(id) ?? {};
      const lastResult = String(result.lastResult ?? '暂无');
      const lastPushAt = Number(result.lastPushAt ?? 0);

      const card = document.createElement('div');
      card.className = 'slot';
      card.innerHTML = `
        <div class="slot-head">
          <span class="${dotClass(enabled, lastResult)}" title="${enabled ? '启用' : '已停用'}"></span>
          <span>${escapeHtml(name)}</span>
        </div>
        <div class="slot-url">${escapeHtml(serverUrl || '(未填 serverUrl)')}</div>
        <div class="slot-result">上次结果：${escapeHtml(lastResult)}</div>
        <div class="slot-time">上次推送：${fmtTime(lastPushAt)}</div>
      `;
      listEl.appendChild(card);
    }
  }

  // 立即推送全部：通知后台 pushAllSlots，完成后刷新展示
  async function pushAll(): Promise<void> {
    const btn = el('btn-push-all') as HTMLButtonElement;
    btn.disabled = true;
    btn.textContent = '推送中…';
    try {
      await chrome.runtime.sendMessage({ type: 'push-all' });
    } finally {
      btn.disabled = false;
      btn.textContent = '立即推送全部';
    }
    await refresh();
  }

  document.addEventListener('DOMContentLoaded', () => {
    el('btn-push-all').addEventListener('click', () => void pushAll());
    el('btn-options').addEventListener('click', () => void chrome.runtime.openOptionsPage());
    // 后台推送/状态变化时自动刷新展示
    chrome.storage.onChanged.addListener(() => void refresh());
    void refresh();
  });
})();
