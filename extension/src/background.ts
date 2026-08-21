// background.ts —— NSAuth2 Cookie Keeper 后台 Service Worker
//
// 职责：
// 1. 多槽位推送：对每个启用槽位，读取 nodeseek.com 域 Cookie 组装 "name=value; ..."，
//    POST 到该槽位自己的 {serverUrl}/api/admin/cookie（Header: X-Admin-Token），
//    防止各系统账号 Cookie 失效（多账号冗余、故障转移的基础）；
// 2. 触发时机：Cookie 变化（onChanged，10s 防抖推全部槽位）/ 周期闹钟（按各槽位
//    intervalMin 取最小到期时间调度，到点只推到期槽位）/ popup「立即推送全部」；
// 3. 失败简单退避：单槽位 1min、5min 后各重试一次，不阻塞其他槽位；
// 4. 一键授权：接收 fill-pm 消息（来自网页桥 web-bridge），找/建 nodeseek 私信页并自动填充验证码。
//
// 配置（chrome.storage.sync）：{ slots: [{id,name,serverUrl,adminToken,intervalMin=30,targetAccountId?,enabled}] }
//   —— 多槽位：每个浏览器 profile 只配自己的账号槽位；
//      旧版单配置 {serverUrl,adminToken,intervalMin} 存在时自动迁移为默认槽位「默认」（id="default"）。
// 运行状态（chrome.storage.local）：{ slotResults: [{id,name,lastPushAt,lastResult}] }

// ==================== 常量 ====================
const DUE_ALARM_NAME = 'push-due'; // 主调度闹钟：所有槽位的最小到期时间
const RETRY_ALARM_PREFIX = 'slot-retry'; // 槽位退避闹钟前缀：slot-retry-<id>-1 / slot-retry-<id>-2
const LEGACY_KEYS = ['serverUrl', 'adminToken', 'intervalMin']; // 旧版单配置键（迁移后清理）
const DEBOUNCE_MS = 10_000; // onChanged 防抖窗口
const INITIAL_DELAY_MS = 60_000; // 从未推过的槽位：1min 后首推
const RETRY_DELAYS = [1, 5]; // 失败退避：1min、5min 各重试一次

interface Slot {
  id: string;
  name: string;
  serverUrl: string;
  adminToken: string;
  intervalMin: number;
  targetAccountId: string; // 可选：有值时请求带 account_id（服务端 NS_COOKIE_AUTO_DETECT=0 时手动绑定）
  enabled: boolean;
  account?: { id: string | number; name: string; rank: string | number };
}

// 新的结构化推送结果（popup 与 options 展示用）。
// lastResult 兼容旧数据：可能是字符串（旧版）或下述对象（新版）。
type ErrorType = 'unauthorized' | 'network' | 'unrecognized' | 'other';
interface SlotResultDetail {
  ok: boolean;
  at: number; // 本次推送时间
  account_id?: string | number;
  account_name?: string;
  rank?: string | number;
  error_type?: ErrorType;
  message?: string;
}

interface SlotResult {
  id: string;
  name: string;
  lastPushAt: number;
  lastResult: string | SlotResultDetail; // 旧版字符串 / 新版对象
}

// ==================== 配置读取（含旧版迁移） ====================
async function getConfig(): Promise<{ slots: Slot[] }> {
  const stored = await chrome.storage.sync.get({
    slots: [],
    serverUrl: '',
    adminToken: '',
    intervalMin: 30,
  });
  let slots: Slot[] = Array.isArray(stored.slots) ? (stored.slots as Slot[]) : [];

  // 迁移：旧版单配置存在且尚无 slots → 自动包装为默认槽位「默认」（id="default"）
  if (slots.length === 0) {
    const legacyServerUrl = String(stored.serverUrl ?? '');
    const legacyToken = String(stored.adminToken ?? '');
    const legacyInterval = Number(stored.intervalMin ?? 30) || 30;
    if (legacyServerUrl || legacyToken) {
      slots = [
        {
          id: 'default',
          name: '默认',
          serverUrl: legacyServerUrl,
          adminToken: legacyToken,
          intervalMin: legacyInterval,
          targetAccountId: '',
          enabled: true,
        },
      ];
      await chrome.storage.sync.set({ slots });
      await chrome.storage.sync.remove(LEGACY_KEYS); // 清理旧键，避免下次重复迁移
    }
  }

  // 规范化：丢弃损坏条目，补默认值
  slots = slots.filter((s) => s && typeof s.id === 'string').map(normalizeSlot);
  return { slots };
}

