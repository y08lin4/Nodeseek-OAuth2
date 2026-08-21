<script setup lang="ts">
// 管理页：
// Admin Token 输入并保存 localStorage（key: ns_admin_token），
// GET /api/admin/status 展示系统 Cookie 状态（set / updated_at / mock_mode），
// 表单 POST /api/admin/cookie 更新系统 Cookie（多账号：选择目标账号或自动识别）；
// GET /api/admin/reviews + POST /api/admin/review 审核队列；GET /api/admin/accounts + POST/PATCH/DELETE 系统账号管理。
// 所有请求携带 X-Admin-Token 头。提示统一 useMessage / useDialog。
import { computed, h, onMounted, ref } from 'vue'
import {
  NCard,
  NInput,
  NSelect,
  NButton,
  NTag,
  NAlert,
  NEmpty,
  NSpin,
  useMessage,
  useDialog,
} from 'naive-ui'
import {
  getAdminStatus,
  updateAdminCookie,
  testMail,
  listReviews,
  reviewAction,
  listAccounts,
  addAccount,
  patchAccount,
  deleteAccount,
  ApiError,
  type AdminStatus,
  type ReviewItem,
  type ReviewType,
  type SysAccount,
} from '../api'

const message = useMessage()
const dialog = useDialog()
const TOKEN_KEY = 'ns_admin_token'

const token = ref(localStorage.getItem(TOKEN_KEY) ?? '')
const cookieText = ref('')
const status = ref<AdminStatus | null>(null)
const loading = ref(false)
const saving = ref(false)
const sendingMail = ref(false)

// —— 审核队列状态 ——
const reviews = ref<ReviewItem[]>([])
const reviewsLoading = ref(false)
const actingKey = ref('') // 正在操作的 client_id:approve|reject，防重复
const rejectReason = ref('') // 拒绝理由（dialog 输入框）

// —— 系统账号状态 ——
const accounts = ref<SysAccount[]>([])
const accountsLoading = ref(false)
const accountActingId = ref('') // 正在操作（优先级/启停/删除）的账号
const addingAccount = ref(false)
const newAccountId = ref('')
const newAccountName = ref('')
// Cookie 更新表单的目标账号：'' = 自动识别（服务端探测归属；AUTO_DETECT=1 时忽略手动选择）
const cookieAccountId = ref('')

const hasToken = computed(() => token.value.trim().length > 0)

// 审核类型徽章映射（NTag type 配色）
const reviewTypeMeta: Record<ReviewType, { text: string; type: 'default' | 'info' | 'success' | 'warning' | 'error' }> = {
  app: { text: '应用申请', type: 'info' },
  pause: { text: '暂停申请', type: 'warning' },
  resume: { text: '恢复申请', type: 'success' },
  delete: { text: '删除申请', type: 'error' },
}

function reviewTypeText(t: ReviewType): string {
  return reviewTypeMeta[t]?.text ?? t
}

function reviewTypeClass(t: ReviewType): 'default' | 'info' | 'success' | 'warning' | 'error' {
  return reviewTypeMeta[t]?.type ?? 'info'
}

// Cookie 更新表单的目标账号下拉选项（'' = 自动识别）
const cookieAccountOptions = computed(() => [
  { label: '自动识别（服务端探测归属；AUTO_DETECT=1 时忽略手动选择）', value: '' },
  ...accounts.value.map((a) => ({
    label: `${a.account_name}（${a.account_id}）`,
    value: a.account_id,
  })),
])

// 创建时间（RFC3339）本地化
function formatTime(s: string): string {
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? s : d.toLocaleString('zh-CN')
}

// 保存 token 并立即拉取一次状态
function saveToken() {
  localStorage.setItem(TOKEN_KEY, token.value.trim())
  message.success('Token 已保存')
  loadStatus()
  loadReviews()
  loadAccounts()
}

function clearToken() {
  token.value = ''
  localStorage.removeItem(TOKEN_KEY)
  status.value = null
  reviews.value = []
  accounts.value = []
  cookieAccountId.value = ''
}

// 拉取 Cookie 状态
async function loadStatus() {
  if (!hasToken.value) return
  loading.value = true
  try {
    status.value = await getAdminStatus(token.value.trim())
  } catch (e) {
    status.value = null
    message.error(e instanceof ApiError ? e.message : '获取状态失败')
  } finally {
    loading.value = false
  }
}

