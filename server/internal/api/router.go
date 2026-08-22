// Package api 负责路由与全部 HTTP handler（按 SPEC 3.3 契约）。
package api

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"nodeseek-oauth2/server/internal/audit"
	"nodeseek-oauth2/server/internal/auth"
	"nodeseek-oauth2/server/internal/config"
	"nodeseek-oauth2/server/internal/mailer"
	"nodeseek-oauth2/server/internal/middleware"
	"nodeseek-oauth2/server/internal/nodeseek"
	"nodeseek-oauth2/server/internal/stats"
	"nodeseek-oauth2/server/internal/store"
)

// 验证码与授权码的时效常量。
const (
	verificationCodeExpirySeconds = 600              // 验证码有效期（秒）
	verificationCodePrefix        = "NS_AUTH_"       // 验证码前缀
	authCodeTTL                   = 10 * time.Minute // 授权码有效期
	adminSessionTTL               = 24 * time.Hour   // 管理会话有效期
	placeholderHTML               = "<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><title>Nodeseek 非官方 OAuth2 授权服务</title></head><body><h1>Nodeseek 非官方 OAuth2 授权服务</h1><p>后端已启动。前端尚未构建（web/dist 不存在），请先构建 web/ 或直接调用 API。</p></body></html>"
)

// AdminCookieName 管理会话 Cookie 名（登录后浏览器携带，checkAdmin 优先校验）。
const AdminCookieName = "ns_admin_session"

// API 持有各 handler 共享的依赖。
type API struct {
	cfg    *config.Config
	store  *store.Store
	ns     *nodeseek.Client
	key    [32]byte
	mail   *mailer.Mailer
	stats  *stats.Stats
	audit  *audit.Logger
	limits *RateLimits

	mu            sync.Mutex
	lastTestAt    time.Time            // 最近一次测试邮件成功时间
	accountAlerts map[string]time.Time // 各账号最近一次 Cookie 告警时间（独立冷却 60min）

	sessionsMu sync.Mutex
	adminSess  map[string]time.Time // 管理会话：会话ID → 创建时间（TTL 24h，启动时清空）
}

// NewRouter 组装路由（Go 1.22+ 方法路由），全部路由外包安全头中间件。
func NewRouter(cfg *config.Config, st *store.Store, ns *nodeseek.Client, key [32]byte, mail *mailer.Mailer, stats *stats.Stats, aud *audit.Logger, limits *RateLimits) http.Handler {
	a := &API{cfg: cfg, store: st, ns: ns, key: key, mail: mail, stats: stats, audit: aud, limits: limits, accountAlerts: map[string]time.Time{}, adminSess: map[string]time.Time{}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", a.handleConfig)
	mux.HandleFunc("POST /oauth/verify", a.handleVerify)
	mux.HandleFunc("POST /oauth/confirm", a.handleConfirm)
	mux.HandleFunc("GET /oauth/authorize", a.handleAuthorize)
	mux.HandleFunc("POST /oauth/authorize/decision", a.handleDecision)
	mux.HandleFunc("POST /oauth/token", a.handleToken)
	mux.HandleFunc("GET /oauth/userinfo", a.handleUserInfo)
	mux.HandleFunc("GET /api/oauth/client", a.handleGetClient)
	mux.HandleFunc("POST /api/client/register", a.handleClientRegister)
	mux.HandleFunc("GET /api/client/list", a.handleClientList)
	mux.HandleFunc("PATCH /api/client/{client_id}", a.handleClientPatch)
	mux.HandleFunc("DELETE /api/client/{client_id}", a.handleClientDelete)
	mux.HandleFunc("POST /api/client/{client_id}/pause", a.handleClientPause)
	mux.HandleFunc("POST /api/client/{client_id}/resume", a.handleClientResume)
	mux.HandleFunc("POST /api/client/{client_id}/delete-request", a.handleClientDeleteRequest)
	mux.HandleFunc("GET /api/grants", a.handleGrants)
	mux.HandleFunc("POST /api/grants/{client_id}/revoke", a.handleGrantRevoke)
	mux.HandleFunc("POST /api/logout", a.handleLogout)
	mux.HandleFunc("GET /api/me", a.handleMe)
	mux.HandleFunc("GET /api/client/notifications/unsubscribe", a.handleClientNotifyUnsubscribe)
	mux.HandleFunc("POST /api/admin/login", a.handleAdminLogin)
	mux.HandleFunc("POST /api/admin/logout", a.handleAdminLogout)
	mux.HandleFunc("PATCH /api/admin/password", a.handleAdminPassword)
	mux.HandleFunc("POST /api/admin/cookie", a.handleAdminCookie)
	mux.HandleFunc("GET /api/admin/status", a.handleAdminStatus)
	mux.HandleFunc("GET /api/admin/accounts", a.handleAdminAccountsList)
	mux.HandleFunc("POST /api/admin/accounts", a.handleAdminAccountCreate)
	mux.HandleFunc("PATCH /api/admin/accounts/{account_id}", a.handleAdminAccountPatch)
	mux.HandleFunc("DELETE /api/admin/accounts/{account_id}", a.handleAdminAccountDelete)
	mux.HandleFunc("POST /api/admin/test-mail", a.handleAdminTestMail)
	mux.HandleFunc("GET /api/admin/reviews", a.handleAdminReviews)
	mux.HandleFunc("POST /api/admin/review", a.handleAdminReview)
	mux.HandleFunc("GET /api/admin/clients", a.handleAdminClients)
	mux.HandleFunc("GET /api/admin/stats", a.handleAdminStats)
	mux.HandleFunc("GET /api/admin/audit", a.handleAdminAudit)
	mux.HandleFunc("GET /api/admin/smtp", a.handleAdminSMTPGet)
	mux.HandleFunc("POST /api/admin/smtp", a.handleAdminSMTPPost)
	mux.HandleFunc("GET /api/admin/users", a.handleAdminUsers)
	mux.HandleFunc("GET /api/admin/users/{user_id}/detail", a.handleAdminUserDetail)
	mux.HandleFunc("GET /api/admin/users/{user_id}/stats", a.handleAdminUserStats)
	mux.HandleFunc("PATCH /api/admin/users/{user_id}", a.handleAdminUserPatch)
	mux.HandleFunc("GET /api/admin/grants", a.handleAdminGrants)
	mux.HandleFunc("POST /api/admin/clients/{client_id}/reset-secret", a.handleAdminResetSecret)
	mux.HandleFunc("GET /api/admin/export/users.csv", a.handleAdminExportUsers)
	mux.HandleFunc("GET /api/admin/export/grants.csv", a.handleAdminExportGrants)
	mux.HandleFunc("GET /healthz", a.handleHealthz)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", a.handleWellKnown)
	mux.HandleFunc("/", a.handleStatic)

	return middleware.SecurityHeaders(a.corsMiddleware(mux))
}

// ---- 工具 ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"success": false, "message": msg})
}

// writeOAuthError 输出 OAuth2 规范错误响应（§3.3 /oauth/token）。
func writeOAuthError(w http.ResponseWriter, status int, code, desc string) {
	writeJSON(w, status, map[string]any{"error": code, "error_description": desc})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
	return json.NewDecoder(r.Body).Decode(v)
}

// isNumeric 判断字符串是否全部为数字（纯数字）。
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// randomHex 从 crypto/rand 生成指定数量的十六进制字符。
func randomHex(chars int, upper bool) string {
	const lower = "0123456789abcdef"
	const upperSet = "0123456789ABCDEF"
	nbytes := (chars + 1) / 2
	b := make([]byte, nbytes)
	if _, err := rand.Read(b); err != nil {
		// 熵源失败属于极端情况，直接 panic（生产可考虑优雅降级，TODO）。
		panic(err)
	}
	var sb strings.Builder
	sb.Grow(chars)
	for _, v := range b {
		if upper {
			sb.WriteByte(upperSet[v>>4])
			sb.WriteByte(upperSet[v&0x0f])
		} else {
			sb.WriteByte(lower[v>>4])
			sb.WriteByte(lower[v&0x0f])
		}
	}
	return sb.String()[:chars]
}

// currentSession 从请求 Cookie 读取并校验会话，失败返回 nil。
func (a *API) currentSession(r *http.Request) *auth.Session {
	c, err := r.Cookie(auth.SessionCookieName)
	if err != nil {
		return nil
	}
	sess, err := auth.VerifySession(a.key, c.Value)
	if err != nil {
		return nil
	}
	return sess
}

// checkAdmin 校验管理鉴权（两种方式任一通过即可）：
//   a) 请求 Cookie ns_admin_session 命中会话 map 且未过期；
//   b) X-Admin-Token 头与 cfg.AdminToken 常量时间相等（保留浏览器扩展调用路径）；
//   c) 都不满足 → 401。
// 未配置任何管理凭据（token / env 账号 / admin.json）时管理系统未启用，一律 403。
func (a *API) checkAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !a.adminCredentialsConfigured() {
		writeError(w, http.StatusForbidden, "管理接口未启用（未设置管理凭据）")
		return false
	}
	// a) 会话 Cookie。
	if c, err := r.Cookie(AdminCookieName); err == nil && c.Value != "" {
		if a.validAdminSession(c.Value) {
			return true
		}
	}
	// b) X-Admin-Token 头（浏览器扩展 MV3 路径）。
	if got := r.Header.Get("X-Admin-Token"); got != "" &&
		subtle.ConstantTimeCompare([]byte(a.cfg.AdminToken), []byte(got)) == 1 {
		return true
	}
	// c) 都未通过。
	writeError(w, http.StatusUnauthorized, "管理鉴权失败（需登录或提供 X-Admin-Token）")
	return false
}

// adminCredentialsConfigured 判断管理系统是否已配置任一凭据：
// env 令牌 / env 账号 / admin.json 存储凭据，任一即启用。
func (a *API) adminCredentialsConfigured() bool {
	if a.cfg.AdminToken != "" || a.cfg.AdminUser != "" {
		return true
	}
	c, err := a.store.GetAdminCred()
	return err == nil && c != nil && c.Username != ""
}

