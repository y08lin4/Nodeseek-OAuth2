<script setup lang="ts">
// 页头：h2 绿左条 + 副标题 + 右侧 actions（复用 .page-head/.page-title/.page-sub 样式）
interface Props {
  title: string
  subtitle?: string
}

withDefaults(defineProps<Props>(), { subtitle: '' })
</script>

<template>
  <div class="page-head">
    <div>
      <h2 class="page-title">{{ title }}</h2>
      <p v-if="subtitle" class="page-sub">{{ subtitle }}</p>
    </div>
    <div v-if="$slots.actions" class="page-actions">
      <slot name="actions" />
    </div>
    <slot v-if="$slots.default" name="default" />
  </div>
</template>
