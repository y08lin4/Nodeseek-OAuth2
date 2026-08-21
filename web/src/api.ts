// 类型化 API 客户端：与 SPEC.md 3.3 节契约一一对应
// 统一错误约定：错误响应为 {"success":false,"message":"..."}，
// 此处统一提取 data.message 抛 ApiError，组件只负责展示。

/** GET /api/config 返回的应用级配置 */
export interface AppConfig {
  nodeseek: {
    base_url: string
    space_url_template: string
    message_url: string
    auth_account_id: string
    auth_account_username: string
    /** 启用系统账号列表（与 /api/config 响应字段一致；私信核验可发送对象） */
    accounts: { account_id: string; account_name: string; priority: number; enabled: boolean }[]
  }
  business: {
    min_client_creation_rank: number
  }
  verification: {
    code_expiry_seconds: number
  }
  /** 全局授权门槛（0 = 该项不启用） */
  gate: {
    min_rank: number
    min_join_days: number
  }
}

/** NodeSeek 用户信息（confirm / 授权页随响应附带） */
export interface UserStats {
  rank: number
  join_days: number
  chicken: number
  topics: number
  comments: number
}

/** 按应用生效的授权门槛校验结果 */
export interface GateInfo {
  min_rank: number
  min_join_days: number
  ok: boolean
}

/** verify 响应中的系统账号（按 priority 升序，用户发给任一账号） */
export interface VerifyAccount {
  account_id: string
  account_name: string
  priority: number
}

/** POST /oauth/verify 成功响应 */
export interface VerifyResp {
  success: true
  verification_code: string
  expires_in: number
  /** 启用系统账号列表（按 priority 升序）；后端旧版可能不返回，需容错 */
  accounts: VerifyAccount[]
}

/** POST /oauth/confirm 成功响应 */
export interface ConfirmResp {
  success: true
  redirect_to?: string
  /** 用户信息；服务端拉取失败时为 null（不阻塞登录） */
  stats: UserStats | null
}

/** 应用审核状态（SPEC 3.3） */
export type ClientStatus =
  | 'pending_review'
  | 'approved'
  | 'rejected'
  | 'paused'
  | 'pause_request'
  | 'resume_request'
  | 'delete_request'

/** 应用授权统计（client.stats） */
export interface ClientStats {
  auth_ok_today: number
  auth_fail_today: number
  auth_ok_total: number
  auth_fail_total: number
}

/** GET /api/oauth/client 中的应用信息 */
export interface OAuthClient {
  client_id: string
  client_name: string
  owner_user_id: string
  homepage_url?: string
  description?: string
  redirect_uris: string[]
  icon_url?: string
  min_rank?: number
  disabled?: boolean
  status?: ClientStatus
  /** access_token 有效期（秒） */
  token_ttl?: number
}

/** GET /api/oauth/client 成功响应（应用信息 + 当前用户 stats + 门槛状态） */
export interface ClientInfo {
  success: true
  client: OAuthClient
  stats: UserStats | null
  gate: GateInfo
}

/** POST /api/client/register 请求体 */
export interface RegisterReq {
  name: string
  homepage_url: string
  description: string
  redirect_uris: string[]
  icon_url: string
  min_rank: number
  /** access_token 有效期（秒，默认 3600，范围 60-86400） */
  token_ttl: number
}

/** POST /api/client/register 成功响应中的应用信息（client_secret 仅此一次明文返回） */
export interface RegisteredClient {
  client_id: string
  client_secret: string
  client_name: string
  owner_user_id: string
  homepage_url: string
  description: string
  redirect_uris: string[]
  icon_url: string
  min_rank: number
  token_ttl: number
  status: ClientStatus
  created_at: string
}

/** POST /api/client/register 成功响应 */
export interface RegisterResp {
  success: true
  client: RegisteredClient
}

/** GET /api/client/list 中的应用条目（不含 secret） */
export interface ClientListItem {
  client_id: string
  client_name: string
  owner_user_id: string
  homepage_url: string
  description: string
  redirect_uris: string[]
  icon_url: string
  min_rank: number
  /** access_token 有效期（秒） */
  token_ttl: number
  status: ClientStatus
  /** 授权统计（今日/累计成功失败） */
  stats: ClientStats
  created_at: string
}

/** GET /api/client/list 成功响应 */
export interface ClientListResp {
  success: true
  clients: ClientListItem[]
}

/** POST /api/client/{id}/pause|resume|delete-request 成功响应 */
export interface ClientStatusResp {
  success: true
  status: ClientStatus
}

/** PATCH /api/client/{id} 成功响应（client 含 disabled） */
export interface PatchClientResp {
  success: true
  client: OAuthClient
}

/** GET /api/me 成功响应（当前登录用户） */
export interface MeResp {
  success: true
  user_id: string
}