// adminLoginValid 校验管理账号登录（password 校验）。
// 存储优先于 env：admin.json 存在则只匹配存储，否则匹配 env；
// 两者都未配置返回 false（由调用方回退令牌登录）。
func (a *API) adminLoginValid(username, password string) bool {
	if username == "" || password == "" {
		return false
	}
	c, err := a.store.GetAdminCred()
	if err == nil && c != nil && c.Username != "" {
		// 存储凭据优先。
		if uidOK := subtle.ConstantTimeCompare([]byte(c.Username), []byte(username)) == 1; !uidOK {
			return false
		}
		storedPass, derr := a.store.DecryptAdminPassword(c)
		if derr != nil {
			return false
		}
		return subtle.ConstantTimeCompare([]byte(storedPass), []byte(password)) == 1
	}
	// 回退 env 账号。
	if a.cfg.AdminUser == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a.cfg.AdminUser), []byte(username)) == 1 &&
		subtle.ConstantTimeCompare([]byte(a.cfg.AdminPassword), []byte(password)) == 1
}

// currentAdminUsername 返回当前生效的管理用户名（存储优先于 env；都无则回退空）。
func (a *API) currentAdminUsername() string {
	if c, err := a.store.GetAdminCred(); err == nil && c != nil && c.Username != "" {
		return c.Username
	}
	return a.cfg.AdminUser
}

// validAdminSession 校验会话 ID 是否有效且未过期（失效项同时清除）。并发安全。
func (a *API) validAdminSession(id string) bool {
	a.sessionsMu.Lock()
	defer a.sessionsMu.Unlock()
	if id == "" {
		return false
	}
	now := time.Now()
	createdAt, ok := a.adminSess[id]
	if !ok {
		return false
	}
	if now.Sub(createdAt) > adminSessionTTL {
		delete(a.adminSess, id)
		return false
	}
	return true
}

// checkUserBlocked 校验用户是否被拉黑；若被拉黑则写 403 并记审计 user.blocked，返回 true（已拦截）。
func (a *API) checkUserBlocked(w http.ResponseWriter, ip, userID string) bool {
	u, err := a.store.GetUser(userID)
	if err != nil {
		return false // 读取失败不拦截（fail-open 读失败属存储异常，放行）
	}
	if u != nil && u.Blacklisted {
		a.audit.Eventf("user.blocked", ip, userID, "", "该账号已被服务禁用")
		writeError(w, http.StatusForbidden, "该账号已被服务禁用")
		return true
	}
	return false
}

// fullRequestURL 重建当前请求的完整 URL（用于登录后跳转）。
func fullRequestURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// TODO: 反向代理场景应尊重 X-Forwarded-Proto / X-Forwarded-Host。
	return scheme + "://" + r.Host + r.URL.RequestURI()
}

// requestOrigin 动态生成请求 origin（scheme+host，尊重 X-Forwarded-Proto）。
func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if xfp := r.Header.Get("X-Forwarded-Proto"); xfp != "" {
		if i := strings.IndexByte(xfp, ','); i >= 0 {
			xfp = xfp[:i]
		}
		scheme = strings.TrimSpace(xfp)
	}
	return scheme + "://" + r.Host
}

// remoteIP 取客户端 IP：优先 X-Forwarded-For 首个，否则 RemoteAddr 的 host。
func remoteIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// denyRate 写 429 响应并记审计 rate.limit。
func (a *API) denyRate(w http.ResponseWriter, ip, userID, clientID, detail string) {
	a.audit.Eventf("rate.limit", ip, userID, clientID, detail)
	writeError(w, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
}

// appendQuery 安全地向 URL 追加查询参数（保留已有参数）。
func appendQuery(rawURL, key, val string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL + "?" + key + "=" + url.QueryEscape(val)
	}
	q := u.Query()
	q.Set(key, val)
	u.RawQuery = q.Encode()
	return u.String()
}

// contains 判断字符串切片是否包含目标值。
func contains(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

// statsJSON 将用户信息统计转为响应 JSON。
func statsJSON(s nodeseek.UserStats) map[string]any {
	return map[string]any{
		"rank":      s.Rank,
		"join_days": s.JoinDays,
		"chicken":   s.Chicken,
		"topics":    s.Topics,
		"comments":  s.Comments,
	}
}

// clientStatsJSON 应用授权统计（今日/累计成功失败）。
func clientStatsJSON(s store.ClientStats) map[string]any {
	return map[string]any{
		"auth_ok_today":   s.AuthOKToday,
		"auth_fail_today": s.AuthFailToday,
		"auth_ok_total":   s.AuthOKTotal,
		"auth_fail_total": s.AuthFailTotal,
	}
}

// clientAuthJSON 授权页展示的 client 字段（无 secret）。
func clientAuthJSON(c store.Client) map[string]any {
	return map[string]any{
		"client_id":     c.ClientID,
		"client_name":   c.ClientName,
		"owner_user_id": c.OwnerUserID,
		"homepage_url":  c.HomepageURL,
		"description":   c.Description,
		"redirect_uris": c.RedirectURIs,
		"icon_url":      c.IconURL,
		"min_rank":      c.MinRank,
		"status":        c.Status,
		"disabled":      c.Disabled,
	}
}

// clientListJSON 应用列表的 client 字段（无 secret，含 status / stats / disabled）。
func clientListJSON(c store.Client) map[string]any {
	return map[string]any{
		"client_id":     c.ClientID,
		"client_name":   c.ClientName,
		"owner_user_id": c.OwnerUserID,
		"homepage_url":  c.HomepageURL,
		"description":   c.Description,
		"redirect_uris": c.RedirectURIs,
		"icon_url":      c.IconURL,
		"min_rank":      c.MinRank,
		"token_ttl":     c.TokenTTL,
		"status":        c.Status,
		"disabled":      c.Disabled,
		"stats":         clientStatsJSON(c.Stats),
		"created_at":    c.CreatedAt,
		"notify_email":   c.NotifyEmail,
		"notify_enabled": c.NotifyEnabled,
	}
}

// clientOwnerJSON PATCH 后返回的完整 client 字段（无 secret，含 status / stats / token_ttl / disabled）。
func clientOwnerJSON(c store.Client) map[string]any {
	return map[string]any{
		"client_id":     c.ClientID,
		"client_name":   c.ClientName,
		"owner_user_id": c.OwnerUserID,
		"homepage_url":  c.HomepageURL,
		"description":   c.Description,
		"redirect_uris": c.RedirectURIs,
		"icon_url":      c.IconURL,
		"min_rank":      c.MinRank,
		"token_ttl":     c.TokenTTL,
		"status":        c.Status,
		"disabled":      c.Disabled,
		"stats":         clientStatsJSON(c.Stats),
		"created_at":    c.CreatedAt,
		"notify_email":   c.NotifyEmail,
		"notify_enabled": c.NotifyEnabled,
	}
}

// isHTTPURL 判断字符串是否为合法 http(s) URL。
func isHTTPURL(u string) bool {
	p, err := url.Parse(u)
	if err != nil {
		return false
	}
	if p.Scheme != "http" && p.Scheme != "https" {
		return false
	}
	return p.Host != ""
}

// isValidEmail 轻量校验邮箱格式（本地部分 + @ + 域名包含点）。
func isValidEmail(s string) bool {
	if len(s) > 254 {
		return false
	}
	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 {
		return false
	}
	local := s[:at]
	domain := s[at+1:]
	if local == "" || domain == "" || !strings.Contains(domain, ".") {
		return false
	}
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// sha256Hex 返回字符串的 SHA-256 十六进制摘要。
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// clientTokenTTL 计算 access_token 有效期：默认 3600s，范围 60-86400s。
func clientTokenTTL(ttl int) int {
	if ttl <= 0 {
		ttl = 3600
	}
	if ttl < 60 {
		ttl = 60
	}
	if ttl > 86400 {
		ttl = 86400
	}
	return ttl
}

// clientStatusError 返回非 approved 状态对应的 403 文案；approved 返回空串（放行）。
func clientStatusError(status string) string {
	switch status {
	case "pending_review":
		return "应用审核中"
	case "rejected":
		return "应用未通过审核"
	case "paused":
		return "应用已暂停"
	case "pause_request":
		return "应用暂停申请处理中"
	case "resume_request":
		return "应用恢复申请处理中"
	case "delete_request":
		return "应用删除申请处理中"
	}
	return ""
}

// clientUsableError 返回应用「不可用」对应的 403 文案；可用返回空串（放行）。
// disabled 优先（fail-closed），其次沿用 status 拦截，与现有 paused 等处理保持一致。
func clientUsableError(c *store.Client) string {
	if c == nil {
		return "未知的应用"
	}
	if c.Disabled {
		return "应用已被管理端禁用"
	}
	return clientStatusError(c.Status)
}

// bumpStats 更新应用授权统计：跨日清零今日计数，再按成功/失败累加。
func bumpStats(s store.ClientStats, ok bool) store.ClientStats {
	today := time.Now().Format("2006-01-02")
	if s.StatsDate != today {
		s.AuthOKToday = 0
		s.AuthFailToday = 0
		s.StatsDate = today
	}
	if ok {
		s.AuthOKToday++
		s.AuthOKTotal++
	} else {
		s.AuthFailToday++
		s.AuthFailTotal++
	}
	return s
}

// effectiveMinRank 计算按应用生效的等级门槛：app.min_rank > 0 用 app 值，否则回退全局 NS_GATE_MIN_RANK。
func (a *API) effectiveMinRank(client *store.Client) int {
	if client != nil && client.MinRank > 0 {
		return client.MinRank
	}
	return a.cfg.GateMinRank
}

// checkGate 校验授权门槛（AND 语义，0=关闭），返回是否通过与失败文案。
// minRank 为按应用生效后的等级门槛；加入天数门槛仅全局。
func (a *API) checkGate(s nodeseek.UserStats, minRank int) (bool, string) {
	if minRank > 0 && s.Rank < minRank {
		return false, fmt.Sprintf("等级不足：需要 ≥ %d，当前 %d", minRank, s.Rank)
	}
	if a.cfg.GateMinJoinDays > 0 && s.JoinDays < a.cfg.GateMinJoinDays {
		return false, fmt.Sprintf("加入天数不足：需要 ≥ %d 天，当前 %d 天", a.cfg.GateMinJoinDays, s.JoinDays)
	}
	return true, ""
}

// clientAuthOK 记一次授权成功统计并 upsert 授权记录。
func (a *API) clientAuthOK(userID, clientID string) {
	if _, err := a.store.UpdateClient(clientID, func(c *store.Client) {
		c.Stats = bumpStats(c.Stats, true)
	}); err != nil {
		log.Printf("记录授权成功统计失败: %v", err)
	}
	if err := a.store.UpsertGrantActive(userID, clientID); err != nil {
		log.Printf("写入授权记录失败: %v", err)
	}
}

// clientAuthFail 记一次授权失败统计（含跨日清零）。
func (a *API) clientAuthFail(clientID string) {
	if clientID == "" {
		return
	}
	if _, err := a.store.UpdateClient(clientID, func(c *store.Client) {
		c.Stats = bumpStats(c.Stats, false)
	}); err != nil {
		log.Printf("记录授权失败统计失败: %v", err)
	}
}

// recordAccountFailure 记录账号读信失败（last_error + fail_count），Cookie 类错误触发告警邮件。
func (a *API) recordAccountFailure(ip, userID, accountID, errMsg string) {
	a.store.RecordAccountError(accountID, errMsg)
	if strings.Contains(strings.ToLower(errMsg), "cookie") {
		a.cookieAlert(ip, userID, accountID, errMsg)
	}
}

// cookieAlert 异步触发某账号 Cookie 失效告警邮件（按账号独立冷却 60min），并记审计。
func (a *API) cookieAlert(ip, userID, accountID, errMsg string) {
	now := time.Now()
	a.mu.Lock()
	if t, ok := a.accountAlerts[accountID]; ok && now.Sub(t) < time.Duration(a.cfg.MailCooldownMin)*time.Minute {
		a.mu.Unlock()
		return
	}
	a.accountAlerts[accountID] = now
	a.mu.Unlock()

	a.stats.IncCookieAlert()
	a.audit.Eventf("mail.cookie_alert", ip, userID, accountID, errMsg)
	subject := fmt.Sprintf("【NSAuth2】系统账号 Cookie 失效（账号 %s）", accountID)
	body := fmt.Sprintf("系统账号 %s 的 Cookie 缺失或已失效，请通过管理页更新。\n错误信息: %s\n检测时间: %s", accountID, errMsg, now.Format(time.RFC3339))
	go func() {
		if err := a.mail.Send(subject, body); err != nil {
			log.Printf("Cookie 告警邮件发送失败: %v", err)
		} else {
			a.audit.Eventf("mail.sent", ip, userID, "", "Cookie 失效告警")
		}
	}()
}

// lastTestAtString 返回最近一次测试邮件时间（RFC3339），未测过返回空串。
func (a *API) lastTestAtString() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastTestAt.IsZero() {
		return ""
	}
	return a.lastTestAt.Format(time.RFC3339)
}

// ---- 3.3 契约 handler ----

// enabledAccountsJSON 返回启用账号列表（按 priority 升序，不含 Cookie）。
// withEnabled 控制是否带 enabled 字段（config 带、verify 不带，见 SPEC §3.3）。
func (a *API) enabledAccountsJSON(withEnabled bool) []map[string]any {
	accounts, err := a.store.ListAccounts()
	if err != nil {
		return []map[string]any{}
	}
	out := []map[string]any{}
	for _, ac := range accounts {
		if !ac.Enabled {
			continue
		}
		item := map[string]any{
			"account_id":   ac.AccountID,
			"account_name": ac.AccountName,
			"priority":     ac.Priority,
		}
		if withEnabled {
			item["enabled"] = ac.Enabled
		}
		out = append(out, item)
	}
	return out
}

// handleConfig GET /api/config
func (a *API) handleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"nodeseek": map[string]any{
			"base_url":              a.cfg.NSBaseURL,
			"space_url_template":    "{base}/space/{id}",
			"message_url":           "{base}/notification#/message?mode=talk&to={id}",
			"auth_account_id":       a.cfg.AuthAccountID,
			"auth_account_username": a.cfg.AuthAccountName,
			"accounts":              a.enabledAccountsJSON(true),
		},
		"business": map[string]any{
			"min_client_creation_rank": a.cfg.MinClientCreationRank,
		},
		"verification": map[string]any{
			"code_expiry_seconds": verificationCodeExpirySeconds,
		},
		"gate": map[string]any{
			"min_rank":      a.cfg.GateMinRank,
			"min_join_days": a.cfg.GateMinJoinDays,
		},
	})
}

