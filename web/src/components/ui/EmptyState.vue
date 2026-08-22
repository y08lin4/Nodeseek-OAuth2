<script setup lang="ts">
// 统一空态：NEmpty 居中，min-height 160
import { NEmpty } from 'naive-ui'

withDefaults(defineProps<{ description?: string; size?: 'small' | 'medium' | 'large' | 'huge' }>(), {
  description: '暂无数据',
  size: 'medium',
})
</script>

<template>
  <n-empty :description="description" :size="size" class="empty-state" />
</template>