/** POST /api/logout 成功响应 */
export interface LogoutResp {
  success: true
}

/** GET /api/admin/status 中的 Cookie 状态 */
export interface AdminCookieStatus {
  set: boolean
  updated_at: string
  age_seconds: number
}

/** GET /api/admin/status 中的邮件配置状态 */
export interface AdminMailStatus {
  configured: boolean
  report_time: string
  last_test_at: string
  /** 新应用提交邮件通知开关（服务端 NS_REVIEW_EMAIL_NOTIFY，环境变量控制，重启生效） */
  review_email_notify: boolean
}

/** GET /api/admin/status 成功响应 */
export interface AdminStatus {
  success: true
  cookie: AdminCookieStatus
  mock_mode: boolean
  mail: AdminMailStatus
}

/** POST /api/admin/test-mail 成功响应 */
export interface TestMailResp {
  success: true
  message?: string
}

/** POST /api/admin/cookie 成功响应 */
export interface AdminCookieResp {
  success: true
  account_id?: string
  account_name?: string
  updated_at: string
}

/** GET /api/admin/accounts 中的系统账号（不含 Cookie 明文） */
export interface SysAccount {
  account_id: string
  account_name: string
  priority: number
  enabled: boolean
  updated_at: string
  last_error: string
  fail_count: number
  auto_detected: boolean
}

/** GET /api/admin/accounts 成功响应 */
export interface SysAccountsResp {
  success: true
  accounts: SysAccount[]
}

/** POST /api/admin/accounts 请求体 */
export interface AddAccountReq {
  account_id: string
  account_name: string
  priority: number
  enabled: boolean
}

/** POST/PATCH /api/admin/accounts 成功响应 */
export interface SysAccountResp {
  success: true
  account: SysAccount
}

/** 统一的业务错误：message 取自响应体 data.message；statusCode 为 HTTP 状态码（如 403） */
export class ApiError extends Error {
  statusCode: number

  constructor(message: string, statusCode = 0) {
    super(message)
    this.name = 'ApiError'
    this.statusCode = statusCode
  }
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE'
  body?: unknown
  headers?: Record<string, string>
}

/**
 * 底层请求封装：
 * - 同源携带 Cookie（ns_oauth_session 等）
 * - 非 2xx 或业务 success=false 时统一抛 ApiError（取 data.message）
 */
async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      method: options.method ?? 'GET',
      credentials: 'same-origin',
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
    })
  } catch {
    // 网络层失败（后端未启动 / 代理不通）
    throw new ApiError('网络请求失败，请确认服务端已启动')
  }

  let data: unknown = null
  try {
    data = await res.json()
  } catch {
    // 响应体非 JSON（如网关 502 页面），按状态码报错
    if (!res.ok) {
      throw new ApiError(`请求失败（HTTP ${res.status}）`)
    }
  }

  const obj = data as { success?: boolean; message?: string } | null
  if (!res.ok || (obj && obj.success === false)) {
    throw new ApiError(obj?.message || `请求失败（HTTP ${res.status}）`, res.status)
  }
  return data as T
}

/** GET /api/config：前端全局配置（私信链接模板、验证码有效期等） */
export function getConfig(): Promise<AppConfig> {
  return request<AppConfig>('/api/config')
}

/** POST /oauth/verify：第一步，用 NS 数字 ID 换取一次性验证码 */
export function verifyUser(userId: string): Promise<VerifyResp> {
  return request<VerifyResp>('/oauth/verify', {
    method: 'POST',
    body: { user_id: userId },
  })
}

/** POST /oauth/confirm：第三步，确认已私信验证码，成功返回跳转目标 */
export function confirmLogin(userId: string, verificationCode: string): Promise<ConfirmResp> {
  return request<ConfirmResp>('/oauth/confirm', {
    method: 'POST',
    body: { user_id: userId, verification_code: verificationCode },
  })
}

/** GET /api/oauth/client：授权页展示应用信息、用户 stats 与门槛状态 */
export function getClient(clientId: string): Promise<ClientInfo> {
  return request<ClientInfo>(`/api/oauth/client?client_id=${encodeURIComponent(clientId)}`)
}

/** GET /api/admin/status：管理页查询系统 Cookie 状态（凭管理会话，401 即未登录） */
export function getAdminStatus(adminToken?: string): Promise<AdminStatus> {
  return request<AdminStatus>('/api/admin/status', {
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
  })
}

/** POST /api/admin/login：验证管理令牌并签发 httpOnly 管理会话 Cookie */
export function adminLogin(token: string): Promise<LogoutResp> {
  return request<LogoutResp>('/api/admin/login', {
    method: 'POST',
    body: { token },
  })
}

/** POST /api/admin/logout：登出，清除管理会话 Cookie */
export function adminLogout(): Promise<LogoutResp> {
  return request<LogoutResp>('/api/admin/logout', { method: 'POST' })
}