// handleVerify POST /oauth/verify
func (a *API) handleVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "请求体格式错误")
		return
	}
	if !isNumeric(req.UserID) {
		writeError(w, http.StatusUnprocessableEntity, "user_id 必须为纯数字")
		return
	}

	// 限流（ip + uid 双键，任一超限即 429）。
	ip := remoteIP(r)
	if !a.limits.verifyIP.Allow("ip:"+ip) || !a.limits.verifyUID.Allow("uid:"+req.UserID) {
		a.denyRate(w, ip, req.UserID, "", "verify")
		return
	}
	// 黑名单拦截（fail-closed）。
	if a.checkUserBlocked(w, ip, req.UserID) {
		return
	}

	code := verificationCodePrefix + randomHex(8, true)
	exp := time.Now().Add(verificationCodeExpirySeconds * time.Second).Unix()
	if err := a.store.AddCode(store.Code{Code: code, UserID: req.UserID, ExpiresAt: exp, Used: false}); err != nil {
		writeError(w, http.StatusInternalServerError, "存储验证码失败")
		return
	}
	a.stats.IncVerify()
	a.audit.Eventf("login.verify", ip, req.UserID, "", "")

	writeJSON(w, http.StatusOK, map[string]any{
		"success":           true,
		"verification_code": code,
		"expires_in":        verificationCodeExpirySeconds,
		"accounts":          a.enabledAccountsJSON(false),
	})
}

// handleConfirm POST /oauth/confirm
func (a *API) handleConfirm(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID           string `json:"user_id"`
		VerificationCode string `json:"verification_code"`
		Next             string `json:"next"` // 可选：登录后跳转目标（非契约字段，用于支持 ?next=）
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "请求体格式错误")
		return
	}
	if !isNumeric(req.UserID) {
		writeError(w, http.StatusUnprocessableEntity, "user_id 必须为纯数字")
		return
	}
	if req.VerificationCode == "" {
		writeError(w, http.StatusUnprocessableEntity, "verification_code 不能为空")
		return
	}

	// 限流（ip + uid 双键，任一超限即 429）。
	ip := remoteIP(r)
	if !a.limits.confirmIP.Allow("ip:"+ip) || !a.limits.confirmUID.Allow("uid:"+req.UserID) {
		a.denyRate(w, ip, req.UserID, "", "confirm")
		return
	}
	// 黑名单拦截（fail-closed）。
	if a.checkUserBlocked(w, ip, req.UserID) {
		return
	}

	rec, err := a.store.GetCode(req.VerificationCode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取验证码失败")
		return
	}
	if rec == nil || rec.UserID != req.UserID {
		a.stats.IncLoginFail()
		a.audit.Eventf("login.confirm.fail", ip, req.UserID, "", "验证码不匹配")
		writeError(w, http.StatusBadRequest, "验证码不匹配")
		return
	}
	if rec.Used {
		a.stats.IncLoginFail()
		a.audit.Eventf("login.confirm.fail", ip, req.UserID, "", "验证码已使用")
		writeError(w, http.StatusBadRequest, "验证码已使用")
		return
	}
	if time.Now().Unix() > rec.ExpiresAt {
		a.stats.IncLoginFail()
		a.audit.Eventf("login.confirm.fail", ip, req.UserID, "", "验证码已过期")
		writeError(w, http.StatusBadRequest, "验证码已过期")
		return
	}

	// 按 priority 升序遍历启用账号读私信，命中即成功；失败账号跳过并告警（故障转移）。
	accounts, err := a.store.ListAccounts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取系统账号失败")
		return
	}
	received := false
	failCount := 0
	checkedCount := 0
	for i := range accounts {
		ac := accounts[i]
		if !ac.Enabled {
			continue
		}
		cookie, derr := a.store.GetAccountCookie(ac.AccountID)
		if derr != nil {
			failCount++
			a.recordAccountFailure(ip, req.UserID, ac.AccountID, fmt.Sprintf("账号 %s Cookie 解密失败", ac.AccountID))
			continue
		}
		if cookie == "" {
			// 未设置 Cookie：视为该账号读信失败（跳过），保证轮询/故障转移可测。
			failCount++
			a.recordAccountFailure(ip, req.UserID, ac.AccountID, fmt.Sprintf("系统账号 Cookie 未设置（账号 %s）", ac.AccountID))
			continue
		}
		ok, cerr := a.ns.CheckCodeReceived(cookie, req.UserID, req.VerificationCode)
		if cerr != nil {
			failCount++
			a.recordAccountFailure(ip, req.UserID, ac.AccountID, cerr.Error())
			continue
		}
		checkedCount++
		if ok {
			received = true
			break
		}
		// 该账号未命中，继续下一个。
	}

	if !received {
		a.stats.IncLoginFail()
		if checkedCount == 0 && failCount > 0 {
			// 全部账号读信失败。
			a.audit.Eventf("login.confirm.fail", ip, req.UserID, "", fmt.Sprintf("%d 个账号读信失败", failCount))
			writeError(w, http.StatusUnauthorized, fmt.Sprintf("所有系统账号读信失败（%d 个），请稍后重试或联系管理员", failCount))
			return
		}
		a.audit.Eventf("login.confirm.fail", ip, req.UserID, "", "未检测到验证码私信")
		writeError(w, http.StatusBadRequest, "未检测到验证码私信，请确认已发送后重试")
		return
	}

	if err := a.store.MarkCodeUsed(req.VerificationCode); err != nil {
		writeError(w, http.StatusInternalServerError, "更新验证码状态失败")
		return
	}

	sessionTTL := time.Duration(a.cfg.SessionTTLMin) * time.Minute
	token, err := auth.SignSession(a.key, req.UserID, sessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "签发会话失败")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
		// Secure 未设置以支持本地 http 开发；生产 https 下应开启（TODO）。
	})
	a.stats.IncLoginOK()
	a.audit.Eventf("login.confirm.ok", ip, req.UserID, "", "")

	redirectTo := req.Next
	if redirectTo == "" || !strings.HasPrefix(redirectTo, "/") {
		redirectTo = "/"
	}

	// 顺带拉取用户信息并记录用户表；失败时 stats 为 null，不阻塞登录。
	var statsPayload any
	if profile, err := a.ns.FetchUserProfile(req.UserID); err == nil {
		statsPayload = statsJSON(profile.Stats)
		// upsert 用户记录（name 取 getInfo member_name）。
		if _, uerr := a.store.UpsertUser(req.UserID, profile.MemberName); uerr != nil {
			log.Printf("upsert 用户记录失败: %v", uerr)
		}
	} else {
		// getInfo 失败仍记录用户（无昵称），保证登录计数不丢失。
		if _, uerr := a.store.UpsertUser(req.UserID, ""); uerr != nil {
			log.Printf("upsert 用户记录失败: %v", uerr)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"redirect_to": redirectTo,
		"stats":       statsPayload,
	})
}

