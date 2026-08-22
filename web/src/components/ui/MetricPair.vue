<script setup lang="ts">
// 成对数值：绿=成功 / 红=失败，可选 label（复用 .num-ok/.num-fail/.sep 样式）
import { computed } from 'vue'
import { formatNum } from '../../views/admin/adminShared'

interface Props {
  ok: number
  fail: number
  label?: string
}

const props = withDefaults(defineProps<Props>(), { label: '' })

const okText = computed(() => formatNum(props.ok))
const failText = computed(() => formatNum(props.fail))
</script>

<template>
  <span class="metric-pair" :title="label || '成功 / 失败'">
    <template v-if="label"><span class="metric-label">{{ label }}</span></template>
    <span class="num-ok">{{ okText }}</span>
    <span class="sep">/</span>
    <span class="num-fail">{{ failText }}</span>
  </span>
</template>
