<script setup lang="ts">
// 根组件：全局 Naive UI Provider（zhCN + message + dialog）+ 导航（登录态探测/退出）+ 布局容器 + 页脚
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NConfigProvider,
  NMessageProvider,
  NDialogProvider,
  NButton,
  zhCN,
  dateZhCN,
} from 'naive-ui'
import { me, logout } from './api'
import { themeOverrides } from './theme'
import NSLogo from './components/NSLogo.vue'

const route = useRoute()
const router = useRouter()
const userId = ref('') // 已登录时为 NS 数字 ID，空 = 未登录
const showFloatLogin = ref(false) // 未登录且滚动超过 320px 时，顶栏浮现登录按钮

// 管理后台路由独立壳：不套首页 nav/main/footer（AdminLayout 自带顶栏+侧栏全屏布局）
const isAdmin = computed(() => route.path.startsWith('/admin'))

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

// 滚动监听：未登录时滚动超过 320px 显示顶栏登录按钮
function handleScroll() {
  showFloatLogin.value = window.scrollY > 320
}

onMounted(() => {
  refreshMe()
  window.addEventListener('scroll', handleScroll, { passive: true })
  handleScroll()
})

onUnmounted(() => {
  window.removeEventListener('scroll', handleScroll)
})

// 路由变化后重新探测（登录/退出后导航状态保持正确）
watch(() => route.fullPath, refreshMe)
</script>

<template>
  <n-config-provider :locale="zhCN" :date-locale="dateZhCN" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-dialog-provider>
        <!-- 管理后台：独立全屏壳（AdminLayout 自带顶栏/侧栏），不套首页导航与页脚 -->
        <router-view v-if="isAdmin" />
        <div v-else class="app-shell">
          <!-- 全局导航 -->
          <nav class="app-nav">
            <div class="app-nav-inner">
              <RouterLink to="/" class="app-brand"><NSLogo :size="28" /></RouterLink>
              <div class="app-nav-right">
                <!-- 浮动登录按钮：未登录且滚动后出现 -->
                <n-button
                  v-if="!userId && showFloatLogin"
                  type="primary"
                  size="small"
                  @click="router.push('/login')"
                >
                  登录
                </n-button>
                <template v-else-if="userId">
                  <!-- 面板与我的授权入口：仅登录后显示 -->
                  <router-link to="/dashboard" custom v-slot="{ navigate }"><n-button text @click="navigate">面板</n-button></router-link>
                  <router-link to="/grants" custom v-slot="{ navigate }"><n-button text @click="navigate">我的授权</n-button></router-link>
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
            Nodeseek 非官方 OAuth2 授权服务 · 私信验证码确认账号归属
          </footer>
        </div>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