// handleAuthorize GET /oauth/authorize
func (a *API) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")
	responseType := q.Get("response_type")
	scope := q.Get("scope")
	state := q.Get("state")

	// scope 仅支持 "user"（缺省按 user；其他值按 OAuth2 规范报错）。
	if scope != "" && scope != "user" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_scope", "不支持的 scope")
		return
	}
	// state 可选、≤256 字符（超长 invalid_request）。
	if len(state) > 256 {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "state 过长（最多 256 字符）")
		return
	}

	if clientID == "" || redirectURI == "" || responseType == "" {
		writeError(w, http.StatusUnprocessableEntity, "缺少 client_id / redirect_uri / response_type")
		return
	}
	if responseType != "code" {
		writeError(w, http.StatusBadRequest, "不支持的 response_type（仅支持 code）")
		return
	}

	// 限流（cid + ip 双键）。
	ip := remoteIP(r)
	if !a.limits.authorizeCID.Allow("cid:"+clientID) || !a.limits.appIP.Allow("ip:"+ip) {
		a.denyRate(w, ip, "", clientID, "authorize")
		return
	}

	client, err := a.store.GetClient(clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取客户端失败")
		return
	}
	if client == nil {
		writeError(w, http.StatusBadRequest, "未知的 client_id")
		return
	}
	if msg := clientUsableError(client); msg != "" {
		writeError(w, http.StatusForbidden, msg)
		return
	}
	if !contains(client.RedirectURIs, redirectURI) {
		writeError(w, http.StatusUnprocessableEntity, "redirect_uri 不在白名单中")
		return
	}

	sess := a.currentSession(r)
	if sess == nil {
		next := fullRequestURL(r)
		http.Redirect(w, r, "/login?next="+url.QueryEscape(next), http.StatusFound)
		return
	}
	// 黑名单拦截（fail-closed）。
	if u, err := a.store.GetUser(sess.UserID); err == nil && u != nil && u.Blacklisted {
		ip := remoteIP(r)
		a.audit.Eventf("user.blocked", ip, u.UserID, clientID, "该账号已被服务禁用")
		writeError(w, http.StatusForbidden, "该账号已被服务禁用")
		return
	}
	// 已登录：返回 SPA HTML，由前端渲染授权确认页。
	a.serveSPA(w, r)
}

// handleDecision POST /oauth/authorize/decision
func (a *API) handleDecision(w http.ResponseWriter, r *http.Request) {
	sess := a.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusUnauthorized, "需要登录")
		return
	}
	// 黑名单拦截（fail-closed）。
	if u, err := a.store.GetUser(sess.UserID); err == nil && u != nil && u.Blacklisted {
		ip := remoteIP(r)
		a.audit.Eventf("user.blocked", ip, sess.UserID, "", "该账号已被服务禁用")
		writeError(w, http.StatusForbidden, "该账号已被服务禁用")
		return
	}

	var req struct {
		Approve      bool   `json:"approve"`
		ClientID     string `json:"client_id"`
		RedirectURI  string `json:"redirect_uri"`
		ResponseType string `json:"response_type"`
		State        string `json:"state"`
		Scope        string `json:"scope"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "请求体格式错误")
		return
	}

	// 限流（cid + ip 双键）。
	ip := remoteIP(r)
	if !a.limits.decisionCID.Allow("cid:"+req.ClientID) || !a.limits.appIP.Allow("ip:"+ip) {
		a.denyRate(w, ip, sess.UserID, req.ClientID, "decision")
		return
	}

	client, err := a.store.GetClient(req.ClientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取客户端失败")
		return
	}
	if client == nil {
		writeError(w, http.StatusBadRequest, "未知的 client_id")
		return
	}
	if msg := clientUsableError(client); msg != "" {
		a.clientAuthFail(req.ClientID)
		writeError(w, http.StatusForbidden, msg)
		return
	}
	if !contains(client.RedirectURIs, req.RedirectURI) {
		a.clientAuthFail(req.ClientID)
		writeError(w, http.StatusUnprocessableEntity, "redirect_uri 不在白名单中")
		return
	}

	if !req.Approve {
		a.clientAuthFail(req.ClientID)
		redirect := appendQuery(req.RedirectURI, "error", "access_denied")
		if req.State != "" {
			redirect = appendQuery(redirect, "state", req.State)
		}
		http.Redirect(w, r, redirect, http.StatusFound)
		return
	}

	// 服务端复检授权门槛（权威、fail-closed、按应用生效门槛），不满足返回 403 JSON 不重定向。
	stats, err := a.ns.FetchUserStats(sess.UserID)
	if err != nil {
		a.clientAuthFail(req.ClientID)
		writeError(w, http.StatusForbidden, "无法获取用户信息，请稍后重试")
		return
	}
	if ok, msg := a.checkGate(stats, a.effectiveMinRank(client)); !ok {
		a.stats.IncGateBlock()
		a.audit.Eventf("gate.block", ip, sess.UserID, req.ClientID, msg)
		a.clientAuthFail(req.ClientID)
		writeError(w, http.StatusForbidden, msg)
		return
	}

	code := randomHex(32, false) // 32 位十六进制授权码
	exp := time.Now().Add(authCodeTTL).Unix()
	scope := req.Scope
	if scope == "" {
		scope = "user"
	}
	if err := a.store.AddCode(store.Code{
		Code:        code,
		UserID:      sess.UserID,
		ClientID:    req.ClientID,
		RedirectURI: req.RedirectURI,
		Scope:       scope,
		ExpiresAt:   exp,
		Used:        false,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "存储授权码失败")
		return
	}
	a.clientAuthOK(sess.UserID, req.ClientID)
	a.audit.Eventf("authorize.code", ip, sess.UserID, req.ClientID, code)
	redirect := appendQuery(req.RedirectURI, "code", code)
	if req.State != "" {
		redirect = appendQuery(redirect, "state", req.State)
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

// handleGetClient GET /api/oauth/client
func (a *API) handleGetClient(w http.ResponseWriter, r *http.Request) {
	sess := a.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusUnauthorized, "需要登录")
		return
	}
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		writeError(w, http.StatusUnprocessableEntity, "缺少 client_id")
		return
	}
	client, err := a.store.GetClient(clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取客户端失败")
		return
	}
	if client == nil {
		writeError(w, http.StatusNotFound, "客户端不存在")
		return
	}
	if msg := clientUsableError(client); msg != "" {
		writeError(w, http.StatusForbidden, msg)
		return
	}

	// 实时拉取用户信息并校验按应用生效的授权门槛（fail-closed）。
	stats, err := a.ns.FetchUserStats(sess.UserID)
	if err != nil {
		writeError(w, http.StatusForbidden, "无法获取用户信息，请稍后重试")
		return
	}
	minRank := a.effectiveMinRank(client)
	if ok, msg := a.checkGate(stats, minRank); !ok {
		a.stats.IncGateBlock()
		a.audit.Eventf("gate.block", remoteIP(r), sess.UserID, clientID, msg)
		writeError(w, http.StatusForbidden, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"client":  clientAuthJSON(*client),
		"stats":   statsJSON(stats),
		"gate": map[string]any{
			"min_rank":      minRank,
			"min_join_days": a.cfg.GateMinJoinDays,
			"ok":            true,
		},
	})
}

// handleClientRegister POST /api/client/register
func (a *API) handleClientRegister(w http.ResponseWriter, r *http.Request) {
	sess := a.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusUnauthorized, "需要登录")
		return
	}

	var req struct {
		Name         string   `json:"name"`
		HomepageURL  string   `json:"homepage_url"`
		Description  string   `json:"description"`
		RedirectURIs []string `json:"redirect_uris"`
		IconURL      string   `json:"icon_url"`
		MinRank      int      `json:"min_rank"`
		TokenTTL     int      `json:"token_ttl"`
		NotifyEmail  string   `json:"notify_email"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "请求体格式错误")
		return
	}
	// notify_email 可选：非空时须为合法邮箱（非法则 400 明确拒绝）。
	if req.NotifyEmail != "" && !isValidEmail(req.NotifyEmail) {
		writeError(w, http.StatusBadRequest, "notify_email 格式不正确")
		return
	}

	// 应用名非空
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusUnprocessableEntity, "应用名不能为空")
		return
	}
	// 应用名唯一（大小写不敏感）
	exists, err := a.store.ClientNameExists(req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "校验应用名失败")
		return
	}
	if exists {
		writeError(w, http.StatusBadRequest, "应用名已存在")
		return
	}
	// redirect_uris 至少 1 个且均为合法 http(s) URL
	if len(req.RedirectURIs) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "至少需要一个回调地址")
		return
	}
	for _, u := range req.RedirectURIs {
		if !isHTTPURL(u) {
			writeError(w, http.StatusUnprocessableEntity, "回调地址必须为合法的 http(s) URL")
			return
		}
	}
	// icon_url 可选：合法 URL 或空串
	if req.IconURL != "" && !isHTTPURL(req.IconURL) {
		writeError(w, http.StatusUnprocessableEntity, "图标地址必须为合法的 http(s) URL")
		return
	}
	// min_rank 为 0-6 整数（NodeSeek 最高 6 级，0=不限）
	if req.MinRank < 0 || req.MinRank > 6 {
		writeError(w, http.StatusUnprocessableEntity, "min_rank 必须为 0-6 的整数")
		return
	}
	// token_ttl 为 access_token 有效期（秒），默认 3600，范围 60-86400
	ttl := req.TokenTTL
	if ttl == 0 {
		ttl = 3600
	}
	if ttl < 60 || ttl > 86400 {
		writeError(w, http.StatusUnprocessableEntity, "token_ttl 必须在 60-86400 秒之间")
		return
	}

	// 创建人等级门槛：mock 模式放行；否则 FetchUserStats 校验，拉取失败 fail-closed。
	if !a.cfg.MockMode {
		stats, err := a.ns.FetchUserStats(sess.UserID)
		if err != nil {
			writeError(w, http.StatusForbidden, "无法获取用户信息，请稍后重试")
			return
		}
		if stats.Rank < a.cfg.MinClientCreationRank {
			writeError(w, http.StatusForbidden, fmt.Sprintf("等级不足：创建应用需要等级 ≥ %d，当前 %d", a.cfg.MinClientCreationRank, stats.Rank))
			return
		}
	}

	// 生成 client_id / client_secret（32 位小写 hex），secret 仅本次明文返回，存储只存 SHA-256。
	clientID := randomHex(32, false)
	secret := randomHex(32, false)
	// 新应用默认 pending_review；mock 模式自动 approved（便于本地联调）。
	status := "pending_review"
	if a.cfg.MockMode {
		status = "approved"
	}
	client := store.Client{
		ClientID:         clientID,
		ClientSecretHash: sha256Hex(secret),
		ClientName:       req.Name,
		OwnerUserID:      sess.UserID,
		HomepageURL:      req.HomepageURL,
		Description:      req.Description,
		RedirectURIs:     req.RedirectURIs,
		IconURL:          req.IconURL,
		MinRank:          req.MinRank,
		TokenTTL:         ttl,
		Status:           status,
		Stats:            store.ClientStats{},
		Builtin:          false,
		Scopes:           []string{},
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		NotifyEmail:      req.NotifyEmail,
		NotifyEnabled:    true,
		NotifyToken:      randomHex(32, false),
	}
	if err := a.store.AddClient(client); err != nil {
		writeError(w, http.StatusInternalServerError, "存储应用失败")
		return
	}
	a.audit.Eventf("client.register", remoteIP(r), sess.UserID, clientID, "")

	// 审核通知邮件（NS_REVIEW_EMAIL_NOTIFY=1 时异步发送；mock 模式走 [MAIL-MOCK] 日志）。
	if a.cfg.ReviewEmailNotify {
		a.sendReviewNotify(client)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"client": map[string]any{
			"client_id":     client.ClientID,
			"client_secret": secret,
			"client_name":   client.ClientName,
			"owner_user_id": client.OwnerUserID,
			"homepage_url":  client.HomepageURL,
			"description":   client.Description,
			"redirect_uris": client.RedirectURIs,
			"icon_url":      client.IconURL,
			"min_rank":      client.MinRank,
			"token_ttl":     client.TokenTTL,
			"status":        client.Status,
			"created_at":    client.CreatedAt,
		},
	})
}

