<script setup lang="ts">
// 登录后面板（需登录，未登录跳 /login?next=/dashboard）：
// - 欢迎区：「欢迎回来，ID xxx」（GET /api/me 的 user_id）
// - 用户 stats 卡片：等级/加入天数/鸡腿/主题帖/评论（登录流程缓存的 confirm 响应 stats）
// - 快捷入口卡片：我的应用 / 我的授权 / 接入文档
// - 我的应用统计摘要：GET /api/client/list 前 3 个应用「应用名 · 今日成功 X」
import { onMounted, ref } from 'vue'
import { me, listClients, ApiError, type ClientListItem, type UserStats } from '../api'

const STATS_KEY = 'ns_user_stats' // 与 LoginView 缓存 key 一致

const userId = ref('')
const userStats = ref<UserStats | null>(null)
const topClients = ref<ClientListItem[]>([])
const loading = ref(true)
const error = ref('')

// 读取登录流程缓存的 stats（无缓存 / 脏数据 → null）
function readCachedStats(): UserStats | null {
  try {
    const raw = sessionStorage.getItem(STATS_KEY)
    if (!raw) return null
    const obj = JSON.parse(raw) as UserStats
    if (obj && typeof obj.rank === 'number' && typeof obj.join_days === 'number') return obj
    return null
  } catch {
    return null
  }
}

// 登录检查：未登录（302 / 401 / 403）→ 跳登录页并带回跳地址
async function checkLogin(): Promise<boolean> {
  try {
    const res = await fetch('/api/client/list', {
      credentials: 'same-origin',
      redirect: 'manual',
    })
    if (res.type === 'opaqueredirect' || res.status === 401 || res.status === 403) {
      window.location.href = `/login?next=${encodeURIComponent(window.location.href)}`
      return false
    }
    return true
  } catch {
    return true // 网络异常不跳转，交给下方加载展示错误
  }
}

onMounted(async () => {
  if (!(await checkLogin())) return
  userStats.value = readCachedStats()
  try {
    const meResp = await me()
    userId.value = meResp?.user_id ?? ''
  } catch {
    // 欢迎语拿不到 ID 不阻塞面板
  }
  try {
    const resp = await listClients()
    topClients.value = resp.clients.slice(0, 3) // 统计摘要取前 3 个
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '获取应用统计失败'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="ns-card">
    <h1 class="ns-card-title">欢迎回来，ID {{ userId || '—' }}</h1>
    <p class="ns-card-sub">管理你的应用与授权</p>

    <div v-if="error" class="ns-alert ns-alert-danger">{{ error }}</div>

    <!-- 用户 stats 卡片（登录流程缓存；无缓存显示占位） -->
    <h2 class="h6 mt-4 mb-3">我的信息</h2>
    <div v-if="userStats" class="stats-grid">
      <div class="stat-item">
        <div class="stat-value">{{ userStats.rank }}</div>
        <div class="stat-label">等级</div>
      </div>
      <div class="stat-item">
        <div class="stat-value">{{ userStats.join_days }}</div>
        <div class="stat-label">加入天数</div>
      </div>
      <div class="stat-item">
        <div class="stat-value">{{ userStats.chicken }}</div>
        <div class="stat-label">鸡腿</div>
      </div>
      <div class="stat-item">
        <div class="stat-value">{{ userStats.topics }}</div>
        <div class="stat-label">主题帖</div>
      </div>
      <div class="stat-item">
        <div class="stat-value">{{ userStats.comments }}</div>
        <div class="stat-label">评论</div>
      </div>
    </div>
    <div v-else class="text-muted text-center py-3">登录后更新（重新登录完成后即可查看等级/加入天数/鸡腿等）</div>

    <!-- 快捷入口 -->
    <h2 class="h6 mt-4 mb-3">快捷入口</h2>
    <div class="role-grid">
      <RouterLink to="/console" class="role-card role-link">
        <div class="role-icon">🧩</div>
        <h3>我的应用</h3>
        <p class="mb-0 text-muted small">注册与管理第三方应用</p>
      </RouterLink>
      <RouterLink to="/grants" class="role-card role-link">
        <div class="role-icon">🔑</div>
        <h3>我的授权</h3>
        <p class="mb-0 text-muted small">查看与撤销授权</p>
      </RouterLink>
      <RouterLink to="/docs" class="role-card role-link">
        <div class="role-icon">📖</div>
        <h3>接入文档</h3>
        <p class="mb-0 text-muted small">第三方接入教程</p>
      </RouterLink>
    </div>

    <!-- 我的应用统计摘要 -->
    <h2 class="h6 mt-4 mb-3">我的应用统计</h2>
    <div v-if="loading" class="text-muted text-center py-3">加载中…</div>
    <div v-else-if="topClients.length === 0" class="text-muted text-center py-3">
      还没有应用，去「申请接入」创建一个吧。
    </div>
    <div v-else class="review-list">
      <div v-for="c in topClients" :key="c.client_id" class="review-item">
        <div class="review-item-head">
          <span class="review-name">{{ c.client_name }}</span>
          <code class="review-client-id">{{ c.client_id }}</code>
        </div>
        <div class="text-muted small">{{ c.client_name }} · 今日成功 {{ c.stats.auth_ok_today }}</div>
      </div>
    </div>
  </div>
</template>
