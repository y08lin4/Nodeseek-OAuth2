<script setup lang="ts">
// 管理后台 · 设置：邮件配置 + 系统信息
import { onMounted, ref } from 'vue'
import { NCard, NButton, NTag, NSpin, useMessage } from 'naive-ui'
import { getAdminStatus, testMail, ApiError, type AdminStatus } from '../../api'
import { formatTime } from './adminShared'

const message = useMessage()

const status = ref<AdminStatus | null>(null)
const loading = ref(false)
const sendingMail = ref(false)

async function loadStatus() {
  loading.value = true
  try {
    status.value = await getAdminStatus()
  } catch (e) {
    status.value = null
    message.error(e instanceof ApiError ? e.message : '获取状态失败')
  } finally {
    loading.value = false
  }
}

async function handleTestMail() {
  sendingMail.value = true
  try {
    const resp = await testMail()
    message.success(resp.message || '测试邮件已发送')
    await loadStatus()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '发送测试邮件失败')
  } finally {
    sendingMail.value = false
  }
}

onMounted(loadStatus)
</script>

<template>
  <n-card class="admin-page-card">
    <template #header>
      <span class="page-title">设置</span>
    </template>
    <p class="ns-card-sub">邮件通知与系统信息</p>

    <n-spin :show="loading">
      <!-- 邮件配置 -->
      <h2 class="ns-h6 ns-mb-3">邮件配置</h2>
      <div v-if="status" class="detail-row">
        <span class="detail-label">SMTP 配置</span>
        <span class="detail-value">
          <n-tag :type="status.mail?.configured ? 'success' : 'warning'" size="small" round>
            {{ status.mail?.configured ? '已配置' : '未配置' }}
          </n-tag>
        </span>
      </div>
      <div v-if="status?.mail?.configured" class="detail-row">
        <span class="detail-label">报告时间</span>
        <span class="detail-value">{{ status.mail.report_time || '—' }}</span>
      </div>
      <div v-if="status?.mail?.last_test_at" class="detail-row">
        <span class="detail-label">上次测试</span>
        <span class="detail-value">{{ formatTime(status.mail.last_test_at) }}</span>
      </div>
      <div v-if="status" class="detail-row">
        <span class="detail-label">新应用提交邮件通知</span>
        <span class="detail-value">
          <n-tag :type="status.mail?.review_email_notify === true ? 'success' : 'default'" size="small" round>
            {{ status.mail?.review_email_notify === true ? '已开启' : '未开启' }}
          </n-tag>
        </span>
      </div>
      <div v-if="status" class="detail-row">
        <span class="detail-label">发送测试邮件</span>
        <span class="detail-value">
          <n-button size="small" :loading="sendingMail" @click="handleTestMail">发送测试邮件</n-button>
        </span>
      </div>

      <!-- 系统信息 -->
      <h2 class="ns-h6 ns-mt-4 ns-mb-3">系统信息</h2>
      <div v-if="status" class="detail-row">
        <span class="detail-label">Mock 模式</span>
        <span class="detail-value">
          <n-tag :type="status.mock_mode ? 'warning' : 'success'" size="small" round>
            {{ status.mock_mode ? '开启（跳过真实私信核验）' : '关闭' }}
          </n-tag>
        </span>
      </div>
      <div v-if="status?.cookie?.set !== undefined" class="detail-row">
        <span class="detail-label">Cookie 状态</span>
        <span class="detail-value">
          <n-tag :type="status.cookie.set ? 'success' : 'warning'" size="small" round>
            {{ status.cookie.set ? '已设置' : '未设置' }}
          </n-tag>
        </span>
      </div>
    </n-spin>
  </n-card>
</template>

<style scoped>
.admin-page-card {
  border-radius: 6px;
}
</style>
