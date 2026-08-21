<script setup lang="ts">
// 管理后台 · 系统账号：列表 / 新增 / 启停 / 优先级 / 删除 + 更新 Cookie 表单
import { computed, onMounted, ref } from 'vue'
import {
  NCard,
  NButton,
  NTag,
  NInput,
  NSelect,
  NFormItem,
  NSpin,
  NEmpty,
  useMessage,
  useDialog,
} from 'naive-ui'
import {
  listAccounts,
  addAccount,
  patchAccount,
  deleteAccount,
  updateAdminCookie,
  getAdminStatus,
  ApiError,
  type SysAccount,
  type AdminStatus,
} from '../../api'
import { formatTime } from './adminShared'

const message = useMessage()
const dialog = useDialog()

const accounts = ref<SysAccount[]>([])
const accountsLoading = ref(false)
const accountActingId = ref('')
const status = ref<AdminStatus | null>(null)

const addingAccount = ref(false)
const newAccountId = ref('')
const newAccountName = ref('')
const newAccountPriority = ref(0)

const cookieAccountId = ref('')
const cookieText = ref('')
const saving = ref(false)

async function loadAccounts() {
  accountsLoading.value = true
  try {
    const resp = await listAccounts()
    accounts.value = resp.accounts
    if (
      cookieAccountId.value &&
      !accounts.value.some((a) => a.account_id === cookieAccountId.value)
    ) {
      cookieAccountId.value = ''
    }
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '获取系统账号失败')
  } finally {
    accountsLoading.value = false
  }
}

async function loadStatus() {
  try {
    status.value = await getAdminStatus()
  } catch (e) {
    status.value = null
  }
}

const cookieAccountOptions = computed(() => [
  { label: '自动识别（服务端探测归属）', value: '' },
  ...accounts.value.map((a) => ({
    label: `${a.account_name}（${a.account_id}）`,
    value: a.account_id,
  })),
])

async function handleAddAccount() {
  if (!/^\d+$/.test(newAccountId.value.trim())) {
    message.error('账号 ID 必须是纯数字')
    return
  }
  if (!newAccountName.value.trim()) {
    message.error('请填写账号名称')
    return
  }
  addingAccount.value = true
  try {
    await addAccount({
      account_id: newAccountId.value.trim(),
      account_name: newAccountName.value.trim(),
      priority: Math.max(0, Math.round(newAccountPriority.value || 0)),
      enabled: true,
    })
    message.success('系统账号已添加')
    newAccountId.value = ''
    newAccountName.value = ''
    newAccountPriority.value = 0
    await loadAccounts()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '添加账号失败')
  } finally {
    addingAccount.value = false
  }
}

async function handlePriority(account: SysAccount, delta: number) {
  const next = Math.max(0, account.priority + delta)
  if (next === account.priority) return
  accountActingId.value = account.account_id
  try {
    await patchAccount(account.account_id, { priority: next })
    await loadAccounts()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '调整优先级失败')
  } finally {
    accountActingId.value = ''
  }
}

async function handleToggleEnabled(account: SysAccount) {
  accountActingId.value = account.account_id
  try {
    await patchAccount(account.account_id, { enabled: !account.enabled })
    await loadAccounts()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '操作失败')
  } finally {
    accountActingId.value = ''
  }
}

