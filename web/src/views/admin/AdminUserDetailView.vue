<script setup lang="ts">
// 管理后台 · 用户详情（/admin/users/:id）：实时 NS stats + 今日/累计统计 + 授权记录 + 拉黑
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NCard, NButton, NTag, NSpin, NTable, useMessage, useDialog } from 'naive-ui'
import { ExternalLink, Ban, ShieldOff, ArrowLeft } from 'lucide-vue-next'
import {
  getAdminUserDetail,
  getAdminUserStats,
  listAdminGrants,
  patchAdminUser,
  ApiError,
  type AdminUserDetail,
  type AdminUserStats,
  type AdminGrant,
} from '../../api'
import { formatTime } from './adminShared'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()

const userId = String(route.params.id)
const detail = ref<AdminUserDetail | null>(null)
const stats = ref<AdminUserStats | null>(null)
const grants = ref<AdminGrant[]>([])
const loading = ref(false)
const acting = ref(false)

const communityUrl = (id: string) => `https://www.nodeseek.com/space/${id}`

async function loadAll() {
  loading.value = true
  await Promise.allSettled([loadDetail(), loadStats(), loadGrants()])
  loading.value = false
}

async function loadDetail() {
  try {
    detail.value = await getAdminUserDetail(userId)
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '获取用户详情失败')
  }
}

async function loadStats() {
  try {
    stats.value = await getAdminUserStats(userId)
  } catch (e) {
    stats.value = null
    message.error(e instanceof ApiError ? e.message : '获取统计失败')
  }
}

async function loadGrants() {
  try {
    const resp = await listAdminGrants({ user_id: userId })
    grants.value = resp.grants
  } catch (e) {
    grants.value = []
    message.error(e instanceof ApiError ? e.message : '获取授权记录失败')
  }
}

function handleBlacklist() {
  if (!detail.value) return
  const u = detail.value
  dialog.warning({
    title: u.blacklisted ? '解除拉黑' : '拉黑用户',
    content: u.blacklisted
      ? `确认解除对「${u.nickname}」（${u.user_id}）的拉黑？`
      : `确认拉黑「${u.nickname}」（${u.user_id}）？拉黑将吊销其全部授权与令牌。`,
    positiveText: u.blacklisted ? '解除拉黑' : '拉黑',
    negativeText: '取消',
    onPositiveClick: async () => {
      acting.value = true
      try {
        await patchAdminUser(u.user_id, { blacklisted: !u.blacklisted })
        message.success(u.blacklisted ? '已解除拉黑' : '已拉黑')
        await loadDetail()
      } catch (e) {
        message.error(e instanceof ApiError ? e.message : '操作失败')
      } finally {
        acting.value = false
      }
    },
  })
}

onMounted(loadAll)
</script>

