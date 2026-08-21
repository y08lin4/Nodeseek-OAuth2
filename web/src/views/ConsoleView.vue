<script setup lang="ts">
// 应用管理页（需登录）：
// - 顶部展示创建门槛（/api/config 的 business.min_client_creation_rank）
// - 注册表单 → POST /api/client/register，成功后一次性展示 client_id + client_secret（仅此一次）
// - 我的应用列表 → GET /api/client/list（不含 secret）
import { computed, onMounted, ref } from 'vue'
import {
  getConfig,
  registerClient,
  listClients,
  pauseClient,
  resumeClient,
  deleteRequestClient,
  ApiError,
  type AppConfig,
  type RegisteredClient,
  type ClientListItem,
  type ClientStatus,
} from '../api'

const config = ref<AppConfig | null>(null)
const clients = ref<ClientListItem[]>([])
const credentials = ref<RegisteredClient | null>(null) // 注册成功的一次性凭据

const loading = ref(true)
const submitting = ref(false)
const error = ref('')
const formError = ref('')
const successMsg = ref('')
const copiedKey = ref('') // 复制的凭据项（client_id / client_secret），用于按钮反馈

// —— 注册表单状态 ——
const name = ref('')
const homepageUrl = ref('')
const description = ref('')
const redirectUrisText = ref('')
const iconUrl = ref('')
const minRank = ref('0')
const tokenTtl = ref('60') // access_token 有效期（分钟），默认 60，范围 1-1440

// 最低等级选项：NodeSeek 最高 6 级，0 = 不限（SPEC 3.3）
const minRankOptions = [0, 1, 2, 3, 4, 5, 6]

// 创建门槛文案（0 = 无门槛，不显示提示）
const creationGateText = computed(() => {
  const rank = config.value?.business.min_client_creation_rank
  if (!rank) return ''
  return `创建应用需 NodeSeek 等级 ≥ ${rank}`
})

// 校验 http(s) URL
function isValidHttpUrl(s: string): boolean {
  try {
    const u = new URL(s)
    return u.protocol === 'http:' || u.protocol === 'https:'
  } catch {
    return false
  }
}

// 回调地址：逗号分隔 → 去空白
function parseRedirectUris(text: string): string[] {
  return text
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
}

// 表单校验（本地完成，失败不发请求）
function validateForm(): string {
  if (!name.value.trim()) return '请填写应用名称'
  if (!homepageUrl.value.trim()) return '请填写应用主页（如 https://linux.do）'
  if (!isValidHttpUrl(homepageUrl.value.trim())) return '应用主页必须是合法的 http(s) URL'
  if (iconUrl.value.trim() && !isValidHttpUrl(iconUrl.value.trim())) {
    return '应用图标 URL 必须是合法的 http(s) URL'
  }
  const uris = parseRedirectUris(redirectUrisText.value)
  if (uris.length === 0) return '请至少填写一个回调地址'
  for (const u of uris) {
    if (!isValidHttpUrl(u)) return `回调地址不合法：${u}`
  }
  // access_token 有效期：整数分钟，范围 1-1440（对应秒 60-86400，契约 3.3）
  const ttl = Number(tokenTtl.value)
  if (!Number.isInteger(ttl) || ttl < 1 || ttl > 1440) {
    return 'access_token 有效期需为 1-1440 的整数（分钟）'
  }
  return ''
}

