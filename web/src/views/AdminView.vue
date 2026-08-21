<script setup lang="ts">
// 管理页：
// Admin Token 输入并保存 localStorage（key: ns_admin_token），
// GET /api/admin/status 展示系统 Cookie 状态（set / updated_at / mock_mode），
// 表单 POST /api/admin/cookie 更新系统 Cookie（多账号：选择目标账号或自动识别）；
// GET /api/admin/reviews + POST /api/admin/review 审核队列；GET /api/admin/accounts + POST/PATCH/DELETE 系统账号管理。
// 所有请求携带 X-Admin-Token 头。
import { computed, onMounted, ref } from 'vue'
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

const TOKEN_KEY = 'ns_admin_token'

const token = ref(localStorage.getItem(TOKEN_KEY) ?? '')
const cookieText = ref('')
const status = ref<AdminStatus | null>(null)
const loading = ref(false)
const saving = ref(false)
const sendingMail = ref(false)
const error = ref('')
const successMsg = ref('')

// —— 审核队列状态 ——
const reviews = ref<ReviewItem[]>([])
const reviewsLoading = ref(false)
const actingKey = ref('') // 正在操作的 client_id:approve|reject，防重复

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

// 审核类型徽章映射
const reviewTypeMeta: Record<ReviewType, { text: string; cls: string }> = {
  app: { text: '应用申请', cls: 'badge-pending' },
  pause: { text: '暂停申请', cls: 'badge-unset' },
  resume: { text: '恢复申请', cls: 'badge-set' },
  delete: { text: '删除申请', cls: 'badge-danger' },
}

function reviewTypeText(t: ReviewType): string {
  return reviewTypeMeta[t]?.text ?? t
}

function reviewTypeClass(t: ReviewType): string {
  return reviewTypeMeta[t]?.cls ?? 'badge-pending'
}

// 创建时间（RFC3339）本地化
function formatTime(s: string): string {
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? s : d.toLocaleString('zh-CN')
}

// 保存 token 并立即拉取一次状态
function saveToken() {
  localStorage.setItem(TOKEN_KEY, token.value.trim())
  successMsg.value = 'Token 已保存'
  error.value = ''
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
  successMsg.value = ''
  error.value = ''
}

// 拉取 Cookie 状态
async function loadStatus() {
  if (!hasToken.value) return
  loading.value = true
  error.value = ''
  try {
    status.value = await getAdminStatus(token.value.trim())
  } catch (e) {
    status.value = null
    error.value = e instanceof ApiError ? e.message : '获取状态失败'
  } finally {
    loading.value = false
  }
}

// 更新系统 Cookie（选具体账号时带 account_id；自动识别时 body 只有 cookie）
async function submitCookie() {
  if (!hasToken.value) {
    error.value = '请先保存 Admin Token'
    return
  }
  if (!cookieText.value.trim()) {
    error.value = 'Cookie 内容不能为空（格式：name=value; name2=value2）'
    return
  }
  saving.value = true
  error.value = ''
  successMsg.value = ''
  try {
    await updateAdminCookie(
      token.value.trim(),
      cookieText.value.trim(),
      cookieAccountId.value || undefined,
    )
    successMsg.value = 'Cookie 已更新'
    cookieText.value = ''
    await loadStatus()
    await loadAccounts() // 自动识别可能新建账号记录
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '更新失败'
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
    error.value = '请先保存 Admin Token'
    return
  }
  sendingMail.value = true
  error.value = ''
  successMsg.value = ''
  try {
    const resp = await testMail(token.value.trim())
    successMsg.value = resp.message || '测试邮件已发送'
    await loadStatus()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '发送测试邮件失败'
  } finally {
    sendingMail.value = false
  }
}

// 拉取待审核队列（GET /api/admin/reviews）
async function loadReviews() {
  if (!hasToken.value) return
  reviewsLoading.value = true
  error.value = ''
  try {
    const resp = await listReviews(token.value.trim())
    reviews.value = resp.reviews
  } catch (e) {
    // 队列加载失败不阻塞其他区块，仅提示
    error.value = e instanceof ApiError ? e.message : '获取审核队列失败'
  } finally {
    reviewsLoading.value = false
  }
}

