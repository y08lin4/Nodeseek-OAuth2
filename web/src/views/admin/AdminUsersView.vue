<script setup lang="ts">
// 管理后台 · 用户：列表 / 搜索（ID/昵称前端过滤）/ 拉黑解禁 / 社区主页 / CSV 导出
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NTag, NInput, NSpin, NEmpty, NTable, useMessage, useDialog } from 'naive-ui'
import { ExternalLink, Ban, ShieldOff, Download, Search } from 'lucide-vue-next'
import {
  listAdminUsers,
  patchAdminUser,
  exportUsersUrl,
  ApiError,
  type AdminUser,
} from '../../api'

const message = useMessage()
const dialog = useDialog()
const router = useRouter()

const users = ref<AdminUser[]>([])
const loading = ref(false)
const actingId = ref('')
const keyword = ref('') // 搜索：ID/昵称前端过滤

const communityUrl = (id: string) => `https://www.nodeseek.com/space/${id}`

async function loadUsers() {
  loading.value = true
  try {
    const resp = await listAdminUsers()
    users.value = resp.users
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '获取用户列表失败')
  } finally {
    loading.value = false
  }
}

// 摘要
const summary = computed(() => ({
  total: users.value.length,
  active: users.value.reduce((s, u) => s + (u.active_today ?? 0), 0),
  login: users.value.reduce((s, u) => s + (u.login_today ?? 0), 0),
}))

// 前端过滤
const filtered = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return users.value
  return users.value.filter(
    (u) =>
      String(u.user_id).toLowerCase().includes(k) ||
      (u.nickname || '').toLowerCase().includes(k),
  )
})

function handleBlacklist(user: AdminUser) {
  dialog.warning({
    title: user.blacklisted ? '解除拉黑' : '拉黑用户',
    content: user.blacklisted
      ? `确认解除对「${user.nickname}」（${user.user_id}）的拉黑？`
      : `确认拉黑「${user.nickname}」（${user.user_id}）？拉黑将吊销其全部授权与令牌。`,
    positiveText: user.blacklisted ? '解除拉黑' : '拉黑',
    negativeText: '取消',
    onPositiveClick: async () => {
      actingId.value = user.user_id
      try {
        await patchAdminUser(user.user_id, { blacklisted: !user.blacklisted })
        message.success(user.blacklisted ? '已解除拉黑' : '已拉黑')
        await loadUsers()
      } catch (e) {
        message.error(e instanceof ApiError ? e.message : '操作失败')
      } finally {
        actingId.value = ''
      }
    },
  })
}

// CSV 导出：同源凭 Cookie，直接打开下载链接
function exportCsv() {
  window.open(exportUsersUrl(keyword.value.trim() ? { q: keyword.value.trim() } : {}), '_blank')
}

onMounted(loadUsers)
</script>

<template>
  <!-- 页头 -->
  <div class="page-head">
    <h2 class="page-title">用户管理</h2>
    <div class="page-actions">
      <n-button size="small" :loading="loading" @click="loadUsers">刷新</n-button>
      <n-button size="small" @click="exportCsv">
        <template #icon><Download :size="14" /></template>
        导出 CSV
      </n-button>
    </div>
  </div>

  <!-- 摘要卡 -->
  <div class="admin-stats ns-mb-4">
      <div class="admin-stat-card">
        <div class="admin-stat-value">{{ summary.total }}</div>
        <div class="admin-stat-label">总用户</div>
      </div>
      <div class="admin-stat-card">
        <div class="admin-stat-value">{{ summary.active }}</div>
        <div class="admin-stat-label">今日活跃</div>
      </div>
      <div class="admin-stat-card">
        <div class="admin-stat-value">{{ summary.login }}</div>
        <div class="admin-stat-label">今日登录</div>
      </div>
    </div>

    <!-- 搜索 -->
    <div class="ns-mb-3 admin-filter-bar">
      <n-input v-model:value="keyword" size="small" placeholder="搜索 ID / 昵称" clearable>
        <template #prefix><Search :size="14" /></template>
      </n-input>
    </div>

    <!-- 表格 -->
    <n-spin :show="loading">
      <div class="admin-table">
        <NTable :bordered="true" size="small">
          <thead>
            <tr>
              <th>ID</th>
              <th>昵称</th>
              <th>等级</th>
              <th>注册天数</th>
              <th>登录次数</th>
              <th>授权数</th>
              <th>状态</th>
              <th class="tbl-act">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in filtered" :key="u.user_id">
              <td><code class="review-client-id">{{ u.user_id }}</code></td>
              <td class="tbl-ellipsis" :title="u.nickname">{{ u.nickname || '—' }}</td>
              <td class="tbl-num">{{ u.rank ?? '—' }}</td>
              <td class="tbl-num">{{ u.signup_days ?? '—' }}</td>
              <td class="tbl-num">{{ u.login_count ?? '—' }}</td>
              <td class="tbl-num">{{ u.grant_count ?? '—' }}</td>
              <td>
                <n-tag v-if="u.blacklisted" type="error" size="small" round>已拉黑</n-tag>
                <n-tag v-else type="success" size="small" round>正常</n-tag>
              </td>
              <td class="tbl-act">
                <div class="admin-btn-group">
                  <n-button size="tiny" text tag="a" :href="communityUrl(u.user_id)" target="_blank" rel="noopener noreferrer" title="社区主页">
                    <template #icon><ExternalLink :size="14" /></template>
                  </n-button>
                  <n-button size="tiny" text @click="router.push(`/admin/users/${u.user_id}`)" title="详情">详情</n-button>
                  <n-button
                    size="tiny"
                    type="error"
                    text
                    :loading="actingId === u.user_id"
                    @click="handleBlacklist(u)"
                    :title="u.blacklisted ? '解除拉黑' : '拉黑'"
                  >
                    <template #icon><Ban v-if="!u.blacklisted" :size="14" /><ShieldOff v-else :size="14" /></template>
                  </n-button>
                </div>
              </td>
            </tr>
          </tbody>
        </NTable>
      </div>
    </n-spin>
    <n-empty v-if="!loading && filtered.length === 0" description="无匹配用户" size="small" class="ns-py-3" />
</template>

<style scoped>
.admin-filter-bar {
  max-width: 320px;
}

.admin-btn-group {
  display: flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}
</style>
