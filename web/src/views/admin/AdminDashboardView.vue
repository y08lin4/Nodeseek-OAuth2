<script setup lang="ts">
// 管理后台仪表盘：今日统计 + Cookie 状态 + 系统账号概览 + 审核待办 + 最近审计 + 应用排行
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NCard, NTag, NButton, NSpin, NTable, NEmpty, useMessage } from 'naive-ui'
import { Blocks } from 'lucide-vue-next'
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
  ageText,
  reviewTypeText,
} from './adminShared'

const message = useMessage()
const router = useRouter()

const stats = ref<AdminStats | null>(null)
const status = ref<AdminStatus | null>(null)
const accounts = ref<SysAccount[]>([])
const reviews = ref<ReviewItem[]>([])
const recentAudit = ref<AuditEvent[]>([])
const clients = ref<AdminClient[]>([])

const loading = ref(false)

async function loadAll() {
  loading.value = true
  // 各区块独立容错：任一失败不阻塞其余
  await Promise.allSettled([
    loadStats().then(() => {}),
    loadStatus().then(() => {}),
    loadAccounts().then(() => {}),
    loadReviews().then(() => {}),
    loadAudit().then(() => {}),
    loadClientsTop().then(() => {}),
  ])
  loading.value = false
}

async function loadStats() {
  try {
    const r = await getAdminStats()
    stats.value = r.stats
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '获取统计失败')
  }
}

async function loadStatus() {
  try {
    status.value = await getAdminStatus()
  } catch (e) {
    status.value = null
    message.error(e instanceof ApiError ? e.message : '获取状态失败')
  }
}

async function loadAccounts() {
  try {
    const r = await listAccounts()
    accounts.value = r.accounts
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '获取系统账号失败')
  }
}

async function loadReviews() {
  try {
    const r = await listReviews()
    reviews.value = r.reviews
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '获取审核队列失败')
  }
}

async function loadAudit() {
  try {
    const r = await listAudit(5)
    recentAudit.value = r.events
  } catch (e) {
    recentAudit.value = []
    message.error(e instanceof ApiError ? e.message : '获取审计日志失败')
  }
}

// 应用排行：按今日授权成功数取前 5
const topApps = (): { client_id: string; client_name: string; count: number }[] =>
  [...clients.value]
    .sort((a, b) => b.stats.auth_ok_today - a.stats.auth_ok_today)
    .slice(0, 5)
    .map((c) => ({ client_id: c.client_id, client_name: c.client_name, count: c.stats.auth_ok_today }))

async function loadClientsTop() {
  try {
    const r = await listAdminClients()
    clients.value = r.clients
  } catch (e) {
    clients.value = []
    message.error(e instanceof ApiError ? e.message : '获取应用排行失败')
  }
}

// 系统账号概览
const accountSummary = () => {
  const enabled = accounts.value.filter((a) => a.enabled).length
  const broken = accounts.value.filter((a) => a.enabled && a.fail_count > 0).length
  return { total: accounts.value.length, enabled, broken }
}

// 审核待办分类计数（仅返回存在的类型）
const reviewCounts = () => {
  const counts: Partial<Record<ReviewType, number>> = {}
  for (const r of reviews.value) counts[r.type] = (counts[r.type] ?? 0) + 1
  return (Object.keys(counts) as ReviewType[]).map((type) => ({
    type,
    count: counts[type] ?? 0,
  }))
}

onMounted(loadAll)
</script>