// 审核操作：通过直接提交；拒绝先 prompt 输入理由（可选，取消则不提交）
async function handleReview(item: ReviewItem, action: 'approve' | 'reject') {
  let reason = ''
  if (action === 'reject') {
    const input = window.prompt(`拒绝「${item.client_name}」？可输入理由（可选，留空直接提交）：`)
    if (input === null) return // 用户取消
    reason = input.trim()
  }
  actingKey.value = `${item.client_id}:${action}`
  error.value = ''
  successMsg.value = ''
  try {
    await reviewAction(token.value.trim(), {
      type: item.type,
      client_id: item.client_id,
      action,
      reason: reason || undefined,
    })
    successMsg.value = action === 'approve' ? '已通过' : '已拒绝'
    await loadReviews()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '审核操作失败，请重试'
  } finally {
    actingKey.value = ''
  }
}

// —— 系统账号管理（GET/POST/PATCH/DELETE /api/admin/accounts）——

// 拉取系统账号列表
async function loadAccounts() {
  if (!hasToken.value) return
  accountsLoading.value = true
  error.value = ''
  try {
    const resp = await listAccounts(token.value.trim())
    accounts.value = resp.accounts
    // 若 Cookie 表单选中的账号已被删除，重置为自动识别
    if (cookieAccountId.value && !accounts.value.some((a) => a.account_id === cookieAccountId.value)) {
      cookieAccountId.value = ''
    }
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '获取系统账号失败'
  } finally {
    accountsLoading.value = false
  }
}

// 新增系统账号（POST）
async function handleAddAccount() {
  error.value = ''
  if (!/^\d+$/.test(newAccountId.value.trim())) {
    error.value = '账号 ID 必须是纯数字'
    return
  }
  if (!newAccountName.value.trim()) {
    error.value = '请填写账号名称'
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
    successMsg.value = '系统账号已添加'
    newAccountId.value = ''
    newAccountName.value = ''
    await loadAccounts()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '添加账号失败'
  } finally {
    addingAccount.value = false
  }
}

// 调整优先级（PATCH priority，数值小者优先，不低于 0）
async function handlePriority(account: SysAccount, delta: number) {
  const next = Math.max(0, account.priority + delta)
  if (next === account.priority) return
  accountActingId.value = account.account_id
  error.value = ''
  try {
    await patchAccount(token.value.trim(), account.account_id, { priority: next })
    await loadAccounts()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '调整优先级失败'
  } finally {
    accountActingId.value = ''
  }
}

// 启用/停用（PATCH enabled）
async function handleToggleEnabled(account: SysAccount) {
  accountActingId.value = account.account_id
  error.value = ''
  try {
    await patchAccount(token.value.trim(), account.account_id, { enabled: !account.enabled })
    await loadAccounts()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '操作失败'
  } finally {
    accountActingId.value = ''
  }
}

