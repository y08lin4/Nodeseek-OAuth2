<script setup lang="ts">
// 管理后台 · 审计日志：limit + 刷新 + 表格
import { onMounted, ref } from 'vue'
import { NCard, NButton, NInputNumber, NSpin, NTable, NEmpty, useMessage } from 'naive-ui'
import { listAudit, ApiError, type AuditEvent } from '../../api'
import { formatTime } from './adminShared'

const message = useMessage()

const events = ref<AuditEvent[]>([])
const auditLoading = ref(false)
const auditLimit = ref<number | null>(50)

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
      <n-button size="small" :loading="auditLoading" @click="loadAudit">刷新</n-button>
    </div>

    <n-spin :show="auditLoading">
      <n-empty v-if="!auditLoading && events.length === 0" description="暂无审计事件" size="small" class="ns-py-3" />
      <n-table v-else :bordered="true" size="small" class="docs-table">
        <thead>
          <tr>
            <th>时间</th>
            <th>事件</th>
            <th>IP</th>
            <th>user_id</th>
            <th>client_id</th>
            <th>详情</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(ev, i) in events" :key="i">
            <td class="audit-cell ts">{{ formatTime(ev.ts) }}</td>
            <td class="audit-cell"><code class="audit-event">{{ ev.event }}</code></td>
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
