<script setup lang="ts">
// 授权确认页：
// 从 URL 读取 client_id / redirect_uri / response_type，
// 先经 GET /oauth/authorize 校验登录态与参数合法性（未登录 302 → 跳登录），
// 再 GET /api/oauth/client 展示应用信息、当前用户 stats 与门槛状态；
// 门槛不满足时该接口返回 403 → 渲染错误面板（不显示同意/拒绝按钮）。
// 同意/拒绝 → POST /oauth/authorize/decision。
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { NCard, NAlert, NTag, NButton, NSpin } from 'naive-ui'
import { getClient, ApiError, type ClientInfo } from '../api'

const route = useRoute()

// —— 从 URL 查询参数读取授权请求（契约 3.3）——
const clientId = computed(() => (typeof route.query.client_id === 'string' ? route.query.client_id : ''))
const redirectUri = computed(() =>
  typeof route.query.redirect_uri === 'string' ? route.query.redirect_uri : '',
)
const responseType = computed(() =>
  typeof route.query.response_type === 'string' ? route.query.response_type : '',
)

const info = ref<ClientInfo | null>(null) // client + stats + gate
const gateError = ref('') // 403：门槛未满足，渲染错误面板
const error = ref('') // 其他错误（参数无效/404/网络等）
const loading = ref(true)
const submitting = ref(false)
const granted = ref(false) // 已提交决策，等待跳转

const paramsOk = computed(() => clientId.value && redirectUri.value && responseType.value)

// 门槛文案片段（值为 0 的项不显示，SPEC 3.8）
const gateParts = computed(() => {
  const g = info.value?.gate
  if (!g) return [] as string[]
  const parts: string[] = []
  if (g.min_rank > 0) parts.push(`等级 ≥ ${g.min_rank}`)
  if (g.min_join_days > 0) parts.push(`加入天数 ≥ ${g.min_join_days} 天`)
  return parts
})

// 校验授权参数 + 登录态：
// GET /oauth/authorize 未登录时返回 302 /login?next=...，
// 用 redirect:'manual' 捕获 opaque redirect，未登录则整页跳登录页。
async function checkAuth() {
  const qs = new URLSearchParams({
    client_id: clientId.value,
    redirect_uri: redirectUri.value,
    response_type: responseType.value,
  })
  let res: Response
  try {
    res = await fetch(`/oauth/authorize?${qs.toString()}`, {
      credentials: 'same-origin',
      redirect: 'manual',
    })
  } catch {
    error.value = '网络请求失败，请确认服务端已启动'
    return false
  }
  if (res.type === 'opaqueredirect') {
    // 未登录：302 到 /login，把当前完整 URL 带过去作为 next
    window.location.href = `/login?next=${encodeURIComponent(window.location.href)}`
    return false
  }
  if (!res.ok) {
    // 参数不合法（400/422 JSON）
    try {
      const data = await res.json()
      error.value = data?.message || `请求失败（HTTP ${res.status}）`
    } catch {
      error.value = `授权请求参数无效（HTTP ${res.status}）`
    }
    return false
  }
  return true
}

onMounted(async () => {
  if (!paramsOk.value) {
    error.value = '缺少必要参数：client_id / redirect_uri / response_type'
    loading.value = false
    return
  }
  const authed = await checkAuth()
  if (!authed) {
    loading.value = false
    return
  }
  // 拉取应用信息 + 用户 stats + 门槛状态（门槛不满足 → 403 → 错误面板）
  try {
    const resp = await getClient(clientId.value)
    info.value = resp
  } catch (e) {
    if (e instanceof ApiError && e.statusCode === 403) {
      gateError.value = e.message
    } else {
      error.value = e instanceof ApiError ? e.message : '获取应用信息失败'
    }
  } finally {
    loading.value = false
  }
})

