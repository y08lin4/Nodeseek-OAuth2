<script setup lang="ts">
// 品牌 Logo：3×3 格点阵 + 右下钥匙孔（OAuth 授权语义）
// props：size（像素，字号随尺寸缩放）；withText（true 时右侧显示 "NSAuth2"，NS 两字母品牌绿）
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    size?: number
    withText?: boolean
  }>(),
  {
    size: 28,
    withText: false,
  },
)

const fontSize = computed(() => Math.round(props.size * 0.62))
</script>

<template>
  <span class="ns-logo" :class="{ 'ns-logo--text': withText }">
    <svg
      :width="size"
      :height="size"
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
    >
      <!-- 左上 2 格：品牌绿实心 -->
      <rect x="0.5" y="0.5" width="7" height="7" rx="1" fill="#2ea44f" />
      <rect x="8.5" y="0.5" width="7" height="7" rx="1" fill="#2ea44f" />
      <!-- 其余格点：浅灰描边 -->
      <rect x="16.5" y="0.5" width="7" height="7" rx="1" fill="none" stroke="#e5e7eb" stroke-width="1.2" />
      <rect x="0.5" y="8.5" width="7" height="7" rx="1" fill="none" stroke="#e5e7eb" stroke-width="1.2" />
      <rect x="8.5" y="8.5" width="7" height="7" rx="1" fill="none" stroke="#e5e7eb" stroke-width="1.2" />
      <rect x="16.5" y="8.5" width="7" height="7" rx="1" fill="none" stroke="#e5e7eb" stroke-width="1.2" />
      <rect x="0.5" y="16.5" width="7" height="7" rx="1" fill="none" stroke="#e5e7eb" stroke-width="1.2" />
      <rect x="8.5" y="16.5" width="7" height="7" rx="1" fill="none" stroke="#e5e7eb" stroke-width="1.2" />
      <!-- 右下格：钥匙孔（授权语义） -->
      <circle cx="20" cy="18.4" r="2.3" fill="#2ea44f" />
      <path d="M20 20.4 L20 22.6" stroke="#2ea44f" stroke-width="1.6" stroke-linecap="round" />
    </svg>
    <span
      v-if="withText"
      class="ns-logo__text"
      :style="{ fontSize: fontSize + 'px' }"
    >
      <span class="ns-logo__ns">NS</span>Auth2
    </span>
  </span>
</template>

<style scoped>
.ns-logo {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  line-height: 1;
}

.ns-logo__text {
  font-weight: 700;
  letter-spacing: -0.3px;
}

.ns-logo__ns {
  color: var(--ns-primary, #2ea44f);
}
</style>
