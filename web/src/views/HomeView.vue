<script setup lang="ts">
// 门户首页（公开）：全屏 Hero + 接入文档摘要区
// Hero CTA 随登录态切换；文档摘要锚点滚动到 #ns-docs
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NCard, NTag, NButton } from 'naive-ui'
import { me } from '../api'
import NSLogo from '../components/NSLogo.vue'
import { Blocks, Code, UserCheck } from 'lucide-vue-next'

const router = useRouter()
const loggedIn = ref(false)
const heroTags = ['OAuth2 标准', '私信验证码', '多账号冗余']

onMounted(async () => {
  try {
    const resp = await me()
    loggedIn.value = Boolean(resp?.user_id)
  } catch {
    loggedIn.value = false
  }
})

// 锚点滚动到接入文档摘要区
function scrollToDocs() {
  document.getElementById('ns-docs')?.scrollIntoView({ behavior: 'smooth' })
}
</script>

<template>
  <div class="portal">
    <!-- Hero：全屏首屏 -->
    <section class="portal-hero">
      <div class="portal-logo">
        <NSLogo :size="64" with-text />
      </div>
      <h1 class="portal-title">Nodeseek 非官方 OAuth2 授权服务</h1>
      <p class="portal-sub">用 NodeSeek 账号安全登录第三方应用，私信验证码确认归属，全程无需密码</p>
      <div class="portal-tags">
        <n-tag v-for="t in heroTags" :key="t" round>{{ t }}</n-tag>
      </div>
      <div class="portal-cta">
        <n-button v-if="!loggedIn" type="primary" size="large" @click="router.push('/login')">
          登录 Nodeseek 账号
        </n-button>
        <n-button v-else type="primary" size="large" @click="router.push('/dashboard')">
          进入面板
        </n-button>
        <n-button size="large" @click="scrollToDocs">接入文档</n-button>
        <n-button size="large" @click="router.push('/console')">申请接入</n-button>
      </div>
      <div class="portal-stats">已接入应用 · 今日授权</div>
    </section>

    <!-- 接入文档摘要 -->
    <section id="ns-docs" class="portal-section">
      <h2 class="portal-h2">接入文档</h2>
      <div class="role-grid">
        <n-card>
          <div class="role-icon" aria-hidden="true"><Blocks :size="20" :stroke-width="1.8" /></div>
          <h3>注册应用</h3>
          <p class="ns-mb-0 ns-text-muted ns-small">
            在「申请接入」创建应用，获取 client_id / client_secret。
          </p>
        </n-card>
        <n-card>
          <div class="role-icon" aria-hidden="true"><Code :size="20" :stroke-width="1.8" /></div>
          <h3>集成 OAuth</h3>
          <p class="ns-mb-0 ns-text-muted ns-small">
            标准授权码流程，接入约 10 分钟。
          </p>
        </n-card>
        <n-card>
          <div class="role-icon" aria-hidden="true"><UserCheck :size="20" :stroke-width="1.8" /></div>
          <h3>用户授权</h3>
          <p class="ns-mb-0 ns-text-muted ns-small">
            私信验证码确认归属，授权后可随时撤销。
          </p>
        </n-card>
      </div>
      <div class="docs-more">
        <n-button quaternary @click="router.push('/docs')">查看完整文档 →</n-button>
      </div>
    </section>
  </div>
</template>