// 删除系统账号（DELETE，confirm；至少保留 1 个否则 400）
async function handleDeleteAccount(account: SysAccount) {
  if (!window.confirm(`确定删除系统账号「${account.account_name}」（${account.account_id}）？`)) return
  accountActingId.value = account.account_id
  error.value = ''
  try {
    await deleteAccount(token.value.trim(), account.account_id)
    successMsg.value = '系统账号已删除'
    await loadAccounts()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '删除失败'
  } finally {
    accountActingId.value = ''
  }
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
  <div class="ns-card">
    <h1 class="ns-card-title">管理页</h1>
    <p class="ns-card-sub">维护系统账号 Cookie（私信核验依赖它，失效则服务不可用）</p>

    <div v-if="error" class="ns-alert ns-alert-error">{{ error }}</div>
    <div v-if="successMsg" class="ns-alert ns-alert-success">{{ successMsg }}</div>

    <!-- Token 管理 -->
    <div class="mb-4">
      <label for="admin-token" class="form-label">Admin Token</label>
      <div class="d-flex gap-2">
        <input
          id="admin-token"
          v-model="token"
          type="password"
          class="form-control"
          placeholder="输入管理令牌（仅保存在本浏览器 localStorage）"
          autocomplete="off"
        />
        <button class="btn btn-primary text-nowrap" :disabled="!hasToken" @click="saveToken">
          保存并查询
        </button>
        <button class="btn btn-outline-secondary text-nowrap" @click="clearToken">清除</button>
      </div>
      <div class="form-text">Token 仅保存在本浏览器 localStorage（key: ns_admin_token），不会上传到其他存储。</div>
    </div>

    <!-- Cookie 状态 -->
    <template v-if="status">
      <h2 class="h6 mb-3">系统 Cookie 状态</h2>
      <div class="detail-row">
        <span class="detail-label">Cookie 状态</span>
        <span class="detail-value">
          <span v-if="status.cookie.set" class="badge-set">已设置</span>
          <span v-else class="badge-unset">未设置</span>
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
          <span v-if="status.mock_mode" class="badge-unset">开启（跳过真实私信核验）</span>
          <span v-else class="badge-set">关闭</span>
        </span>
      </div>
      <div v-if="!status.cookie.set" class="ns-alert ns-alert-error mt-3">
        系统 Cookie 未设置，登录核验将不可用，请立即更新。
      </div>
    </template>
    <div v-else-if="hasToken && !loading" class="text-muted text-center py-3">
      Token 已保存，点击「保存并查询」加载状态
    </div>

    <!-- 邮件配置状态 -->
    <div v-if="status" class="mt-4">
      <h2 class="h6 mb-3">邮件配置</h2>
      <div class="detail-row">
        <span class="detail-label">邮件通知</span>
        <span class="detail-value">
          <span v-if="status.mail?.configured" class="badge-set">{{ mailText }}</span>
          <span v-else class="badge-unset">{{ mailText }}</span>
        </span>
      </div>
      <div class="detail-row">
        <span class="detail-label">上次测试</span>
        <span class="detail-value">{{ lastTestAtText }}</span>
      </div>
      <button
        class="btn btn-outline-primary mt-2"
        :disabled="sendingMail || !hasToken"
        @click="handleTestMail"
      >
        {{ sendingMail ? '发送中…' : '发送测试邮件' }}
      </button>
      <div class="form-text mt-1">用于验证 SMTP 邮件发送是否可用（未配置时提示 SMTP 未配置）。</div>
      <!-- 新提交邮件通知状态：开关由服务端环境变量 NS_REVIEW_EMAIL_NOTIFY 控制（未暴露给前端，仅提示） -->
      <div class="form-text mt-1">新应用提交邮件通知：由服务端 NS_REVIEW_EMAIL_NOTIFY 控制</div>
    </div>

    <!-- 审核队列（Admin Token 已填时显示） -->
    <div v-if="hasToken" class="mt-4">
      <h2 class="h6 mb-3">审核队列</h2>
      <div v-if="reviewsLoading" class="text-muted text-center py-3">加载中…</div>
      <div v-else-if="reviews.length === 0" class="text-muted text-center py-3">暂无待审核项</div>
      <div v-else class="review-list">
        <div v-for="r in reviews" :key="r.client_id" class="review-item">
          <div class="review-item-head">
            <span class="badge" :class="reviewTypeClass(r.type)">{{ reviewTypeText(r.type) }}</span>
            <span class="review-name">{{ r.client_name }}</span>
            <code class="review-client-id">{{ r.client_id }}</code>
          </div>
          <div class="text-muted small">owner: {{ r.owner_user_id }} · {{ formatTime(r.created_at) }}</div>
          <div v-if="r.detail" class="review-detail">{{ r.detail }}</div>
          <div class="review-actions">
            <button
              class="btn btn-sm btn-outline-success"
              :disabled="actingKey === `${r.client_id}:approve`"
              @click="handleReview(r, 'approve')"
            >
              {{ actingKey === `${r.client_id}:approve` ? '处理中…' : '通过' }}
            </button>
            <button
              class="btn btn-sm btn-outline-danger"
              :disabled="actingKey === `${r.client_id}:reject`"
              @click="handleReview(r, 'reject')"
            >
              {{ actingKey === `${r.client_id}:reject` ? '处理中…' : '拒绝' }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 系统账号（Admin Token 已填时显示） -->
    <div v-if="hasToken" class="mt-4">
      <h2 class="h6 mb-3">系统账号</h2>
      <div v-if="accountsLoading" class="text-muted text-center py-3">加载中…</div>
      <div v-else-if="accounts.length === 0" class="text-muted text-center py-3">暂无系统账号</div>
      <div v-else class="review-list">
        <div v-for="a in accounts" :key="a.account_id" class="review-item">
          <div class="review-item-head">
            <span class="badge" :class="a.enabled ? 'badge-set' : 'badge-unset'">
              {{ a.enabled ? '启用' : '停用' }}
            </span>
            <span class="review-name">{{ a.account_name }}</span>
            <code class="review-client-id">{{ a.account_id }}</code>
            <span v-if="a.auto_detected" class="badge badge-pending">自动识别</span>
          </div>
          <div class="text-muted small">
            优先级 {{ a.priority }} · 更新时间 {{ formatTime(a.updated_at) }}
            <template v-if="a.last_error"> · 最近错误：{{ a.last_error }}（失败 {{ a.fail_count }} 次）</template>
          </div>
          <div class="review-actions">
            <button
              class="btn btn-sm btn-outline-secondary"
              :disabled="accountActingId === a.account_id"
              @click="handlePriority(a, -1)"
            >
              优先级 -1
            </button>
            <button
              class="btn btn-sm btn-outline-secondary"
              :disabled="accountActingId === a.account_id"
              @click="handlePriority(a, 1)"
            >
              优先级 +1
            </button>
            <button
              class="btn btn-sm btn-outline-primary"
              :disabled="accountActingId === a.account_id"
              @click="handleToggleEnabled(a)"
            >
              {{ a.enabled ? '停用' : '启用' }}
            </button>
            <button
              class="btn btn-sm btn-outline-danger"
              :disabled="accountActingId === a.account_id"
              @click="handleDeleteAccount(a)"
            >
              删除
            </button>
          </div>
        </div>
      </div>

      <!-- 新增系统账号 -->
      <div class="mt-3">
        <label class="form-label">新增系统账号</label>
        <div class="d-flex gap-2">
          <input
            v-model="newAccountId"
            type="text"
            inputmode="numeric"
            class="form-control"
            placeholder="账号 ID（纯数字）"
            autocomplete="off"
          />
          <input
            v-model="newAccountName"
            type="text"
            class="form-control"
            placeholder="账号名称"
            autocomplete="off"
          />
          <button
            class="btn btn-outline-primary text-nowrap"
            :disabled="addingAccount"
            @click="handleAddAccount"
          >
            {{ addingAccount ? '添加中…' : '添加' }}
          </button>
        </div>
        <div class="form-text">手动新增系统账号（Cookie 由扩展推送或下方 Cookie 表单按账号更新）。</div>
      </div>
    </div>

    <!-- 更新 Cookie -->
    <div class="mt-4">
      <h2 class="h6 mb-3">更新系统 Cookie</h2>
      <label for="cookie-account" class="form-label">目标账号</label>
      <select id="cookie-account" v-model="cookieAccountId" class="form-select mb-3">
        <option value="">自动识别（服务端探测归属；AUTO_DETECT=1 时忽略手动选择）</option>
        <option v-for="a in accounts" :key="a.account_id" :value="a.account_id">
          {{ a.account_name }}（{{ a.account_id }}）
        </option>
      </select>
      <label for="cookie-input" class="form-label">Cookie 内容</label>
      <textarea
        id="cookie-input"
        v-model="cookieText"
        class="form-control"
        rows="4"
        placeholder="name=value; name2=value2"
      ></textarea>
      <div class="form-text mb-3">
        从 NodeSeek 登录态复制完整 Cookie 字符串；服务端将加密存储，并用于读取私信核验验证码。
      </div>
      <button class="btn btn-primary" :disabled="saving || !hasToken" @click="submitCookie">
        {{ saving ? '提交中…' : '更新 Cookie' }}
      </button>
    </div>
  </div>
</template>
