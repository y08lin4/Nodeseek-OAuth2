<script setup lang="ts">
// 管理后台 · 授权记录（/admin/grants）：用户名/ID 搜索 + 应用/状态过滤 + 分页 + CSV
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  NCard,
  NButton,
  NTag,
  NInput,
  NSelect,
  NSpin,
  NEmpty,
  NTable,
  NPagination,
  useMessage,
} from 'naive-ui'
import { Download, Search } from 'lucide-vue-next'
import {
  listAdminGrants,
  listAdminUsers,
  listAdminClients,
  exportGrantsUrl,
  ApiError,
  type AdminGrant,
  type AdminClient,
} from '../../api'
import { formatTime } from './adminShared'

const message = useMessage()
const router = useRouter()

const grants = ref<AdminGrant[]>([])
const loading = ref(false)

const searchInput = ref('') // 用户名/ID 输入
const searchApplied = ref('') // 已生效的 user_id

const appFilter = ref<string | null>(null)
const statusFilter = ref<string | null>(null)

const clients = ref<AdminClient[]>([])
const appOptions = computed(() =>
  clients.value.map((c) => ({ label: c.client_name, value: c.client_id })),
)

const page = ref(1)
const perPage = 50
const paged = computed(() => {
  const start = (page.value - 1) * perPage
  return grants.value.slice(start, start + perPage)
})

// 前端过滤已交给后端（user_id/client_id/status）；分页仅作用于当前 grants 列表
const total = computed(() => grants.value.length)

async function loadClients() {
  try {
    const resp = await listAdminClients()
    clients.value = resp.clients
  } catch {
    clients.value = []
  }
}

async function loadGrants() {
  loading.value = true
  try {
    const resp = await listAdminGrants({
      user_id: searchApplied.value || undefined,
      client_id: appFilter.value ?? undefined,
      status: statusFilter.value ?? undefined,
    })
    grants.value = resp.grants
    page.value = 1
  } catch (e) {
    grants.value = []
    message.error(e instanceof ApiError ? e.message : '获取授权记录失败')
  } finally {
    loading.value = false
  }
}

// 用户名搜索：查 users 列表匹配 name/id 得 user_id 再传参
async function handleSearch() {
  const k = searchInput.value.trim()
  if (!k) {
    searchApplied.value = ''
    await loadGrants()
    return
  }
  loading.value = true
  try {
    const userResp = await listAdminUsers()
    const hit = userResp.users.find(
      (u) =>
        String(u.user_id) === k ||
        (u.nickname && u.nickname.toLowerCase() === k.toLowerCase()),
    )
    searchApplied.value = hit ? hit.user_id : 'nomatch'
    await loadGrants()
    if (!hit) message.warning('未找到该用户，可尝试改用用户 ID 搜索')
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '搜索失败')
  } finally {
    loading.value = false
  }
}

function exportCsv() {
  window.open(
    exportGrantsUrl({
      user_id: searchApplied.value || undefined,
      client_id: appFilter.value ?? undefined,
      status: statusFilter.value ?? undefined,
    }),
    '_blank',
  )
}

onMounted(() => {
  loadClients()
  loadGrants()
})
</script>

<template>
  <n-card class="admin-page-card">
    <template #header>
      <div class="ns-flex ns-align-center ns-justify-between ns-w-100">
        <span class="page-title">授权记录</span>
        <n-button size="small" @click="exportCsv">
          <template #icon><Download :size="14" /></template>
          导出 CSV
        </n-button>
      </div>
    </template>

    <!-- 过滤条 -->
    <div class="grants-filters ns-mb-3 ns-flex ns-align-center ns-gap-2 ns-flex-wrap">
      <n-input
        v-model:value="searchInput"
        size="small"
        placeholder="用户名 / ID"
        style="width: 200px"
        clearable
        @keydown.enter="handleSearch"
      >
        <template #prefix><Search :size="14" /></template>
      </n-input>
      <n-button size="small" @click="handleSearch">搜索</n-button>
      <n-select
        v-model:value="appFilter"
        size="small"
        placeholder="全部应用"
        :options="appOptions"
        clearable
        style="width: 200px"
        @update:value="loadGrants"
      />
      <n-select
        v-model:value="statusFilter"
        size="small"
        placeholder="全部状态"
        :options="[
          { label: '有效', value: 'active' },
          { label: '已撤销', value: 'revoked' },
        ]"
        clearable
        style="width: 140px"
        @update:value="loadGrants"
      />
    </div>

    <!-- 表格 -->
    <n-spin :show="loading">
      <div class="admin-table">
        <NTable :bordered="true" size="small">
          <thead>
            <tr>
              <th>用户 ID</th>
              <th>昵称</th>
              <th>应用名</th>
              <th>scope</th>
              <th>授权时间</th>
              <th>状态</th>
              <th>撤销时间</th>
              <th>token 次数</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="g in paged" :key="`${g.user_id}-${g.client_id}`">
              <td>
                <n-button size="tiny" text type="primary" @click="router.push(`/admin/users/${g.user_id}`)">
                  <code class="review-client-id">{{ g.user_id }}</code>
                </n-button>
              </td>
              <td class="tbl-ellipsis" :title="g.user_name">{{ g.user_name || '—' }}</td>
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
    </n-spin>
    <n-empty v-if="!loading && grants.length === 0" description="暂无授权记录" size="small" class="ns-py-3" />

    <!-- 分页 -->
    <div class="ns-mt-3" v-if="total > perPage">
      <n-pagination v-model:page="page" :page-size="perPage" :item-count="total" size="small" />
    </div>
  </n-card>
</template>

<style scoped>
.admin-page-card {
  border-radius: 6px;
}
</style>
