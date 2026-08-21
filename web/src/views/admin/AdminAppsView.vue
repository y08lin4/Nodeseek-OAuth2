<script setup lang="ts">
// 管理后台 · 应用管理：表格列表 + 暂停/恢复/调整 token_ttl/reset-secret/详情模态/强制删除
import { onMounted, ref } from 'vue'
import {
  NCard,
  NButton,
  NTag,
  NSpin,
  NEmpty,
  NTable,
  NModal,
  NAlert,
  useMessage,
  useDialog,
} from 'naive-ui'
import { RefreshCw, ExternalLink, Info, Pause, Play, Trash2 } from 'lucide-vue-next'
import {
  listAdminClients,
  patchAdminClient,
  deleteAdminClient,
  resetClientSecret,
  ApiError,
  type AdminClient,
  type ClientStatus,
} from '../../api'
import { clientStatusText, clientStatusClass, ttlToMinutes } from './adminShared'

const message = useMessage()
const dialog = useDialog()

const clients = ref<AdminClient[]>([])
const clientsLoading = ref(false)
const clientActingId = ref('')

// 详情模态
const detailClient = ref<AdminClient | null>(null)
const detailOpen = ref(false)

// Secret 重置一次展示
const secretModalOpen = ref(false)
const newSecret = ref('')
const secretClientName = ref('')

const communityUrl = (id: string) => `https://www.nodeseek.com/space/${id}`

async function loadClients() {
  clientsLoading.value = true
  try {
    const resp = await listAdminClients()
    clients.value = resp.clients
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '获取应用列表失败')
  } finally {
    clientsLoading.value = false
  }
}

async function setClientStatus(client: AdminClient, status: ClientStatus) {
  clientActingId.value = client.client_id
  try {
    await patchAdminClient(client.client_id, { status })
    message.success(status === 'paused' ? '已暂停' : '已恢复')
    await loadClients()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '操作失败')
  } finally {
    clientActingId.value = ''
  }
}

function handlePause(client: AdminClient) {
  dialog.warning({
    title: '暂停应用',
    content: `确认暂停「${client.client_name}」？暂停后该应用无法发起授权。`,
    positiveText: '暂停',
    negativeText: '取消',
    onPositiveClick: () => setClientStatus(client, 'paused'),
  })
}

function handleResume(client: AdminClient) {
  dialog.warning({
    title: '恢复应用',
    content: `确认恢复「${client.client_name}」？`,
    positiveText: '恢复',
    negativeText: '取消',
    onPositiveClick: () => setClientStatus(client, 'approved'),
  })
}

function handleResetSecret(client: AdminClient) {
  dialog.warning({
    title: '重置密钥',
    content: `确认重置应用「${client.client_name}」的 Client Secret？重置后旧 secret 立即失效。`,
    positiveText: '重置',
    negativeText: '取消',
    onPositiveClick: async () => {
      clientActingId.value = client.client_id
      try {
        const resp = await resetClientSecret(client.client_id)
        newSecret.value = resp.client_secret
        secretClientName.value = client.client_name
        secretModalOpen.value = true
      } catch (e) {
        message.error(e instanceof ApiError ? e.message : '重置失败')
      } finally {
        clientActingId.value = ''
      }
    },
  })
}

async function copySecret() {
  try {
    await navigator.clipboard.writeText(newSecret.value)
    message.success('已复制')
  } catch {
    message.error('复制失败，请手动选择复制')
  }
}

function openDetail(client: AdminClient) {
  detailClient.value = client
  detailOpen.value = true
}