// 更新系统 Cookie（选具体账号时带 account_id；自动识别时 body 只有 cookie）
async function submitCookie() {
  if (!hasToken.value) {
    message.error('请先保存 Admin Token')
    return
  }
  if (!cookieText.value.trim()) {
    message.error('Cookie 内容不能为空（格式：name=value; name2=value2）')
    return
  }
  saving.value = true
  try {
    await updateAdminCookie(
      token.value.trim(),
      cookieText.value.trim(),
      cookieAccountId.value || undefined,
    )
    message.success('Cookie 已更新')
    cookieText.value = ''
    await loadStatus()
    await loadAccounts() // 自动识别可能新建账号记录
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '更新失败')
  } finally {
    saving.value = false
  }
}

// updated_at（RFC3339）格式化为本地时间
const updatedAtText = computed(() => {
  if (!status.value?.cookie.updated_at) return '—'
  const d = new Date(status.value.cookie.updated_at)
  return Number.isNaN(d.getTime()) ? status.value.cookie.updated_at : d.toLocaleString()
})

// age_seconds 人类可读
const ageText = computed(() => {
  const s = status.value?.cookie.age_seconds ?? 0
  if (s < 60) return `${s} 秒前`
  if (s < 3600) return `${Math.floor(s / 60)} 分钟前`
  return `${Math.floor(s / 3600)} 小时前`
})

// 邮件配置状态文案：configured + report_time（如「已配置（日报 20:00）」）
const mailText = computed(() => {
  const m = status.value?.mail
  if (!m?.configured) return '未配置'
  return m.report_time ? `已配置（日报 ${m.report_time}）` : '已配置'
})

// last_test_at（RFC3339）格式化为本地时间
const lastTestAtText = computed(() => {
  const t = status.value?.mail?.last_test_at
  if (!t) return '—'
  const d = new Date(t)
  return Number.isNaN(d.getTime()) ? t : d.toLocaleString()
})

// 发送测试邮件：成功/失败均提示 data.message，成功后刷新状态（更新 last_test_at）
async function handleTestMail() {
  if (!hasToken.value) {
    message.error('请先保存 Admin Token')
    return
  }
  sendingMail.value = true
  try {
    const resp = await testMail(token.value.trim())
    message.success(resp.message || '测试邮件已发送')
    await loadStatus()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '发送测试邮件失败')
  } finally {
    sendingMail.value = false
  }
}

// 拉取待审核队列（GET /api/admin/reviews）
async function loadReviews() {
  if (!hasToken.value) return
  reviewsLoading.value = true
  try {
    const resp = await listReviews(token.value.trim())
    reviews.value = resp.reviews
  } catch (e) {
    // 队列加载失败不阻塞其他区块，仅提示
    message.error(e instanceof ApiError ? e.message : '获取审核队列失败')
  } finally {
    reviewsLoading.value = false
  }
}

// 执行审核动作（通过/拒绝共用；拒绝带可选理由）
async function doReview(item: ReviewItem, action: 'approve' | 'reject', reason?: string) {
  actingKey.value = `${item.client_id}:${action}`
  try {
    await reviewAction(token.value.trim(), {
      type: item.type,
      client_id: item.client_id,
      action,
      reason,
    })
    message.success(action === 'approve' ? '已通过' : '已拒绝')
    await loadReviews()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '审核操作失败，请重试')
  } finally {
    actingKey.value = ''
  }
}

// 审核操作：通过走确认 dialog；拒绝走带理由输入框的 dialog（理由可选，取消则不提交）
function handleReview(item: ReviewItem, action: 'approve' | 'reject') {
  if (action === 'approve') {
    dialog.warning({
      title: '审核通过',
      content: `确认通过「${item.client_name}」的申请？`,
      positiveText: '通过',
      negativeText: '取消',
      onPositiveClick: () => doReview(item, 'approve'),
    })
    return
  }
  rejectReason.value = ''
  dialog.warning({
    title: `拒绝「${item.client_name}」`,
    content: () =>
      h(NInput, {
        type: 'textarea',
        rows: 3,
        placeholder: '可输入拒绝理由（可选，留空直接提交）',
        value: rejectReason.value,
        'onUpdate:value': (v: string) => {
          rejectReason.value = v
        },
      }),
    positiveText: '确认拒绝',
    negativeText: '取消',
    onPositiveClick: () =>
      doReview(item, 'reject', rejectReason.value.trim() || undefined),
  })
}

// —— 系统账号管理（GET/POST/PATCH/DELETE /api/admin/accounts）——

// 拉取系统账号列表
async function loadAccounts() {
  if (!hasToken.value) return
  accountsLoading.value = true
  try {
    const resp = await listAccounts(token.value.trim())
    accounts.value = resp.accounts
    // 若 Cookie 表单选中的账号已被删除，重置为自动识别
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

// 新增系统账号（POST）
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
    await addAccount(token.value.trim(), {
      account_id: newAccountId.value.trim(),
      account_name: newAccountName.value.trim(),
      priority: 0,
      enabled: true,
    })
    message.success('系统账号已添加')
    newAccountId.value = ''
    newAccountName.value = ''
    await loadAccounts()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '添加账号失败')
  } finally {
    addingAccount.value = false
  }
}

