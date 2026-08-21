import { createRouter, createWebHistory, type NavigationGuard } from 'vue-router'
import HomeView from './views/HomeView.vue'
import LoginView from './views/LoginView.vue'
import AuthorizeView from './views/AuthorizeView.vue'
import ConsoleView from './views/ConsoleView.vue'
import GrantsView from './views/GrantsView.vue'
import DocsView from './views/DocsView.vue'
import DashboardView from './views/DashboardView.vue'
import NotFoundView from './views/NotFoundView.vue'
import { getAdminStatus, ApiError } from './api'
import AdminLayout from './views/admin/AdminLayout.vue'
import AdminLoginView from './views/admin/AdminLoginView.vue'
import AdminDashboardView from './views/admin/AdminDashboardView.vue'
import AdminUsersView from './views/admin/AdminUsersView.vue'
import AdminUserDetailView from './views/admin/AdminUserDetailView.vue'
import AdminAppsView from './views/admin/AdminAppsView.vue'
import AdminReviewsView from './views/admin/AdminReviewsView.vue'
import AdminAccountsView from './views/admin/AdminAccountsView.vue'
import AdminGrantsView from './views/admin/AdminGrantsView.vue'
import AdminAuditView from './views/admin/AdminAuditView.vue'
import AdminSettingsView from './views/admin/AdminSettingsView.vue'

// 管理后台守卫：GET /api/admin/status 探活，401（未登录）→ /admin/login?next=<path>
const adminGuard: NavigationGuard = async (to) => {
  try {
    await getAdminStatus()
    return true
  } catch (e) {
    const code = e instanceof ApiError ? e.statusCode : 0
    if (code === 401) {
      return { path: '/admin/login', query: { next: to.fullPath } }
    }
    // 非 401（网络/服务端异常）不阻断，交由子页自身容错提示
    return true
  }
}

// 路由配置（history 模式，SPA）
const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    // ?next= 登录成功后跳转目标
    { path: '/login', name: 'login', component: LoginView },
    // 授权确认页：client_id / redirect_uri / response_type 均来自 URL 查询参数
    { path: '/authorize', name: 'authorize', component: AuthorizeView },
    // 接入文档页（公开）
    { path: '/docs', name: 'docs', component: DocsView },
    // 登录后面板（需登录）
    { path: '/dashboard', name: 'dashboard', component: DashboardView },
    // 应用管理页（需登录）：注册应用 + 我的应用列表
    { path: '/console', name: 'console', component: ConsoleView },
    // 我的授权页（需登录）：查看与撤销授权
    { path: '/grants', name: 'grants', component: GrantsView },
    // 管理后台：登录页不进守卫；其余子路由经守卫探活
    { path: '/admin/login', name: 'admin-login', component: AdminLoginView },
    {
      path: '/admin',
      component: AdminLayout,
      redirect: '/admin/dashboard',
      beforeEnter: adminGuard,
      children: [
        { path: 'dashboard', name: 'admin-dashboard', component: AdminDashboardView },
        { path: 'users', name: 'admin-users', component: AdminUsersView },
        { path: 'users/:id', name: 'admin-user-detail', component: AdminUserDetailView },
        { path: 'apps', name: 'admin-apps', component: AdminAppsView },
        { path: 'reviews', name: 'admin-reviews', component: AdminReviewsView },
        { path: 'accounts', name: 'admin-accounts', component: AdminAccountsView },
        { path: 'grants', name: 'admin-grants', component: AdminGrantsView },
        { path: 'audit', name: 'admin-audit', component: AdminAuditView },
        { path: 'settings', name: 'admin-settings', component: AdminSettingsView },
      ],
    },
    // 404 兜底：未知路径渲染 NotFoundView
    { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundView },
  ],
})

export default router
