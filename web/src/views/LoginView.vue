<script setup lang="ts">
// 登录三步向导：
// ① 输入 NS 数字 ID → POST /oauth/verify
// ② 展示验证码 + 倒计时 + 私信链接（/api/config 的 message_url）+ 重新生成
// ③ 点「我已发送验证码」→ POST /oauth/confirm → 跳转 redirect_to / next / 首页
import { onBeforeUnmount, onMounted, ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  getConfig,
  verifyUser,
  confirmLogin,
  ApiError,
  type AppConfig,
  type UserStats,
  type VerifyAccount,
} from '../api'

const route = useRoute()
const router = useRouter()

// 当前步骤：1 = 输入 ID，2 = 验证码阶段，3 = 登录成功（展示 stats + 门槛 + 跳转）
const step = ref<1 | 2 | 3>(1)
const userId = ref('')
const verificationCode = ref('')
const expiresIn = ref(0) // 服务端给的验证码有效期（秒），契约默认 600
const countdown = ref(0) // 本地倒计时
const config = ref<AppConfig | null>(null)
const loading = ref(false)
const error = ref('')
const copied = ref(false)
const userStats = ref<UserStats | null>(null) // confirm 响应附带的用户信息
const redirectTarget = ref('/') // confirm 成功后的跳转目标

let timer: number | undefined

// ?next= 登录后跳转目标（仅允许站内相对路径，避免开放重定向）
const next = computed(() => {
  const n = route.query.next
  if (typeof n !== 'string' || !n) return ''
  return n.startsWith('/') && !n.startsWith('//') ? n : ''
})

// 私信链接：模板里的 {base} / {id} 用配置与选中账号替换
const messageUrl = computed(() => {
  const c = config.value
  if (!c) return ''
  const id = selectedAccount.value?.account_id ?? c.nodeseek.auth_account_id
  return c.nodeseek.message_url
    .replace('{base}', c.nodeseek.base_url)
    .replace('{id}', id)
})

// —— 账号 chips（verify 响应 accounts，按 priority 升序，默认选中第一个）——
const accounts = ref<VerifyAccount[]>([])
const selectedAccountId = ref('')

// 当前选中账号（fallback：config 默认账号）
const selectedAccount = computed(
  () => accounts.value.find((a) => a.account_id === selectedAccountId.value) ?? accounts.value[0] ?? null,
)

// 倒计时显示 mm:ss
const countdownText = computed(() => {
  const s = Math.max(0, countdown.value)
  const m = Math.floor(s / 60)
  const rest = s % 60
  return `${String(m).padStart(2, '0')}:${String(rest).padStart(2, '0')}`
})

const expired = computed(() => countdown.value <= 0)

// 授权门槛提示文案：值为 0 的项不显示（SPEC 3.8：0 = 该门槛不启用）
const gateText = computed(() => {
  const g = config.value?.gate
  if (!g) return ''
  const parts: string[] = []
  if (g.min_rank > 0) parts.push(`等级 ≥ ${g.min_rank}`)
  if (g.min_join_days > 0) parts.push(`加入天数 ≥ ${g.min_join_days} 天`)
  return parts.length > 0 ? `本服务授权门槛：${parts.join(' · ')}` : '本服务无授权门槛'
})

// 拉取全局配置（私信链接模板等），失败不阻塞主流程，步骤 2 里提示
onMounted(async () => {
  try {
    config.value = await getConfig()
  } catch (e) {
    if (step.value === 2) {
      error.value = e instanceof ApiError ? e.message : '获取配置失败'
    }
  }
})

onBeforeUnmount(() => {
  if (timer !== undefined) window.clearInterval(timer)
})

function startCountdown(seconds: number) {
  countdown.value = seconds
  if (timer !== undefined) window.clearInterval(timer)
  timer = window.setInterval(() => {
    countdown.value = Math.max(0, countdown.value - 1)
    if (countdown.value <= 0 && timer !== undefined) {
      window.clearInterval(timer)
      timer = undefined
    }
  }, 1000)
}

