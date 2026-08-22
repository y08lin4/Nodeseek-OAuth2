<script setup lang="ts">
// 应用管理页（需登录）：
// - 顶部展示创建门槛（/api/config 的 business.min_client_creation_rank）
// - 注册表单 → POST /api/client/register（提交前 useDialog 确认），成功后一次性展示 client_id + client_secret
// - 我的应用列表 → GET /api/client/list（不含 secret），按审核状态展示徽章与申请按钮
import { computed, onMounted, ref } from 'vue'
import {
  NCard,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NSelect,
  NButton,
  NTag,
  NAlert,
  NSpin,
  useMessage,
  useDialog,
} from 'naive-ui'
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
import PageHeader from '../components/ui/PageHeader.vue'
import EmptyState from '../components/ui/EmptyState.vue'
import MetricPair from '../components/ui/MetricPair.vue'

const message = useMessage()
const dialog = useDialog()

const config = ref<AppConfig | null>(null)
const clients = ref<ClientListItem[]>([])
const credentials = ref<RegisteredClient | null>(null) // 注册成功的一次性凭据

const loading = ref(true)
const submitting = ref(false)

// —— 注册表单状态 ——
const name = ref('')
const homepageUrl = ref('')
const description = ref('')
const redirectUrisText = ref('')
const iconUrl = ref('')
const minRank = ref<number>(0)
const tokenTtl = ref<number>(60) // access_token 有效期（分钟），默认 60，范围 1-1440
const notifyEmail = ref('') // 通知邮箱（可选）

// 最低等级选项：NodeSeek 最高 6 级，0 = 不限（SPEC 3.3）
const minRankOptions = [0, 1, 2, 3, 4, 5, 6].map((r) => ({
  label: r === 0 ? '0 级（不限）' : `${r} 级`,
  value: r,
}))

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

// 校验邮箱（简单格式校验）
function isValidEmail(s: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s.trim())
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
  const ttl = tokenTtl.value
  if (!Number.isInteger(ttl) || ttl < 1 || ttl > 1440) {
    return 'access_token 有效期需为 1-1440 的整数（分钟）'
  }
  // 通知邮箱可选；非空时须为合法邮箱
  const email = notifyEmail.value.trim()
  if (email && !isValidEmail(email)) {
    return '通知邮箱格式不正确'
  }
  return ''
}

// 提交注册（提交前确认：提交后不可修改，走审核）
function handleRegister() {
  const err = validateForm()
  if (err) {
    message.error(err)
    return
  }
  // 提交前确认（SPEC §4：提交后不可修改）
  dialog.warning({
    title: '确认提交',
    content: '提交后不可修改，确认提交？',
    positiveText: '确认提交',
    negativeText: '取消',
    onPositiveClick: async () => {
      submitting.value = true
      try {
        const resp = await registerClient({
          name: name.value.trim(),
          homepage_url: homepageUrl.value.trim(),
          description: description.value.trim(),
          redirect_uris: parseRedirectUris(redirectUrisText.value),
          icon_url: iconUrl.value.trim(),
          min_rank: minRank.value,
          token_ttl: tokenTtl.value * 60, // 分钟 → 秒
          notify_email: notifyEmail.value.trim() || undefined,
        })
        // 凭据一次性展示；成功提示以响应 status 为准（mock 后端直接 approved）
        credentials.value = resp.client
        message.success(
          resp.client.status === 'approved' ? '应用已创建并通过审核' : '已提交，等待审核',
        )
        // 清空表单并刷新列表
        name.value = ''
        homepageUrl.value = ''
        description.value = ''
        redirectUrisText.value = ''
        iconUrl.value = ''
        minRank.value = 0
        tokenTtl.value = 60
        notifyEmail.value = ''
        await loadList()
      } catch (e) {
        message.error(e instanceof ApiError ? e.message : '创建应用失败，请重试')
      } finally {
        submitting.value = false
      }
    },
  })
}

// 复制凭据值
async function copyValue(key: 'client_id' | 'client_secret', value: string) {
  try {
    await navigator.clipboard.writeText(value)
    message.success(key === 'client_id' ? '已复制 Client ID' : '已复制 Client Secret')
  } catch {
    message.error('复制失败，请手动复制')
  }
}

