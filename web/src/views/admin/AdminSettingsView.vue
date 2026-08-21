<script setup lang="ts">
// 管理后台 · 设置：邮件配置 + SMTP 运行时配置 + 系统信息
import { onMounted, ref } from 'vue'
import {
  NCard,
  NButton,
  NTag,
  NSpin,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NSelect,
  NSwitch,
  NAlert,
  useMessage,
} from 'naive-ui'
import {
  getAdminStatus,
  testMail,
  getAdminSmtp,
  saveAdminSmtp,
  ApiError,
  type AdminStatus,
  type SmtpTlsMode,
} from '../../api'
import { formatTime } from './adminShared'

const message = useMessage()

const status = ref<AdminStatus | null>(null)
const loading = ref(false)
const sendingMail = ref(false)

// SMTP 表单
const smtpLoading = ref(false)
const smtpSaving = ref(false)
const smtpLoaded = ref(false)
const smtpForm = ref<{
  host: string
  port: number | null
  tls: SmtpTlsMode | null
  user: string
  password: string
  enabled: boolean
}>({
  host: '',
  port: 587,
  tls: 'starttls',
  user: '',
  password: '',
  enabled: false,
})
const hasPassword = ref(false)

const tlsOptions = [
  { label: 'SSL/TLS', value: 'ssl' },
  { label: 'STARTTLS', value: 'starttls' },
  { label: '无加密', value: 'none' },
]

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

async function loadSmtp() {
  smtpLoading.value = true
  try {
    const resp = await getAdminSmtp()
    smtpForm.value = {
      host: resp.host || '',
      port: resp.port || 587,
      tls: (resp.tls || 'starttls') as SmtpTlsMode,
      user: resp.user || '',
      password: '',
      enabled: resp.enabled,
    }
    hasPassword.value = resp.has_password
    smtpLoaded.value = true
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '读取 SMTP 配置失败')
  } finally {
    smtpLoading.value = false
  }
}

async function handleSaveSmtp() {
  const f = smtpForm.value
  if (!f.host.trim()) {
    message.error('请填写 SMTP 服务器地址')
    return
  }
  if (!f.port || f.port < 1 || f.port > 65535) {
    message.error('端口必须在 1-65535')
    return
  }
  smtpSaving.value = true
  try {
    const resp = await saveAdminSmtp({
      host: f.host.trim(),
      port: f.port,
      tls: (f.tls || 'starttls') as SmtpTlsMode,
      user: f.user.trim(),
      password: f.password, // 空串 = 保留旧密码
      enabled: f.enabled,
    })
    hasPassword.value = resp.has_password
    f.password = ''
    message.success('已保存并生效')
    await loadSmtp()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '保存 SMTP 配置失败')
  } finally {
    smtpSaving.value = false
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

onMounted(() => {
  loadStatus()
  loadSmtp()
})
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

      <!-- SMTP 运行时配置 -->
      <h2 class="ns-h6 ns-mt-4 ns-mb-3">SMTP 配置</h2>
      <n-form class="admin-smtp-form" label-placement="top" label-width="auto">
        <n-spin :show="smtpLoading">
          <div v-if="smtpLoaded" class="smtp-grid">
            <n-form-item label="服务器" class="admin-form-item">
              <n-input v-model:value="smtpForm.host" size="small" placeholder="smtp.example.com" />
            </n-form-item>
            <n-form-item label="端口" class="admin-form-item">
              <n-input-number
                v-model:value="smtpForm.port"
                size="small"
                :min="1"
                :max="65535"
                placeholder="587"
                style="width: 100%"
              />
            </n-form-item>
            <n-form-item label="加密方式" class="admin-form-item">
              <n-select v-model:value="smtpForm.tls" size="small" :options="tlsOptions" />
            </n-form-item>
            <n-form-item label="用户名" class="admin-form-item">
              <n-input v-model:value="smtpForm.user" size="small" placeholder="登录用户名" />
            </n-form-item>
            <n-form-item label="密码" class="admin-form-item">
              <n-input
                v-model:value="smtpForm.password"
                size="small"
                type="password"
                show-password-on="click"
                :placeholder="hasPassword ? '不修改请留空（已设置）' : '设置密码'"
              />
            </n-form-item>
            <n-form-item label="启用" class="admin-form-item">
              <n-switch v-model:value="smtpForm.enabled" size="small" />
            </n-form-item>
          </div>
          <n-alert v-if="smtpLoaded && !smtpForm.host" type="warning" class="ns-mt-2">
            当前未配置 SMTP（显示环境变量默认值），保存后可持久化到 data/smtp.json 并热更新。
          </n-alert>
          <div class="ns-mt-3">
            <n-button type="primary" size="small" :loading="smtpSaving" @click="handleSaveSmtp">保存</n-button>
            <span v-if="hasPassword" class="ns-text-muted ns-small ns-mt-1 smtp-pass-hint">
              密码已设置，留空保存将保留旧密码。
            </span>
          </div>
        </n-spin>
      </n-form>

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

.admin-smtp-form {
  max-width: 640px;
}

.smtp-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 0 16px;
}

.smtp-pass-hint {
  display: inline-block;
  margin-left: 8px;
}
</style>