// 提交注册（提交前 confirm：提交后不可修改，走审核）
async function handleRegister() {
  error.value = ''
  formError.value = validateForm()
  if (formError.value) return
  // 提交前确认（SPEC §4：提交后不可修改）
  if (!window.confirm('提交后不可修改，确认提交？')) return
  submitting.value = true
  try {
    const resp = await registerClient({
      name: name.value.trim(),
      homepage_url: homepageUrl.value.trim(),
      description: description.value.trim(),
      redirect_uris: parseRedirectUris(redirectUrisText.value),
      icon_url: iconUrl.value.trim(),
      min_rank: Number(minRank.value),
      token_ttl: Number(tokenTtl.value) * 60, // 分钟 → 秒
    })
    // 凭据一次性展示；成功提示以响应 status 为准（mock 后端直接 approved）
    credentials.value = resp.client
    successMsg.value =
      resp.client.status === 'approved' ? '应用已创建并通过审核' : '已提交，等待审核'
    // 清空表单并刷新列表
    name.value = ''
    homepageUrl.value = ''
    description.value = ''
    redirectUrisText.value = ''
    iconUrl.value = ''
    minRank.value = '0'
    tokenTtl.value = '60'
    await loadList()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '创建应用失败，请重试'
  } finally {
    submitting.value = false
  }
}

// 复制凭据值
async function copyValue(key: string, value: string) {
  try {
    await navigator.clipboard.writeText(value)
    copiedKey.value = key
    setTimeout(() => (copiedKey.value = ''), 1500)
  } catch {
    // 剪贴板不可用时静默失败
  }
}

// 我的应用列表
async function loadList() {
  try {
    const resp = await listClients()
    clients.value = resp.clients
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '获取应用列表失败'
  }
}

// —— 审核制状态展示（SPEC 3.3 / §4）——
const statusMeta: Record<ClientStatus, { text: string; cls: string }> = {
  pending_review: { text: '审核中', cls: 'badge-pending' },
  approved: { text: '已通过', cls: 'badge-set' },
  rejected: { text: '未通过', cls: 'badge-danger' },
  paused: { text: '已暂停', cls: 'badge-unset' },
  pause_request: { text: '申请处理中', cls: 'badge-pending' },
  resume_request: { text: '申请处理中', cls: 'badge-pending' },
  delete_request: { text: '申请处理中', cls: 'badge-pending' },
}

function statusText(s: ClientStatus): string {
  return statusMeta[s]?.text ?? s
}

function statusClass(s: ClientStatus): string {
  return statusMeta[s]?.cls ?? 'badge-pending'
}

// —— 申请操作（pause/resume/delete-request，提交后等待管理员审核）——
const actingId = ref('')
async function handleStatusRequest(client: ClientListItem, action: 'pause' | 'resume' | 'delete') {
  const confirmText =
    action === 'pause'
      ? `确定申请暂停「${client.client_name}」？提交后需管理员审核。`
      : action === 'resume'
        ? `确定申请恢复「${client.client_name}」？提交后需管理员审核。`
        : `确定申请删除「${client.client_name}」？删除后不可恢复，历史授权将失效。`
  if (!window.confirm(confirmText)) return
  error.value = ''
  actingId.value = client.client_id
  try {
    if (action === 'pause') await pauseClient(client.client_id)
    else if (action === 'resume') await resumeClient(client.client_id)
    else await deleteRequestClient(client.client_id)
    successMsg.value = '已提交申请，等待审核'
    await loadList()
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '提交申请失败，请重试'
  } finally {
    actingId.value = ''
  }
}

// 登录检查：未登录（302 / 401 / 403）→ 跳登录页并带回跳地址
async function checkLogin(): Promise<boolean> {
  try {
    const res = await fetch('/api/client/list', {
      credentials: 'same-origin',
      redirect: 'manual',
    })
    if (res.type === 'opaqueredirect' || res.status === 401 || res.status === 403) {
      window.location.href = `/login?next=${encodeURIComponent(window.location.href)}`
      return false
    }
    return true
  } catch {
    return true // 网络异常不跳转，交给列表加载展示错误
  }
}

onMounted(async () => {
  // 全局配置（创建门槛提示）
  try {
    config.value = await getConfig()
  } catch {
    // 配置拉取失败不阻塞页面
  }
  const authed = await checkLogin()
  if (authed) {
    await loadList()
  }
  loading.value = false
})
</script>

