<script setup lang="ts">
// 登录后面板（需登录，未登录跳 /login?next=/dashboard）：
// - 欢迎区：「欢迎回来，ID xxx」（GET /api/me 的 user_id）
// - 用户 stats 卡片：等级/加入天数/鸡腿/主题帖/评论（登录流程缓存的 confirm 响应 stats）
// - 快捷入口卡片：我的应用 / 我的授权 / 接入文档
// - 我的应用统计摘要：GET /api/client/list 前 3 个应用「应用名 · 今日成功 X」
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NCard, NEmpty, NSpin, useMessage } from 'naive-ui'
import { me, listClients, ApiError, type ClientListItem, type UserStats } from '../api'

const STATS_KEY = 'ns_user_stats' // 与 LoginView 缓存 key 一致
const router = useRouter()
const message = useMessage()

const userId = ref('')
const userStats = ref<UserStats | null>(null)
const topClients = ref<ClientListItem[]>([])
const loading = ref(true)

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
    message.error(e instanceof ApiError ? e.message : '获取应用统计失败')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <n-card class="page-card">
    <template #header>
      <span class="page-title">欢迎回来，ID {{ userId || '—' }}</span>
    </template>

    <!-- 用户 stats 卡片（登录流程缓存；无缓存显示占位） -->
    <h2 class="ns-h6 ns-mt-2 ns-mb-3">我的信息</h2>
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
    <n-empty
      v-else
      size="small"
      description="登录后更新（重新登录完成后即可查看等级/加入天数/鸡腿等）"
      class="ns-py-3"
    />

    <!-- 快捷入口 -->
    <h2 class="ns-h6 ns-mt-4 ns-mb-3">快捷入口</h2>
    <div class="role-grid">
      <n-card hoverable @click="router.push('/console')">
        <div class="role-icon">🧩</div>
        <h3>我的应用</h3>
        <p class="ns-mb-0 ns-text-muted ns-small">注册与管理第三方应用</p>
      </n-card>
      <n-card hoverable @click="router.push('/grants')">
        <div class="role-icon">🔑</div>
        <h3>我的授权</h3>
        <p class="ns-mb-0 ns-text-muted ns-small">查看与撤销授权</p>
      </n-card>
      <n-card hoverable @click="router.push('/docs')">
        <div class="role-icon">📖</div>
        <h3>接入文档</h3>
        <p class="ns-mb-0 ns-text-muted ns-small">第三方接入教程</p>
      </n-card>
    </div>

    <!-- 我的应用统计摘要 -->
    <h2 class="ns-h6 ns-mt-4 ns-mb-3">我的应用统计</h2>
    <n-spin :show="loading">
      <n-empty
        v-if="!loading && topClients.length === 0"
        description="还没有应用，去「申请接入」创建一个吧。"
      />
      <div v-else class="review-list">
        <n-card v-for="c in topClients" :key="c.client_id" size="small" class="review-item">
          <div class="ns-flex ns-justify-between ns-align-center">
            <span class="review-name">{{ c.client_name }}</span>
            <code class="review-client-id">{{ c.client_id }}</code>
          </div>
          <div class="ns-text-muted ns-small">{{ c.client_name }} · 今日成功 {{ c.stats.auth_ok_today }}</div>
        </n-card>
      </div>
    </n-spin>
  </n-card>
</template>
