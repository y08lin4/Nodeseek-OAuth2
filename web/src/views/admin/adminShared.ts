// 管理后台共享辅助：状态徽章映射 / 时间格式化 / 统计标签
import type { ReviewType, ClientStatus } from '../../api'

// 审核类型徽章映射
export const reviewTypeMeta: Record<ReviewType, { text: string; type: 'default' | 'info' | 'success' | 'warning' | 'error' }> = {
  app: { text: '应用申请', type: 'info' },
  pause: { text: '暂停申请', type: 'warning' },
  resume: { text: '恢复申请', type: 'success' },
  delete: { text: '删除申请', type: 'error' },
}

export function reviewTypeText(t: ReviewType): string {
  return reviewTypeMeta[t]?.text ?? t
}

export function reviewTypeClass(t: ReviewType): 'default' | 'info' | 'success' | 'warning' | 'error' {
  return reviewTypeMeta[t]?.type ?? 'info'
}

// 应用状态徽章映射
export const clientStatusMeta: Record<ClientStatus, { text: string; type: 'info' | 'success' | 'warning' | 'error' }> = {
  pending_review: { text: '审核中', type: 'info' },
  approved: { text: '已通过', type: 'success' },
  rejected: { text: '未通过', type: 'error' },
  paused: { text: '已暂停', type: 'warning' },
  pause_request: { text: '暂停申请中', type: 'warning' },
  resume_request: { text: '恢复申请中', type: 'info' },
  delete_request: { text: '删除申请中', type: 'error' },
}

export function clientStatusText(s: ClientStatus): string {
  return clientStatusMeta[s]?.text ?? s
}

export function clientStatusClass(s: ClientStatus): 'info' | 'success' | 'warning' | 'error' {
  return clientStatusMeta[s]?.type ?? 'info'
}

// token_ttl（秒）转分钟
export function ttlToMinutes(sec: number): number {
  return Math.round(sec / 60)
}

// RFC3339 → zh-CN 本地化时间
export function formatTime(s: string): string {
  const d = new Date(s)
  return Number.isNaN(d.getTime()) ? s : d.toLocaleString('zh-CN')
}

// 统计面板 5 卡片标签
export function statItemLabel(): Array<{ key: 'verifies' | 'login_ok' | 'login_fail' | 'gate_block' | 'cookie_alert'; label: string }> {
  return [
    { key: 'verifies', label: '验证码生成' },
    { key: 'login_ok', label: '登录成功' },
    { key: 'login_fail', label: '登录失败' },
    { key: 'gate_block', label: '门槛拦截' },
    { key: 'cookie_alert', label: 'Cookie 告警' },
  ]
}

// age_seconds 人类可读
export function ageText(seconds: number): string {
  const s = seconds ?? 0
  if (s < 60) return `${s} 秒前`
  if (s < 3600) return `${Math.floor(s / 60)} 分钟前`
  if (s < 86400) return `${Math.floor(s / 3600)} 小时前`
  return `${Math.floor(s / 86400)} 天前`
}
