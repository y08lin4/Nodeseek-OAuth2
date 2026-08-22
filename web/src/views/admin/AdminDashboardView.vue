<script setup lang="ts">
// 管理后台仪表盘 v3：欢迎行 + 6 统计卡 + 应用排行 Top5 / 近 7 天登录趋势 + 待办 + 最近审计
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NCard, NTag, NButton, NSpin, NTable, NEmpty, useMessage } from 'naive-ui'
import { Blocks, Activity, Cookie } from 'lucide-vue-next'
import {
  getAdminStatus,
  getAdminStats,
  listAccounts,
  listReviews,
  listAudit,
  listAdminClients,
  ApiError,
  type AdminStatus,
  type AdminStats,
  type SysAccount,
  type ReviewItem,
  type ReviewType,
  type AuditEvent,
  type AdminClient,
} from '../../api'
import {
  statItemLabel,
  formatTime,
  reviewTypeText,
  auditEventCN,
  auditLevel,
  AUDIT_LEVEL_TEXT,
} from './adminShared'

const message = useMessage()
const router = useRouter()

const stats = ref<AdminStats | null>(null)
const status = ref<AdminStatus | null>(null)
const accounts = ref<SysAccount[]>([])
const reviews = ref<ReviewItem[]>([])
const clients = ref<AdminClient[]>([])
const auditEvents = ref<AuditEvent[]>([])

const loading = ref(false)

async function loadAll() {
  loading.value = true
  // 各区块独立容错：任一失败不阻塞其余
  await Promise.allSettled([
    loadStats(),
    loadStatus(),
    loadAccounts(),
    loadReviews(),
    loadAudit(),
    loadClientsTop(),
  ])
  loading.value = false
}

async function loadStats() {
  try {
    const r = await getAdminStats()
    stats.value = r.stats
  } catch {
    stats.value = null
  }
}

async function loadStatus() {
  try {
    status.value = await getAdminStatus()
  } catch {
    status.value = null
  }
}

async function loadAccounts() {
  try {
    const r = await listAccounts()
    accounts.value = r.accounts
  } catch {
    accounts.value = []
  }
}

async function loadReviews() {
  try {
    const r = await listReviews()
    reviews.value = r.reviews
  } catch {
    reviews.value = []
  }
}

// 审计：一次拉足够多，用于欢迎行 / 趋势 / 最近审计
async function loadAudit() {
  try {
    const r = await listAudit(200)
    auditEvents.value = r.events
  } catch (e) {
    auditEvents.value = []
    message.error(e instanceof ApiError ? e.message : '获取审计日志失败')
  }
}

// 欢迎行：从最新 admin.login.ok 解析最后登录 IP/时间
const lastLogin = computed(() => {
  const ok = [...auditEvents.value]
    .filter((ev) => ev.event === 'admin.login.ok')
    .sort((a, b) => new Date(b.ts).getTime() - new Date(a.ts).getTime())[0]
  return ok ? { ts: ok.ts, ip: ok.ip || '' } : null
})

// 6 统计卡
const statCards = computed(() => {
  const items = statItemLabel().map((s) => ({
    label: s.label,
    value: stats.value?.[s.key] ?? 0,
  }))
  items.push({ label: '应用总数', value: clients.value.length })
  return items
})

// 应用排行 Top5：按今日授权成功数
const topApps = (): { client_id: string; client_name: string; count: number }[] =>
  [...clients.value]
    .sort((a, b) => b.stats.auth_ok_today - a.stats.auth_ok_today)
    .slice(0, 5)
    .map((c) => ({ client_id: c.client_id, client_name: c.client_name, count: c.stats.auth_ok_today }))

