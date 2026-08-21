<script setup lang="ts">
// 管理后台 · 审核队列：通过/拒绝（拒绝可填理由）
import { h, onMounted, ref } from 'vue'
import { NCard, NButton, NTag, NInput, NSpin, NEmpty, useMessage, useDialog } from 'naive-ui'
import { listReviews, reviewAction, ApiError, type ReviewItem } from '../../api'
import { reviewTypeText, reviewTypeClass, formatTime } from './adminShared'

const message = useMessage()
const dialog = useDialog()

const reviews = ref<ReviewItem[]>([])
const reviewsLoading = ref(false)
const actingKey = ref('')
const rejectReason = ref('')

async function loadReviews() {
  reviewsLoading.value = true
  try {
    const resp = await listReviews()
    reviews.value = resp.reviews
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '获取审核队列失败')
  } finally {
    reviewsLoading.value = false
  }
}

async function doReview(item: ReviewItem, action: 'approve' | 'reject', reason?: string) {
  actingKey.value = `${item.client_id}:${action}`
  try {
    await reviewAction({ type: item.type, client_id: item.client_id, action, reason })
    message.success(action === 'approve' ? '已通过' : '已拒绝')
    await loadReviews()
  } catch (e) {
    message.error(e instanceof ApiError ? e.message : '审核操作失败，请重试')
  } finally {
    actingKey.value = ''
  }
}

function handleReview(item: ReviewItem, action: 'approve' | 'reject') {
  if (action === 'approve') {
    dialog.warning({
      title: '审核通过',
      content: `确认通过「${item.client_name}」的申请？`,
      positiveText: '通过',
      negativeText: '取消',
      onPositiveClick: () => doReview(item, 'approve'),
    })
    return
  }
  rejectReason.value = ''
  dialog.warning({
    title: `拒绝「${item.client_name}」`,
    content: () =>
      h(NInput, {
        type: 'textarea',
        rows: 3,
        placeholder: '可输入拒绝理由（可选，留空直接提交）',
        value: rejectReason.value,
        'onUpdate:value': (v: string) => {
          rejectReason.value = v
        },
      }),
    positiveText: '确认拒绝',
    negativeText: '取消',
    onPositiveClick: () => doReview(item, 'reject', rejectReason.value.trim() || undefined),
  })
}

onMounted(loadReviews)
</script>

<template>
  <n-card class="admin-page-card">
    <template #header>
      <span class="page-title">审核队列</span>
    </template>
    <p class="ns-card-sub">待处理的应用申请 / 暂停 / 恢复 / 删除请求</p>

    <n-spin :show="reviewsLoading">
      <n-empty
        v-if="!reviewsLoading && reviews.length === 0"
        description="暂无待审核项"
        size="small"
        class="ns-py-3"
      />
      <div v-else class="review-list">
        <n-card v-for="r in reviews" :key="r.client_id" size="small" class="review-item">
          <div class="ns-flex ns-align-center ns-gap-2 ns-mb-1 ns-flex-wrap">
            <n-tag :type="reviewTypeClass(r.type)" size="small" round>
              {{ reviewTypeText(r.type) }}
            </n-tag>
            <span class="review-name">{{ r.client_name }}</span>
            <code class="review-client-id">{{ r.client_id }}</code>
          </div>
          <div class="ns-text-muted ns-small">owner: {{ r.owner_user_id }} · {{ formatTime(r.created_at) }}</div>
          <div v-if="r.detail" class="review-detail">{{ r.detail }}</div>
          <div class="review-actions">
            <n-button
              size="small"
              type="success"
              :loading="actingKey === `${r.client_id}:approve`"
              @click="handleReview(r, 'approve')"
            >
              通过
            </n-button>
            <n-button
              size="small"
              type="error"
              :loading="actingKey === `${r.client_id}:reject`"
              @click="handleReview(r, 'reject')"
            >
              拒绝
            </n-button>
          </div>
        </n-card>
      </div>
    </n-spin>
  </n-card>
</template>

<style scoped>
.admin-page-card {
  border-radius: 6px;
}
</style>