// ① 第一步：纯数字校验 + 发起 verify
async function handleVerify() {
  error.value = ''
  if (!/^\d+$/.test(userId.value)) {
    error.value = '请输入纯数字的 NodeSeek ID'
    return
  }
  loading.value = true
  try {
    const resp = await verifyUser(userId.value.trim())
    verificationCode.value = resp.verification_code
    expiresIn.value = resp.expires_in
    startCountdown(resp.expires_in)
    // 账号 chips：默认选中第一个（后端旧版不返回时清空，fallback 默认账号）
    accounts.value = resp.accounts ?? []
    selectedAccountId.value = accounts.value[0]?.account_id ?? ''
    fillMsg.value = ''
    fillState.value = 'idle'
    step.value = 2
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '请求失败，请重试'
  } finally {
    loading.value = false
  }
}

// ② 重新生成验证码（新验证码 + 重置倒计时）
async function handleRegenerate() {
  error.value = ''
  loading.value = true
  try {
    const resp = await verifyUser(userId.value.trim())
    verificationCode.value = resp.verification_code
    expiresIn.value = resp.expires_in
    startCountdown(resp.expires_in)
    accounts.value = resp.accounts ?? []
    selectedAccountId.value = accounts.value[0]?.account_id ?? ''
    fillMsg.value = ''
    fillState.value = 'idle'
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '请求失败，请重试'
  } finally {
    loading.value = false
  }
}

// ③ 确认已发送验证码：成功后进入第 3 步结果面板（stats + 门槛 + 跳转）
async function handleConfirm() {
  error.value = ''
  loading.value = true
  try {
    const resp = await confirmLogin(userId.value.trim(), verificationCode.value)
    userStats.value = resp.stats
    // 缓存 stats 供 /dashboard 展示（仅有效 stats 才缓存；存储失败不影响登录）
    try {
      if (resp.stats) {
        sessionStorage.setItem('ns_user_stats', JSON.stringify(resp.stats))
      } else {
        sessionStorage.removeItem('ns_user_stats')
      }
    } catch {
      // 忽略存储异常
    }
    redirectTarget.value = resp.redirect_to || next.value || '/'
    step.value = 3
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : '请求失败，请重试'
    // 契约约定：系统 Cookie 缺失/失效时 message 含 "Cookie" 字样，据此提示运维
    error.value = msg.includes('Cookie')
      ? `登录核验失败：${msg}（系统账号 Cookie 可能已失效，请联系管理员）`
      : msg
  } finally {
    loading.value = false
  }
}

// 结果面板：用户确认后跳转
function goAfterLogin() {
  router.push(redirectTarget.value)
}

// 点击验证码复制到剪贴板
async function copyCode() {
  try {
    await navigator.clipboard.writeText(verificationCode.value)
    copied.value = true
    setTimeout(() => (copied.value = false), 1500)
  } catch {
    // 剪贴板不可用时静默失败
  }
}

// —— 第 2 步双主按钮：复制验证码 + 打开私信页 ——
const copiedMsg = ref('') // 复制按钮的反馈提示
const copiedMsgOk = ref(true)

// 显式「复制验证码」按钮：成功提示「已复制验证码」，失败提示手动复制
async function copyCodeBtn() {
  try {
    await navigator.clipboard.writeText(verificationCode.value)
    copiedMsg.value = '已复制验证码'
    copiedMsgOk.value = true
  } catch {
    copiedMsg.value = '复制失败，请手动复制验证码'
    copiedMsgOk.value = false
  }
}

// 「打开私信页」按钮：新标签页打开私信（message_url 来自 /api/config）
function openMessagePage() {
  if (messageUrl.value) {
    window.open(messageUrl.value, '_blank', 'noopener')
  }
}

// —— 自动打开私信并填充验证码（扩展 web-bridge 协议，SPEC §4/§5；次要入口）——
const fillMsg = ref('') // 自动填充的反馈提示
const fillState = ref<'idle' | 'waiting' | 'done' | 'degraded'>('idle')

// 降级方案：复制验证码 + 打开私信页，提示手动粘贴
function degradeFill() {
  navigator.clipboard.writeText(verificationCode.value).catch(() => {})
  if (messageUrl.value) {
    window.open(messageUrl.value, '_blank', 'noopener')
  }
  fillMsg.value = '已复制验证码，请手动粘贴发送'
  fillState.value = 'degraded'
}

