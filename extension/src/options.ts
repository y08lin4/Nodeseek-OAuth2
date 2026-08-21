// options.ts —— NSAuth2 Cookie Keeper 设置页逻辑（4 步初始化向导 + 已配置摘要）
//
// 未配置时自动进入 4 步向导：
//   1. 服务地址：只输域名，自动补 https:// 并校验格式
//   2. 管理令牌：POST {url}/api/admin/status（X-Admin-Token）验证，通过自动下一步
//   3. 识别信息：抓取 nodeseek Cookie 并 POST /api/admin/cookie，回显 ID/用户名/等级
//   4. 保存并开启：写入 chrome.storage.sync.slots（单槽位为主，保留数组结构）
// 已配置时显示摘要卡：服务器域名/账号/间隔/启用状态 + 重新配置/立即推送/停用启用。
(() => {
  const CONFIG_KEY = 'slots';
  const DEFAULT_INTERVAL = 30;

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
  type Step = 1 | 2 | 3 | 4;

  // ==================== 工具 ====================
  function el<T extends HTMLElement = HTMLElement>(id: string): T {
    const n = document.getElementById(id);
    if (!n) throw new Error(`缺少元素 #${id}`);
    return n as T;
  }
  function genId(): string {
    return 'slot-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 8);
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
  // 规范化用户输入为完整 serverUrl（https://host）；非法返回 null
  function normalizeServerUrl(raw: string): { host: string; url: string } | null {
    const host = raw.trim().replace(/^https?:\/\//i, '').replace(/\/+$/, '');
    if (!host) return null;
    // 合法域名：字母/数字/连字符的标签，点分隔
    const re = /^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$/;
    if (!re.test(host)) return null;
    return { host, url: `https://${host}` };
  }
  function iconRefresh(): string {
    return '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="spin"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>';
  }
  const iconCheck =
    '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>';
  const iconX =
    '<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>';

  // ==================== 状态 ====================
  let slots: Slot[] = [];
  let editingIndex = -1; // -1 = 新增槽位；>=0 = 重新配置既有槽位（保留其 id/enabled）
  let step: Step = 1;
  let tokenValid = false; // 第 2 步验证通过
  let recognized = false; // 第 3 步识别成功
  let accountInfo: AccountInfo | null = null;
  let serverUrl = ''; // 规范化后的完整 https://host

  // ==================== 视图切换 ====================
  function hideViews(): void {
    el('wizard-view').hidden = true;
    el('complete-view').hidden = true;
    el('summary-view').hidden = true;
  }
  function showWizard(): void {
    hideViews();
    el('wizard-view').hidden = false;
    renderStep();
  }
  function showComplete(): void {
    hideViews();
    el('complete-view').hidden = false;
  }
  function showSummary(): void {
    hideViews();
    el('summary-view').hidden = false;
    renderSummary();
  }

  // ==================== 步骤条 ====================
  const STEP_LABELS: Record<Step, string> = { 1: '服务地址', 2: '管理令牌', 3: '识别信息', 4: '保存并开启' };
  function renderSteps(): void {
    const c = el('step-indicator');
    c.innerHTML = '';
    (Object.keys(STEP_LABELS) as unknown as Step[]).forEach((s, i) => {
      const num = Number(s);
      const done = num < step;
      const active = num === step;
      const dot = document.createElement('div');
      dot.className = `step${active ? ' active' : ''}${done ? ' done' : ''}`;
      dot.innerHTML = `
        <div class="step-dot">${done ? iconCheck : esc(s)}</div>
        <div class="step-label">${STEP_LABELS[s]}</div>`;
      c.appendChild(dot);
      if (i < 3) {
        const conn = document.createElement('div');
        conn.className = `step-connector${num < step ? ' done' : ''}`;
        c.appendChild(conn);
      }
    });
  }
  function renderStep(): void {
    renderSteps();
    for (let s = 1 as Step; s <= 4; s++) {
      el(`panel-${s}`).hidden = s !== step;
    }
    const back = el('btn-back') as HTMLButtonElement;
    const next = el('btn-next') as HTMLButtonElement;
    const save = el('btn-save-step') as HTMLButtonElement;
    back.hidden = step === 1;
    next.hidden = step === 4;
    save.hidden = step !== 4;
    // 第 2/3 步在验证通过前禁用「下一步」
    if (step === 3) next.disabled = !recognized;
    if (step === 2) next.disabled = !tokenValid;
    if (step !== 2 && step !== 3) next.disabled = false;
  }

  // ==================== 第 1 步：服务地址 ====================
  function validateStep1(showError = true): boolean {
    const raw = (el('server-input') as HTMLInputElement).value;
    const norm = normalizeServerUrl(raw);
    const msg = el('step1-msg');
    if (!norm) {
      if (showError) {
        msg.className = 'msg err';
        msg.innerHTML = `${iconX}<span>请输入合法域名，如 nodeseek-ouath.ailinyu.de</span>`;
        msg.hidden = false;
      }
      serverUrl = '';
      return false;
    }
    serverUrl = norm.url;
    msg.hidden = true;
    return true;
  }

  // ==================== 第 2 步：管理令牌验证 ====================
  async function verifyToken(): Promise<void> {
    const msg = el('step2-msg');
    const btn = el('btn-verify') as HTMLButtonElement;
    const token = (el('token-input') as HTMLInputElement).value.trim();
    if (!serverUrl) {
      msg.className = 'msg err';
      msg.innerHTML = `${iconX}<span>请先完成第 1 步服务地址</span>`;
      msg.hidden = false;
      return;
    }
    if (!token) {
      msg.className = 'msg err';
      msg.innerHTML = `${iconX}<span>请输入管理令牌</span>`;
      msg.hidden = false;
      return;
    }
    btn.disabled = true;
    msg.className = 'msg loading';
    msg.innerHTML = `${iconRefresh()}<span>正在验证…</span>`;
    msg.hidden = false;
    try {
      let resp: Response;
      try {
        resp = await fetch(`${serverUrl}/api/admin/status`, {
          method: 'POST',
          headers: { 'X-Admin-Token': token },
        });
      } catch (e) {
        const reason = e instanceof Error ? e.message : String(e);
        msg.className = 'msg err';
        msg.innerHTML = `${iconX}<span>无法连接服务器：${esc(reason)}</span>`;
        return;
      }
      if (resp.ok) {
        tokenValid = true;
        msg.className = 'msg ok';
        msg.innerHTML = `${iconCheck}<span>连接正常 · 服务可用</span>`;
        // 自动下一步
        setTimeout(() => {
          step = 3;
          tokenValid = true;
          renderStep();
        }, 500);
      } else {
        tokenValid = false;
        const reason = resp.status === 401 ? '管理令牌错误（HTTP 401）' : `验证失败（HTTP ${resp.status}）`;
        msg.className = 'msg err';
        msg.innerHTML = `${iconX}<span>${esc(reason)}</span>`;
      }
    } finally {
      btn.disabled = false;
    }
  }

  // ==================== 第 3 步：识别账号 ====================
  async function recognizeAccount(): Promise<void> {
    const msg = el('step3-msg');
    const btn = el('btn-recognize') as HTMLButtonElement;
    const token = (el('token-input') as HTMLInputElement).value.trim();
    if (!serverUrl || !tokenValid) {
      msg.className = 'msg err';
      msg.innerHTML = `${iconX}<span>请先在第 2 步验证管理令牌</span>`;
      msg.hidden = false;
      return;
    }
    btn.disabled = true;
    msg.className = 'msg loading';
    msg.innerHTML = `${iconRefresh()}<span>正在读取并识别 Cookie…</span>`;
    msg.hidden = false;
    el('account-box').hidden = true;
    try {
      const cookies = await chrome.cookies.getAll({ domain: '.nodeseek.com' });
      if (!cookies || cookies.length === 0) {
        recognized = false;
        msg.className = 'msg err';
        msg.innerHTML = `${iconX}<span>未找到 nodeseek.com Cookie（请先登录 Nodeseek）</span>`;
        return;
      }
      const cookieStr = cookies.map((c) => `${c.name}=${c.value}`).join('; ');
      let resp: Response;
      try {
        resp = await fetch(`${serverUrl}/api/admin/cookie`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'X-Admin-Token': token },
          body: JSON.stringify({ cookie: cookieStr }),
        });
      } catch (e) {
        const reason = e instanceof Error ? e.message : String(e);
        recognized = false;
        msg.className = 'msg err';
        msg.innerHTML = `${iconX}<span>无法连接服务器：${esc(reason)}</span>`;
        return;
      }
      if (!resp.ok) {
        recognized = false;
        const reason =
          resp.status === 401
            ? '管理令牌错误（HTTP 401）'
            : resp.status === 400
              ? 'Cookie 识别失败（HTTP 400）'
              : `识别失败（HTTP ${resp.status}）`;
        msg.className = 'msg err';
        msg.innerHTML = `${iconX}<span>${esc(reason)}</span>`;
        return;
      }
      const data = (await resp.json().catch(() => ({}))) as {
        account_id?: string | number;
        account_name?: string;
        stats?: { rank?: string | number } | null;
      };
      accountInfo = {
        id: data.account_id ?? '—',
        name: data.account_name ?? '未知',
        rank: data.stats?.rank == null ? '—' : data.stats.rank,
      };
      recognized = true;
      msg.className = 'msg ok';
      msg.innerHTML = `${iconCheck}<span>识别成功</span>`;
      msg.hidden = false;
      renderAccountBox(accountInfo);
    } finally {
      btn.disabled = false;
    }
  }
  function renderAccountBox(a: AccountInfo): void {
    const box = el('account-box');
    box.innerHTML = `
      <div class="row"><span class="k">ID</span><span class="v">${esc(a.id)}</span></div>
      <div class="row"><span class="k">用户名</span><span class="v">${esc(a.name)}</span></div>
      <div class="row"><span class="k">等级</span><span class="v${a.rank === '—' ? ' rank-missing' : ''}">${esc(a.rank)}</span></div>`;
    box.hidden = false;
  }

  // ==================== 第 4 步：保存 ====================
  function hydrateTokenValidations(): void {
    // 从已填状态恢复「下一步」可用性
    if (step === 2) (el('btn-next') as HTMLButtonElement).disabled = !tokenValid;
    if (step === 3) (el('btn-next') as HTMLButtonElement).disabled = !recognized;
  }
  async function saveAndEnable(): Promise<void> {
    const msg = el('step4-msg');
    msg.hidden = true;
    if (!serverUrl) {
      msg.className = 'msg err';
      msg.innerHTML = `${iconX}<span>服务地址无效</span>`;
      msg.hidden = false;
      return;
    }
    const token = (el('token-input') as HTMLInputElement).value.trim();
    if (!token) {
      msg.className = 'msg err';
      msg.innerHTML = `${iconX}<span>请填写管理令牌</span>`;
      msg.hidden = false;
      return;
    }
    const base: Slot = {
      id: '',
      name: editingIndex >= 0 ? (slots[editingIndex]?.name ?? '默认') : slots.length === 0 ? '默认' : `槽位 ${slots.length + 1}`,
      serverUrl,
      adminToken: token,
      intervalMin: DEFAULT_INTERVAL,
      targetAccountId: editingIndex >= 0 ? (slots[editingIndex]?.targetAccountId ?? '') : '',
      enabled: editingIndex >= 0 ? slots[editingIndex]?.enabled !== false : true,
      account: accountInfo ?? undefined,
    };

    try {
      if (editingIndex >= 0 && slots[editingIndex]) {
        base.id = slots[editingIndex].id;
        slots[editingIndex] = base;
      } else {
        base.id = genId();
        slots.push(base);
      }
      await chrome.storage.sync.set({ [CONFIG_KEY]: slots });
      editingIndex = -1;
      showComplete();
    } catch (e) {
      msg.className = 'msg err';
      msg.innerHTML = `${iconX}<span>保存失败：${esc(e instanceof Error ? e.message : String(e))}</span>`;
      msg.hidden = false;
    }
  }

  // ==================== 摘要视图 ====================
  function renderSummary(): void {
    const list = el('summary-list');
    el('sum-count').textContent = String(slots.filter((s) => s).length);
    list.innerHTML = '';
    slots.forEach((s, i) => {
      const host = hostFromUrl(s.serverUrl);
      const rankTxt = s.account && s.account.rank != null ? String(s.account.rank) : '—';
      const acctName = s.account?.name ? String(s.account.name) : '未识别';
      const acctId = s.account?.id != null ? String(s.account.id) : '—';
      const card = document.createElement('div');
      card.className = 'card';
      card.innerHTML = `
        <div class="row" style="align-items:center;">
          <strong>${esc(s.name)}</strong>
          <span class="enabled-badge ${s.enabled ? 'on' : 'off'}">${s.enabled ? '启用' : '已停用'}</span>
        </div>
        <div class="kv-grid" style="margin-top:12px;">
          <div class="kv-row"><span class="kv-k">服务器域名</span><span class="kv-v">${esc(host || s.serverUrl)}</span></div>
          <div class="kv-row"><span class="kv-k">账号</span><span class="kv-v">${esc(acctName)} (${esc(acctId)}) · 等级 ${esc(rankTxt)}</span></div>
          <div class="kv-row"><span class="kv-k">推送间隔</span><span class="kv-v">${s.intervalMin} 分钟</span></div>
        </div>
        <div class="op-group">
          <button class="btn btn-secondary" data-act="reconfig" data-idx="${i}">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 3a2.828 2.828 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3z"/></svg>
            <span>重新配置</span>
          </button>
          <button class="btn btn-secondary" data-act="push" data-idx="${i}">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"/><polyline points="1 20 1 14 7 14"/><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"/></svg>
            <span>立即推送一次</span>
          </button>
          <button class="btn btn-secondary" data-act="toggle" data-idx="${i}">
            <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18.36 6.64a9 9 0 1 1-12.73 0"/><line x1="12" x2="12" y1="2" y2="12"/></svg>
            <span>${s.enabled ? '停用' : '启用'}</span>
          </button>
        </div>
      `;
      card.querySelectorAll('button[data-act]').forEach((btn) => {
        btn.addEventListener('click', () => {
          const idx = Number((btn as HTMLElement).dataset.idx);
          const act = (btn as HTMLElement).dataset.act;
          if (act === 'reconfig') reconfig(idx);
          else if (act === 'toggle') void toggleSlot(idx);
          else if (act === 'push') void pushSlotNow();
        });
      });
      list.appendChild(card);
    });
  }
  // 重新配置：进入向导并预填既有值（保留旧 id / enabled）
  function reconfig(idx: number): void {
    const s = slots[idx];
    if (!s) return;
    editingIndex = idx;
    (el('server-input') as HTMLInputElement).value = hostFromUrl(s.serverUrl);
    (el('token-input') as HTMLInputElement).value = s.adminToken;
    serverUrl = s.serverUrl;
    step = 1;
    tokenValid = true;
    recognized = !!s.account;
    accountInfo = s.account ?? null;
    hideViews();
    el('wizard-view').hidden = false;
    if (accountInfo) renderAccountBox(accountInfo);
    renderStep();
  }
  async function toggleSlot(idx: number): Promise<void> {
    const s = slots[idx];
    if (!s) return;
    s.enabled = !s.enabled;
    await chrome.storage.sync.set({ [CONFIG_KEY]: slots });
    renderSummary();
  }
  // 立即推送一次：通知后台 pushAllSlots（简化：推送全部启用槽位）
  async function pushSlotNow(): Promise<void> {
    await chrome.runtime.sendMessage({ type: 'push-all' });
  }

  // ==================== 初始化 ====================
  function startWizard(mode: 'new' | 'reconfig-shared'): void {
    if (mode === 'new') {
      editingIndex = -1;
      serverUrl = '';
      tokenValid = false;
      recognized = false;
      accountInfo = null;
    }
    step = 1;
    hideViews();
    el('wizard-view').hidden = false;
    renderStep();
  }

  async function init(): Promise<void> {
    const stored = await chrome.storage.sync.get({ slots: [] });
    const raw = Array.isArray(stored.slots) ? (stored.slots as Slot[]) : [];
    slots = raw
      .filter((s) => s && typeof s.id === 'string')
      .map((s) => ({
        id: s.id,
        name: String(s.name ?? '') || '未命名槽位',
        serverUrl: String(s.serverUrl ?? '').trim(),
        adminToken: String(s.adminToken ?? ''),
        intervalMin: Math.max(1, Math.min(1440, Math.floor(Number(s.intervalMin) || DEFAULT_INTERVAL))),
        targetAccountId: String(s.targetAccountId ?? '').trim(),
        enabled: s.enabled !== false,
        account: s.account && typeof s.account === 'object' ? s.account : undefined,
      }));

    // 无任何槽位 → 自动进入向导；否则显示摘要
    if (slots.length === 0) {
      startWizard('new');
    } else {
      showSummary();
    }
  }

  // ==================== 事件 ====================
  document.addEventListener('DOMContentLoaded', () => {
    (el('server-input') as HTMLInputElement).addEventListener('input', () => {
      tokenValid = false;
      recognized = false;
      accountInfo = null;
      el('account-box').hidden = true;
      el('step2-msg').hidden = true;
      el('step3-msg').hidden = true;
      el('btn-next').hidden = step === 4;
      (el('btn-next') as HTMLButtonElement).disabled = false;
    });

    el('btn-verify').addEventListener('click', () => void verifyToken());
    el('btn-recognize').addEventListener('click', () => void recognizeAccount());

    el('btn-back').addEventListener('click', () => {
      if (step > 1) {
        step = (step - 1) as Step;
        renderStep();
      }
    });
    el('btn-next').addEventListener('click', () => {
      if (step === 1) {
        if (!validateStep1(true)) return;
        step = 2;
      } else if (step === 2) {
        if (!tokenValid) return;
        step = 3;
      } else if (step === 3) {
        if (!recognized) return;
        step = 4;
      }
      renderStep();
      if (step === 4) renderSaveSummary();
    });
    el('btn-save-step').addEventListener('click', () => void saveAndEnable());

    el('btn-complete-done').addEventListener('click', () => {
      void (async () => {
        const stored = await chrome.storage.sync.get({ slots: [] });
        slots = Array.isArray(stored.slots) ? (stored.slots as Slot[]) : [];
        showSummary();
      })();
    });

    el('btn-add-slot').addEventListener('click', () => {
      startWizard('new');
    });

    void init();
  });

  // 第 4 步的保存摘要文案
  function renderSaveSummary(): void {
    const name = editingIndex >= 0 ? slots[editingIndex]?.name ?? '默认' : slots.length === 0 ? '默认' : `槽位 ${slots.length + 1}`;
    const rankTxt = accountInfo?.rank != null ? String(accountInfo.rank) : '—';
    const acctName = accountInfo?.name ? String(accountInfo.name) : '未识别';
    const acctId = accountInfo?.id != null ? String(accountInfo.id) : '—';
    (el('save-summary') as HTMLElement).innerHTML = `
      即将保存槽位 <strong>${esc(name)}</strong>：<br/>
      服务地址 <strong>${esc(serverUrl)}</strong> · 账号 <strong>${esc(acctName)} (${esc(acctId)}) · 等级 ${esc(rankTxt)}</strong><br/>
      推送间隔 <strong>${DEFAULT_INTERVAL} 分钟</strong>（静默推送 nodeseek Cookie）。
    `;
    hydrateTokenValidations();
  }
})();