// 近 7 天登录趋势：按 ts 对 login.confirm.ok 计数（今天在内）
const trend = computed(() => {
  const days: { key: string; label: string; count: number }[] = []
  const now = new Date()
  for (let i = 6; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() - i)
    const key = `${d.getFullYear()}-${d.getMonth() + 1}-${d.getDate()}`
    const label = `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
    days.push({ key, label, count: 0 })
  }
  for (const ev of auditEvents.value) {
    if (ev.event !== 'login.confirm.ok') continue
    const ts = new Date(ev.ts)
    const key = `${ts.getFullYear()}-${ts.getMonth() + 1}-${ts.getDate()}`
    const day = days.find((d) => d.key === key)
    if (day) day.count++
  }
  return days
})

const trendMax = computed(() => Math.max(1, ...trend.value.map((d) => d.count)))
// 柱高度：按最大值归一，最低 8px；无数据 0
const barHeight = (count: number) => (count > 0 ? `${Math.max(8, (count / trendMax.value) * 100)}%` : '4px')

async function loadClientsTop() {
  try {
    const r = await listAdminClients()
    clients.value = r.clients
  } catch (e) {
    clients.value = []
    message.error(e instanceof ApiError ? e.message : '获取应用排行失败')
  }
}

// 系统账号概览（待办区 Cookie 卡）
const accountSummary = () => {
  const enabled = accounts.value.filter((a) => a.enabled).length
  const broken = accounts.value.filter((a) => a.enabled && a.fail_count > 0).length
  return { total: accounts.value.length, enabled, broken }
}

// 审核待办分类计数
const reviewCounts = () => {
  const counts: Partial<Record<ReviewType, number>> = {}
  for (const r of reviews.value) counts[r.type] = (counts[r.type] ?? 0) + 1
  return (Object.keys(counts) as ReviewType[]).map((type) => ({
    type,
    count: counts[type] ?? 0,
  }))
}

// 最近审计（前 5 条，汉化 + 级别色点）
const recentAudit = computed(() => auditEvents.value.slice(0, 5))

onMounted(loadAll)
</script>

<template>
  <!-- 欢迎行 -->
  <div class="page-head">
    <div>
      <h2 class="page-title">仪表盘</h2>
      <p class="page-sub">
        欢迎，admin
        <template v-if="lastLogin">
          · 最后登录 {{ formatTime(lastLogin.ts) }}<template v-if="lastLogin.ip">（IP {{ lastLogin.ip }}）</template>
        </template>
      </p>
    </div>
    <div class="page-actions">
      <n-button size="small" :loading="loading" @click="loadAll">刷新</n-button>
    </div>
  </div>

  <n-spin :show="loading">
    <!-- 6 统计卡 -->
    <div class="admin-stats ns-mb-4">
      <div v-for="s in statCards" :key="s.label" class="admin-stat-card">
        <div class="admin-stat-value">{{ s.value }}</div>
        <div class="admin-stat-label">{{ s.label }}</div>
      </div>
    </div>

    <!-- 双区块：应用排行 + 登录趋势 -->
    <div class="dash-row ns-mb-4">
      <n-card size="small" class="dash-block" title="应用排行 Top5">
        <template #header-extra><Blocks :size="16" class="rank-icon" /></template>
        <template v-if="clients.length">
          <div v-for="(a, i) in topApps()" :key="a.client_id" class="rank-row">
            <span class="rank-idx">{{ i + 1 }}</span>
            <button type="button" class="rank-name" @click="router.push('/admin/apps')" :title="a.client_name">
              {{ a.client_name }}
            </button>
            <span class="rank-count">{{ a.count }}</span>
          </div>
          <n-button size="small" text class="ns-mt-2" @click="router.push('/admin/apps')">查看全部应用 →</n-button>
        </template>
        <n-empty v-else description="暂无应用" size="small" class="ns-py-2" />
      </n-card>

      <n-card size="small" class="dash-block" title="近 7 天登录趋势">
        <template #header-extra><Activity :size="16" class="rank-icon" /></template>
        <div class="trend">
          <div v-for="d in trend" :key="d.key" class="trend-col">
            <span class="trend-val">{{ d.count }}</span>
            <div class="trend-bar-wrap">
              <div class="trend-bar" :style="{ height: barHeight(d.count) }"></div>
            </div>
            <span class="trend-label">{{ d.label }}</span>
          </div>
        </div>
        <div v-if="trend.every((d) => d.count === 0)" class="ns-text-muted ns-small ns-mt-2">近 7 天暂无登录记录</div>
      </n-card>
    </div>

    <!-- 待办区：审核待办 + Cookie 状态 -->
    <div class="dash-row ns-mb-4">
      <!-- 审核待办 -->
      <n-card size="small" class="dash-block" title="审核待办">
        <template v-if="reviews.length">
          <div v-for="item in reviewCounts()" :key="item.type" class="detail-row">
            <span class="detail-label">{{ reviewTypeText(item.type) }}</span>
            <span class="detail-value">{{ item.count }}</span>
          </div>
          <n-button size="small" type="primary" class="ns-mt-2" @click="router.push('/admin/reviews')">
            去审核 →
          </n-button>
        </template>
        <n-empty v-else description="暂无待审核项" size="small" class="ns-py-2" />
      </n-card>

      <!-- Cookie 状态 -->
      <n-card size="small" class="dash-block" title="Cookie 状态">
        <template #header-extra><Cookie :size="16" class="rank-icon" /></template>
        <template v-if="status">
          <div class="detail-row">
            <span class="detail-label">状态</span>
            <span class="detail-value">
              <n-tag :type="status.cookie.set ? 'success' : 'warning'" size="small" round>
                {{ status.cookie.set ? '已设置' : '未设置' }}
              </n-tag>
            </span>
          </div>
          <div class="detail-row">
            <span class="detail-label">账号总数</span>
            <span class="detail-value">{{ accountSummary().total }}（启用 {{ accountSummary().enabled }}）</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">最近更新</span>
            <span class="detail-value">
              {{ status.cookie.updated_at ? formatTime(status.cookie.updated_at) : '—' }}
            </span>
          </div>
          <n-alert v-if="!status.cookie.set" type="error" class="ns-mt-2">
            Cookie 未设置，登录核验将不可用。
          </n-alert>
          <n-button size="small" class="ns-mt-2" @click="router.push('/admin/accounts')">去账号 →</n-button>
        </template>
        <n-empty v-else description="暂无状态" size="small" class="ns-py-2" />
      </n-card>
    </div>

    <!-- 最近审计 -->
    <h2 class="ns-h6 ns-mt-3 ns-mb-3">最近审计</h2>
    <n-table :bordered="true" size="small" class="docs-table">
      <thead>
        <tr>
          <th>时间</th>
          <th>级别</th>
          <th>事件</th>
          <th>user_id</th>
          <th>client_id</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(ev, i) in recentAudit" :key="i">
          <td class="audit-cell ts">{{ formatTime(ev.ts) }}</td>
          <td class="audit-cell">
            <n-tag :type="auditLevel(ev.event) === 'error' ? 'error' : auditLevel(ev.event) === 'warn' ? 'warning' : 'default'" size="small" round>
              {{ AUDIT_LEVEL_TEXT[auditLevel(ev.event)] }}
            </n-tag>
          </td>
          <td class="audit-cell"><span :title="ev.event">{{ auditEventCN(ev.event) }}</span></td>
          <td class="audit-cell ts">{{ ev.user_id || '—' }}</td>
          <td class="audit-cell ts">{{ ev.client_id || '—' }}</td>
        </tr>
      </tbody>
    </n-table>
    <div v-if="!loading && recentAudit.length === 0" class="ns-text-muted ns-small ns-mt-2">暂无审计事件</div>
  </n-spin>
</template>

<style scoped>
.dash-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(360px, 1fr));
  gap: 16px;
}

.dash-block {
  border-radius: 6px;
}

.rank-icon {
  color: var(--ns-primary);
}

.rank-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 0;
  border-bottom: 1px solid var(--ns-border);
  font-size: 13px;
}

.rank-row:last-of-type {
  border-bottom: none;
}

.rank-idx {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  border-radius: 4px;
  background: var(--ns-primary-soft);
  color: var(--ns-primary-hover);
  font-size: 12px;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.rank-name {
  flex: 1;
  min-width: 0;
  text-align: left;
  background: none;
  border: none;
  padding: 0;
  font-size: 13px;
  color: var(--ns-text);
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rank-name:hover {
  color: var(--ns-primary);
}

.rank-count {
  font-variant-numeric: tabular-nums;
  color: var(--ns-muted);
}

/* 登录趋势柱状图 */
.trend {
  display: flex;
  align-items: flex-end;
  gap: 10px;
  height: 160px;
  padding-top: 8px;
}

.trend-col {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  height: 100%;
}

.trend-val {
  font-size: 12px;
  color: var(--ns-muted);
  font-variant-numeric: tabular-nums;
}

.trend-bar-wrap {
  flex: 1;
  width: 100%;
  display: flex;
  align-items: flex-end;
  justify-content: center;
}

.trend-bar {
  width: 60%;
  min-width: 10px;
  max-width: 32px;
  background: var(--ns-primary);
  border-radius: 3px 3px 0 0;
  opacity: 0.85;
}

.trend-label {
  font-size: 12px;
  color: var(--ns-faint);
  font-variant-numeric: tabular-nums;
}
</style>