<template>
  <div class="ns-card">
    <h1 class="ns-card-title">应用管理</h1>
    <p class="ns-card-sub">注册并管理接入 Nodeseek OAuth2 的第三方应用</p>

    <!-- 创建门槛提示 -->
    <div v-if="creationGateText" class="ns-alert ns-alert-info">{{ creationGateText }}</div>

    <div v-if="error" class="ns-alert ns-alert-error">{{ error }}</div>
    <div v-if="successMsg" class="ns-alert ns-alert-success">{{ successMsg }}</div>

    <!-- 注册成功：一次性凭据展示 -->
    <div v-if="credentials" class="credential-panel mt-3">
      <div class="credential-warning">⚠ client_secret 仅显示一次，请立即保存，关闭后无法再次查看</div>
      <div class="credential-row">
        <span class="credential-label">Client ID</span>
        <code class="credential-value">{{ credentials.client_id }}</code>
        <button class="btn btn-sm btn-outline-secondary" @click="copyValue('client_id', credentials.client_id)">
          {{ copiedKey === 'client_id' ? '已复制' : '复制' }}
        </button>
      </div>
      <div class="credential-row">
        <span class="credential-label">Client Secret</span>
        <code class="credential-value">{{ credentials.client_secret }}</code>
        <button class="btn btn-sm btn-outline-secondary" @click="copyValue('client_secret', credentials.client_secret)">
          {{ copiedKey === 'client_secret' ? '已复制' : '复制' }}
        </button>
      </div>
      <button class="btn btn-link text-muted small mt-2" @click="credentials = null">
        我已保存，关闭提示
      </button>
    </div>

    <!-- 注册表单 -->
    <h2 class="h6 mt-4 mb-3">注册新应用</h2>
    <div v-if="formError" class="ns-alert ns-alert-error">{{ formError }}</div>
    <form @submit.prevent="handleRegister">
      <div class="mb-3">
        <label for="app-name" class="form-label">应用名称</label>
        <input id="app-name" v-model="name" type="text" class="form-control" placeholder="如 My App" />
        <div class="form-text">应用名称需唯一（不区分大小写）。</div>
      </div>
      <div class="mb-3">
        <label for="app-homepage" class="form-label">应用主页</label>
        <input id="app-homepage" v-model="homepageUrl" type="text" class="form-control" placeholder="https://linux.do" />
      </div>
      <div class="mb-3">
        <label for="app-desc" class="form-label">应用描述</label>
        <textarea id="app-desc" v-model="description" class="form-control" rows="3" placeholder="简要描述应用用途"></textarea>
      </div>
      <div class="mb-3">
        <label for="app-redirects" class="form-label">回调地址</label>
        <input
          id="app-redirects"
          v-model="redirectUrisText"
          type="text"
          class="form-control"
          placeholder="https://app.example.com/callback, https://app.example.com/oauth/callback"
        />
        <div class="form-text">多个地址用英文逗号分隔，均为合法 http(s) URL。</div>
      </div>
      <div class="mb-3">
        <label for="app-icon" class="form-label">应用图标 URL（可选）</label>
        <input id="app-icon" v-model="iconUrl" type="text" class="form-control" placeholder="https://example.com/logo.png" />
      </div>
      <div class="mb-3">
        <label for="app-min-rank" class="form-label">最低等级</label>
        <select id="app-min-rank" v-model="minRank" class="form-select">
          <option v-for="r in minRankOptions" :key="r" :value="String(r)">
            {{ r === 0 ? '0 级（不限）' : `${r} 级` }}
          </option>
        </select>
        <div class="form-text">使用该应用登录的最低 NodeSeek 等级要求（NodeSeek 最高 6 级）。</div>
      </div>
      <div class="mb-3">
        <label for="app-token-ttl" class="form-label">access_token 有效期（分钟）</label>
        <input
          id="app-token-ttl"
          v-model="tokenTtl"
          type="number"
          min="1"
          max="1440"
          step="1"
          class="form-control"
        />
        <div class="form-text">授权码兑换的 access_token 有效时长，范围 1-1440 分钟（默认 60）。</div>
      </div>
      <button type="submit" class="btn btn-primary" :disabled="submitting">
        {{ submitting ? '提交中…' : '创建应用' }}
      </button>
    </form>

    <!-- 我的应用列表 -->
    <h2 class="h6 mt-5 mb-3">我的应用</h2>
    <div v-if="loading" class="text-center text-muted py-4">加载中…</div>
    <div v-else-if="clients.length === 0" class="text-center text-muted py-4">
      还没有应用，先创建一个吧。
    </div>
    <div v-else class="client-list">
      <div
        v-for="c in clients"
        :key="c.client_id"
        class="client-card"
        :class="{ 'client-card-disabled': c.status !== 'approved' }"
      >
        <div class="client-card-head">
          <img v-if="c.icon_url" :src="c.icon_url" alt="应用图标" class="client-icon" />
          <div v-else class="client-icon client-icon-placeholder">A</div>
          <div>
            <div class="client-name">{{ c.client_name }}</div>
            <code class="client-id">{{ c.client_id }}</code>
          </div>
          <div class="ms-auto d-flex align-items-center gap-2">
            <!-- 状态徽章（审核制） -->
            <span class="badge" :class="statusClass(c.status)">{{ statusText(c.status) }}</span>
            <span class="badge-set" :class="{ 'badge-unset': c.min_rank > 0 }">
              {{ c.min_rank > 0 ? `最低等级 ${c.min_rank}` : '不限等级' }}
            </span>
          </div>
        </div>
        <p v-if="c.description" class="client-desc">{{ c.description }}</p>
        <div class="detail-row">
          <span class="detail-label">主页</span>
          <span class="detail-value">
            <a v-if="c.homepage_url" :href="c.homepage_url" target="_blank" rel="noopener noreferrer">
              {{ c.homepage_url }}
            </a>
            <template v-else>—</template>
          </span>
        </div>
        <div class="detail-row">
          <span class="detail-label">回调地址</span>
          <span class="detail-value">
            <span v-for="u in c.redirect_uris" :key="u" class="d-block">{{ u }}</span>
          </span>
        </div>
        <div class="detail-row">
          <span class="detail-label">token 有效期</span>
          <span class="detail-value">{{ c.token_ttl / 60 }} 分钟</span>
        </div>
        <!-- 授权统计行（今日/累计成功失败） -->
        <div class="detail-row">
          <span class="detail-label">授权统计</span>
          <span class="detail-value">
            今日 成功 {{ c.stats.auth_ok_today }} · 失败 {{ c.stats.auth_fail_today }}
            ｜ 累计 成功 {{ c.stats.auth_ok_total }} · 失败 {{ c.stats.auth_fail_total }}
          </span>
        </div>
        <!-- 审核制操作按钮：仅 approved / paused 状态可申请 -->
        <div
          v-if="c.status === 'approved' || c.status === 'paused'"
          class="client-card-actions"
        >
          <button
            v-if="c.status === 'approved'"
            class="btn btn-sm btn-outline-warning"
            :disabled="actingId === c.client_id"
            @click="handleStatusRequest(c, 'pause')"
          >
            {{ actingId === c.client_id ? '提交中…' : '申请暂停' }}
          </button>
          <button
            v-if="c.status === 'paused'"
            class="btn btn-sm btn-outline-success"
            :disabled="actingId === c.client_id"
            @click="handleStatusRequest(c, 'resume')"
          >
            {{ actingId === c.client_id ? '提交中…' : '申请恢复' }}
          </button>
          <button
            class="btn btn-sm btn-outline-danger"
            :disabled="actingId === c.client_id"
            @click="handleStatusRequest(c, 'delete')"
          >
            {{ actingId === c.client_id ? '提交中…' : '申请删除' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
