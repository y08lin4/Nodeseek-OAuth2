// popup.ts —— NSAuth2 Cookie Keeper 弹窗逻辑
// 状态区：服务器域名 · 账号（昵称+ID+等级）· 上次推送（时间 + 结果）；
// 结果兼容显示：lastResult 旧字符串或新对象 {ok,error_type,message}。
// 底部：立即推送（push-all）/ 设置（openOptionsPage）；未配置时引导去配置。
(() => {
  const CONFIG_DEFAULTS = { slots: [] };
  const LOCAL_DEFAULTS = { slotResults: [] };

  interface AccountInfo {
    id: string | number;
    name: string;
    rank: string | number;
  }
  interface Slot {
    id: string;
    name: string;
    serverUrl: string;
    adminToken: string;
    intervalMin: number;
    targetAccountId: string;
    enabled: boolean;
    account?: AccountInfo;
  }
  interface ResultDetail {
    ok: boolean;
    error_type?: 'unauthorized' | 'network' | 'unrecognized' | 'other';
    message?: string;
  }
  interface SlotResult {
    id: string;
    name: string;
    lastPushAt: number;
    lastResult: string | ResultDetail;
  }

  function el(id: string): HTMLElement {
    const n = document.getElementById(id);
    if (!n) throw new Error(`缺少元素 #${id}`);
    return n;
  }
  function esc(v: unknown): string {
    return String(v ?? '').replace(/[&<>"']/g, (c) => {
      const m: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
      return m[c] ?? c;
    });
  }
  function hostFromUrl(url: string): string {
    return url.trim().replace(/^https?:\/\//i, '').replace(/\/+$/, '');
  }
  function fmtRel(ts: number): string {
    if (!ts) return '从未推送';
    const diff = Date.now() - ts;
    if (diff < 60_000) return '刚刚';
    if (diff < 3600_000) return `${Math.floor(diff / 60_000)} 分钟前`;
    if (diff < 86400_000) return `${Math.floor(diff / 3600_000)} 小时前`;
    return new Date(ts).toLocaleString('zh-CN');
  }

  interface Described {
    status: 'ok' | 'bad' | 'none';
    key: string; // ok 时成功文案；bad 时红字原因
    extra: string; // 附加说明（等级/HTTP 等）
  }
  const iconCheck =
    '<svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg>';
  const iconX =
    '<svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>';

  // 兼容显示：lastResult 可能是旧字符串或新对象
  function describeResult(lastResult: string | ResultDetail | undefined): Described {
    if (lastResult == null) return { status: 'none', key: '', extra: '' };
    // 新对象
    if (typeof lastResult === 'object') {
      if (lastResult.ok) {
        return { status: 'ok', key: '成功', extra: '' };
      }
      const t = lastResult.error_type;
      if (t === 'unauthorized') return { status: 'bad', key: '令牌错误', extra: '' };
      if (t === 'network') return { status: 'bad', key: '无法连接', extra: '' };
      if (t === 'unrecognized') return { status: 'bad', key: '识别失败', extra: '' };
      return { status: 'bad', key: lastResult.message || '推送失败', extra: '' };
    }
    // 旧字符串
    const s = lastResult.trim();
    if (s === 'ok') return { status: 'ok', key: '成功', extra: '' };
    if (s === '未知') return { status: 'none', key: '', extra: '' };
    if (/HTTP\s+401/i.test(s)) return { status: 'bad', key: '令牌错误', extra: '' };
    if (/HTTP\s+400/i.test(s)) return { status: 'bad', key: '识别失败', extra: '' };
    if (/HTTP\s+40[3-9]|HTTP\s+5\d\d/i.test(s))
      return { status: 'bad', key: '推送失败', extra: s };
    if (/无法连接|网络/i.test(s)) return { status: 'bad', key: '无法连接', extra: s };
    if (/失败|已停用|未配置|未找到/i.test(s)) return { status: 'bad', key: s, extra: '' };
    return { status: 'none', key: '', extra: s };
  }

  function resultIcon(d: Described): string {
    return d.status === 'ok' ? iconCheck : d.status === 'bad' ? iconX : '';
  }

  function renderBlock(s: Slot, r: SlotResult | undefined): HTMLElement {
    const block = document.createElement('div');
    const host = hostFromUrl(s.serverUrl);
    const name = s.name || '未命名';
    const account = s.account;
    const acctTxt = account
      ? `${esc(account.name || '未知')} (${esc(String(account.id ?? '—'))}) · 等级 ${esc(String(account.rank ?? '—'))}`
      : '未识别';
    const lastPushAt = r?.lastPushAt ?? 0;
    const d = describeResult(r?.lastResult);

    let resultHtml = '';
    if (d.status !== 'none') {
      const cls = d.status === 'ok' ? 'ok' : 'bad';
      const extra = d.extra ? `<span class="res-msg">${esc(d.extra)}</span>` : '';
      resultHtml = `<div class="result-line ${cls}">${resultIcon(d)}<span>${esc(d.key)}</span>${extra}</div>`;
    }

    const title = s.enabled ? `${esc(name)}（启用）` : `${esc(name)}（已停用）`;
    block.innerHTML = `
      <div style="font-weight:600;margin:4px 0 2px;">${title}</div>
      <div class="kv-row"><span class="k">服务器</span><span class="v">${esc(host || s.serverUrl || '—')}</span></div>
      <div class="kv-row"><span class="k">账号</span><span class="v">${acctTxt}</span></div>
      <div class="kv-row"><span class="k">上次推送</span><span class="v sub">${fmtRel(lastPushAt)}</span></div>
      ${resultHtml}`;
    return block;
  }

  async function refresh(): Promise<void> {
    const cfg = await chrome.storage.sync.get(CONFIG_DEFAULTS);
    const local = await chrome.storage.local.get(LOCAL_DEFAULTS);
    const slots = (cfg.slots ?? []) as Slot[];
    const results = new Map<string, SlotResult>();
    for (const rr of (local.slotResults ?? []) as SlotResult[]) {
      if (rr && typeof rr.id === 'string') results.set(rr.id, rr);
    }

    const emptyView = el('empty-view');
    const statusView = el('status-view');
    const badge = el('status-badge');

    const effective = slots.filter((s) => s && typeof s.id === 'string');
    if (effective.length === 0) {
      statusView.hidden = true;
      emptyView.hidden = false;
      badge.textContent = '无配置';
      return;
    }
    emptyView.hidden = true;
    statusView.hidden = false;
    const enabledCount = effective.filter((s) => s.enabled !== false).length;
    badge.textContent = enabledCount > 0 ? `推送中 · ${enabledCount}` : '已停用';

    statusView.innerHTML = '';
    effective.forEach((s, i) => {
      const block = renderBlock(s, results.get(s.id));
      if (i > 0) {
        const gap = document.createElement('div');
        gap.style.cssText = 'height:1px;background:var(--border);margin:10px 0;';
        statusView.appendChild(gap);
      }
      statusView.appendChild(block);
    });
  }

  async function pushAll(): Promise<void> {
    const btn = el('btn-push') as HTMLButtonElement;
    const span = btn.querySelector('span') as HTMLElement;
    const icon = btn.querySelector('svg') as SVGElement | null;
    btn.disabled = true;
    span.textContent = '推送中…';
    if (icon) icon.classList.add('spin');
    try {
      await chrome.runtime.sendMessage({ type: 'push-all' });
    } finally {
      span.textContent = '立即推送';
      if (icon) icon.classList.remove('spin');
      btn.disabled = false;
    }
    await refresh();
  }

  document.addEventListener('DOMContentLoaded', () => {
    el('btn-configure').addEventListener('click', () => void chrome.runtime.openOptionsPage());
    el('btn-push').addEventListener('click', () => void pushAll());
    el('btn-options').addEventListener('click', () => void chrome.runtime.openOptionsPage());
    chrome.storage.onChanged.addListener(() => void refresh());
    void refresh();
  });
})();
