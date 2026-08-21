<script setup lang="ts">
// 我的授权页（需登录）：
// - 登录检查：未登录（302 / 401 / 403）→ 跳 /login?next=当前页（与 /console 同款）
// - GET /api/grants 展示授权过的应用列表
// - active 项「撤销授权」（useDialog 确认）→ POST /api/grants/{id}/revoke → 刷新
import { onMounted, ref } from 'vue'
import { NCard, NButton, NEmpty, NSpin, NTag, useMessage, useDialog } from 'naive-ui'
import { listGrants, revokeGrant, ApiError, type Grant } from '../api'

const message = useMessage()
const dialog = useDialog()

const grants = ref<Grant[]>([])
const loading = ref(true)
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
    message.error(e instanceof ApiError ? e.message : '获取授权列表失败')
  } finally {
    loading.value = false
  }
}

// 撤销授权：useDialog 确认 → POST /api/grants/{id}/revoke → 提示并刷新
function handleRevoke(grant: Grant) {
  dialog.warning({
    title: '撤销授权',
    content: `确定撤销对「${grant.client_name}」的授权？该应用的访问令牌将立即失效。`,
    positiveText: '撤销',
    negativeText: '取消',
    onPositiveClick: async () => {
      revokingId.value = grant.client_id
      try {
        await revokeGrant(grant.client_id)
        message.success('已撤销，该应用的访问令牌已作废')
        await loadGrants()
      } catch (e) {
        message.error(e instanceof ApiError ? e.message : '撤销失败，请重试')
      } finally {
        revokingId.value = ''
      }
    },
  })
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
  <n-card class="page-card">
    <template #header>
      <span class="page-title">我的授权</span>
    </template>
    <p class="ns-card-sub">管理你授权给第三方应用的身份访问权限</p>

    <n-spin :show="loading">
      <n-empty
        v-if="!loading && grants.length === 0"
        description="还没有授权任何应用。"
        class="ns-py-4"
      />
      <div v-else class="review-list">
        <n-card v-for="g in grants" :key="g.client_id" size="small" class="review-item">
          <div class="ns-flex ns-align-center ns-gap-3">
            <img v-if="g.icon_url" :src="g.icon_url" alt="应用图标" class="client-icon" />
            <div v-else class="client-icon client-icon-placeholder">A</div>
            <div class="ns-flex-grow-1">
              <div class="review-name">{{ g.client_name }}</div>
              <div class="ns-text-muted ns-small">授权时间：{{ formatTime(g.granted_at) }}</div>
            </div>
            <n-tag :type="g.min_rank > 0 ? 'warning' : 'default'" size="small" round>
              {{ g.min_rank > 0 ? `最低等级 ${g.min_rank}` : '不限等级' }}
            </n-tag>
            <!-- 状态徽章：active 有效 / revoked 已撤销 -->
            <n-tag :type="g.status === 'active' ? 'success' : 'default'" size="small" round>
              {{ g.status === 'active' ? '有效' : '已撤销' }}
            </n-tag>
          </div>
          <!-- 仅 active 授权可撤销 -->
          <div v-if="g.status === 'active'" class="review-actions">
            <n-button
              size="small"
              type="error"
              :loading="revokingId === g.client_id"
              @click="handleRevoke(g)"
            >
              撤销授权
            </n-button>
          </div>
        </n-card>
      </div>
    </n-spin>
  </n-card>
</template>
