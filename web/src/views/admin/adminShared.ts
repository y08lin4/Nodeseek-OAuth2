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

// 数值千分位格式化（配合 tabular-nums）
export function formatNum(n: number | null | undefined): string {
  if (n == null || Number.isNaN(n)) return '—'
  return n.toLocaleString('zh-CN')
}

// —— 审计日志：事件汉化映射 + 级别分类 ——

/** 审计事件 → 中文名（覆盖后端全部事件；未命中回退原文） */
export const AUDIT_EVENT_CN: Record<string, string> = {
  // 登录
  'login.verify': '发起登录',
  'login.confirm.ok': '登录成功',
  'login.confirm.fail': '登录失败',
  'user.blocked': '黑名单拦截',
  'rate.limit': '频率限制',
  // 授权 / 令牌
  'gate.block': '门槛拦截',
  'authorize.code': '签发授权码',
  'token.exchange.ok': '兑换令牌',
  'token.exchange.fail': '令牌兑换失败',
  'userinfo.access': '读取用户信息',
  'grant.revoke': '撤销授权',
  // 应用
  'client.register': '注册应用',
  'client.patch': '修改应用',
  'client.delete': '删除应用',
  'client.delete_request': '申请删除应用',
  // 审核
  'review.approve': '审核通过',
  'review.reject': '审核拒绝',
  // Cookie / 邮件
  'cookie.update': '更新 Cookie',
  'cookie.alert': 'Cookie 告警',
  'mail.cookie_alert': 'Cookie 失效告警',
  'mail.sent': '邮件发送',
  // 管理端
  'admin.login.ok': '管理登录',
  'admin.login.fail': '管理登录失败',
  'admin.logout': '管理登出',
  'admin.account.create': '新增系统账号',
  'admin.account.patch': '修改系统账号',
  'admin.account.delete': '删除系统账号',
  'admin.cookie.update': '管理更新 Cookie',
  'admin.test_mail': '发送测试邮件',
  'admin.clients.view': '查看应用列表',
  'admin.client.reset_secret': '重置应用密钥',
  'admin.smtp.update': '更新 SMTP',
  'admin.users.view': '查看用户列表',
  'admin.user.detail': '查看用户详情',
  'admin.user.stats': '查看用户统计',
  'user.blacklist': '拉黑用户',
  'user.unblacklist': '解禁用户',
  'admin.grants.view': '查看授权记录',
  'admin.export.users': '导出用户数据',
  'admin.export.grants': '导出授权数据',
  'admin.stats.view': '查看统计',
  'admin.audit.view': '查看日志',
  // 泛型兜底（未来可能新增）
  'admin.client.create': '新建应用',
  'admin.client.delete': '删除应用',
  'admin.client.update': '修改应用',
  'admin.review.approve': '管理审核通过',
  'admin.review.deny': '管理审核拒绝',
}

export function auditEventCN(event: string): string {
  if (AUDIT_EVENT_CN[event]) return AUDIT_EVENT_CN[event]
  // 前缀兜底
  if (event.startsWith('login.')) return '登录操作'
  if (event.startsWith('authorize.')) return '授权操作'
  if (event.startsWith('token.')) return '令牌操作'
  if (event.startsWith('cookie.')) return 'Cookie 操作'
  if (event.startsWith('mail.')) return '邮件操作'
  if (event.startsWith('admin.client.')) return '管理应用操作'
  if (event.startsWith('admin.review.')) return '管理审核'
  if (event.startsWith('admin.user.')) return '用户管理操作'
  if (event.startsWith('admin.export.')) return '导出数据'
  if (event.startsWith('user.block')) return '黑名单拦截'
  if (event.startsWith('admin.')) return '管理操作'
  return event
}

export type AuditLevel = 'info' | 'warn' | 'error'

/** 事件 → 级别（info 正常 / warn 异常 / error 安全事件） */
export function auditLevel(event: string): AuditLevel {
  // error：安全事件
  const errSet = [
    'user.blocked',
    'admin.login.fail',
    'token.exchange.fail',
    'user.blacklist',
    'gate.block',
  ]
  if (errSet.includes(event)) return 'error'
  // warn：异常
  const warnSet = ['login.confirm.fail', 'rate.limit', 'mail.cookie_alert']
  if (warnSet.includes(event)) return 'warn'
  return 'info'
}

/** 级别标签文案 */
export const AUDIT_LEVEL_TEXT: Record<AuditLevel, string> = {
  info: '正常',
  warn: '警告',
  error: '异常',
}

/** 级别徽章类型 */
export const AUDIT_LEVEL_TAG: Record<AuditLevel, 'default' | 'warning' | 'error'> = {
  info: 'default',
  warn: 'warning',
  error: 'error',
}