/** POST /api/admin/cookie：更新/新增系统账号 Cookie（account_id 可选：AUTO_DETECT=1 时服务端忽略并自动识别） */
export function updateAdminCookie(
  cookie: string,
  accountId?: string,
  adminToken?: string,
): Promise<AdminCookieResp> {
  return request<AdminCookieResp>('/api/admin/cookie', {
    method: 'POST',
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
    body: accountId ? { cookie, account_id: accountId } : { cookie },
  })
}

/** POST /api/admin/test-mail：发送测试邮件（SMTP 未配置 → 400「SMTP 未配置」） */
export function testMail(adminToken?: string): Promise<TestMailResp> {
  return request<TestMailResp>('/api/admin/test-mail', {
    method: 'POST',
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
  })
}

/** POST /api/client/register：创建第三方应用（需登录） */
export function registerClient(req: RegisterReq): Promise<RegisterResp> {
  return request<RegisterResp>('/api/client/register', {
    method: 'POST',
    body: req,
  })
}

/** GET /api/client/list：当前用户的应用列表（需登录，不含 secret） */
export function listClients(): Promise<ClientListResp> {
  return request<ClientListResp>('/api/client/list')
}

/** DELETE /api/client/{id}：管理端强制删除应用（Header X-Admin-Token，应用方删除走 delete-request 审核） */
export function deleteClient(clientId: string): Promise<LogoutResp> {
  return request<LogoutResp>(`/api/client/${encodeURIComponent(clientId)}`, {
    method: 'DELETE',
  })
}

/** POST /api/client/{id}/pause：申请暂停（approved 态 → pause_request） */
export function pauseClient(clientId: string): Promise<ClientStatusResp> {
  return request<ClientStatusResp>(`/api/client/${encodeURIComponent(clientId)}/pause`, {
    method: 'POST',
  })
}

/** POST /api/client/{id}/resume：申请恢复（paused 态 → resume_request） */
export function resumeClient(clientId: string): Promise<ClientStatusResp> {
  return request<ClientStatusResp>(`/api/client/${encodeURIComponent(clientId)}/resume`, {
    method: 'POST',
  })
}

/** POST /api/client/{id}/delete-request：申请删除（approved/paused 态 → delete_request） */
export function deleteRequestClient(clientId: string): Promise<ClientStatusResp> {
  return request<ClientStatusResp>(`/api/client/${encodeURIComponent(clientId)}/delete-request`, {
    method: 'POST',
  })
}

/** GET /api/me：当前登录用户（导航/控制台登录态探测） */
export function me(): Promise<MeResp> {
  return request<MeResp>('/api/me')
}

/** POST /api/logout：退出登录（清除 ns_oauth_session 会话） */
export function logout(): Promise<LogoutResp> {
  return request<LogoutResp>('/api/logout', { method: 'POST' })
}

// —— 我的授权（/grants） ——

/** 授权记录状态 */
export type GrantStatus = 'active' | 'revoked'

/** GET /api/grants 中的授权条目 */
export interface Grant {
  user_id: string
  client_id: string
  client_name: string
  icon_url: string
  min_rank: number
  /** 授权时间（ISO 时间串） */
  granted_at: string
  status: GrantStatus
}

/** GET /api/grants 成功响应 */
export interface GrantsResp {
  success: true
  grants: Grant[]
}

/** POST /api/grants/{id}/revoke 成功响应 */
export interface RevokeGrantResp {
  success: true
}

/** GET /api/grants：我的授权列表（需登录） */
export function listGrants(): Promise<GrantsResp> {
  return request<GrantsResp>('/api/grants')
}

/** POST /api/grants/{id}/revoke：撤销授权（该应用的访问令牌即刻作废） */
export function revokeGrant(clientId: string): Promise<RevokeGrantResp> {
  return request<RevokeGrantResp>(`/api/grants/${encodeURIComponent(clientId)}/revoke`, {
    method: 'POST',
  })
}

// —— 管理端审核队列（/admin） ——

/** 审核类型 */
export type ReviewType = 'app' | 'pause' | 'resume' | 'delete'

/** GET /api/admin/reviews 中的待审核项 */
export interface ReviewItem {
  type: ReviewType
  client_id: string
  client_name: string
  owner_user_id: string
  detail: string
  created_at: string
}

/** GET /api/admin/reviews 成功响应 */
export interface ReviewsResp {
  success: true
  reviews: ReviewItem[]
}

/** POST /api/admin/review 请求体（reason 可选） */
export interface ReviewActionReq {
  type: ReviewType
  client_id: string
  action: 'approve' | 'reject'
  reason?: string
}

/** POST /api/admin/review 成功响应 */
export interface ReviewResp {
  success: true
  client: OAuthClient
}

