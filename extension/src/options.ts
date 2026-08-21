// options.ts —— 设置页逻辑（多槽位列表管理）
// 草稿模式：在内存中编辑槽位列表，点「保存」统一校验并写入 chrome.storage.sync；
// 保存后后台会收到 storage.onChanged 并重排周期调度。
// 校验：启用槽位必须填 serverUrl 与 adminToken；intervalMin 归一化到 1~1440。
(() => {
  const CONFIG_KEY = 'slots';
  const DEFAULT_INTERVAL = 30;

  interface Slot {
    id: string;
    name: string;
    serverUrl: string;
    adminToken: string;
    intervalMin: number;
    targetAccountId: string;
    enabled: boolean;
  }

  let slots: Slot[] = [];

  // 生成槽位 id（不依赖 crypto，各 Chrome 版本均可用）
  function genId(): string {
    return 'slot-' + Date.now().toString(36) + '-' + Math.random().toString(36).slice(2, 8);
  }

  // HTML 转义（用户输入出现在 value 属性中）
  function esc(v: string): string {
    return v.replace(/[&<>"']/g, (c) => {
      const map: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
      return map[c] ?? c;
    });
  }

  async function load(): Promise<void> {
    const stored = await chrome.storage.sync.get({ slots: [] });
    const raw = Array.isArray(stored.slots) ? (stored.slots as Slot[]) : [];
    slots = raw.map((s) => ({
      id: String(s.id ?? genId()),
      name: String(s.name ?? ''),
      serverUrl: String(s.serverUrl ?? ''),
      adminToken: String(s.adminToken ?? ''),
      intervalMin: Number(s.intervalMin) || DEFAULT_INTERVAL,
      targetAccountId: String(s.targetAccountId ?? ''),
      enabled: s.enabled !== false,
    }));
    render();
  }

  // 把输入框的 data-field 写回对应草稿槽位
  function applyInput(input: Element): void {
    const idx = Number((input as HTMLElement).dataset.idx);
    const field = (input as HTMLElement).dataset.field;
    if (!Number.isInteger(idx) || !field) return;
    const slot = slots[idx];
    if (!slot) return;

    const value = (input as HTMLInputElement).value;
    if (field === 'enabled') slot.enabled = (input as HTMLInputElement).checked;
    else if (field === 'name') slot.name = value;
    else if (field === 'serverUrl') slot.serverUrl = value;
    else if (field === 'adminToken') slot.adminToken = value;
    else if (field === 'intervalMin') slot.intervalMin = Number(value);
    else if (field === 'targetAccountId') slot.targetAccountId = value;
  }

  function render(): void {
    const listEl = document.getElementById('slot-list') as HTMLElement;
    listEl.innerHTML = '';

    slots.forEach((s, i) => {
      const card = document.createElement('div');
      card.className = 'slot';
      card.innerHTML = `
        <div class="slot-head">
          <label class="enabled-label">
            <input type="checkbox" data-idx="${i}" data-field="enabled" ${s.enabled ? 'checked' : ''} /> 启用
          </label>
          <input type="text" data-idx="${i}" data-field="name" value="${esc(s.name)}" placeholder="槽位名称（如 默认）" />
          <button class="del" data-idx="${i}" data-action="del" title="删除该槽位">删除</button>
        </div>
        <div class="field">
          <label>服务器地址（serverUrl）</label>
          <input type="url" data-idx="${i}" data-field="serverUrl" value="${esc(s.serverUrl)}" placeholder="https://ns.example.com" autocomplete="off" />
        </div>
        <div class="field">
          <label>管理 Token（X-Admin-Token）</label>
          <input type="password" data-idx="${i}" data-field="adminToken" value="${esc(s.adminToken)}" placeholder="对应服务端 NS_ADMIN_TOKEN" autocomplete="off" />
        </div>
        <div class="field">
          <label>推送间隔（分钟）</label>
          <input type="number" data-idx="${i}" data-field="intervalMin" value="${s.intervalMin}" min="1" max="1440" step="1" />
        </div>
        <div class="field">
          <label>目标账号 ID（targetAccountId，可选）</label>
          <input type="text" data-idx="${i}" data-field="targetAccountId" value="${esc(s.targetAccountId)}" placeholder="服务端自动识别开启（NS_COOKIE_AUTO_DETECT=1）时忽略，可留空" autocomplete="off" />
          <p class="hint">有值时推送请求会附带 account_id；服务端关闭自动识别（NS_COOKIE_AUTO_DETECT=0）时必须填写。</p>
        </div>
      `;
      listEl.appendChild(card);
    });

    // 绑定输入与删除事件
    listEl.querySelectorAll('input').forEach((input) => {
      input.addEventListener('input', () => applyInput(input));
    });
    listEl.querySelectorAll('button[data-action="del"]').forEach((btn) => {
      btn.addEventListener('click', () => {
        const idx = Number((btn as HTMLElement).dataset.idx);
        if (Number.isInteger(idx) && idx >= 0 && idx < slots.length) {
          slots.splice(idx, 1);
          render();
        }
      });
    });
  }

  async function save(): Promise<void> {
    const errorEl = document.getElementById('save-error') as HTMLElement;
    errorEl.textContent = '';

    // 规范化草稿
    const normalized: Slot[] = slots.map((s) => ({
      ...s,
      name: s.name.trim() || '未命名槽位',
      serverUrl: s.serverUrl.trim(),
      intervalMin: Math.max(1, Math.min(1440, Math.floor(Number(s.intervalMin)) || DEFAULT_INTERVAL)),
      targetAccountId: s.targetAccountId.trim(),
    }));

    // 表单校验：启用槽位必须填 serverUrl 与 adminToken
    for (let i = 0; i < normalized.length; i++) {
      const s = normalized[i];
      if (s.enabled && (!s.serverUrl || !s.adminToken)) {
        errorEl.textContent = `槽位「${s.name}」（#${i + 1}）已启用，但 serverUrl 或 adminToken 为空。`;
        return;
      }
    }

    await chrome.storage.sync.set({ [CONFIG_KEY]: normalized });
    slots = normalized;
    render();

    const status = document.getElementById('save-status') as HTMLElement;
    status.textContent = `已保存（${slots.length} 个槽位）`;
    setTimeout(() => {
      status.textContent = '';
    }, 3000);
  }

  document.addEventListener('DOMContentLoaded', () => {
    document.getElementById('btn-add')?.addEventListener('click', () => {
      slots.push({
        id: genId(),
        name: `槽位 ${slots.length + 1}`,
        serverUrl: '',
        adminToken: '',
        intervalMin: DEFAULT_INTERVAL,
        targetAccountId: '',
        enabled: true,
      });
      render();
    });
    document.getElementById('btn-save')?.addEventListener('click', () => void save());
    void load();
  });
})();