function normalizeSlot(s: Slot): Slot {
  const account = s.account && typeof s.account === 'object'
    ? {
        id: String(s.account.id ?? ''),
        name: String(s.account.name ?? ''),
        rank: s.account.rank == null ? '—' : String(s.account.rank),
      }
    : undefined;
  return {
    id: s.id,
    name: String(s.name ?? '') || '未命名槽位',
    serverUrl: String(s.serverUrl ?? '').trim(),
    adminToken: String(s.adminToken ?? ''),
    intervalMin: Math.max(1, Math.min(1440, Math.floor(Number(s.intervalMin ?? 30)) || 30)),
    targetAccountId: String(s.targetAccountId ?? '').trim(),
    enabled: s.enabled !== false,
    account,
  };
}

// ==================== 运行状态（slotResults） ====================
const resultsCache = new Map<string, SlotResult>(); // 内存镜像：id → 最近结果

async function loadResults(): Promise<void> {
  const local = await chrome.storage.local.get({ slotResults: [] });
  const arr = Array.isArray(local.slotResults) ? (local.slotResults as SlotResult[]) : [];
  resultsCache.clear();
  for (const r of arr) if (r && r.id) resultsCache.set(r.id, r);
}

async function storeResult(r: SlotResult): Promise<void> {
  resultsCache.set(r.id, r);
  await chrome.storage.local.set({ slotResults: Array.from(resultsCache.values()) });
}

// ==================== 单槽位推送 ====================
// retry：0 首次尝试；1 = 1min 后重试；2 = 5min 后重试（最多 2 次退避）
async function pushSlot(slot: Slot, retry = 0): Promise<void> {
  const at = Date.now();
  const fail = (error_type: ErrorType, message: string): SlotResultDetail => ({
    ok: false,
    at,
    error_type,
    message,
  });

  // 槽位可能推送途中被停用：记录状态但不推送
  if (!slot.enabled) {
    await storeResult({
      id: slot.id,
      name: slot.name,
      lastPushAt: at,
      lastResult: fail('other', '已停用'),
    });
    return;
  }

  // 启用槽位必须填 serverUrl 与 adminToken（options 已校验，此处兜底）
  if (!slot.serverUrl || !slot.adminToken) {
    await storeResult({
      id: slot.id,
      name: slot.name,
      lastPushAt: at,
      lastResult: fail('other', '未配置（缺 serverUrl 或 adminToken）'),
    });
    return;
  }

  try {
    // 1. 抓取 nodeseek.com 域下全部 Cookie（domain 参数匹配该域及其子域）
    const cookies = await chrome.cookies.getAll({ domain: '.nodeseek.com' });
    if (cookies.length === 0) {
      await storeResult({
        id: slot.id,
        name: slot.name,
        lastPushAt: at,
        lastResult: fail('unrecognized', '未找到 nodeseek Cookie（可能未登录）'),
      });
      return;
    }
    const cookieStr = cookies.map((c) => `${c.name}=${c.value}`).join('; ');

    // 2. 组装请求体：targetAccountId 有值且非空才带 account_id
    const body: { cookie: string; account_id?: string } = { cookie: cookieStr };
    if (slot.targetAccountId) body.account_id = slot.targetAccountId;

    // 3. 推送（去除 serverUrl 尾部多余斜杠，避免拼出双斜杠）
    const base = slot.serverUrl.replace(/\/+$/, '');
    let resp: Response;
    try {
      resp = await fetch(`${base}/api/admin/cookie`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Admin-Token': slot.adminToken,
        },
        body: JSON.stringify(body),
      });
    } catch (e) {
      // 网络层错误（DNS/连接/TLS 失败等）→ 分类为 network
      const msg = e instanceof Error ? e.message : String(e);
      await storeResult({
        id: slot.id,
        name: slot.name,
        lastPushAt: at,
        lastResult: fail('network', `无法连接：${msg}`),
      });
      await scheduleRetry(slot, retry);
      return;
    }

    if (!resp.ok) {
      // 401 令牌错误 / 400 识别失败 / 其他状态
      const detail: SlotResultDetail =
        resp.status === 401
          ? fail('unauthorized', '管理令牌错误（HTTP 401）')
          : resp.status === 400
            ? fail('unrecognized', 'Cookie 识别失败（HTTP 400）')
            : fail('other', `HTTP ${resp.status} ${resp.statusText}`);
      await storeResult({ id: slot.id, name: slot.name, lastPushAt: at, lastResult: detail });
      await scheduleRetry(slot, retry);
      return;
    }

    // 4. 成功：解析账号信息（account_id/account_name/stats.rank），记录结构化结果并清除退避闹钟
    let detail: SlotResultDetail = { ok: true, at };
    try {
      const data = (await resp.json()) as {
        account_id?: string | number;
        account_name?: string;
        stats?: { rank?: string | number } | null;
      };
      if (data.account_id !== undefined) detail.account_id = data.account_id;
      if (data.account_name !== undefined) detail.account_name = data.account_name;
      if (data.stats && data.stats.rank !== undefined) detail.rank = data.stats.rank;
    } catch {
      // 服务端未返回 JSON 或有空 body：仍视为推送成功，仅无账号信息
    }
    await storeResult({ id: slot.id, name: slot.name, lastPushAt: at, lastResult: detail });
    await clearRetryAlarms(slot.id);
  } finally {
    // 无论成败：下一正常周期从「现在 + interval」开始，避免主调度被失败槽位频繁触发
    dueTimes.set(slot.id, Date.now() + slot.intervalMin * 60_000);
  }
}