// sendReviewNotify 异步发送「新应用待审核」通知邮件。
func (a *API) sendReviewNotify(c store.Client) {
	subject := "【NSAuth2】新应用待审核"
	body := fmt.Sprintf("新应用待审核\n应用名: %s\nowner: %s\n主页: %s\n回调地址: %s",
		c.ClientName, c.OwnerUserID, c.HomepageURL, strings.Join(c.RedirectURIs, ", "))
	go func() {
		if err := a.mail.Send(subject, body); err != nil {
			log.Printf("审核通知邮件发送失败: %v", err)
		} else {
			a.audit.Eventf("mail.sent", "", "", c.ClientID, "新应用待审核")
		}
	}()
}

// sendReviewResultMail 发送「应用审核结果」邮件给应用创建者提交的通知邮箱。
// 失败不阻塞审核流程（仅记日志与审计）。
func (a *API) sendReviewResultMail(c store.Client, approve bool, reason string) {
	result := "通过"
	step := "应用已上线，可正常发起授权。"
	if !approve {
		result = "未通过"
		step = "请根据拒绝理由修改后重新提交，或在控制台删除后重新注册。"
	}
	subject := "【NSAuth2】应用审核结果"
	body := fmt.Sprintf("应用「%s」的审核结果为：%s\n\n应用ID: %s\n拒绝理由: %s\n\n下一步: %s",
		c.ClientName, result, c.ClientID, reason, step)
	go func() {
		ev := "mail.review_result"
		if err := a.mail.Send(subject, body); err != nil {
			log.Printf("审核结果邮件发送失败: %v", err)
			ev = "mail.send_fail"
		}
		a.audit.Eventf(ev, "", "", c.ClientID, result)
	}()
}

// handleClientList GET /api/client/list
func (a *API) handleClientList(w http.ResponseWriter, r *http.Request) {
	sess := a.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusUnauthorized, "需要登录")
		return
	}
	cs, err := a.store.ListClients()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取应用列表失败")
		return
	}
	clients := []map[string]any{}
	for i := range cs {
		if cs[i].OwnerUserID == sess.UserID && !cs[i].Builtin {
			clients = append(clients, clientListJSON(cs[i]))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"clients": clients,
	})
}

// handleToken POST /oauth/token（OAuth2 授权码兑换 access_token）。
func (a *API) handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "无法解析表单")
		return
	}
	grantType := r.PostFormValue("grant_type")
	code := r.PostFormValue("code")
	clientID := r.PostFormValue("client_id")
	clientSecret := r.PostFormValue("client_secret")
	redirectURI := r.PostFormValue("redirect_uri")

	// 限流（cid + ip 双键）。
	ip := remoteIP(r)
	if !a.limits.tokenCID.Allow("cid:"+clientID) || !a.limits.appIP.Allow("ip:"+ip) {
		a.denyRate(w, ip, "", clientID, "token")
		return
	}

	// fail 统一写 OAuth2 错误并记审计 token.exchange.fail。
	fail := func(status int, oauthErr, desc string) {
		a.audit.Eventf("token.exchange.fail", ip, "", clientID, desc)
		writeOAuthError(w, status, oauthErr, desc)
	}

	if grantType != "authorization_code" {
		fail(http.StatusBadRequest, "invalid_request", "不支持的 grant_type（仅支持 authorization_code）")
		return
	}
	if code == "" || clientID == "" || clientSecret == "" || redirectURI == "" {
		fail(http.StatusBadRequest, "invalid_request", "缺少必要参数")
		return
	}

	rec, err := a.store.GetCode(code)
	if err != nil {
		fail(http.StatusInternalServerError, "server_error", "读取授权码失败")
		return
	}
	if rec == nil || rec.ClientID == "" {
		fail(http.StatusBadRequest, "invalid_grant", "授权码无效")
		return
	}
	if rec.Used {
		fail(http.StatusBadRequest, "invalid_grant", "授权码已使用")
		return
	}
	if time.Now().Unix() > rec.ExpiresAt {
		fail(http.StatusBadRequest, "invalid_grant", "授权码已过期")
		return
	}
	if rec.ClientID != clientID {
		fail(http.StatusBadRequest, "invalid_grant", "授权码与 client_id 不匹配")
		return
	}
	if rec.RedirectURI != redirectURI {
		fail(http.StatusBadRequest, "invalid_grant", "redirect_uri 与授权请求不一致")
		return
	}

	client, err := a.store.GetClient(clientID)
	if err != nil {
		fail(http.StatusInternalServerError, "server_error", "读取客户端失败")
		return
	}
	if client == nil {
		fail(http.StatusUnauthorized, "invalid_client", "未知的 client_id")
		return
	}
	// client_secret 的 SHA-256 常量时间比较（防时序侧信道）。
	if subtle.ConstantTimeCompare([]byte(sha256Hex(clientSecret)), []byte(client.ClientSecretHash)) != 1 {
		fail(http.StatusUnauthorized, "invalid_client", "client_secret 校验失败")
		return
	}

	// 防重放：先标记授权码 used，再签发 token。
	if err := a.store.MarkCodeUsed(code); err != nil {
		fail(http.StatusInternalServerError, "server_error", "更新授权码状态失败")
		return
	}

	accessToken := randomHex(32, false)
	ttl := clientTokenTTL(client.TokenTTL)
	exp := time.Now().Add(time.Duration(ttl) * time.Second).Unix()
	// TODO: MarkCodeUsed 与 AddToken 非原子；若 AddToken 失败，授权码已消耗但无 token 签发（fail-closed，可接受）。
	if err := a.store.AddToken(store.Token{
		Token:     accessToken,
		UserID:    rec.UserID,
		ClientID:  clientID,
		ExpiresAt: exp,
	}); err != nil {
		fail(http.StatusInternalServerError, "server_error", "签发 token 失败")
		return
	}
	a.audit.Eventf("token.exchange.ok", ip, rec.UserID, clientID, "")

	scope := rec.Scope
	if scope == "" {
		scope = "user"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   ttl,
		"scope":        scope,
	})
}

// extractBearerToken 从 Authorization: Bearer <token> 或 ?access_token= 提取 access_token。
func extractBearerToken(r *http.Request) string {
	if authz := r.Header.Get("Authorization"); authz != "" {
		const prefix = "Bearer "
		if len(authz) > len(prefix) && strings.EqualFold(authz[:len(prefix)], prefix) {
			return strings.TrimSpace(authz[len(prefix):])
		}
	}
	return r.URL.Query().Get("access_token")
}

// handleUserInfo GET /oauth/userinfo（access_token 消费端点，一次性身份授权）。
func (a *API) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	token := extractBearerToken(r)
	ip := remoteIP(r)

	rec, err := a.store.GetToken(token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取 token 失败")
		return
	}
	if rec == nil || time.Now().Unix() > rec.ExpiresAt {
		a.audit.Eventf("userinfo.access", ip, "", "", "token 无效或已过期")
		writeError(w, http.StatusUnauthorized, "token 无效或已过期")
		return
	}

	// 实时拉取用户信息；失败 stats=null 不阻塞。
	var statsPayload any
	if st, err := a.ns.FetchUserStats(rec.UserID); err == nil {
		statsPayload = statsJSON(st)
	}

	a.audit.Eventf("userinfo.access", ip, rec.UserID, rec.ClientID, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"user_id":   rec.UserID,
		"sub":       rec.UserID,
		"client_id": rec.ClientID,
		"stats":     statsPayload,
	})
}