<template>
  <n-card class="admin-page-card">
    <template #header>
      <span class="page-title">仪表盘</span>
    </template>
    <p class="ns-card-sub">管理后台系统概览：今日统计与应用健康状态</p>

    <!-- 今日统计 -->
    <h2 class="ns-h6 ns-mb-3">今日统计</h2>
    <n-spin :show="loading">
      <div v-if="stats" class="stats-grid">
        <div v-for="s in statItemLabel()" :key="s.key" class="stat-item">
          <div class="stat-value">{{ stats?.[s.key] ?? 0 }}</div>
          <div class="stat-label">{{ s.label }}</div>
        </div>
      </div>
      <div class="ns-text-muted ns-small ns-mt-2" v-if="stats">
        统计自 {{ formatTime(stats.reset_at) }} 起
      </div>
    </n-spin>

    <!-- 仪表盘 3 栏：Cookie / 账号 / 审核待办 -->
    <div class="dash-grid">
      <!-- Cookie 状态卡 -->
      <n-card size="small" class="dash-card" title="Cookie 状态">
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
            <span class="detail-label">更新时间</span>
            <span class="detail-value">{{ status.cookie.updated_at ? formatTime(status.cookie.updated_at) : '—' }}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">更新距今</span>
            <span class="detail-value">{{ ageText(status.cookie.age_seconds) }}</span>
          </div>
          <n-alert v-if="!status.cookie.set" type="error" class="ns-mt-2">
            Cookie 未设置，登录核验将不可用。
          </n-alert>
          <n-tag v-if="status.mock_mode" type="warning" size="small" round class="ns-mt-2">Mock 模式</n-tag>
        </template>
        <n-empty v-else description="暂无状态" size="small" />
      </n-card>

      <!-- 系统账号概览 -->
      <n-card size="small" class="dash-card" title="系统账号">
        <div class="detail-row">
          <span class="detail-label">账号总数</span>
          <span class="detail-value">{{ accountSummary().total }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">启用</span>
          <span class="detail-value">{{ accountSummary().enabled }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">失效告警</span>
          <span class="detail-value">
            <n-tag :type="accountSummary().broken > 0 ? 'error' : 'success'" size="small" round>
              {{ accountSummary().broken }}
            </n-tag>
          </span>
        </div>
        <n-button size="small" class="ns-mt-2" @click="router.push('/admin/accounts')">管理账号 →</n-button>
      </n-card>

      <!-- 审核待办 -->
      <n-card size="small" class="dash-card" title="审核待办">
        <template v-if="reviews.length">
          <div v-for="item in reviewCounts()" :key="item.type" class="detail-row">
            <span class="detail-label">{{ reviewTypeText(item.type) }}</span>
            <span class="detail-value">{{ item.count }}</span>
          </div>
          <n-button size="small" type="primary" class="ns-mt-2" @click="router.push('/admin/reviews')">
            去审核 →
          </n-button>
        </template>
        <n-empty v-else description="暂无待审核项" size="small" />
      </n-card>

      <!-- 应用排行 Top5 -->
      <n-card size="small" class="dash-card" title="应用排行 Top5">
        <template #header-extra>
          <Blocks :size="16" class="rank-icon" />
        </template>
        <template v-if="clients.length">
          <div v-for="(a, i) in topApps()" :key="a.client_id" class="rank-row">
            <span class="rank-idx">{{ i + 1 }}</span>
            <button type="button" class="rank-name" @click="router.push('/admin/apps')">{{ a.client_name }}</button>
            <span class="rank-count">{{ a.count }}</span>
          </div>
          <n-button size="small" text class="ns-mt-2" @click="router.push('/admin/apps')">查看全部应用 →</n-button>
        </template>
        <n-empty v-else description="暂无应用" size="small" />
      </n-card>
    </div>

    <!-- 最近审计 -->
    <h2 class="ns-h6 ns-mt-4 ns-mb-3">最近审计</h2>
    <n-table :bordered="true" size="small" class="docs-table">
      <thead>
        <tr>
          <th>时间</th>
          <th>事件</th>
          <th>user_id</th>
          <th>client_id</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(ev, i) in recentAudit" :key="i">
          <td class="audit-cell ts">{{ formatTime(ev.ts) }}</td>
          <td class="audit-cell"><code class="audit-event">{{ ev.event }}</code></td>
          <td class="audit-cell ts">{{ ev.user_id || '—' }}</td>
          <td class="audit-cell ts">{{ ev.client_id || '—' }}</td>
        </tr>
      </tbody>
    </n-table>
    <div v-if="!loading && recentAudit.length === 0" class="ns-text-muted ns-small ns-mt-2">暂无审计事件</div>
  </n-card>
</template>

<style scoped>
.admin-page-card {
  border-radius: 6px;
}

.dash-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.dash-card {
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
</style>