function handleDeleteAccount(account: SysAccount) {
  dialog.warning({
    title: '删除系统账号',
    content: `确定删除系统账号「${account.account_name}」（${account.account_id}）？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      accountActingId.value = account.account_id
      try {
        await deleteAccount(account.account_id)
        message.success('系统账号已删除')
        await loadAccounts()
      } catch (e) {
        message.error(e instanceof ApiError ? e.message : '删除失败')
      } finally {
        accountActingId.value = ''
      }
    },
  })
}

async function submitCookie() {
  if (!cookieText.value.trim()) {
    message.error('Cookie 内容不能为空（格式：name=value; name2=value2）')
    return
  }
  saving.value = true
  try {
    await updateAdminCookie(cookieText.value.trim(), cookieAccountId.value || undefined)
    message.success('Cookie 已更新')
    cookieText.value = ''
    await loadStatus()
    await loadAccounts()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '更新失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  loadAccounts()
  loadStatus()
})
</script>

<template>
  <n-card class="admin-page-card">
    <template #header>
      <span class="page-title">系统账号</span>
    </template>
    <p class="ns-card-sub">私信核验依赖系统账号 Cookie，失效则服务不可用</p>

    <n-spin :show="accountsLoading">
      <n-empty
        v-if="!accountsLoading && accounts.length === 0"
        description="暂无系统账号"
        size="small"
        class="ns-py-3"
      />
      <div v-else class="review-list">
        <n-card v-for="a in accounts" :key="a.account_id" size="small" class="review-item">
          <div class="ns-flex ns-align-center ns-gap-2 ns-mb-1 ns-flex-wrap">
            <n-tag :type="a.enabled ? 'success' : 'default'" size="small" round>
              {{ a.enabled ? '启用' : '停用' }}
            </n-tag>
            <span class="review-name">{{ a.account_name }}</span>
            <code class="review-client-id">{{ a.account_id }}</code>
            <n-tag v-if="a.auto_detected" type="info" size="small" round>自动识别</n-tag>
          </div>
          <div class="ns-text-muted ns-small">
            优先级 {{ a.priority }} · 更新时间 {{ formatTime(a.updated_at) }}
            <template v-if="a.last_error"> · 最近错误：{{ a.last_error }}（失败 {{ a.fail_count }} 次）</template>
          </div>
          <div class="review-actions">
            <n-button size="small" :loading="accountActingId === a.account_id" @click="handlePriority(a, -1)">优先级 -1</n-button>
            <n-button size="small" :loading="accountActingId === a.account_id" @click="handlePriority(a, 1)">优先级 +1</n-button>
            <n-button size="small" type="primary" :loading="accountActingId === a.account_id" @click="handleToggleEnabled(a)">
              {{ a.enabled ? '停用' : '启用' }}
            </n-button>
            <n-button size="small" type="error" :loading="accountActingId === a.account_id" @click="handleDeleteAccount(a)">删除</n-button>
          </div>
        </n-card>
      </div>
    </n-spin>

    <!-- 新增系统账号 -->
    <div class="ns-mt-4">
      <h2 class="ns-h6 ns-mb-3">新增系统账号</h2>
      <n-form-item label="账号 ID">
        <n-input
          v-model:value="newAccountId"
          placeholder="账号 ID（纯数字）"
          :input-props="{ inputmode: 'numeric', autocomplete: 'off' }"
        />
      </n-form-item>
      <n-form-item label="账号名称">
        <n-input v-model:value="newAccountName" placeholder="账号名称" :input-props="{ autocomplete: 'off' }" />
      </n-form-item>
      <n-form-item label="优先级">
        <n-input-number v-model:value="newAccountPriority" :min="0" style="width: 100%" />
      </n-form-item>
      <n-button type="primary" :loading="addingAccount" @click="handleAddAccount">添加账号</n-button>
    </div>

    <!-- 更新 Cookie -->
    <div class="ns-mt-4">
      <h2 class="ns-h6 ns-mb-3">更新系统 Cookie</h2>
      <n-alert v-if="status && !status.cookie.set" type="error" class="ns-mb-3">
        系统 Cookie 未设置，登录核验将不可用，请立即更新。
      </n-alert>
      <n-form-item label="目标账号">
        <n-select v-model:value="cookieAccountId" :options="cookieAccountOptions" />
      </n-form-item>
      <n-form-item label="Cookie 内容">
        <n-input
          v-model:value="cookieText"
          type="textarea"
          :rows="4"
          placeholder="name=value; name2=value2"
        />
        <template #feedback>从 NodeSeek 登录态复制完整 Cookie 字符串；服务端将加密存储，用于读取私信核验验证码。</template>
      </n-form-item>
      <n-button type="primary" :loading="saving" @click="submitCookie">更新 Cookie</n-button>
    </div>
  </n-card>
</template>

<style scoped>
.admin-page-card {
  border-radius: 6px;
}
</style>
