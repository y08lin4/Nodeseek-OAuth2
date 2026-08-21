<script setup lang="ts">
// 门户首页（公开）：hero + 三方角色 + 三步私信验证流程 + 特性列表 + CTA
// CTA 随登录态切换：已登录显示「进入面板」→ /dashboard
import { onMounted, ref } from 'vue'
import { NCard, NTag, NButton } from 'naive-ui'
import { me } from '../api'
import NSLogo from '../components/NSLogo.vue'

const loggedIn = ref(false)
// 特性列表
const features = ['审核制应用', '多系统账号冗余', '授权统计', '等级门槛', '临时 token']
// 副标题标签
const heroTags = ['OAuth2 标准', '私信验证码', '多账号冗余']

onMounted(async () => {
  try {
    const resp = await me()
    loggedIn.value = Boolean(resp?.user_id)
  } catch {
    loggedIn.value = false
  }
})
</script>

<template>
  <div class="portal">
    <!-- Hero -->
    <section class="portal-hero">
      <div class="portal-logo">
        <NSLogo :size="56" with-text />
      </div>
      <h1 class="portal-title">Nodeseek OAuth2 授权服务</h1>
      <p class="portal-sub">用 NodeSeek 账号安全登录第三方应用，私信验证码确认归属，全程无需密码</p>
      <div class="portal-tags">
        <n-tag v-for="t in heroTags" :key="t" round>{{ t }}</n-tag>
      </div>
      <div class="portal-cta">
        <n-button v-if="!loggedIn" type="primary" size="large" to="/login">登录 Nodeseek 账号</n-button>
        <n-button v-else type="primary" size="large" to="/dashboard">进入面板</n-button>
        <n-button size="large" to="/docs">接入文档</n-button>
        <n-button size="large" to="/console">申请接入</n-button>
      </div>
      <div class="portal-stats">已接入应用 · 今日授权</div>
    </section>

    <!-- 三方角色 -->
    <section class="portal-section">
      <h2 class="portal-h2">三方角色</h2>
      <div class="role-grid">
        <n-card>
          <div class="role-icon" aria-hidden="true">🔐</div>
          <h3>授权服务方</h3>
          <p class="ns-mb-0 ns-text-muted ns-small">
            维护系统账号，通过私信验证码确认账号归属，为第三方应用签发授权与令牌。
          </p>
        </n-card>
        <n-card>
          <div class="role-icon" aria-hidden="true">🧩</div>
          <h3>接入的应用</h3>
          <p class="ns-mb-0 ns-text-muted ns-small">
            提交应用申请，审核通过后走标准 OAuth2 授权码流程，换取一次性访问令牌。
          </p>
        </n-card>
        <n-card>
          <div class="role-icon" aria-hidden="true">👤</div>
          <h3>用户</h3>
          <p class="ns-mb-0 ns-text-muted ns-small">
            用 NS 数字 ID 登录，验证码经私信确认，授权状态随时可查、可撤销。
          </p>
        </n-card>
      </div>
    </section>

    <!-- 三步私信验证流程 -->
    <section class="portal-section">
      <h2 class="portal-h2">三步私信验证</h2>
      <div class="flow-steps">
        <div class="flow-step">
          <span class="flow-num">1</span>
          <div>
            <strong>输入 NS ID</strong>
            <p class="ns-mb-0 ns-text-muted ns-small">纯数字，取自个人主页 URL</p>
          </div>
        </div>
        <div class="flow-step">
          <span class="flow-num">2</span>
          <div>
            <strong>私信验证码</strong>
            <p class="ns-mb-0 ns-text-muted ns-small">发给任一系统账号</p>
          </div>
        </div>
        <div class="flow-step">
          <span class="flow-num">3</span>
          <div>
            <strong>确认登录</strong>
            <p class="ns-mb-0 ns-text-muted ns-small">服务端核验私信后完成</p>
          </div>
        </div>
      </div>
    </section>

    <!-- 特性列表 -->
    <section class="portal-section">
      <h2 class="portal-h2">特性</h2>
      <div class="feature-list">
        <n-tag v-for="f in features" :key="f" round>{{ f }}</n-tag>
      </div>
    </section>
  </div>
</template>