// 发起自动填充：postMessage 通知扩展（目标 = 选中账号），800ms 内无回传则降级
function handleAutoFill() {
  const toUserId = selectedAccount.value?.account_id ?? config.value?.nodeseek.auth_account_id
  if (!toUserId || !verificationCode.value) {
    degradeFill()
    return
  }
  let settled = false
  const onResult = (e: MessageEvent) => {
    const d = e.data as { type?: string; ok?: boolean } | null
    if (!d || d.type !== 'nsauth2-fill-pm-result') return
    window.removeEventListener('message', onResult)
    settled = true
    if (d.ok) {
      fillMsg.value = '已自动填充，请在私信页点击发送'
      fillState.value = 'done'
    } else {
      // 扩展返回失败 → 降级为复制 + 手动
      degradeFill()
    }
  }
  window.addEventListener('message', onResult)
  fillState.value = 'waiting'
  fillMsg.value = '正在请求扩展填充…'
  window.postMessage(
    { type: 'nsauth2-fill-pm', code: verificationCode.value, toUserId },
    location.origin,
  )
  // 800ms 内未收到扩展回传（扩展缺失/未注入）→ 降级
  window.setTimeout(() => {
    if (!settled) {
      window.removeEventListener('message', onResult)
      degradeFill()
    }
  }, 800)
}

function backToStep1() {
  step.value = 1
  error.value = ''
}
</script>

