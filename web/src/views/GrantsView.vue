<script setup lang="ts">
// 我的授权页（需登录）：
// - 登录检查：未登录（302 / 401 / 403）→ 跳 /login?next=当前页（与 /console 同款）
// - GET /api/grants 展示授权过的应用列表
// - active 项「撤销授权」（confirm）→ POST /api/grants/{id}/revoke → 刷新
import { onMounted, ref } from 'vue'
import { listGrants, revokeGrant, ApiError, type Grant } from '../api'

const grants = ref<Grant[]>([])
const loading = ref(true)
const error = ref('')
const successMsg = ref('')
const revokingId = ref('') // 正在撤销授权的 client_id

// 登录检查：未登录（302 / 401 / 403）→ 跳登录页并带回跳地址
async function checkLogin(): Promise<boolean> {
  try {
    const res = await fetch('/api/grants', {
      credentials: 'same-origin',
      redirect: 'manual',
    })
    if (res.type === 'opaqueredirect' || res.status === 401 || res.status === 403) {
      window.location.href = `/login?next=${encodeURIComponent(window.location.href)}`
      return false
    }
    return true
  } catch {
    return true // 网络异常不跳转，交给列表加载展示错误
  }
}

async function loadGrants() {
  loading.value = true
  try {
    const resp = await listGrants()
    grants.value = resp.grants
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '获取授权列表失败'
  } finally {
    loading.value = false
  }
}

// 撤销授权：confirm → POST /api/grants/{id}/revoke → 提示并刷新
async function handleRevoke(grant: Grant) {
  if (!window.confirm(`确定撤销对「${grant.client_name}」的授权？该应用的访问令牌将立即失效。`)) {
    return
  }
  error.value = ''
  revokingId.value = grant.client_id
  try {
    await revokeGrant(grant.client_id)
    successMsg.value = '已撤销，该应用的访问令牌已作废'
    await loadGrants()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '撤销失败，请重试'
  } finally {
    revokingId.value = ''
  }
}

// 授权时间本地化显示
function formatTime(s: string): string {
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? s : d.toLocaleString('zh-CN')
}

onMounted(async () => {
  if (await checkLogin()) {
    await loadGrants()
  }
})
</script>

<template>
  <div class="ns-card">
    <h1 class="ns-card-title">我的授权</h1>
    <p class="ns-card-sub">管理你授权给第三方应用的身份访问权限</p>

    <div v-if="error" class="ns-alert ns-alert-danger">{{ error }}</div>
    <div v-if="successMsg" class="ns-alert ns-alert-success">{{ successMsg }}</div>

    <div v-if="loading" class="text-center text-muted py-4">加载中…</div>
    <div v-else-if="grants.length === 0" class="text-center text-muted py-4">
      还没有授权任何应用。
    </div>
    <div v-else class="client-list">
      <div v-for="g in grants" :key="g.client_id" class="client-card">
        <div class="client-card-head">
          <img v-if="g.icon_url" :src="g.icon_url" alt="应用图标" class="client-icon" />
          <div v-else class="client-icon client-icon-placeholder">A</div>
          <div>
            <div class="client-name">{{ g.client_name }}</div>
            <div class="text-muted small">授权时间：{{ formatTime(g.granted_at) }}</div>
          </div>
          <div class="ms-auto d-flex align-items-center gap-2">
            <span class="badge-set" :class="{ 'badge-unset': g.min_rank > 0 }">
              {{ g.min_rank > 0 ? `最低等级 ${g.min_rank}` : '不限等级' }}
            </span>
            <!-- 状态徽章：active 有效 / revoked 已撤销 -->
            <span class="badge" :class="g.status === 'active' ? 'badge-set' : 'badge-muted'">
              {{ g.status === 'active' ? '有效' : '已撤销' }}
            </span>
          </div>
        </div>
        <!-- 仅 active 授权可撤销 -->
        <div v-if="g.status === 'active'" class="client-card-actions">
          <button
            class="btn btn-sm btn-outline-danger"
            :disabled="revokingId === g.client_id"
            @click="handleRevoke(g)"
          >
            {{ revokingId === g.client_id ? '撤销中…' : '撤销授权' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