// 调整优先级（PATCH priority，数值小者优先，不低于 0）
async function handlePriority(account: SysAccount, delta: number) {
  const next = Math.max(0, account.priority + delta)
  if (next === account.priority) return
  accountActingId.value = account.account_id
  try {
    await patchAccount(token.value.trim(), account.account_id, { priority: next })
    await loadAccounts()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '调整优先级失败')
  } finally {
    accountActingId.value = ''
  }
}

// 启用/停用（PATCH enabled）
async function handleToggleEnabled(account: SysAccount) {
  accountActingId.value = account.account_id
  try {
    await patchAccount(token.value.trim(), account.account_id, { enabled: !account.enabled })
    await loadAccounts()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '操作失败')
  } finally {
    accountActingId.value = ''
  }
}

// 删除系统账号（DELETE，useDialog 确认；至少保留 1 个否则 400）
function handleDeleteAccount(account: SysAccount) {
  dialog.warning({
    title: '删除系统账号',
    content: `确定删除系统账号「${account.account_name}」（${account.account_id}）？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      accountActingId.value = account.account_id
      try {
        await deleteAccount(token.value.trim(), account.account_id)
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

onMounted(() => {
  if (hasToken.value) {
    loadStatus()
    loadReviews()
    loadAccounts()
  }
})
</script>

<template>
  <n-card class="page-card">
    <template #header>
      <span class="page-title">管理页</span>
    </template>
    <p class="ns-card-sub">维护系统账号 Cookie（私信核验依赖它，失效则服务不可用）</p>

    <!-- Token 管理 -->
    <div class="ns-mb-4">
      <n-form-item label="Admin Token">
        <n-input
          v-model:value="token"
          type="password"
          show-password-on="click"
          placeholder="输入管理令牌（仅保存在本浏览器 localStorage）"
          :input-props="{ autocomplete: 'off' }"
        />
        <template #feedback>
          Token 仅保存在本浏览器 localStorage（key: ns_admin_token），不会上传到其他存储。
        </template>
      </n-form-item>
      <div class="ns-flex ns-gap-2">
        <n-button type="primary" :disabled="!hasToken" @click="saveToken">
          保存并查询
        </n-button>
        <n-button @click="clearToken">清除</n-button>
      </div>
    </div>

    <!-- Cookie 状态 -->
    <template v-if="status">
      <h2 class="ns-h6 ns-mb-3">系统 Cookie 状态</h2>
      <div class="detail-row">
        <span class="detail-label">Cookie 状态</span>
        <span class="detail-value">
          <n-tag v-if="status.cookie.set" type="success" size="small" round>已设置</n-tag>
          <n-tag v-else type="warning" size="small" round>未设置</n-tag>
        </span>
      </div>
      <div class="detail-row">
        <span class="detail-label">更新时间</span>
        <span class="detail-value">{{ updatedAtText }}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label">更新距今</span>
        <span class="detail-value">{{ ageText }}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label">Mock 模式</span>
        <span class="detail-value">
          <n-tag v-if="status.mock_mode" type="warning" size="small" round>
            开启（跳过真实私信核验）
          </n-tag>
          <n-tag v-else type="success" size="small" round>关闭</n-tag>
        </span>
      </div>
      <n-alert v-if="!status.cookie.set" type="error" class="ns-mt-3">
        系统 Cookie 未设置，登录核验将不可用，请立即更新。
      </n-alert>
    </template>
    <n-empty
      v-else-if="hasToken && !loading"
      description="Token 已保存，点击「保存并查询」加载状态"
      size="small"
      class="ns-py-3"
    />

    <!-- 邮件配置状态 -->
    <div v-if="status" class="ns-mt-4">
      <h2 class="ns-h6 ns-mb-3">邮件配置</h2>
      <div class="detail-row">
        <span class="detail-label">邮件通知</span>
        <span class="detail-value">
          <n-tag :type="status.mail?.configured ? 'success' : 'warning'" size="small" round>
            {{ mailText }}
          </n-tag>
        </span>
      </div>
      <div class="detail-row">
        <span class="detail-label">上次测试</span>
        <span class="detail-value">{{ lastTestAtText }}</span>
      </div>
      <n-button :loading="sendingMail" :disabled="!hasToken" @click="handleTestMail">
        发送测试邮件
      </n-button>
      <div class="ns-form-text ns-mt-1">
        用于验证 SMTP 邮件发送是否可用（未配置时提示 SMTP 未配置）。
      </div>
      <!-- 新提交邮件通知状态：开关由服务端环境变量 NS_REVIEW_EMAIL_NOTIFY 控制（未暴露给前端，仅提示） -->
      <div class="ns-form-text ns-mt-1">新应用提交邮件通知：由服务端 NS_REVIEW_EMAIL_NOTIFY 控制</div>
    </div>

    <!-- 审核队列（Admin Token 已填时显示） -->
    <div v-if="hasToken" class="ns-mt-4">
      <h2 class="ns-h6 ns-mb-3">审核队列</h2>
      <n-spin :show="reviewsLoading">
        <n-empty
          v-if="!reviewsLoading && reviews.length === 0"
          description="暂无待审核项"
          size="small"
          class="ns-py-3"
        />
        <div v-else class="review-list">
          <n-card v-for="r in reviews" :key="r.client_id" size="small" class="review-item">
            <div class="ns-flex ns-align-center ns-gap-2 ns-mb-1 ns-flex-wrap">
              <n-tag :type="reviewTypeClass(r.type)" size="small" round>
                {{ reviewTypeText(r.type) }}
              </n-tag>
              <span class="review-name">{{ r.client_name }}</span>
              <code class="review-client-id">{{ r.client_id }}</code>
            </div>
            <div class="ns-text-muted ns-small">owner: {{ r.owner_user_id }} · {{ formatTime(r.created_at) }}</div>
            <div v-if="r.detail" class="review-detail">{{ r.detail }}</div>
            <div class="review-actions">
              <n-button
                size="small"
                type="success"
                :loading="actingKey === `${r.client_id}:approve`"
                @click="handleReview(r, 'approve')"
              >
                通过
              </n-button>
              <n-button
                size="small"
                type="error"
                :loading="actingKey === `${r.client_id}:reject`"
                @click="handleReview(r, 'reject')"
              >
                拒绝
              </n-button>
            </div>
          </n-card>
        </div>
      </n-spin>
    </div>

    <!-- 系统账号（Admin Token 已填时显示） -->
    <div v-if="hasToken" class="ns-mt-4">
      <h2 class="ns-h6 ns-mb-3">系统账号</h2>
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
              <template v-if="a.last_error">
                · 最近错误：{{ a.last_error }}（失败 {{ a.fail_count }} 次）
              </template>
            </div>
            <div class="review-actions">
              <n-button
                size="small"
                :loading="accountActingId === a.account_id"
                @click="handlePriority(a, -1)"
              >
                优先级 -1
              </n-button>
              <n-button
                size="small"
                :loading="accountActingId === a.account_id"
                @click="handlePriority(a, 1)"
              >
                优先级 +1
              </n-button>
              <n-button
                size="small"
                type="primary"
                :loading="accountActingId === a.account_id"
                @click="handleToggleEnabled(a)"
              >
                {{ a.enabled ? '停用' : '启用' }}
              </n-button>
              <n-button
                size="small"
                type="error"
                :loading="accountActingId === a.account_id"
                @click="handleDeleteAccount(a)"
              >
                删除
              </n-button>
            </div>
          </n-card>
        </div>
      </n-spin>

      <!-- 新增系统账号 -->
      <div class="ns-mt-3">
        <n-form-item label="新增系统账号">
          <div class="ns-flex ns-gap-2 ns-w-100">
            <n-input
              v-model:value="newAccountId"
              placeholder="账号 ID（纯数字）"
              :input-props="{ inputmode: 'numeric', autocomplete: 'off' }"
            />
            <n-input
              v-model:value="newAccountName"
              placeholder="账号名称"
              :input-props="{ autocomplete: 'off' }"
            />
            <n-button type="primary" :loading="addingAccount" @click="handleAddAccount">
              添加
            </n-button>
          </div>
          <template #feedback>
            手动新增系统账号（Cookie 由扩展推送或下方 Cookie 表单按账号更新）。
          </template>
        </n-form-item>
      </div>
    </div>

    <!-- 更新 Cookie -->
    <div class="ns-mt-4">
      <h2 class="ns-h6 ns-mb-3">更新系统 Cookie</h2>
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
        <template #feedback>
          从 NodeSeek 登录态复制完整 Cookie 字符串；服务端将加密存储，并用于读取私信核验验证码。
        </template>
      </n-form-item>
      <n-button type="primary" :loading="saving" :disabled="!hasToken" @click="submitCookie">
        更新 Cookie
      </n-button>
    </div>
  </n-card>
</template>