// 提交授权决策（JSON body，契约 3.3）：
// 后端成功返回 302 redirect_uri?code=...（同意）或 redirect_uri?error=access_denied（拒绝）。
// - 同源 redirect_uri：fetch 跟随 302，res.url 即最终地址，整页导航过去；
// - 跨域 redirect_uri：浏览器拦截跨域跟随抛 TypeError，此时决策已被服务端受理，
//   提示用户授权完成并给出应用地址（生产环境建议 redirect_uri 与本站同源，见 notes）。
async function submitDecision(approve: boolean) {
  submitting.value = true
  try {
    const res = await fetch('/oauth/authorize/decision', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        approve,
        client_id: clientId.value,
        redirect_uri: redirectUri.value,
        response_type: responseType.value,
      }),
    })
    if (res.url.includes('/login')) {
      // 会话已过期，被 302 到登录页：跟随导航
      window.location.href = res.url
      return
    }
    if (res.ok || res.type === 'opaqueredirect') {
      // 决策已受理：跟随（同源）或提示（跨域）
      granted.value = true
      if (res.type !== 'opaqueredirect' && res.url) {
        window.location.href = res.url
      }
    } else {
      // 服务端复检门槛失败（403）等：展示 message
      const data = await res.json().catch(() => null)
      if (res.status === 403) {
        gateError.value = data?.message || '授权门槛校验未通过'
      } else {
        error.value = data?.message || `请求失败（HTTP ${res.status}）`
      }
    }
  } catch {
    // 跨域 302 被浏览器拦截：决策已被服务端受理，授权流程完成
    granted.value = true
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <n-card class="page-card">
    <template #header>
      <span class="page-title">授权确认</span>
    </template>
    <p class="ns-card-sub">第三方应用请求使用你的 Nodeseek 账号登录</p>

    <n-alert v-if="error" type="error" :show-icon="true" class="mb-3">{{ error }}</n-alert>

    <!-- 门槛未满足（403）：错误面板，不显示同意/拒绝按钮 -->
    <div v-if="gateError" class="gate-error-panel mt-2">
      <div class="gate-error-icon" aria-hidden="true">⛔</div>
      <h2 class="gate-error-title">无法完成授权</h2>
      <p class="gate-error-msg">{{ gateError }}</p>
      <p class="text-muted small mb-4">
        你的 NodeSeek 账号暂不满足该应用的授权门槛，可提升等级或加入时长后再试。
      </p>
      <div class="d-flex gap-2 justify-content-center">
        <n-button to="/">返回首页</n-button>
        <n-button type="primary" to="/login">重新登录</n-button>
      </div>
    </div>

    <template v-else>
      <n-spin :show="loading">
        <div v-if="!loading && info" class="mt-2">
          <!-- 应用信息 -->
          <div class="app-head d-flex align-items-center gap-3 mb-3">
            <img
              v-if="info.client.icon_url"
              :src="info.client.icon_url"
              alt="应用图标"
              class="app-icon"
            />
            <div v-else class="app-icon app-icon-placeholder" aria-hidden="true">A</div>
            <div>
              <div class="app-name">{{ info.client.client_name }}</div>
              <a
                v-if="info.client.homepage_url"
                :href="info.client.homepage_url"
                target="_blank"
                rel="noopener noreferrer"
                class="text-muted small"
              >
                {{ info.client.homepage_url }}
              </a>
            </div>
          </div>
          <p v-if="info.client.description" class="text-muted mb-3">{{ info.client.description }}</p>

          <n-alert type="info" class="mb-3">
            <strong>{{ info.client.client_name }}</strong> 请求获得你的 NodeSeek 账号授权。
            <template v-if="granted">授权已完成，正在跳转至应用…</template>
          </n-alert>

          <!-- 门槛状态 -->
          <div class="detail-row">
            <span class="detail-label">授权门槛</span>
            <span class="detail-value">
              <n-tag v-if="info.gate.ok" type="success" size="small" round>
                {{ gateParts.length ? `已满足（${gateParts.join(' · ')}）` : '无门槛' }}
              </n-tag>
              <n-tag v-else type="error" size="small" round>不满足授权门槛</n-tag>
            </span>
          </div>

          <!-- 当前用户 stats -->
          <div v-if="info.stats" class="stats-grid mt-3">
            <div class="stat-item">
              <div class="stat-value">{{ info.stats.rank }}</div>
              <div class="stat-label">等级</div>
            </div>
            <div class="stat-item">
              <div class="stat-value">{{ info.stats.join_days }}</div>
              <div class="stat-label">加入天数</div>
            </div>
            <div class="stat-item">
              <div class="stat-value">{{ info.stats.chicken }}</div>
              <div class="stat-label">鸡腿</div>
            </div>
            <div class="stat-item">
              <div class="stat-value">{{ info.stats.topics }}</div>
              <div class="stat-label">主题帖</div>
            </div>
            <div class="stat-item">
              <div class="stat-value">{{ info.stats.comments }}</div>
              <div class="stat-label">评论</div>
            </div>
          </div>

          <div class="detail-row">
            <span class="detail-label">应用 ID</span>
            <span class="detail-value">{{ info.client.client_id }}</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">开发者 NS ID</span>
            <span class="detail-value">{{ info.client.owner_user_id }}</span>
          </div>
          <div class="detail-row" v-if="info.client.min_rank !== undefined">
            <span class="detail-label">最低等级要求</span>
            <span class="detail-value">
              {{ info.client.min_rank > 0 ? `等级 ≥ ${info.client.min_rank}` : '不限' }}
            </span>
          </div>
          <div class="detail-row">
            <span class="detail-label">回调地址</span>
            <span class="detail-value">
              <span v-for="u in info.client.redirect_uris" :key="u" class="d-block">{{ u }}</span>
            </span>
          </div>
          <div class="detail-row">
            <span class="detail-label">授权方式</span>
            <span class="detail-value">Authorization Code（{{ responseType }}）</span>
          </div>
          <div class="detail-row">
            <span class="detail-label">授权回调</span>
            <span class="detail-value">{{ redirectUri }}</span>
          </div>

          <n-alert v-if="granted" type="success" :show-icon="true" class="mt-4">
            已授权，正在跳转至应用（{{ redirectUri }}）。若未自动跳转请直接访问该地址。
          </n-alert>
          <div v-else class="d-flex gap-3 mt-4">
            <n-button type="primary" block :loading="submitting" @click="submitDecision(true)">
              同意授权
            </n-button>
            <n-button block :disabled="submitting" @click="submitDecision(false)">拒绝</n-button>
          </div>
        </div>

        <n-empty
          v-if="!loading && !info && !error"
          description="未找到该应用信息"
          class="py-4"
        />
      </n-spin>
    </template>
  </n-card>
</template>
