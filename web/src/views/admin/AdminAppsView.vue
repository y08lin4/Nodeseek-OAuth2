<script setup lang="ts">
// 管理后台 · 应用管理：全量应用列表 + 暂停/恢复/调整 token_ttl/强制删除
import { onMounted, ref } from 'vue'
import { NCard, NButton, NTag, NInput, NSpin, NEmpty, useMessage, useDialog } from 'naive-ui'
import {
  listAdminClients,
  patchAdminClient,
  deleteAdminClient,
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
const ttlInputs = ref<Record<string, string>>({})

async function loadClients() {
  clientsLoading.value = true
  try {
    const resp = await listAdminClients()
    clients.value = resp.clients
    for (const c of resp.clients) {
      if (ttlInputs.value[c.client_id] === undefined) {
        ttlInputs.value[c.client_id] = String(ttlToMinutes(c.token_ttl))
      }
    }
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

async function handleSetTokenTtl(client: AdminClient) {
  const mins = Number(ttlInputs.value[client.client_id])
  if (!Number.isFinite(mins) || mins <= 0) {
    message.error('请输入有效的分钟数（正整数）')
    return
  }
  const seconds = Math.round(mins * 60)
  clientActingId.value = client.client_id
  try {
    await patchAdminClient(client.client_id, { token_ttl: seconds })
    message.success(`token 有效期已调整为 ${mins} 分钟`)
    await loadClients()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '调整失败')
  } finally {
    clientActingId.value = ''
  }
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
    <p class="ns-card-sub">全部第三方应用：状态 / token 有效期 / 授权统计</p>

    <div class="ns-flex ns-align-center ns-gap-2 ns-mb-3">
      <n-button size="small" :loading="clientsLoading" @click="loadClients">刷新</n-button>
    </div>

    <n-spin :show="clientsLoading">
      <n-empty
        v-if="!clientsLoading && clients.length === 0"
        description="暂无应用"
        size="small"
        class="ns-py-3"
      />
      <div v-else class="review-list">
        <n-card v-for="c in clients" :key="c.client_id" size="small" class="review-item">
          <div class="ns-flex ns-align-center ns-gap-2 ns-mb-1 ns-flex-wrap">
            <n-tag :type="clientStatusClass(c.status)" size="small" round>
              {{ clientStatusText(c.status) }}
            </n-tag>
            <span class="review-name">{{ c.client_name }}</span>
            <code class="review-client-id">{{ c.client_id }}</code>
          </div>
          <div class="ns-text-muted ns-small">
            owner: {{ c.owner_user_id }} · 等级门槛：{{ c.min_rank > 0 ? `≥ ${c.min_rank}` : '不限' }}
            · token 有效期：{{ ttlToMinutes(c.token_ttl) }} 分钟
          </div>
          <div class="ns-text-muted ns-small">
            今日 成功 {{ c.stats.auth_ok_today }} / 失败 {{ c.stats.auth_fail_today }} ｜ 累计 成功
            {{ c.stats.auth_ok_total }} / 失败 {{ c.stats.auth_fail_total }}
          </div>
          <div v-if="c.description" class="review-detail">{{ c.description }}</div>
          <div class="review-actions ns-flex ns-align-center ns-gap-2 ns-flex-wrap">
            <n-button
              v-if="c.status === 'approved'"
              size="small"
              type="warning"
              :disabled="!!clientActingId"
              :loading="clientActingId === c.client_id"
              @click="handlePause(c)"
            >
              暂停
            </n-button>
            <n-button
              v-if="c.status === 'paused'"
              size="small"
              type="success"
              :disabled="!!clientActingId"
              :loading="clientActingId === c.client_id"
              @click="handleResume(c)"
            >
              恢复
            </n-button>
            <n-input
              v-model:value="ttlInputs[c.client_id]"
              size="small"
              style="width: 90px"
              placeholder="分钟"
              :input-props="{ inputmode: 'numeric' }"
            />
            <n-button
              size="small"
              :disabled="!!clientActingId"
              :loading="clientActingId === c.client_id"
              @click="handleSetTokenTtl(c)"
            >
              调整 token 有效期
            </n-button>
            <n-button
              size="small"
              type="error"
              :disabled="!!clientActingId"
              :loading="clientActingId === c.client_id"
              @click="handleForceDelete(c)"
            >
              强制删除
            </n-button>
          </div>
        </n-card>
      </div>
    </n-spin>
  </n-card>
</template>

<style scoped>
.admin-page-card {
  border-radius: 6px;
}
</style>
