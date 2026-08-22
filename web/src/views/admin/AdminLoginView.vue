<script setup lang="ts">
// 管理后台登录页：账号密码 → POST /api/admin/login 签发 httpOnly 会话
// 成功跳 ?next 或 /admin/dashboard
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NCard, NInput, NButton, NAlert, useMessage } from 'naive-ui'
import { adminLogin, ApiError } from '../../api'
import NSLogo from '../../components/NSLogo.vue'

const route = useRoute()
const router = useRouter()
const message = useMessage()

const username = ref('')
const password = ref('')
const loading = ref(false)
const errorText = ref('')

async function handleLogin() {
  if (!username.value.trim() || !password.value) {
    errorText.value = '请输入用户名和密码'
    return
  }
  errorText.value = ''
  loading.value = true
  try {
    await adminLogin(username.value.trim(), password.value)
    message.success('登录成功')
    const next = typeof route.query.next === 'string' ? route.query.next : ''
    router.replace(next && next.startsWith('/admin') ? next : '/admin/dashboard')
  } catch (e) {
    errorText.value = e instanceof ApiError ? e.message : '登录失败，请重试'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="admin-login">
    <n-card class="admin-login-card">
      <div class="admin-login-brand">
        <NSLogo :size="40" with-text />
      </div>
      <h1 class="admin-login-title">管理后台登录</h1>
      <n-alert v-if="errorText" type="error" class="ns-mb-3">{{ errorText }}</n-alert>
      <n-form @submit.prevent="handleLogin">
        <n-form-item label="用户名">
          <n-input
            v-model:value="username"
            placeholder="服务端 NS_ADMIN_USER 配置"
            :input-props="{ autocomplete: 'username' }"
            @keydown.enter="handleLogin"
          />
        </n-form-item>
        <n-form-item label="密码">
          <n-input
            v-model:value="password"
            type="password"
            show-password-on="click"
            placeholder="服务端 NS_ADMIN_PASSWORD 配置"
            :input-props="{ autocomplete: 'current-password' }"
            @keydown.enter="handleLogin"
          />
        </n-form-item>
        <n-button type="primary" block :loading="loading" @click="handleLogin">
          登录
        </n-button>
      </n-form>
      <p class="admin-login-hint ns-mt-3">
        账号密码在服务端通过 <code>NS_ADMIN_USER</code> / <code>NS_ADMIN_PASSWORD</code> 环境变量配置，登录成功后签发管理会话。
      </p>
    </n-card>
  </div>
</template>

<style scoped>
.admin-login {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--ns-bg);
  padding: 20px;
}

.admin-login-card {
  width: 100%;
  max-width: 420px;
  border-radius: 6px;
}

.admin-login-brand {
  display: flex;
  justify-content: center;
  margin-bottom: 8px;
}

.admin-login-title {
  font-size: 20px;
  font-weight: 700;
  text-align: center;
  color: var(--ns-text);
  margin-bottom: 20px;
}

.admin-login-hint {
  font-size: 12px;
  color: var(--ns-faint);
  text-align: center;
  margin-bottom: 0;
}

.admin-login-hint code {
  font-family: 'SFMono-Regular', Consolas, Menlo, monospace;
}
</style>
