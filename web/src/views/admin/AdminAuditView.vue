<script setup lang="ts">
// 管理后台 · 审计日志：limit + 刷新 + 过滤（事件类型 / 级别 / 用户 ID，前端过滤）+ 汉化分层
import { computed, onMounted, ref } from 'vue'
import { NCard, NButton, NInputNumber, NInput, NSelect, NSpin, NTable, NEmpty, NTag, useMessage } from 'naive-ui'
import { listAudit, ApiError, type AuditEvent } from '../../api'
import {
  formatTime,
  auditEventCN,
  auditLevel,
  AUDIT_LEVEL_TEXT,
  AUDIT_LEVEL_TAG,
} from './adminShared'

const message = useMessage()

const events = ref<AuditEvent[]>([])
const auditLoading = ref(false)
const auditLimit = ref<number | null>(50)

// 过滤
const eventFilter = ref<string | null>(null)
const levelFilter = ref<string | null>(null)
const userFilter = ref('')

const eventOptions = [
  { label: '全部事件', value: 'all' },
  { label: '登录 (login.*)', value: 'login.' },
  { label: '授权 (authorize.*)', value: 'authorize.' },
  { label: '令牌 (token.*)', value: 'token.' },
  { label: '应用/审核 (client./review.)', value: 'client.' },
  { label: '管理 (admin.*)', value: 'admin.' },
  { label: '门槛/频率拦截 (gate.block/rate.limit)', value: 'gate.block' },
  { label: '邮件 (mail.*)', value: 'mail.' },
  { label: 'Cookie (cookie.*)', value: 'cookie.' },
]

const levelOptions = [
  { label: '全部级别', value: 'all' },
  { label: '正常', value: 'info' },
  { label: '警告', value: 'warn' },
  { label: '异常', value: 'error' },
]

const filtered = computed(() => {
  let list = events.value
  const ef = eventFilter.value
  if (ef && ef !== 'all') {
    if (ef === 'gate.block') {
      list = list.filter((ev) => ev.event === 'gate.block' || ev.event === 'rate.limit')
    } else {
      list = list.filter((ev) => ev.event.startsWith(ef))
    }
  }
  const lf = levelFilter.value
  if (lf && lf !== 'all') {
    list = list.filter((ev) => auditLevel(ev.event) === lf)
  }
  const k = userFilter.value.trim()
  if (k) {
    list = list.filter((ev) => String(ev.user_id || '').includes(k))
  }
  return list
})

async function loadAudit() {
  auditLoading.value = true
  try {
    const raw = Number(auditLimit.value)
    const limit = Number.isFinite(raw) ? Math.min(200, Math.max(1, Math.round(raw))) : 50
    const resp = await listAudit(limit)
    events.value = resp.events
  } catch (e) {
    events.value = []
    message.error(e instanceof ApiError ? e.message : '获取审计日志失败')
  } finally {
    auditLoading.value = false
  }
}

onMounted(loadAudit)
</script>

<template>
  <n-card class="admin-page-card">
    <template #header>
      <span class="page-title">审计日志</span>
    </template>
    <p class="ns-card-sub">系统操作审计：登录 / 授权 / 管理动作</p>

    <div class="ns-flex ns-align-center ns-gap-2 ns-mb-3 ns-flex-wrap">
      <n-input-number
        v-model:value="auditLimit"
        size="small"
        style="width: 110px"
        placeholder="limit"
        :min="1"
        :max="200"
      />
      <n-select
        v-model:value="eventFilter"
        size="small"
        :options="eventOptions"
        style="width: 200px"
      />
      <n-select
        v-model:value="levelFilter"
        size="small"
        :options="levelOptions"
        style="width: 120px"
      />
      <n-input
        v-model:value="userFilter"
        size="small"
        placeholder="用户 ID 过滤"
        style="width: 180px"
        clearable
      />
      <n-button size="small" :loading="auditLoading" @click="loadAudit">刷新</n-button>
    </div>

    <n-spin :show="auditLoading">
      <n-empty v-if="!auditLoading && filtered.length === 0" description="暂无审计事件" size="small" class="ns-py-3" />
      <n-table v-else :bordered="true" size="small" class="docs-table">
        <thead>
          <tr>
            <th>时间</th>
            <th>级别</th>
            <th>事件</th>
            <th>IP</th>
            <th>user_id</th>
            <th>client_id</th>
            <th>详情</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(ev, i) in filtered" :key="i">
            <td class="audit-cell ts">{{ formatTime(ev.ts) }}</td>
            <td class="audit-cell">
              <n-tag :type="AUDIT_LEVEL_TAG[auditLevel(ev.event)]" size="small" round>
                {{ AUDIT_LEVEL_TEXT[auditLevel(ev.event)] }}
              </n-tag>
            </td>
            <td class="audit-cell">
              <span :title="ev.event">{{ auditEventCN(ev.event) }}</span>
            </td>
            <td class="audit-cell ts">{{ ev.ip || '—' }}</td>
            <td class="audit-cell ts">{{ ev.user_id || '—' }}</td>
            <td class="audit-cell ts">{{ ev.client_id || '—' }}</td>
            <td class="audit-cell">{{ ev.detail || '—' }}</td>
          </tr>
        </tbody>
      </n-table>
    </n-spin>
  </n-card>
</template>

<style scoped>
.admin-page-card {
  border-radius: 6px;
}
</style>
