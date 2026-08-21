<script setup lang="ts">
// 管理后台框架：顶栏 + 浅色侧边栏 + 内容区（router-view）
// 守卫由 router 的 beforeEnter 处理；本地负责顶栏品牌/退出与侧栏导航/响应式抽屉。
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  LayoutDashboard,
  Blocks,
  ClipboardCheck,
  Users,
  ServerCog,
  KeyRound,
  ScrollText,
  Settings,
  LogOut,
  Menu,
  X,
} from 'lucide-vue-next'
import { NButton, useMessage } from 'naive-ui'
import { adminLogout } from '../../api'
import NSLogo from '../../components/NSLogo.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()

const drawerOpen = ref(false)
const loggingOut = ref(false)

// 导航项定义（分组）
const navGroups = [
  { items: [{ to: '/admin/dashboard', label: '仪表盘', icon: LayoutDashboard }] },
  {
    label: '管理',
    items: [
      { to: '/admin/users', label: '用户', icon: Users },
      { to: '/admin/apps', label: '应用', icon: Blocks },
      { to: '/admin/reviews', label: '审核', icon: ClipboardCheck },
      { to: '/admin/accounts', label: '账号', icon: ServerCog },
      { to: '/admin/grants', label: '授权记录', icon: KeyRound },
    ],
  },
  {
    label: '系统',
    items: [
      { to: '/admin/audit', label: '日志', icon: ScrollText },
      { to: '/admin/settings', label: '设置', icon: Settings },
    ],
  },
]

// 激活判断：精确匹配，或用户详情页 /admin/users/:id 也高亮「用户」项
function isActive(to: string): boolean {
  if (route.path === to) return true
  if (to === '/admin/users' && route.path.startsWith('/admin/users/')) return true
  return false
}

function go(to: string) {
  drawerOpen.value = false
  router.push(to)
}

// 退出登录：POST /api/admin/logout 后回登录页
async function handleLogout() {
  loggingOut.value = true
  try {
    await adminLogout()
  } catch {
    // 即便接口失败也清除本地视图会话，回登录页
  } finally {
    loggingOut.value = false
    message.success('已退出登录')
    router.replace('/admin/login')
  }
}
</script>

<template>
  <div class="admin-shell">
    <!-- 顶栏 -->
    <header class="admin-topbar">
      <div class="admin-topbar-inner">
        <div class="ns-flex ns-align-center ns-gap-2">
          <!-- 响应式：窄屏汉堡按钮 -->
          <n-button
            class="admin-hamburger"
            text
            :aria-label="drawerOpen ? '关闭菜单' : '打开菜单'"
            @click="drawerOpen = !drawerOpen"
          >
            <X v-if="drawerOpen" :size="22" />
            <Menu v-else :size="22" />
          </n-button>
          <RouterLink to="/admin/dashboard" class="admin-brand">
            <NSLogo :size="28" />
            <span class="admin-brand-text">NSAuth2 管理后台</span>
          </RouterLink>
        </div>
        <div class="ns-flex ns-align-center ns-gap-2">
          <n-button
            text
            :loading="loggingOut"
            @click="handleLogout"
            class="admin-logout-btn"
          >
            <template #icon><LogOut :size="16" /></template>
            退出登录
          </n-button>
        </div>
      </div>
    </header>

    <div class="admin-body">
      <!-- 侧边栏（桌面常驻；窄屏为顶部抽屉遮罩） -->
      <aside class="admin-sidebar" :class="{ 'admin-sidebar--open': drawerOpen }">
        <nav class="admin-nav">
          <template v-for="(group, gi) in navGroups" :key="gi">
            <div v-if="group.label" class="admin-nav-group">{{ group.label }}</div>
            <div
              v-for="item in group.items"
              :key="item.to"
              class="admin-nav-item"
              :class="{ 'is-active': isActive(item.to) }"
              role="button"
              tabindex="0"
              @click="go(item.to)"
              @keydown.enter="go(item.to)"
            >
              <component :is="item.icon" :size="16" class="admin-nav-icon" />
              <span>{{ item.label }}</span>
            </div>
          </template>
        </nav>
      </aside>
      <!-- 窄屏抽屉遮罩 -->
      <div v-if="drawerOpen" class="admin-sidebar-mask" @click="drawerOpen = false" />

      <!-- 内容区 -->
      <main class="admin-content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<style scoped>
.admin-shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--ns-bg);
}

.admin-topbar {
  position: sticky;
  top: 0;
  z-index: 100;
  background: #fff;
  border-bottom: 1px solid var(--ns-border);
}

.admin-topbar-inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 10px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.admin-brand {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  color: var(--ns-text);
}

.admin-brand-text {
  font-size: 15px;
  font-weight: 700;
}

.admin-hamburger {
  margin-left: -4px;
}

.admin-body {
  flex: 1;
  display: flex;
  max-width: 1420px;
  width: 100%;
  margin: 0 auto;
}

.admin-sidebar {
  width: 220px;
  flex-shrink: 0;
  background: #fff;
  border-right: 1px solid var(--ns-border);
  padding: 16px 12px;
}

.admin-nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.admin-nav-group {
  font-size: 12px;
  color: var(--ns-faint);
  padding: 12px 10px 4px;
}

.admin-nav-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 6px;
  font-size: 14px;
  color: var(--ns-text);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.admin-nav-item:hover {
  background: var(--ns-primary-soft);
}

.admin-nav-item.is-active {
  background: var(--ns-primary-soft);
  color: var(--ns-primary-hover);
  font-weight: 600;
  border-left: 3px solid var(--ns-primary);
}

.admin-nav-icon {
  flex-shrink: 0;
}

.admin-content {
  flex: 1;
  min-width: 0;
  max-width: 1200px;
  padding: 24px 20px 48px;
}

.admin-logout-btn {
  color: var(--ns-muted);
}

/* 窄屏抽屉 */
.admin-sidebar-mask {
  position: fixed;
  inset: 0;
  background: rgba(51, 51, 51, 0.3);
  z-index: 90;
}

@media (min-width: 768px) {
  .admin-hamburger {
    display: none;
  }
  .admin-sidebar-mask {
    display: none;
  }
}

@media (max-width: 767px) {
  .admin-body {
    max-width: 100%;
  }
  .admin-sidebar {
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    z-index: 95;
    transform: translateX(-100%);
    transition: transform 0.2s ease;
  }
  .admin-sidebar--open {
    transform: translateX(0);
  }
  .admin-content {
    max-width: 100%;
    padding: 16px 12px 40px;
  }
  .admin-brand-text {
    font-size: 14px;
  }
}
</style>