// handleHealthz GET /healthz（探活端点，无鉴权，不含敏感信息）。
func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// handleWellKnown GET /.well-known/oauth-authorization-server（RFC 8414 元数据，无需鉴权）。
func (a *API) handleWellKnown(w http.ResponseWriter, r *http.Request) {
	origin := requestOrigin(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                origin,
		"authorization_endpoint":                origin + "/oauth/authorize",
		"token_endpoint":                        origin + "/oauth/token",
		"userinfo_endpoint":                     origin + "/oauth/userinfo",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"scopes_supported":                      []string{"user"},
		"token_endpoint_auth_methods_supported": []string{"client_secret_post"},
		"code_challenge_methods_supported":      []string{},
	})
}

// handleLogout POST /api/logout
func (a *API) handleLogout(w http.ResponseWriter, r *http.Request) {
	if a.currentSession(r) == nil {
		writeError(w, http.StatusUnauthorized, "需要登录")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// handleMe GET /api/me
func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	sess := a.currentSession(r)
	if sess == nil {
		writeError(w, http.StatusUnauthorized, "需要登录")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "user_id": sess.UserID})
}

// handleClientNotifyUnsubscribe GET /api/client/notifications/unsubscribe?token={notify_token}
// 应用所有者通过邮件内链接关闭通知（token 泄露面=邮件持有者）。返回简单绿色提示页，无需登录。
func (a *API) handleClientNotifyUnsubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeHTML(w, notifyUnsubscribeHTML(true, "参数缺失", ""))
		return
	}
	cs, err := a.store.ListClients()
	if err != nil {
		writeHTML(w, notifyUnsubscribeHTML(true, "系统错误，请稍后重试", ""))
		return
	}
	for i := range cs {
		if cs[i].NotifyToken == token {
			if _, uerr := a.store.UpdateClient(cs[i].ClientID, func(c *store.Client) {
				c.NotifyEnabled = false
			}); uerr != nil {
				writeHTML(w, notifyUnsubscribeHTML(true, "系统错误，请稍后重试", cs[i].ClientName))
				return
			}
			a.audit.Eventf("client.notify_off", "", "", cs[i].ClientID, "用户通过链接关闭通知")
			writeHTML(w, notifyUnsubscribeHTML(false, "", cs[i].ClientName))
			return
		}
	}
	writeHTML(w, notifyUnsubscribeHTML(true, "链接无效或已失效", ""))
}

func notifyUnsubscribeHTML(err bool, msg, appName string) string {
	head := "<!doctype html><html lang=\"zh-CN\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>通知设置</title></head><body style=\"font-family:system-ui,sans-serif;background:#f5f5f5;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0\">"
	cardStyle := "background:#fff;border-radius:12px;padding:40px;box-shadow:0 2px 12px rgba(0,0,0,.08);text-align:center;max-width:420px"
	titleColor := "#1a7f37"
	title := "✅ 通知已关闭"
	sub := "你已关闭应用 " + appName + " 的审核与错误率告警通知。"
	if err {
		titleColor = "#c0392b"
		title = "⚠️ 操作未完成"
		sub = msg
	}
	return head + "<div style=\"" + cardStyle + "\"><h2 style=\"color:" + titleColor + ";margin:0 0 12px\">" + title + "</h2><p style=\"color:#666;margin:0\">" + sub + "</p><p style=\"color:#aaa;margin-top:24px;font-size:12px\">Nodeseek OAuth2</p></div></body></html>"
}

// handleAdminLogin POST /api/admin/login（JSON {"username":"...","password":"..."}）管理端账号密码登录。
// 成功 → 200 + Set-Cookie ns_admin_session；失败 → 401；登录失败限流（每 IP 10/min）。
// 兼容旧 {"token"} 请求体（调用方为 NS_ADMIN_TOKEN 时通过），扩展推送仍走 X-Admin-Token 头。
func (a *API) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Token    string `json:"token"` // 兼容旧令牌登录
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "请求体格式错误")
		return
	}
	ip := remoteIP(r)

	// 登录失败限流（每 IP 每分钟 10 次）。
	if !a.limits.adminLoginIP.Allow("ip:" + ip) {
		a.denyRate(w, ip, "", "", "admin.login")
		return
	}

	ok := false
	// 账号密码登录：存储凭据优先，其次 env；两者都未命中则回退旧令牌。
	if a.adminLoginValid(req.Username, req.Password) {
		ok = true
	}
	// 兼容：旧令牌登录（账号密码未通过时回退；任一命中即通过）。
	if !ok && a.cfg.AdminToken != "" && req.Token != "" &&
		subtle.ConstantTimeCompare([]byte(a.cfg.AdminToken), []byte(req.Token)) == 1 {
		ok = true
	}

	if !ok {
		a.audit.Eventf("admin.login.fail", ip, "", "", "账号或密码错误")
		writeError(w, http.StatusUnauthorized, "账号或密码错误")
		return
	}

	id := randomHex(32, false)
	a.sessionsMu.Lock()
	a.adminSess[id] = time.Now()
	a.sessionsMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     AdminCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure 不加：nginx 层 https；浏览器即便 https 也无碍。
	})
	a.audit.Eventf("admin.login.ok", ip, "", "", "")
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// handleAdminLogout POST /api/admin/logout 管理端登出：删除会话并清 Cookie。
func (a *API) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(AdminCookieName); err == nil && c.Value != "" {
		a.sessionsMu.Lock()
		delete(a.adminSess, c.Value)
		a.sessionsMu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     AdminCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	a.audit.Eventf("admin.logout", remoteIP(r), "", "", "")
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// handleAdminPassword PATCH /api/admin/password（X-Admin-Token 或会话鉴权）修改管理凭据密码。
// 请求体 {old_password, new_password}；成功写入 admin.json 且立即生效（当前会话不失效）。
func (a *API) handleAdminPassword(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "请求体格式错误")
		return
	}
	if req.OldPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusUnprocessableEntity, "old_password 与 new_password 均必填")
		return
	}
	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusUnprocessableEntity, "new_password 长度至少 8 位")
		return
	}
	if subtle.ConstantTimeCompare([]byte(req.OldPassword), []byte(req.NewPassword)) == 1 {
		writeError(w, http.StatusUnprocessableEntity, "new_password 不能与 old_password 相同")
		return
	}
	ip := remoteIP(r)

	// 校验旧密码（存储或 env 当前凭据）。
	if !a.storePasswordValid(req.OldPassword) {
		a.audit.Eventf("admin.password.change_fail", ip, "", "", "旧密码错误")
		writeError(w, http.StatusUnauthorized, "旧密码错误")
		return
	}

	username := a.currentAdminUsername()
	if username == "" {
		// 理论上 checkAdmin 已保证有凭据；防御性兜底。
		a.audit.Eventf("admin.password.change_fail", ip, "", "", "无可用用户名")
		writeError(w, http.StatusInternalServerError, "无法确定当前管理用户名")
		return
	}

	if _, err := a.store.SetAdminCred(username, req.NewPassword); err != nil {
		a.audit.Eventf("admin.password.change_fail", ip, "", "", "写入失败")
		writeError(w, http.StatusInternalServerError, "保存凭据失败")
		return
	}
	a.audit.Eventf("admin.password.change", ip, "", "", "密码已修改")
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// storePasswordValid 校验密码是否匹配当前生效凭据（存储优先，其次 env）。
func (a *API) storePasswordValid(password string) bool {
	if password == "" {
		return false
	}
	if c, err := a.store.GetAdminCred(); err == nil && c != nil && c.PasswordCipher != "" {
		stored, derr := a.store.DecryptAdminPassword(c)
		if derr != nil {
			return false
		}
		return subtle.ConstantTimeCompare([]byte(stored), []byte(password)) == 1
	}
	// 回退 env 密码。
	if a.cfg.AdminPassword == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a.cfg.AdminPassword), []byte(password)) == 1
}

// handleClientPatch PATCH /api/client/{client_id}
// handleClientPatch PATCH /api/client/{client_id}（管理端专用，X-Admin-Token 鉴权）。
func (a *API) handleClientPatch(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	clientID := r.PathValue("client_id")
	client, err := a.store.GetClient(clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取客户端失败")
		return
	}
	if client == nil {
		writeError(w, http.StatusNotFound, "客户端不存在")
		return
	}

	var req struct {
		TokenTTL *int    `json:"token_ttl"`
		Status   *string `json:"status"`
		Disabled *bool   `json:"disabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "请求体格式错误")
		return
	}
	if req.TokenTTL != nil && (*req.TokenTTL < 60 || *req.TokenTTL > 86400) {
		writeError(w, http.StatusUnprocessableEntity, "token_ttl 必须在 60-86400 秒之间")
		return
	}
	if req.Status != nil && !isValidClientStatus(*req.Status) {
		writeError(w, http.StatusUnprocessableEntity, "status 不合法")
		return
	}

	updated, err := a.store.UpdateClient(clientID, func(c *store.Client) {
		if req.TokenTTL != nil {
			c.TokenTTL = *req.TokenTTL
		}
		if req.Status != nil {
			c.Status = *req.Status
		}
		if req.Disabled != nil {
			c.Disabled = *req.Disabled
		}
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "更新应用失败")
		return
	}
	if updated == nil {
		writeError(w, http.StatusNotFound, "客户端不存在")
		return
	}
	a.audit.Eventf("client.patch", remoteIP(r), "", clientID, fmt.Sprintf("disabled=%v", updated.Disabled))

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"client":  clientOwnerJSON(*updated),
	})
}

// handleClientDelete DELETE /api/client/{client_id}（管理端专用）。
func (a *API) handleClientDelete(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	clientID := r.PathValue("client_id")
	deleted, err := a.store.DeleteClient(clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "删除应用失败")
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "客户端不存在")
		return
	}
	a.audit.Eventf("client.delete", remoteIP(r), "", clientID, "")
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// handleAdminCookie POST /api/admin/cookie
func (a *API) handleAdminCookie(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	var req struct {
		Cookie    string `json:"cookie"`
		AccountID string `json:"account_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "请求体格式错误")
		return
	}
	if strings.TrimSpace(req.Cookie) == "" {
		writeError(w, http.StatusUnprocessableEntity, "cookie 不能为空")
		return
	}

	ip := remoteIP(r)
	accountID := req.AccountID
	accountName := ""
	autoDetected := false

	if a.cfg.CookieAutoDetect {
		// 自动识别：用推送的 Cookie 调 whoami 探测归属账号。
		who, err := a.ns.WhoAmI(req.Cookie)
		if err != nil {
			writeError(w, http.StatusBadRequest, "无法识别 Cookie 对应账号")
			return
		}
		accountID = who.UserID
		accountName = who.Username
		autoDetected = true
	} else if accountID == "" {
		// 手动绑定：必须带 account_id。
		writeError(w, http.StatusBadRequest, "未启用自动识别，必须带 account_id 手动绑定")
		return
	}

	acc, err := a.store.UpsertAccountCookie(accountID, accountName, req.Cookie, autoDetected)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "存储 Cookie 失败")
		return
	}
	a.audit.Eventf("admin.cookie.update", ip, "", "", accountID)

	// 顺带拉取账号等级信息（供扩展向导显示）；失败为 null 不阻塞。
	var statsPayload any
	if st, err := a.ns.FetchUserStats(accountID); err == nil {
		statsPayload = map[string]any{
			"rank":      st.Rank,
			"join_days": st.JoinDays,
			"coin":      st.Chicken,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"account_id":   acc.AccountID,
		"account_name": acc.AccountName,
		"updated_at":   acc.UpdatedAt,
		"stats":        statsPayload,
	})
}

