<script setup lang="ts">
// 统计卡：语义色图标方块 + 标签 + 千分位数值（复用批 1 .stat-card 样式）
import { computed } from 'vue'
import { formatNum } from '../../views/admin/adminShared'

interface Props {
  label: string
  value?: number | string | null
  unit?: string
  icon?: unknown
  variant?: 'success' | 'danger' | 'warning' | 'info' | 'neutral'
  trend?: string
  trendUp?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  value: null,
  unit: '',
  icon: undefined,
  variant: 'neutral',
  trend: '',
  trendUp: true,
})

const SEM_MAP: Record<NonNullable<Props['variant']>, string> = {
  success: 'var(--ns-success)',
  danger: 'var(--ns-danger)',
  warning: 'var(--ns-warning)',
  info: 'var(--ns-info)',
  neutral: 'var(--ns-slate)',
}

const sem = computed(() => SEM_MAP[props.variant])
const display = computed(() => (typeof props.value === 'number' ? formatNum(props.value) : props.value ?? '—'))
</script>

<template>
  <div class="stat-card">
    <div v-if="icon" class="stat-icon" :style="{ '--sem': sem }">
      <component :is="icon" :size="20" />
    </div>
    <div class="stat-meta">
      <div class="stat-label">{{ label }}</div>
      <div class="stat-value">
        {{ display }}<template v-if="unit"><span class="stat-unit">{{ unit }}</span></template>
      </div>
      <div v-if="trend" class="stat-trend" :class="trendUp ? 'stat-trend--up' : 'stat-trend--down'">
        {{ trendUp ? '▲' : '▼' }} {{ trend }}
      </div>
    </div>
  </div>
</template>