function handleForceDelete(client: AdminClient) {
  dialog.warning({
    title: '强制删除应用',
    content: `确认强制删除应用「${client.client_name}」（${client.client_id}）？\n该操作不可撤销，将删除应用及其授权记录。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      clientActingId.value = client.client_id
      try {
        await deleteAdminClient(client.client_id)
        message.success('应用已删除')
        detailOpen.value = false
        await loadClients()
      } catch (e) {
        message.error(e instanceof ApiError ? e.message : '删除失败')
      } finally {
        clientActingId.value = ''
      }
    },
  })
}

onMounted(loadClients)
</script>

<template>
  <n-card class="admin-page-card">
    <template #header>
      <span class="page-title">应用管理</span>
    </template>
    <p class="ns-card-sub">全部第三方应用：状态 / token 有效期 / 授权统计 / 密钥管理</p>

    <div class="ns-flex ns-align-center ns-gap-2 ns-mb-3">
      <n-button size="small" :loading="clientsLoading" @click="loadClients">刷新</n-button>
    </div>

    <n-spin :show="clientsLoading">
      <n-empty v-if="!clientsLoading && clients.length === 0" description="暂无应用" size="small" class="ns-py-3" />
      <div v-else class="admin-table">
        <NTable :bordered="true" size="small">
          <thead>
            <tr>
              <th>应用</th>
              <th>创建者</th>
              <th>状态</th>
              <th>等级门槛</th>
              <th>token 时长</th>
              <th>今日授权</th>
              <th>累计授权</th>
              <th class="tbl-act">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="c in clients" :key="c.client_id">
              <td>
                <div class="ns-flex ns-align-center ns-gap-2">
                  <span class="app-name" :title="c.client_name">{{ c.client_name }}</span>
                  <code class="review-client-id">{{ c.client_id }}</code>
                </div>
              </td>
              <td>
                <span class="ns-flex ns-align-center ns-gap-1">
                  {{ c.owner_name || c.owner_user_id }}
                  <n-button
                    size="tiny"
                    text
                    tag="a"
                    :href="communityUrl(c.owner_user_id)"
                    target="_blank"
                    rel="noopener noreferrer"
                    title="社区主页"
                  >
                    <template #icon><ExternalLink :size="13" /></template>
                  </n-button>
                </span>
              </td>
              <td>
                <n-tag :type="clientStatusClass(c.status)" size="small" round>
                  {{ clientStatusText(c.status) }}
                </n-tag>
              </td>
              <td class="tbl-num">{{ c.min_rank > 0 ? `≥ ${c.min_rank}` : '不限' }}</td>
              <td class="tbl-num">{{ ttlToMinutes(c.token_ttl) }} 分钟</td>
              <td class="tbl-num" :title="`成功 ${c.stats.auth_ok_today} / 失败 ${c.stats.auth_fail_today}`">
                {{ c.stats.auth_ok_today }} / {{ c.stats.auth_fail_today }}
              </td>
              <td class="tbl-num" :title="`成功 ${c.stats.auth_ok_total} / 失败 ${c.stats.auth_fail_total}`">
                {{ c.stats.auth_ok_total }} / {{ c.stats.auth_fail_total }}
              </td>
              <td class="tbl-act">
                <div class="admin-btn-group app-actions">
                  <n-button size="tiny" text :loading="clientActingId === c.client_id" @click="openDetail(c)" title="详情">
                    <template #icon><Info :size="15" /></template>
                  </n-button>
                  <n-button
                    v-if="c.status === 'approved'"
                    size="tiny"
                    type="warning"
                    text
                    :disabled="!!clientActingId"
                    @click="handlePause(c)"
                    title="暂停"
                  >
                    <template #icon><Pause :size="15" /></template>
                  </n-button>
                  <n-button
                    v-if="c.status === 'paused'"
                    size="tiny"
                    type="success"
                    text
                    :disabled="!!clientActingId"
                    @click="handleResume(c)"
                    title="恢复"
                  >
                    <template #icon><Play :size="15" /></template>
                  </n-button>
                  <n-button size="tiny" text type="primary" :disabled="!!clientActingId" @click="handleResetSecret(c)" title="重置密钥">
                    <template #icon><RefreshCw :size="15" /></template>
                  </n-button>
                  <n-button size="tiny" text type="error" :disabled="!!clientActingId" @click="handleForceDelete(c)" title="删除">
                    <template #icon><Trash2 :size="15" /></template>
                  </n-button>
                </div>
              </td>
            </tr>
          </tbody>
        </NTable>
      </div>
    </n-spin>

    <!-- 详情模态 -->
    <n-modal v-model:show="detailOpen" preset="card" :title="detailClient?.client_name" style="max-width: 620px">
      <div v-if="detailClient" class="detail-body">
        <n-alert v-if="detailClient.status === 'paused'" type="warning" class="ns-mb-3">该应用已暂停。</n-alert>
        <div class="detail-row">
          <span class="detail-label">Client ID</span>
          <span class="detail-value"><code>{{ detailClient.client_id }}</code></span>
        </div>
        <div class="detail-row">
          <span class="detail-label">创建者</span>
          <span class="detail-value">
            {{ detailClient.owner_name || detailClient.owner_user_id }}
            <n-button
              size="tiny"
              text
              tag="a"
              :href="communityUrl(detailClient.owner_user_id)"
              target="_blank"
              rel="noopener noreferrer"
            >
              <template #icon><ExternalLink :size="13" /></template>社区主页
            </n-button>
          </span>
        </div>
        <div class="detail-row">
          <span class="detail-label">图标 URL</span>
          <span class="detail-value">
            <a v-if="detailClient.icon_url" :href="detailClient.icon_url" target="_blank" rel="noopener noreferrer" class="url-link">{{ detailClient.icon_url }}</a>
            <span v-else>—</span>
          </span>
        </div>
        <div class="detail-row">
          <span class="detail-label">主页</span>
          <span class="detail-value">
            <a v-if="detailClient.homepage_url" :href="detailClient.homepage_url" target="_blank" rel="noopener noreferrer" class="url-link">{{ detailClient.homepage_url }}</a>
            <span v-else>—</span>
          </span>
        </div>
        <div class="detail-row">
          <span class="detail-label">回调地址</span>
          <span class="detail-value">
            <ul v-if="detailClient.redirect_uris.length" class="uri-list">
              <li v-for="u in detailClient.redirect_uris" :key="u"><code>{{ u }}</code></li>
            </ul>
            <span v-else>—</span>
          </span>
        </div>
        <div class="detail-row" v-if="detailClient.description">
          <span class="detail-label">描述</span>
          <span class="detail-value">{{ detailClient.description }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">等级门槛</span>
          <span class="detail-value">{{ detailClient.min_rank > 0 ? `≥ ${detailClient.min_rank}` : '不限' }}</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">token 时长</span>
          <span class="detail-value">{{ ttlToMinutes(detailClient.token_ttl) }} 分钟</span>
        </div>
        <div class="detail-row">
          <span class="detail-label">授权统计</span>
          <span class="detail-value">
            今日 成功 {{ detailClient.stats.auth_ok_today }} / 失败 {{ detailClient.stats.auth_fail_today }}
            ｜ 累计 成功 {{ detailClient.stats.auth_ok_total }} / 失败 {{ detailClient.stats.auth_fail_total }}
          </span>
        </div>
      </div>
    </n-modal>

    <!-- 新 secret 一次性展示 -->
    <n-modal v-model:show="secretModalOpen" preset="card" title="新密钥已生成" style="max-width: 480px">
      <n-alert type="warning" class="ns-mb-3">
        密钥仅本次显示一次，关闭后无法再次查看，请立即复制保存。
      </n-alert>
      <div class="secret-box">
        <code>{{ newSecret }}</code>
      </div>
      <template #footer>
        <div class="admin-btn-group ns-justify-end">
          <n-button @click="secretModalOpen = false">关闭</n-button>
          <n-button type="primary" @click="copySecret">复制</n-button>
        </div>
      </template>
    </n-modal>
  </n-card>
</template>

<style scoped>
.admin-page-card {
  border-radius: 6px;
}

.app-actions {
  /* 继承全局 .admin-btn-group 的 nowrap：操作按钮恒横排，不竖排堆叠 */
}

.app-name {
  font-weight: 600;
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-row {
  display: flex;
  gap: 12px;
  padding: 7px 0;
  border-bottom: 1px solid var(--ns-border);
  font-size: 13px;
}

.detail-label {
  flex: 0 0 88px;
  color: var(--ns-muted);
}

.detail-value {
  flex: 1;
  min-width: 0;
  word-break: break-all;
}

.uri-list {
  margin: 0;
  padding-left: 16px;
  color: var(--ns-text);
}

.url-link {
  color: var(--ns-primary);
  word-break: break-all;
}

.secret-box {
  background: var(--ns-bg);
  border: 1px dashed var(--ns-border);
  border-radius: 6px;
  padding: 12px;
  font-size: 13px;
  word-break: break-all;
}
</style>