<template>
  <div class="ns-card">
    <h1 class="ns-card-title">登录 Nodeseek 账号</h1>
    <p class="ns-card-sub">私信验证码确认账号归属，全程无需密码</p>

    <!-- 步骤指示器 1-2-3 -->
    <div class="steps">
      <div class="step-item" :class="{ done: step >= 2, active: step === 1 }">
        <span class="step-badge">1</span>
        <span class="step-label">输入 NS ID</span>
      </div>
      <div class="step-line"></div>
      <div class="step-item" :class="{ done: step === 3, active: step === 2 }">
        <span class="step-badge">2</span>
        <span class="step-label">私信验证码</span>
      </div>
      <div class="step-line"></div>
      <div class="step-item" :class="{ active: step === 3 }">
        <span class="step-badge">3</span>
        <span class="step-label">确认登录</span>
      </div>
    </div>

    <div v-if="error" class="ns-alert ns-alert-error">{{ error }}</div>

    <!-- 第一步：输入数字 ID -->
    <form v-if="step === 1" @submit.prevent="handleVerify">
      <label for="ns-user-id" class="form-label">NodeSeek 数字 ID</label>
      <input
        id="ns-user-id"
        v-model="userId"
        type="text"
        inputmode="numeric"
        class="form-control form-control-lg"
        placeholder="例如 9037"
        autocomplete="off"
      />
      <div class="form-text mb-3">
        在 NodeSeek 个人主页 URL 中可找到你的数字 ID（/space/ 后的一串数字）。
      </div>
      <button type="submit" class="btn btn-primary w-100" :disabled="loading">
        {{ loading ? '请求中…' : '下一步' }}
      </button>
    </form>

    <!-- 第二步：展示验证码 + 账号 chips + 倒计时 + 私信链接 -->
    <div v-else-if="step === 2">
      <p class="mb-1">请将以下验证码通过私信发送给任一系统账号</p>

      <!-- 账号 chips：可点选，默认高亮第一个；私信链接与自动填充按选中账号生成 -->
      <div v-if="accounts.length > 0" class="account-chips mt-2 mb-1">
        <span class="text-muted small me-1">发送给任一系统账号：</span>
        <button
          v-for="a in accounts"
          :key="a.account_id"
          type="button"
          class="account-chip"
          :class="{ active: a.account_id === selectedAccountId }"
          @click="selectedAccountId = a.account_id"
        >
          {{ a.account_name }}({{ a.account_id }})
        </button>
      </div>
      <p v-if="accounts.length > 0" class="text-muted small mb-0">
        当前选中：<strong>{{ selectedAccount?.account_name }}</strong>（NS ID：{{ selectedAccount?.account_id }}）
      </p>
      <p v-else class="text-muted small mb-0">
        系统账号：<strong>{{ config?.nodeseek.auth_account_username ?? '—' }}</strong>
        （NS ID：{{ config?.nodeseek.auth_account_id ?? '—' }}）
      </p>

      <div class="code-box" :title="'点击复制'" @click="copyCode">
        {{ verificationCode }}
      </div>
      <div v-if="copied" class="text-success text-center small mb-2">验证码已复制</div>

      <div
        class="countdown"
        :class="{ expired }"
      >
        <template v-if="expired">验证码已过期，请重新生成</template>
        <template v-else>验证码有效期：{{ countdownText }} 后过期</template>
      </div>

      <!-- 双主按钮并排：复制验证码 + 打开私信页 -->
      <div class="d-flex gap-2 mt-3">
        <button class="btn btn-primary flex-fill" :disabled="loading" @click="copyCodeBtn">
          复制验证码
        </button>
        <button
          class="btn btn-outline-primary flex-fill"
          :disabled="loading || !messageUrl"
          @click="openMessagePage"
        >
          打开私信页
        </button>
      </div>
      <div
        v-if="copiedMsg"
        class="text-center small mt-1"
        :class="copiedMsgOk ? 'text-success' : 'text-danger'"
      >
        {{ copiedMsg }}
      </div>

      <!-- 扩展自动填充（次要链接，面向维护者；无扩展时点击等同复制+打开） -->
      <div class="text-center mt-2">
        <a
          href="#"
          class="small text-muted"
          :class="{ 'pe-none opacity-50': fillState === 'waiting' }"
          @click.prevent="handleAutoFill"
        >
          {{ fillState === 'waiting' ? '正在请求扩展…' : '已安装 NSAuth2 扩展？点此自动填充' }}
        </a>
      </div>
      <div
        v-if="fillMsg"
        class="ns-alert mt-2"
        :class="fillState === 'done' ? 'ns-alert-success' : 'ns-alert-info'"
      >
        {{ fillMsg }}
      </div>

      <div class="d-grid gap-2 mt-3">
        <button class="btn btn-primary" :disabled="loading" @click="handleConfirm">
          {{ loading ? '核验中…' : '我已发送验证码' }}
        </button>
        <button class="btn btn-link" :disabled="loading" @click="handleRegenerate">
          重新生成验证码
        </button>
        <button class="btn btn-link text-muted" :disabled="loading" @click="backToStep1">
          ← 返回修改 ID
        </button>
      </div>
    </div>

    <!-- 第三步：登录成功，展示用户 stats 与授权门槛提示 -->
    <div v-else>
      <div class="ns-alert ns-alert-success">
        登录成功！账号归属已通过私信验证码确认。
      </div>

      <!-- 用户信息卡片（confirm 响应附带 stats；拉取失败时为 null 则不显示） -->
      <div v-if="userStats" class="stats-grid mt-3">
        <div class="stat-item">
          <div class="stat-value">{{ userStats.rank }}</div>
          <div class="stat-label">等级</div>
        </div>
        <div class="stat-item">
          <div class="stat-value">{{ userStats.join_days }}</div>
          <div class="stat-label">加入天数</div>
        </div>
        <div class="stat-item">
          <div class="stat-value">{{ userStats.chicken }}</div>
          <div class="stat-label">鸡腿</div>
        </div>
        <div class="stat-item">
          <div class="stat-value">{{ userStats.topics }}</div>
          <div class="stat-label">主题帖</div>
        </div>
        <div class="stat-item">
          <div class="stat-value">{{ userStats.comments }}</div>
          <div class="stat-label">评论</div>
        </div>
      </div>

      <!-- 授权门槛提示（gate 中值为 0 的项不显示） -->
      <div v-if="gateText" class="ns-alert ns-alert-info mt-3">{{ gateText }}</div>
      <div v-else class="ns-alert ns-alert-info mt-3">授权门槛信息获取失败，请稍后重试</div>

      <button class="btn btn-primary w-100 mt-3" @click="goAfterLogin">
        进入服务（{{ redirectTarget }}）
      </button>
    </div>
  </div>
</template>