// handleAdminStatus GET /api/admin/status
func (a *API) handleAdminStatus(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	accounts, err := a.store.ListAccounts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取状态失败")
		return
	}
	acctList := []map[string]any{}
	cookieSet := false
	var cookieUpdatedAt string // 已设置 Cookie 账号中最近更新的 updated_at
	for _, ac := range accounts {
		acctList = append(acctList, map[string]any{
			"account_id":   ac.AccountID,
			"account_name": ac.AccountName,
			"priority":     ac.Priority,
			"enabled":      ac.Enabled,
			"updated_at":   ac.UpdatedAt,
			"last_error":   ac.LastError,
			"fail_count":   ac.FailCount,
		})
		if ac.CookieEncrypted != "" {
			cookieSet = true
			if ac.UpdatedAt > cookieUpdatedAt {
				cookieUpdatedAt = ac.UpdatedAt
			}
		}
	}
	// cookie.age_seconds：now-updated_at 秒（RFC3339 字符串对比，同格式可字典序取最大）。
	cookieAgeSeconds := 0
	if cookieUpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, cookieUpdatedAt); err == nil {
			age := int(time.Since(t).Seconds())
			if age > 0 {
				cookieAgeSeconds = age
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"accounts": map[string]any{
			"count": len(accounts),
			"list":  acctList,
		},
		"cookie": map[string]any{
			"set":          cookieSet,
			"updated_at":   cookieUpdatedAt,
			"age_seconds":  cookieAgeSeconds,
		},
		"mock_mode": a.cfg.MockMode,
		"mail": map[string]any{
			"configured":          a.mail.Configured(),
			"report_time":         a.cfg.ReportTime,
			"last_test_at":        a.lastTestAtString(),
			"review_email_notify": a.cfg.ReviewEmailNotify,
		},
	})
}

// handleAdminTestMail POST /api/admin/test-mail
func (a *API) handleAdminTestMail(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	if !a.mail.Configured() {
		writeError(w, http.StatusBadRequest, "SMTP 未配置")
		return
	}
	subject := "NSAuth2 测试邮件"
	if err := a.mail.Send(subject, a.mail.Summary()); err != nil {
		writeError(w, http.StatusInternalServerError, "测试邮件发送失败: "+err.Error())
		return
	}
	a.mu.Lock()
	a.lastTestAt = time.Now()
	a.mu.Unlock()
	a.audit.Eventf("admin.test_mail", remoteIP(r), "", "", "")
	a.audit.Eventf("mail.sent", remoteIP(r), "", "", "测试邮件")
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "测试邮件已发送",
	})
}

// handleAdminClients GET /api/admin/clients（X-Admin-Token）列出全部应用（不限 owner）。
func (a *API) handleAdminClients(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	cs, err := a.store.ListClients()
	if err != nil || cs == nil {
		writeError(w, http.StatusInternalServerError, "读取应用列表失败")
		return
	}
	// 按 created_at 倒序（最新在前）。
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].CreatedAt > cs[j].CreatedAt })
	clients := make([]map[string]any, 0, len(cs))
	for i := range cs {
		clients = append(clients, clientListJSON(cs[i]))
	}
	a.audit.Eventf("admin.clients.view", remoteIP(r), "", "", fmt.Sprintf("list=%d", len(clients)))
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"clients": clients,
	})
}

// handleAdminSMTPGet GET /api/admin/smtp（X-Admin-Token）返回 SMTP 配置（密码脱敏）。
func (a *API) handleAdminSMTPGet(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	cfg, err := a.store.GetSMTPConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取 SMTP 配置失败")
		return
	}
	if cfg == nil {
		// 未保存过：返回环境变量默认值（脱敏）。
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"host":        a.cfg.SMTPHost,
			"port":        a.cfg.SMTPPort,
			"tls":         a.cfg.SMTPTLS,
			"user":        a.cfg.SMTPUser,
			"has_password": a.cfg.SMTPPass != "",
			"enabled":     a.cfg.SMTPHost != "",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"host":        cfg.Host,
		"port":        cfg.Port,
		"tls":         cfg.TLSMode,
		"user":        cfg.User,
		"has_password": cfg.PasswordCipher != "",
		"enabled":     cfg.Enabled,
	})
}

// handleAdminSMTPPost POST /api/admin/smtp（X-Admin-Token）保存 SMTP 配置并热更新 mailer。
func (a *API) handleAdminSMTPPost(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	var req struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		TLS      string `json:"tls"`
		User     string `json:"user"`
		Password string `json:"password"` // 空串 = 保留旧密码
		Enabled  bool   `json:"enabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "请求体格式错误")
		return
	}
	if req.TLS != "" && req.TLS != "starttls" && req.TLS != "ssl" && req.TLS != "none" {
		writeError(w, http.StatusUnprocessableEntity, "tls 必须为 starttls|ssl|none")
		return
	}
	if req.Port < 1 || req.Port > 65535 {
		writeError(w, http.StatusUnprocessableEntity, "port 必须在 1-65535")
		return
	}
	if req.TLS == "" {
		req.TLS = "starttls"
	}

	saved, plain, err := a.store.SaveSMTPConfig(req.Host, req.Port, req.TLS, req.User, req.Password, req.Enabled)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存 SMTP 配置失败")
		return
	}
	// 热更新 mailer（后续发信立即生效）。
	if a.mail != nil {
		a.mail.UpdateConfig(req.Host, req.Port, req.TLS, req.User, plain, req.Enabled)
	}
	a.audit.Eventf("admin.smtp.update", remoteIP(r), "", "", fmt.Sprintf("%s:%d enabled=%v", req.Host, req.Port, req.Enabled))
	writeJSON(w, http.StatusOK, map[string]any{
		"success":      true,
		"host":        saved.Host,
		"port":        saved.Port,
		"tls":         saved.TLSMode,
		"user":        saved.User,
		"has_password": saved.PasswordCipher != "",
		"enabled":     saved.Enabled,
	})
}

// userAdminJSON 用户管理列表用的 user 对象（含 grant_count）。
func (a *API) userAdminJSON(u store.User) map[string]any {
	grantCount, err := a.store.CountUserGrants(u.UserID)
	if err != nil {
		grantCount = 0
	}
	return map[string]any{
		"user_id":     u.UserID,
		"name":        u.Name,
		"first_seen":  u.FirstSeen,
		"last_seen":   u.LastSeen,
		"login_count": u.LoginCount,
		"grant_count": grantCount,
		"blacklisted": u.Blacklisted,
	}
}

// handleAdminUsers GET /api/admin/users（X-Admin-Token）返回用户列表（按 last_seen 倒序）。
func (a *API) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	us, err := a.store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取用户列表失败")
		return
	}
	sort.SliceStable(us, func(i, j int) bool { return us[i].LastSeen > us[j].LastSeen })
	users := make([]map[string]any, 0, len(us))
	today := time.Now().Format("2006-01-02")
	activeToday := 0
	for i := range us {
		users = append(users, a.userAdminJSON(us[i]))
		if t, err := time.Parse(time.RFC3339, us[i].LastSeen); err == nil && t.Format("2006-01-02") == today {
			activeToday++
		}
	}
	a.audit.Eventf("admin.users.view", remoteIP(r), "", "", fmt.Sprintf("list=%d", len(users)))
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"users":   users,
		"summary": map[string]any{
			"total":        len(users),
			"active_today": activeToday,
		},
	})
}

// handleAdminUserDetail GET /api/admin/users/{user_id}/detail 用户详情（实时拉 NS stats）。
func (a *API) handleAdminUserDetail(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	userID := r.PathValue("user_id")
	u, err := a.store.GetUser(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取用户失败")
		return
	}
	if u == nil {
		writeError(w, http.StatusNotFound, "用户不存在")
		return
	}
	// 实时拉取 NS stats；失败为 null 不阻塞。
	var statsPayload any
	if profile, err := a.ns.FetchUserProfile(userID); err == nil {
		statsPayload = map[string]any{
			"rank":      profile.Stats.Rank,
			"join_days": profile.Stats.JoinDays,
			"coin":      profile.Stats.Chicken,
			"topics":    profile.Stats.Topics,
			"comments":  profile.Stats.Comments,
		}
	}
	a.audit.Eventf("admin.user.detail", remoteIP(r), "", "", userID)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"user": map[string]any{
			"user_id":     u.UserID,
			"name":        u.Name,
			"first_seen":  u.FirstSeen,
			"last_seen":   u.LastSeen,
			"login_count": u.LoginCount,
			"blacklisted": u.Blacklisted,
		},
		"stats": statsPayload,
	})
}

// handleAdminUserStats GET /api/admin/users/{user_id}/stats 今日/累计统计（审计聚合）。
func (a *API) handleAdminUserStats(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	userID := r.PathValue("user_id")
	u, err := a.store.GetUser(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取用户失败")
		return
	}
	if u == nil {
		writeError(w, http.StatusNotFound, "用户不存在")
		return
	}
	evs, err := a.audit.ReadRecent(20000)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取审计失败")
		return
	}
	today := time.Now().Format("2006-01-02")
	var okToday, failToday, okTotal, failTotal, authOKTotal, authFailTotal int
	for _, e := range evs {
		if e.UserID != userID {
			continue
		}
		isToday := false
		if t, terr := time.Parse(time.RFC3339Nano, e.TS); terr == nil && t.Format("2006-01-02") == today {
			isToday = true
		}
		switch e.Event {
		case "login.confirm.ok":
			okTotal++
			if isToday {
				okToday++
			}
		case "login.confirm.fail":
			failTotal++
			if isToday {
				failToday++
			}
		case "authorize.code":
			authOKTotal++
		case "gate.block":
			authFailTotal++
		}
	}
	a.audit.Eventf("admin.user.stats", remoteIP(r), "", "", userID)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"stats": map[string]any{
			"login_ok_today":      okToday,
			"login_fail_today":    failToday,
			"login_ok_total":      okTotal,
			"login_fail_total":    failTotal,
			"authorize_ok_total":  authOKTotal,
			"authorize_fail_total": authFailTotal,
		},
	})
}

// handleAdminUserPatch PATCH /api/admin/users/{user_id} 设置黑名单（拉黑时吊销 token+授权）。
func (a *API) handleAdminUserPatch(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	userID := r.PathValue("user_id")
	var req struct {
		Blacklisted *bool `json:"blacklisted"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "请求体格式错误")
		return
	}
	if req.Blacklisted == nil {
		writeError(w, http.StatusUnprocessableEntity, "缺少 blacklisted 字段")
		return
	}
	u, err := a.store.SetUserBlacklisted(userID, *req.Blacklisted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "设置黑名单失败")
		return
	}
	if u == nil {
		writeError(w, http.StatusNotFound, "用户不存在")
		return
	}
	if *req.Blacklisted {
		// 拉黑只拦新：黑名单拦截由 checkUserBlocked 在 verify/confirm/authorize 入口 fail-closed 生效。
		// 已签发的 access_token 短 TTL 自然过期，不主动吊销，也不撤销已授权记录。
		a.audit.Eventf("user.blacklist", remoteIP(r), "", "", userID)
	} else {
		a.audit.Eventf("user.unblacklist", remoteIP(r), "", "", userID)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success":     true,
		"blacklisted": *req.Blacklisted,
	})
}

