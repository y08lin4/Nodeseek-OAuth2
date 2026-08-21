<script setup lang="ts">
// 根组件：全局 Naive UI Provider（zhCN + message + dialog）+ 导航（登录态探测/退出）+ 布局容器 + 页脚
import { onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  NConfigProvider,
  NMessageProvider,
  NDialogProvider,
  NButton,
  zhCN,
  dateZhCN,
} from 'naive-ui'
import { me, logout } from './api'

const route = useRoute()
const userId = ref('') // 已登录时为 NS 数字 ID，空 = 未登录

// 探测登录态：GET /api/me（401/网络错误均视为未登录，导航不报错）
async function refreshMe() {
  try {
    const resp = await me()
    userId.value = resp?.user_id ?? ''
  } catch {
    userId.value = ''
  }
}

// 退出登录：POST /api/logout 后刷新回未登录
async function handleLogout() {
  try {
    await logout()
  } catch {
    // 退出失败也重置前端登录态（会话可能已失效）
  }
  userId.value = ''
}

onMounted(refreshMe)
// 路由变化后重新探测（登录/退出后导航状态保持正确）
watch(() => route.fullPath, refreshMe)
</script>

<template>
  <n-config-provider :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider>
      <n-dialog-provider>
        <div class="app-shell">
          <!-- 全局导航 -->
          <nav class="app-nav">
            <div class="app-nav-inner">
              <RouterLink to="/" class="app-brand">Nodeseek OAuth2</RouterLink>
              <div class="app-nav-right">
                <!-- 接入文档：公开可见 -->
                <n-button text to="/docs">接入文档</n-button>
                <n-button v-if="!userId" type="primary" size="small" to="/login">登录</n-button>
                <template v-else>
                  <!-- 面板与我的授权入口：仅登录后显示 -->
                  <n-button text to="/dashboard">面板</n-button>
                  <n-button text to="/grants">我的授权</n-button>
                  <span class="app-user">ID {{ userId }}</span>
                  <n-button size="small" @click="handleLogout">退出登录</n-button>
                </template>
              </div>
            </div>
          </nav>

          <main class="ns-main">
            <router-view />
          </main>

          <footer class="app-footer ns-text-center">
            Nodeseek OAuth2 授权服务 · 私信验证码确认账号归属
          </footer>
        </div>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
