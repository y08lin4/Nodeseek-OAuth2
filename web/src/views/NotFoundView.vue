<script setup lang="ts">
// 404 页面：未知路径兜底（router catch-all）
import { useRouter } from 'vue-router'
import { NCard, NButton } from 'naive-ui'

const router = useRouter()
// 返回首页（带 query 参数跳转时保留，直接去首页）
function goHome() {
  router.push('/')
}
</script>

<template>
  <div class="user-page">
    <div class="not-found">
      <n-card class="not-found-card">
        <div class="not-found-code">404</div>
        <h1 class="page-title">页面不存在</h1>
        <p class="ns-text-muted ns-mt-2 ns-mb-4">你访问的页面不存在或已被移除，请返回首页继续。</p>
        <div class="not-found-actions">
          <n-button type="primary" size="large" @click="goHome">返回首页</n-button>
        </div>
      </n-card>
    </div>
  </div>
</template>