// 失败退避：1min、5min 各重试一次；独立闹钟，不阻塞其他槽位
async function scheduleRetry(slot: Slot, retry: number): Promise<void> {
  if (retry < RETRY_DELAYS.length) {
    await chrome.alarms.create(`${RETRY_ALARM_PREFIX}-${slot.id}-${retry + 1}`, {
      delayInMinutes: RETRY_DELAYS[retry],
    });
  }
}

async function clearRetryAlarms(slotId: string): Promise<void> {
  await chrome.alarms.clear(`${RETRY_ALARM_PREFIX}-${slotId}-1`);
  await chrome.alarms.clear(`${RETRY_ALARM_PREFIX}-${slotId}-2`);
}

// ==================== 多槽位聚合推送 ====================
// 推送全部启用槽位（并行，单槽位失败不阻塞其他）
async function pushAllSlots(): Promise<void> {
  const { slots } = await getConfig();
  await Promise.all(slots.filter((s) => s.enabled).map((s) => pushSlot(s, 0)));
  await scheduleNextDue();
}

// 主调度触发：只推「已到期」的槽位，再重排调度
async function pushDueSlots(): Promise<void> {
  const { slots } = await getConfig();
  const now = Date.now();
  const due = slots.filter((s) => s.enabled && nextDueFor(s, now) <= now);
  await Promise.all(due.map((s) => pushSlot(s, 0)));
  await scheduleNextDue();
}

// ==================== 周期调度（按各槽位 intervalMin 取最小到期时间） ====================
const dueTimes = new Map<string, number>(); // 内存到期表；SW 重启后由上次推送时间推算

// 槽位下次到期时间：优先内存表；无记录时按上次推送时间保持周期；从未推过则 1min 后首推
function nextDueFor(slot: Slot, now: number): number {
  const mem = dueTimes.get(slot.id);
  if (mem !== undefined) return mem;
  const last = resultsCache.get(slot.id)?.lastPushAt ?? 0;
  if (last > 0) return Math.max(now, last + slot.intervalMin * 60_000);
  return now + INITIAL_DELAY_MS;
}

// 只设一个闹钟：触发时刻 = 全部启用槽位中的最小到期时间；到点只推到期槽位
async function scheduleNextDue(): Promise<void> {
  await chrome.alarms.clear(DUE_ALARM_NAME);
  const { slots } = await getConfig();
  const now = Date.now();
  let minDue = Infinity;
  for (const s of slots) {
    if (!s.enabled) continue;
    const due = nextDueFor(s, now);
    if (due < minDue) minDue = due;
  }
  if (!Number.isFinite(minDue)) return; // 无启用槽位：不设闹钟
  const delayMin = Math.max(0.5, (minDue - now) / 60_000); // chrome.alarms 最小延迟 0.5min
  await chrome.alarms.create(DUE_ALARM_NAME, { delayInMinutes: delayMin });
}

// 槽位退避重试（闹钟 slot-retry-<id>-<n> 触发）：只重试该槽位
async function retrySlot(slotId: string, retryN: number): Promise<void> {
  const { slots } = await getConfig();
  const slot = slots.find((s) => s.id === slotId);
  if (!slot || !slot.enabled) return; // 槽位已被删除/停用：放弃重试
  await pushSlot(slot, retryN);
}

// ==================== 一键授权：填充私信验证码 ====================
// 消息来源：网页桥 web-bridge（网页 postMessage → chrome.runtime.sendMessage）
function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

