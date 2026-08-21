import { createRouter, createWebHistory } from 'vue-router'
import HomeView from './views/HomeView.vue'
import LoginView from './views/LoginView.vue'
import AuthorizeView from './views/AuthorizeView.vue'
import ConsoleView from './views/ConsoleView.vue'
import GrantsView from './views/GrantsView.vue'
import DocsView from './views/DocsView.vue'
import DashboardView from './views/DashboardView.vue'
import AdminView from './views/AdminView.vue'
import NotFoundView from './views/NotFoundView.vue'

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
    // 管理页：Admin Token 保存在 localStorage
    { path: '/admin', name: 'admin', component: AdminView },
    // 404 兜底：未知路径渲染 NotFoundView
    { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundView },
  ],
})

export default router
