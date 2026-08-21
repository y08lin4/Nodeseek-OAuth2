<script setup lang="ts">
// 门户首页（公开）：hero + 三方角色 + 三步私信验证流程 + 特性列表 + CTA
// CTA 随登录态切换：已登录显示「进入面板」→ /dashboard
import { onMounted, ref } from 'vue'
import { me } from '../api'

const loggedIn = ref(false)

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
      <h1 class="portal-title">NSAuth2 · Nodeseek OAuth2 授权服务</h1>
      <p class="portal-sub">用 NodeSeek 账号安全登录第三方应用，私信验证码确认归属，全程无需密码</p>
      <div class="portal-cta">
        <RouterLink v-if="!loggedIn" to="/login" class="btn btn-primary btn-lg">
          登录 Nodeseek 账号
        </RouterLink>
        <RouterLink v-else to="/dashboard" class="btn btn-primary btn-lg">进入面板</RouterLink>
        <RouterLink to="/docs" class="btn btn-outline-primary btn-lg">接入文档</RouterLink>
        <RouterLink to="/console" class="btn btn-outline-secondary btn-lg">申请接入</RouterLink>
      </div>
    </section>

    <!-- 三方角色 -->
    <section class="portal-section">
      <h2 class="portal-h2">三方角色</h2>
      <div class="role-grid">
        <div class="role-card">
          <div class="role-icon">🔐</div>
          <h3>授权服务方</h3>
          <p class="mb-0 text-muted small">
            维护系统账号，通过私信验证码确认账号归属，为第三方应用签发授权与令牌。
          </p>
        </div>
        <div class="role-card">
          <div class="role-icon">🧩</div>
          <h3>接入的应用</h3>
          <p class="mb-0 text-muted small">
            提交应用申请，审核通过后走标准 OAuth2 授权码流程，换取一次性访问令牌。
          </p>
        </div>
        <div class="role-card">
          <div class="role-icon">👤</div>
          <h3>用户</h3>
          <p class="mb-0 text-muted small">
            用 NS 数字 ID 登录，验证码经私信确认，授权状态随时可查、可撤销。
          </p>
        </div>
      </div>
    </section>

    <!-- 三步私信验证流程 -->
    <section class="portal-section">
      <h2 class="portal-h2">三步私信验证</h2>
      <div class="flow-steps">
        <div class="flow-step">
          <span class="flow-num">①</span>
          <div>
            <strong>输入 NS ID</strong>
            <p class="mb-0 text-muted small">纯数字，取自个人主页 URL</p>
          </div>
        </div>
        <span class="flow-arrow">→</span>
        <div class="flow-step">
          <span class="flow-num">②</span>
          <div>
            <strong>私信验证码</strong>
            <p class="mb-0 text-muted small">发给任一系统账号</p>
          </div>
        </div>
        <span class="flow-arrow">→</span>
        <div class="flow-step">
          <span class="flow-num">③</span>
          <div>
            <strong>确认登录</strong>
            <p class="mb-0 text-muted small">服务端核验私信后完成</p>
          </div>
        </div>
      </div>
    </section>

    <!-- 特性列表 -->
    <section class="portal-section">
      <h2 class="portal-h2">特性</h2>
      <div class="feature-list">
        <span class="feature-item">审核制应用</span>
        <span class="feature-item">多系统账号冗余</span>
        <span class="feature-item">授权统计</span>
        <span class="feature-item">等级门槛</span>
        <span class="feature-item">临时 token</span>
      </div>
    </section>
  </div>
</template>