// 向指定 tab 发送 fill-pm 消息；失败重试（最多 maxAttempts 次，间隔 retryDelayMs）
// 典型失败场景：目标页刚打开、content script 尚未注入 → 需等加载后重试
async function sendFillPm(
  tabId: number,
  code: string,
  toUserId: string,
  maxAttempts = 3,
  retryDelayMs = 1000
): Promise<{ ok: boolean; message: string }> {
  let lastErr: unknown;
  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      const res = await chrome.tabs.sendMessage(tabId, { type: 'fill-pm', code, toUserId });
      const r = (res ?? {}) as { ok?: boolean; message?: string };
      return { ok: Boolean(r.ok), message: String(r.message ?? '') };
    } catch (e) {
      lastErr = e;
      if (attempt < maxAttempts) await sleep(retryDelayMs);
    }
  }
  return { ok: false, message: `content script 无响应：${String(lastErr)}` };
}

// 处理 fill-pm：有已打开的 nodeseek 标签页 → 激活并发送；
// 没有 → 新建私信页（toUserId 定位收件人）→ 等加载后重试发送（最多 3 次、间隔 1s）
async function handleFillPm(code: string, toUserId: string): Promise<{ ok: boolean; message: string }> {
  if (!code || !toUserId) {
    return { ok: false, message: '参数缺失：需要 code 与 toUserId' };
  }

  const tabs = await chrome.tabs.query({ url: '*://*.nodeseek.com/*' });
  if (tabs.length > 0 && tabs[0].id !== undefined) {
    const tabId = tabs[0].id;
    await chrome.tabs.update(tabId, { active: true }); // 激活已有标签页
    return sendFillPm(tabId, code, toUserId);
  }

  // 无已打开标签页：新建私信页后重试发送
  const url = `https://www.nodeseek.com/notification#/message?mode=talk&to=${encodeURIComponent(toUserId)}`;
  const tab = await chrome.tabs.create({ url, active: true });
  if (tab.id === undefined) {
    return { ok: false, message: '新建私信页失败（未拿到 tab id）' };
  }
  return sendFillPm(tab.id, code, toUserId);
}

// ==================== 事件监听 ====================
// Cookie 变化 → 防抖 10s → 推送全部槽位
let debounceTimer: number | undefined;
chrome.cookies.onChanged.addListener((changeInfo) => {
  const domain = changeInfo.cookie?.domain ?? '';
  // 只关心 nodeseek 域（domain 可能形如 .nodeseek.com 或 www.nodeseek.com）
  if (!domain.includes('nodeseek')) return;
  if (debounceTimer !== undefined) clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    debounceTimer = undefined;
    void pushAllSlots();
  }, DEBOUNCE_MS);
});

// 闹钟触发：主调度 → 推到期槽位；退避闹钟 → 单槽位重试
chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === DUE_ALARM_NAME) {
    void pushDueSlots();
  } else if (alarm.name.startsWith(RETRY_ALARM_PREFIX)) {
    // 格式：slot-retry-<id>-<n>（id 可能含连字符，用最后一个 '-' 切分）
    const rest = alarm.name.slice(RETRY_ALARM_PREFIX.length + 1);
    const sep = rest.lastIndexOf('-');
    const slotId = rest.slice(0, sep);
    const retryN = Number(rest.slice(sep + 1));
    if (slotId && Number.isInteger(retryN) && retryN >= 1 && retryN <= RETRY_DELAYS.length) {
      void retrySlot(slotId, retryN);
    }
  }
});

// 消息处理：popup「立即推送全部」+ 网页桥「自动填充私信验证码」
chrome.runtime.onMessage.addListener((msg, _sender, sendResponse) => {
  if (!msg || typeof msg !== 'object') return false;
  const m = msg as { type?: string; code?: string; toUserId?: string };

  if (m.type === 'push-all') {
    pushAllSlots()
      .then(() => sendResponse({ ok: true }))
      .catch((e) => sendResponse({ ok: false, error: String(e) }));
    return true; // 保持消息通道，等待异步 sendResponse
  }

  if (m.type === 'fill-pm') {
    // 一键授权：找/建 nodeseek 私信页并自动填充验证码
    handleFillPm(m.code ?? '', m.toUserId ?? '')
      .then((res) => sendResponse(res))
      .catch((e) => sendResponse({ ok: false, message: String(e) }));
    return true;
  }

  return false;
});

// 配置变化（sync 区）→ 到期表作废并按上次推送时间重新推算，重排调度
chrome.storage.onChanged.addListener((changes, areaName) => {
  if (areaName !== 'sync') return;
  if (changes.slots) {
    dueTimes.clear();
    void scheduleNextDue();
  }
});

// 安装/更新后：加载状态缓存并立即推一次全部槽位
chrome.runtime.onInstalled.addListener(() => {
  void (async () => {
    await loadResults();
    await pushAllSlots();
  })();
});

// SW 每次唤醒：加载状态缓存、重建调度（防止浏览器休眠导致闹钟丢失）
void (async () => {
  await loadResults();
  await scheduleNextDue();
})();
