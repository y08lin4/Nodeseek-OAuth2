// Package config 负责解析服务端环境变量配置。
package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// DefaultDevSecret 未设置 NS_SECRET_KEY 时使用的固定开发密钥。
// 仅用于本地开发，严禁在生产环境使用（启动时会打印警告）。
const DefaultDevSecret = "nodeseek-oauth2-dev-secret-change-me"

// Config 汇总全部运行配置。
type Config struct {
	Port                  string   // 监听端口（PORT）
	SecretKey             string   // 原始密钥字符串（NS_SECRET_KEY），用于 AES-GCM 与 HMAC 派生
	AdminToken            string   // 管理接口令牌（NS_ADMIN_TOKEN），为空则管理接口一律 403
	MockMode              bool     // 是否跳过真实私信核验、用户信息拉取与创建应用等级门槛（NS_MOCK_MODE=1）
	AuthAccountID         string   // 系统账号 NS 数字 ID（NS_AUTH_ACCOUNT_ID）
	AuthAccountName       string   // 系统账号用户名（NS_AUTH_ACCOUNT_NAME）
	CookieAutoDetect      bool     // 推送 Cookie 时自动识别账号（NS_COOKIE_AUTO_DETECT，默认 1）
	NSAPIWhoamiURL        string   // 「探测本人」端点（NS_NS_API_WHOAMI_URL）
	NSBaseURL             string   // NodeSeek 站点基址（NS_NS_BASE_URL）
	NSAPIMessageURL       string   // 私信列表 API（NS_NS_API_MESSAGE_URL）
	NSAPIUserURL          string   // 用户信息 API（NS_NS_API_USER_URL），{user_id} 为占位符
	GateMinRank           int      // 授权门槛：最低等级（NS_GATE_MIN_RANK），0 关闭
	GateMinJoinDays       int      // 授权门槛：最低加入天数（NS_GATE_MIN_JOIN_DAYS），0 关闭
	MinClientCreationRank int      // 创建应用所需最低 NodeSeek 等级（NS_MIN_CLIENT_CREATION_RANK）
	SMTPHost              string   // SMTP 服务器（NS_SMTP_HOST），空 = 邮件功能禁用
	SMTPPort              int      // SMTP 端口（NS_SMTP_PORT）
	SMTPTLS               string   // SMTP TLS 模式：starttls|ssl|none（NS_SMTP_TLS）
	SMTPUser              string   // SMTP 用户名（NS_SMTP_USER）
	SMTPPass              string   // SMTP 密码/授权码（NS_SMTP_PASS）
	SMTPFrom              string   // 发件人地址（NS_SMTP_FROM）
	MailTo                []string // 收件人（NS_MAIL_TO，逗号分隔）
	ReportTime            string   // 每日日报时间 HH:MM（NS_REPORT_TIME）
	MailCooldownMin       int      // Cookie 失效告警最小间隔分钟（NS_MAIL_COOLDOWN_MIN）
	SessionTTLMin         int      // 会话有效期分钟（NS_SESSION_TTL_MIN）
	RateLimitDisabled     bool     // 是否关闭限流（NS_RATE_LIMIT_DISABLED=1）
	ReviewEmailNotify     bool     // 新应用提交时是否发送审核通知邮件（NS_REVIEW_EMAIL_NOTIFY=1）
	AllowOrigins          []string // CORS 允许来源（NS_ALLOW_ORIGIN，逗号分隔）
	DataDir               string   // JSON 存储目录
	WebDistDir            string   // 前端构建产物目录（SPA 静态资源）
}

// Load 从环境变量读取配置并应用默认值。
func Load() *Config {
	c := &Config{
		Port:                  getenv("PORT", "8080"),
		SecretKey:             os.Getenv("NS_SECRET_KEY"),
		AdminToken:            os.Getenv("NS_ADMIN_TOKEN"),
		MockMode:              getenv("NS_MOCK_MODE", "0") == "1",
		AuthAccountID:         getenv("NS_AUTH_ACCOUNT_ID", "9037"),
		AuthAccountName:       getenv("NS_AUTH_ACCOUNT_NAME", "idamie"),
		CookieAutoDetect:      getenv("NS_COOKIE_AUTO_DETECT", "1") == "1",
		NSAPIWhoamiURL:        getenv("NS_NS_API_WHOAMI_URL", "https://www.nodeseek.com/api/account/getInfo/{user_id}"),
		NSBaseURL:             getenv("NS_NS_BASE_URL", "https://www.nodeseek.com"),
		NSAPIMessageURL:       getenv("NS_NS_API_MESSAGE_URL", "https://www.nodeseek.com/api/notification/message/list"),
		NSAPIUserURL:          getenv("NS_NS_API_USER_URL", "https://www.nodeseek.com/api/account/getInfo/{user_id}"),
		GateMinRank:           getenvInt("NS_GATE_MIN_RANK", 0),
		GateMinJoinDays:       getenvInt("NS_GATE_MIN_JOIN_DAYS", 0),
		MinClientCreationRank: getenvInt("NS_MIN_CLIENT_CREATION_RANK", 6),
		SMTPHost:              os.Getenv("NS_SMTP_HOST"),
		SMTPPort:              getenvInt("NS_SMTP_PORT", 587),
		SMTPTLS:               getenv("NS_SMTP_TLS", "starttls"),
		SMTPUser:              os.Getenv("NS_SMTP_USER"),
		SMTPPass:              os.Getenv("NS_SMTP_PASS"),
		SMTPFrom:              os.Getenv("NS_SMTP_FROM"),
		MailTo:                splitCSV(os.Getenv("NS_MAIL_TO")),
		ReportTime:            getenv("NS_REPORT_TIME", "20:00"),
		MailCooldownMin:       getenvInt("NS_MAIL_COOLDOWN_MIN", 60),
		SessionTTLMin:         getenvInt("NS_SESSION_TTL_MIN", 10),
		RateLimitDisabled:     getenv("NS_RATE_LIMIT_DISABLED", "0") == "1",
		ReviewEmailNotify:     getenv("NS_REVIEW_EMAIL_NOTIFY", "0") == "1",
		AllowOrigins:          splitCSV(getenv("NS_ALLOW_ORIGIN", "http://localhost:5173")),
		DataDir:               "./data",
		WebDistDir:            "../web/dist",
	}
	if c.SecretKey == "" {
		c.SecretKey = DefaultDevSecret
		log.Println("【警告】未设置 NS_SECRET_KEY，正在使用固定开发密钥，严禁用于生产环境！")
	}
	return c
}

// getenv 读取环境变量，为空时返回默认值。
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getenvInt 读取整型环境变量，为空或解析失败时返回默认值。
func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return n
}

// splitCSV 将逗号分隔字符串拆分为去空白后的切片。
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