// 我的应用列表
async function loadList() {
  try {
    const resp = await listClients()
    clients.value = resp.clients
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '获取应用列表失败')
  }
}

// —— 审核制状态展示（SPEC 3.3 / §4）——
const statusMeta: Record<ClientStatus, { text: string; type: 'default' | 'info' | 'success' | 'warning' | 'error' }> = {
  pending_review: { text: '审核中', type: 'info' },
  approved: { text: '已通过', type: 'success' },
  rejected: { text: '未通过', type: 'error' },
  paused: { text: '已暂停', type: 'warning' },
  pause_request: { text: '申请处理中', type: 'default' },
  resume_request: { text: '申请处理中', type: 'default' },
  delete_request: { text: '申请处理中', type: 'default' },
}

function statusText(s: ClientStatus): string {
  return statusMeta[s]?.text ?? s
}

function statusType(s: ClientStatus): 'default' | 'info' | 'success' | 'warning' | 'error' {
  return statusMeta[s]?.type ?? 'default'
}

// —— 申请操作（pause/resume/delete-request，提交后等待管理员审核）——
const actingId = ref('')
function handleStatusRequest(client: ClientListItem, action: 'pause' | 'resume' | 'delete') {
  const confirmText =
    action === 'pause'
      ? `确定申请暂停「${client.client_name}」？提交后需管理员审核。`
      : action === 'resume'
        ? `确定申请恢复「${client.client_name}」？提交后需管理员审核。`
        : `确定申请删除「${client.client_name}」？删除后不可恢复，历史授权将失效。`
  dialog.warning({
    title: '提交申请',
    content: confirmText,
    positiveText: '提交',
    negativeText: '取消',
    onPositiveClick: async () => {
      actingId.value = client.client_id
      try {
        if (action === 'pause') await pauseClient(client.client_id)
        else if (action === 'resume') await resumeClient(client.client_id)
        else await deleteRequestClient(client.client_id)
        message.success('已提交申请，等待审核')
        await loadList()
      } catch (e) {
        message.error(e instanceof ApiError ? e.message : '提交申请失败，请重试')
      } finally {
        actingId.value = ''
      }
    },
  })
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
  <div class="user-page">
    <PageHeader title="应用管理" subtitle="注册并管理接入 Nodeseek OAuth2 的第三方应用" />

    <!-- 创建门槛提示 -->
    <n-alert v-if="creationGateText" type="info" class="ns-mb-3">{{ creationGateText }}</n-alert>

    <!-- 注册成功：一次性凭据展示 -->
    <n-card v-if="credentials" size="small" class="ns-mt-3 ns-mb-4">
      <n-alert type="warning" :show-icon="true" class="ns-mb-3">
        client_secret 仅显示一次，请立即保存，关闭后无法再次查看
      </n-alert>
      <div class="credential-row">
        <span class="credential-label">Client ID</span>
        <code class="credential-value">{{ credentials.client_id }}</code>
        <n-button size="small" @click="copyValue('client_id', credentials.client_id)">
          复制
        </n-button>
      </div>
      <div class="credential-row">
        <span class="credential-label">Client Secret</span>
        <code class="credential-value">{{ credentials.client_secret }}</code>
        <n-button size="small" @click="copyValue('client_secret', credentials.client_secret)">
          复制
        </n-button>
      </div>
      <n-button text size="small" class="ns-mt-2" @click="credentials = null">
        我已保存，关闭提示
      </n-button>
    </n-card>

    <!-- 注册表单 -->
    <h2 class="ns-h6 ns-mt-2 ns-mb-3">注册新应用</h2>
    <n-form>
      <n-form-item label="应用名称">
        <n-input v-model:value="name" placeholder="如 My App" />
        <template #feedback>应用名称需唯一（不区分大小写）。</template>
      </n-form-item>
      <n-form-item label="应用主页">
        <n-input v-model:value="homepageUrl" placeholder="https://linux.do" />
      </n-form-item>
      <n-form-item label="应用描述">
        <n-input
          v-model:value="description"
          type="textarea"
          :rows="3"
          placeholder="简要描述应用用途"
        />
      </n-form-item>
      <n-form-item label="回调地址">
        <n-input
          v-model:value="redirectUrisText"
          placeholder="https://app.example.com/callback, https://app.example.com/oauth/callback"
        />
        <template #feedback>多个地址用英文逗号分隔，均为合法 http(s) URL。</template>
      </n-form-item>
      <n-form-item label="应用图标 URL（可选）">
        <n-input v-model:value="iconUrl" placeholder="https://example.com/logo.png" />
      </n-form-item>
      <n-form-item label="最低等级">
        <n-select v-model:value="minRank" :options="minRankOptions" />
        <template #feedback>
          使用该应用登录的最低 NodeSeek 等级要求（NodeSeek 最高 6 级）。
        </template>
      </n-form-item>
      <n-form-item label="access_token 有效期（分钟）">
        <n-input-number v-model:value="tokenTtl" :min="1" :max="1440" :step="1" style="width: 100%" />
        <template #feedback>授权码兑换的 access_token 有效时长，范围 1-1440 分钟（默认 60）。</template>
      </n-form-item>
      <n-form-item label="通知邮箱（可选）">
        <n-input
          v-model:value="notifyEmail"
          placeholder="用于接收审核结果与错误率告警"
          :input-props="{ type: 'email', autocomplete: 'email' }"
        />
        <template #feedback>非必填；用于接收审核结果通知与错误率告警邮件。</template>
      </n-form-item>
      <n-button type="primary" :loading="submitting" @click="handleRegister">
        创建应用
      </n-button>
    </n-form>

    <!-- 我的应用列表 -->
    <h2 class="ns-h6 ns-mt-5 ns-mb-3">我的应用</h2>
    <n-spin :show="loading">
      <EmptyState v-if="!loading && clients.length === 0" description="还没有应用，先创建一个吧。" />
      <div v-else class="review-list">
        <n-card
          v-for="c in clients"
          :key="c.client_id"
          size="small"
          class="review-item"
          :class="{ 'client-card-disabled': c.status !== 'approved' }"
        >
          <div class="ns-flex ns-align-center ns-gap-3 ns-mb-2">
            <img v-if="c.icon_url" :src="c.icon_url" alt="应用图标" class="client-icon" />
            <div v-else class="client-icon client-icon-placeholder">A</div>
            <div class="ns-flex-grow-1">
              <div class="review-name">{{ c.client_name }}</div>
              <code class="review-client-id">{{ c.client_id }}</code>
            </div>
            <!-- 状态徽章（审核制） -->
            <n-tag :type="statusType(c.status)" size="small" round>{{ statusText(c.status) }}</n-tag>
            <n-tag :type="c.min_rank > 0 ? 'warning' : 'default'" size="small" round>
              {{ c.min_rank > 0 ? `最低等级 ${c.min_rank}` : '不限等级' }}
            </n-tag>
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
              <span v-for="u in c.redirect_uris" :key="u" class="ns-d-block">{{ u }}</span>
            </span>
          </div>
          <div class="detail-row">
            <span class="detail-label">token 有效期</span>
            <span class="detail-value">{{ c.token_ttl / 60 }}<span class="stat-unit"> 分钟</span></span>
          </div>
          <!-- 授权统计行（今日/累计成功失败） -->
          <div class="detail-row">
            <span class="detail-label">授权统计</span>
            <span class="detail-value">
              今日 <MetricPair :ok="c.stats.auth_ok_today" :fail="c.stats.auth_fail_today" />
              ｜ 累计 <MetricPair :ok="c.stats.auth_ok_total" :fail="c.stats.auth_fail_total" />
            </span>
          </div>
          <!-- 审核制操作按钮：仅 approved / paused 状态可申请 -->
          <div
            v-if="c.status === 'approved' || c.status === 'paused'"
            class="review-actions"
          >
            <n-button
              v-if="c.status === 'approved'"
              size="small"
              type="warning"
              :loading="actingId === c.client_id"
              @click="handleStatusRequest(c, 'pause')"
            >
              申请暂停
            </n-button>
            <n-button
              v-if="c.status === 'paused'"
              size="small"
              type="success"
              :loading="actingId === c.client_id"
              @click="handleStatusRequest(c, 'resume')"
            >
              申请恢复
            </n-button>
            <n-button
              size="small"
              type="error"
              :loading="actingId === c.client_id"
              @click="handleStatusRequest(c, 'delete')"
            >
              申请删除
            </n-button>
          </div>
        </n-card>
      </div>
    </n-spin>
  </div>
</template>
