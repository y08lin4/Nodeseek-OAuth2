<script setup lang="ts">
// 状态徽章：NTag small round 语义色映射
// active/approved → success；pending → warning；revoked/paused/disabled → default；rejected/fail → error
import { computed } from 'vue'
import { NTag } from 'naive-ui'

interface Props {
  status?: string
  text?: string
}

const props = withDefaults(defineProps<Props>(), { status: '', text: '' })

const TAG_MAP: Record<string, 'success' | 'warning' | 'error' | 'default'> = {
  active: 'success',
  approved: 'success',
  ok: 'success',
  success: 'success',
  pending: 'warning',
  pending_review: 'warning',
  pause_request: 'warning',
  resume_request: 'warning',
  revoked: 'default',
  paused: 'default',
  disabled: 'default',
  rejected: 'error',
  fail: 'error',
  error: 'error',
  delete_request: 'error',
}

const tagType = computed(() => TAG_MAP[props.status] ?? 'default')
</script>

<template>
  <n-tag size="small" round :type="tagType">{{ text || status }}</n-tag>
</template>