// handleAdminGrants GET /api/admin/grants 授权记录（过滤 user_id/client_id/status + 昵称 join）。
func (a *API) handleAdminGrants(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	q := r.URL.Query()
	userID := q.Get("user_id")
	clientID := q.Get("client_id")
	status := q.Get("status")

	gs, err := a.store.ListGrantsAdmin(userID, clientID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取授权记录失败")
		return
	}
	sort.SliceStable(gs, func(i, j int) bool { return gs[i].GrantedAt > gs[j].GrantedAt })

	grants := make([]map[string]any, 0, len(gs))
	for i := range gs {
		g := gs[i]
		userName := ""
		if u, err := a.store.GetUser(g.UserID); err == nil && u != nil {
			userName = u.Name
		}
		clientName := ""
		if c, err := a.store.GetClient(g.ClientID); err == nil && c != nil {
			clientName = c.ClientName
		}
		exCount, _ := a.store.CountTokenExchanges(g.UserID, g.ClientID)
		grants = append(grants, map[string]any{
			"user_id":              g.UserID,
			"user_name":            userName,
			"client_id":            g.ClientID,
			"client_name":          clientName,
			"scope":                "user",
			"created_at":           g.GrantedAt,
			"updated_at":           g.GrantedAt,
			"status":               g.Status,
			"revoked_at":           g.RevokedAt,
			"token_exchange_count": exCount,
		})
	}
	a.audit.Eventf("admin.grants.view", remoteIP(r), "", "", fmt.Sprintf("list=%d", len(grants)))
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"grants":  grants,
	})
}

// handleAdminResetSecret POST /api/admin/clients/{client_id}/reset-secret 重置应用 secret。
func (a *API) handleAdminResetSecret(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	clientID := r.PathValue("client_id")
	client, err := a.store.GetClient(clientID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取客户端失败")
		return
	}
	if client == nil {
		writeError(w, http.StatusNotFound, "客户端不存在")
		return
	}
	secret := randomHex(32, false)
	_, err = a.store.UpdateClient(clientID, func(c *store.Client) {
		c.ClientSecretHash = sha256Hex(secret)
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "重置 secret 失败")
		return
	}
	a.audit.Eventf("admin.client.reset_secret", remoteIP(r), "", clientID, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"client_id":     clientID,
		"client_secret": secret,
	})
}

// buildUsersCSV 构造用户列表 CSV（带 UTF-8 BOM）。
func buildUsersCSV(users []store.User, counts map[string]int) string {
	var b strings.Builder
	b.WriteString("\ufeff")
	b.WriteString("user_id,name,first_seen,last_seen,login_count,grant_count,blacklisted\n")
	for _, u := range users {
		count := counts[u.UserID]
		black := "false"
		if u.Blacklisted {
			black = "true"
		}
		b.WriteString(strings.Join([]string{
			u.UserID, u.Name, u.FirstSeen, u.LastSeen,
			strconv.Itoa(u.LoginCount), strconv.Itoa(count), black,
		}, ","))
		b.WriteString("\n")
	}
	return b.String()
}

// writeCSV 写 CSV 下载响应（text/csv + Content-Disposition attachment）。
func (a *API) writeCSV(w http.ResponseWriter, filename, content string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, content)
}

// handleAdminExportUsers GET /api/admin/export/users.csv 导出用户列表。
func (a *API) handleAdminExportUsers(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	us, err := a.store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取用户列表失败")
		return
	}
	sort.SliceStable(us, func(i, j int) bool { return us[i].LastSeen > us[j].LastSeen })
	counts := map[string]int{}
	for i := range us {
		if c, err := a.store.CountUserGrants(us[i].UserID); err == nil {
			counts[us[i].UserID] = c
		}
	}
	a.audit.Eventf("admin.export.users", remoteIP(r), "", "", fmt.Sprintf("rows=%d", len(us)))
	a.writeCSV(w, "users.csv", buildUsersCSV(us, counts))
}

// handleAdminExportGrants GET /api/admin/export/grants.csv 导出授权记录。
func (a *API) handleAdminExportGrants(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	q := r.URL.Query()
	gs, err := a.store.ListGrantsAdmin(q.Get("user_id"), q.Get("client_id"), q.Get("status"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取授权记录失败")
		return
	}
	sort.SliceStable(gs, func(i, j int) bool { return gs[i].GrantedAt > gs[j].GrantedAt })
	var b strings.Builder
	b.WriteString("\ufeff")
	b.WriteString("user_id,user_name,client_id,client_name,scope,created_at,status,revoked_at,token_exchange_count\n")
	for i := range gs {
		g := gs[i]
		userName := ""
		if u, err := a.store.GetUser(g.UserID); err == nil && u != nil {
			userName = u.Name
		}
		clientName := ""
		if c, err := a.store.GetClient(g.ClientID); err == nil && c != nil {
			clientName = c.ClientName
		}
		exCount, _ := a.store.CountTokenExchanges(g.UserID, g.ClientID)
		row := strings.Join([]string{
			g.UserID, userName, g.ClientID, clientName, "user",
			g.GrantedAt, g.Status, g.RevokedAt, strconv.Itoa(exCount),
		}, ",")
		b.WriteString(row)
		b.WriteString("\n")
	}
	a.audit.Eventf("admin.export.grants", remoteIP(r), "", "", fmt.Sprintf("rows=%d", len(gs)))
	a.writeCSV(w, "grants.csv", b.String())
}

// handleAdminStats GET /api/admin/stats（X-Admin-Token）返回进程内计数快照与 Reset 时间。// handleAdminStats GET /api/admin/stats（X-Admin-Token）返回进程内计数快照与 Reset 时间。
func (a *API) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	snap := a.stats.Snapshot()
	a.audit.Eventf("admin.stats.view", remoteIP(r), "", "", "")
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"stats": map[string]any{
			"verifies":     snap.Verifies,
			"login_ok":     snap.LoginsOK,
			"login_fail":   snap.LoginsFail,
			"gate_block":   snap.GateBlocks,
			"cookie_alert": snap.CookieAlerts,
			"reset_at":     a.stats.SnapshotResetAt().UTC().Format(time.RFC3339),
		},
	})
}

// handleAdminAudit GET /api/admin/audit?limit=50（X-Admin-Token）返回最近审计事件。
func (a *API) handleAdminAudit(w http.ResponseWriter, r *http.Request) {
	if !a.checkAdmin(w, r) {
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 200 {
			limit = n
		}
	}
	events, err := a.audit.ReadRecent(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取审计日志失败")
		return
	}
	a.audit.Eventf("admin.audit.view", remoteIP(r), "", "", fmt.Sprintf("limit=%d returned=%d", limit, len(events)))
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"events":  events,
	})
}

// handleStatic 处理静态资源与 SPA 回退（/ 及非 API 路径）。
func (a *API) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p := r.URL.Path
	if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/oauth/") {
		writeError(w, http.StatusNotFound, "接口不存在")
		return
	}
	a.serveSPA(w, r)
}

// serveSPA 优先服务 ../web/dist/ 静态文件，回退 index.html，再回退占位 HTML。
func (a *API) serveSPA(w http.ResponseWriter, r *http.Request) {
	dist := a.cfg.WebDistDir
	if dist == "" || !dirExists(dist) {
		writeHTML(w, placeholderHTML)
		return
	}

	rel := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if rel == "" || rel == "." {
		rel = "index.html"
	}
	fsPath := filepath.Join(dist, filepath.FromSlash(rel))
	// 防止路径穿越到 dist 之外。
	if rp, err := filepath.Rel(dist, fsPath); err != nil || strings.HasPrefix(rp, "..") {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	if info, err := os.Stat(fsPath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, fsPath)
		return
	}

	indexPath := filepath.Join(dist, "index.html")
	if info, err := os.Stat(indexPath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, indexPath)
		return
	}
	writeHTML(w, placeholderHTML)
}

func writeHTML(w http.ResponseWriter, html string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, html)
}

func dirExists(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// ---- CORS ----

func (a *API) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && a.isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Admin-Token")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) isAllowedOrigin(origin string) bool {
	for _, o := range a.cfg.AllowOrigins {
		if o == origin {
			return true
		}
	}
	return false
}