/** GET /api/admin/reviews：待审核队列（管理端，凭会话） */
export function listReviews(adminToken?: string): Promise<ReviewsResp> {
  return request<ReviewsResp>('/api/admin/reviews', {
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
  })
}

/** POST /api/admin/review：审核操作（app/pause/resume/delete 通过或拒绝） */
export function reviewAction(req: ReviewActionReq, adminToken?: string): Promise<ReviewResp> {
  return request<ReviewResp>('/api/admin/review', {
    method: 'POST',
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
    body: req,
  })
}

// —— 管理端系统账号（/admin） ——

/** GET /api/admin/accounts：系统账号列表（不含 Cookie 明文） */
export function listAccounts(adminToken?: string): Promise<SysAccountsResp> {
  return request<SysAccountsResp>('/api/admin/accounts', {
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
  })
}

/** POST /api/admin/accounts：手动新增系统账号 */
export function addAccount(req: AddAccountReq, adminToken?: string): Promise<SysAccountResp> {
  return request<SysAccountResp>('/api/admin/accounts', {
    method: 'POST',
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
    body: req,
  })
}

/** PATCH /api/admin/accounts/{id}：调整优先级 / 启用状态 */
export function patchAccount(
  accountId: string,
  data: { priority?: number; enabled?: boolean },
  adminToken?: string,
): Promise<SysAccountResp> {
  return request<SysAccountResp>(`/api/admin/accounts/${encodeURIComponent(accountId)}`, {
    method: 'PATCH',
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
    body: data,
  })
}

/** DELETE /api/admin/accounts/{id}：删除系统账号（至少保留 1 个，否则 400） */
export function deleteAccount(accountId: string, adminToken?: string): Promise<LogoutResp> {
  return request<LogoutResp>(`/api/admin/accounts/${encodeURIComponent(accountId)}`, {
    method: 'DELETE',
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
  })
}

// —— 管理端应用/统计/审计（/admin） ——

/** GET /api/admin/clients 中的应用条目（含授权统计，无 secret） */
export interface AdminClient {
  client_id: string
  client_name: string
  owner_user_id: string
  homepage_url: string
  description: string
  icon_url: string
  redirect_uris: string[]
  min_rank: number
  /** access_token 有效期（秒） */
  token_ttl: number
  status: ClientStatus
  /** 授权统计（今日/累计成功失败） */
  stats: ClientStats
  created_at: string
}

/** GET /api/admin/clients 成功响应 */
export interface AdminClientsResp {
  success: true
  clients: AdminClient[]
}

/** GET /api/admin/stats 中的应用统计计数 */
export interface AdminStats {
  verifies: number
  login_ok: number
  login_fail: number
  gate_block: number
  cookie_alert: number
  /** 本期统计起始时间（RFC3339） */
  reset_at: string
}

/** GET /api/admin/stats 成功响应 */
export interface AdminStatsResp {
  success: true
  stats: AdminStats
}

/** GET /api/admin/audit 中的审计事件 */
export interface AuditEvent {
  ts: string
  event: string
  ip: string
  user_id: string
  client_id: string
  detail: string
}

/** GET /api/admin/audit 成功响应 */
export interface AdminAuditResp {
  success: true
  events: AuditEvent[]
}

/** GET /api/admin/clients：全量应用列表（管理端，凭会话） */
export function listAdminClients(adminToken?: string): Promise<AdminClientsResp> {
  return request<AdminClientsResp>('/api/admin/clients', {
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
  })
}

/** GET /api/admin/stats：应用统计计数（管理端，凭会话） */
export function getAdminStats(adminToken?: string): Promise<AdminStatsResp> {
  return request<AdminStatsResp>('/api/admin/stats', {
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
  })
}

/** GET /api/admin/audit：审计日志（管理端，凭会话；limit 1-200，默认 50） */
export function listAudit(limit = 50, adminToken?: string): Promise<AdminAuditResp> {
  return request<AdminAuditResp>(`/api/admin/audit?limit=${limit}`, {
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
  })
}

/** PATCH /api/client/{id}：管理端修改应用（暂停=status:paused / 恢复=status:approved / 调整 token_ttl；凭会话） */
export function patchAdminClient(
  clientId: string,
  data: { disabled?: boolean; token_ttl?: number; status?: ClientStatus },
  adminToken?: string,
): Promise<PatchClientResp> {
  return request<PatchClientResp>(`/api/client/${encodeURIComponent(clientId)}`, {
    method: 'PATCH',
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
    body: data,
  })
}

/** DELETE /api/client/{id}：管理端强制删除应用（凭会话） */
export function deleteAdminClient(clientId: string, adminToken?: string): Promise<LogoutResp> {
  return request<LogoutResp>(`/api/client/${encodeURIComponent(clientId)}`, {
    method: 'DELETE',
    headers: adminToken ? { 'X-Admin-Token': adminToken } : undefined,
  })
}