<template>
  <div>
    <n-button size="small" text class="ns-mb-3" @click="router.push('/admin/users')">
      <template #icon><ArrowLeft :size="14" /></template>
      返回用户列表
    </n-button>

    <n-spin :show="loading">
      <!-- 头部卡 -->
      <n-card class="admin-page-card ns-mb-4" v-if="detail">
        <div class="u-header">
          <div class="ns-flex ns-align-center ns-gap-2 ns-flex-wrap">
            <span class="u-id">{{ detail.user_id }}</span>
            <span class="u-nick">{{ detail.nickname || '—' }}</span>
            <n-tag v-if="detail.blacklisted" type="error" size="small" round>已拉黑</n-tag>
            <n-tag v-else type="success" size="small" round>正常</n-tag>
          </div>
          <div class="u-meta ns-mt-2 ns-text-muted ns-small">
            等级 {{ detail.rank ?? '—' }} · 注册 {{ detail.signup_days != null ? `${detail.signup_days} 天` : '—' }}
            · 鸡腿 {{ detail.chicken ?? '—' }} · 主题 {{ detail.topics ?? '—' }} · 评论 {{ detail.comments ?? '—' }}
          </div>
          <div class="u-actions ns-mt-3 admin-btn-group">
            <n-button size="small" tag="a" :href="communityUrl(detail.user_id)" target="_blank" rel="noopener noreferrer">
              <template #icon><ExternalLink :size="14" /></template>
              社区主页
            </n-button>
            <n-button
              size="small"
              :type="detail.blacklisted ? 'default' : 'error'"
              :loading="acting"
              @click="handleBlacklist"
            >
              <template #icon><Ban v-if="!detail.blacklisted" :size="14" /><ShieldOff v-else :size="14" /></template>
              {{ detail.blacklisted ? '解除拉黑' : '拉黑' }}
            </n-button>
          </div>
        </div>
      </n-card>

      <!-- 统计卡：今日 / 累计 登录与授权 -->
      <n-card class="admin-page-card ns-mb-4" title="登录与授权统计">
        <div class="admin-stats">
          <div class="admin-stat-card">
            <div class="admin-stat-value">{{ stats?.today.login_ok ?? '—' }}</div>
            <div class="admin-stat-label">今日登录成功</div>
          </div>
          <div class="admin-stat-card">
            <div class="admin-stat-value">{{ stats?.today.login_fail ?? '—' }}</div>
            <div class="admin-stat-label">今日登录失败</div>
          </div>
          <div class="admin-stat-card">
            <div class="admin-stat-value">{{ stats?.today.auth_ok ?? '—' }}</div>
            <div class="admin-stat-label">今日授权成功</div>
          </div>
          <div class="admin-stat-card">
            <div class="admin-stat-value">{{ stats?.today.auth_fail ?? '—' }}</div>
            <div class="admin-stat-label">今日授权失败</div>
          </div>
        </div>
        <div class="admin-stats ns-mt-3">
          <div class="admin-stat-card">
            <div class="admin-stat-value">{{ stats?.total.login_ok ?? '—' }}</div>
            <div class="admin-stat-label">累计登录成功</div>
          </div>
          <div class="admin-stat-card">
            <div class="admin-stat-value">{{ stats?.total.login_fail ?? '—' }}</div>
            <div class="admin-stat-label">累计登录失败</div>
          </div>
          <div class="admin-stat-card">
            <div class="admin-stat-value">{{ stats?.total.auth_ok ?? '—' }}</div>
            <div class="admin-stat-label">累计授权成功</div>
          </div>
          <div class="admin-stat-card">
            <div class="admin-stat-value">{{ stats?.total.auth_fail ?? '—' }}</div>
            <div class="admin-stat-label">累计授权失败</div>
          </div>
        </div>
      </n-card>

      <!-- 授权记录区 -->
      <n-card class="admin-page-card" title="授权记录">
        <div class="admin-table">
          <NTable :bordered="true" size="small">
            <thead>
              <tr>
                <th>应用名</th>
                <th>scope</th>
                <th>授权时间</th>
                <th>状态</th>
                <th>撤销时间</th>
                <th>token 次数</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="g in grants" :key="`${g.user_id}-${g.client_id}`">
                <td class="tbl-ellipsis" :title="g.client_name">{{ g.client_name || '—' }}</td>
                <td class="tbl-ellipsis" :title="g.scope">{{ g.scope || '—' }}</td>
                <td>{{ formatTime(g.granted_at) }}</td>
                <td>
                  <n-tag :type="g.status === 'active' ? 'success' : 'default'" size="small" round>
                    {{ g.status === 'active' ? '有效' : '已撤销' }}
                  </n-tag>
                </td>
                <td>{{ g.revoked_at ? formatTime(g.revoked_at) : '—' }}</td>
                <td class="tbl-num">{{ g.token_count ?? '—' }}</td>
              </tr>
            </tbody>
          </NTable>
        </div>
        <n-empty v-if="!loading && grants.length === 0" description="该用户暂无授权记录" size="small" class="ns-py-3" />
      </n-card>
    </n-spin>
  </div>
</template>

<style scoped>
.admin-page-card {
  border-radius: 6px;
}

.u-id {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
  font-size: 20px;
  font-weight: 700;
  color: var(--ns-text);
}

.u-nick {
  font-size: 16px;
  color: var(--ns-muted);
}

.admin-btn-group {
  display: flex;
  gap: 8px;
  align-items: center;
}
</style>
