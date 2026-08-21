// Nodeseek 非官方 OAuth2 授权服务后端入口。
//
// 纯标准库实现：配置解析、JSON 文件存储、会话、私信核验、邮件通知与 HTTP 路由。
package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nodeseek-oauth2/server/internal/api"
	"nodeseek-oauth2/server/internal/audit"
	"nodeseek-oauth2/server/internal/auth"
	"nodeseek-oauth2/server/internal/config"
	"nodeseek-oauth2/server/internal/mailer"
	"nodeseek-oauth2/server/internal/nodeseek"
	"nodeseek-oauth2/server/internal/stats"
	"nodeseek-oauth2/server/internal/store"
)

func main() {
	cfg := config.Load()

	// 由 NS_SECRET_KEY 派生 AES-GCM 与 HMAC 共用的 32 字节密钥。
	key := auth.DeriveKey(cfg.SecretKey)

	st, err := store.New(cfg.DataDir, key, cfg.AuthAccountID, cfg.AuthAccountName)
	if err != nil {
		log.Fatalf("初始化存储失败: %v", err)
	}

	// 审计日志（data/audit.log）与限流器组。
	aud, err := audit.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("初始化审计日志失败: %v", err)
	}
	defer aud.Close()
	limits := api.NewRateLimits(cfg.RateLimitDisabled)

	ns := nodeseek.New(cfg.NSAPIMessageURL, cfg.NSAPIUserURL, cfg.NSAPIWhoamiURL, cfg.MockMode)
	mail := mailer.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPTLS, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom, cfg.MailTo, cfg.MockMode)
	cnt := stats.New()
	handler := api.NewRouter(cfg, st, ns, key, mail, cnt, aud, limits)

	// 每日日报调度（后台 goroutine，到点触发并自动排下一个）。
	startedAt := time.Now()
	go scheduleDailyReport(cfg, st, mail, cnt, aud, startedAt)

	// 应用错误率告警调度（每 10 分钟检查一次）。
	go scheduleErrorAlerts(st, mail, aud)

	addr := ":" + cfg.Port
	log.Printf("Nodeseek OAuth2 服务启动，监听 %s（mock_mode=%v）", addr, cfg.MockMode)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("服务退出: %v", err)
	}
}

// scheduleDailyReport 每日到 NS_REPORT_TIME 触发日报，发送后清零统计并排下一个。
func scheduleDailyReport(cfg *config.Config, st *store.Store, mail *mailer.Mailer, cnt *stats.Stats, aud *audit.Logger, startedAt time.Time) {
	for {
		next := nextReportTime(cfg.ReportTime)
		time.Sleep(time.Until(next))
		body := buildDailyReport(cfg, st, cnt, startedAt)
		if err := mail.Send("NSAuth2 系统日报", body); err != nil {
			log.Printf("日报邮件发送失败: %v", err)
		} else {
			aud.Eventf("mail.sent", "", "", "", "系统日报")
		}
		cnt.Reset()
	}
}

// scheduleErrorAlerts 每 10 分钟检查各应用近 1 小时授权失败率，超阈值且未达每日上限则发送告警邮件。
func scheduleErrorAlerts(st *store.Store, mail *mailer.Mailer, aud *audit.Logger) {
	tick := time.NewTicker(10 * time.Minute)
	defer tick.Stop()
	for range tick.C {
		if err := checkErrorAlerts(st, mail, aud); err != nil {
			log.Printf("错误率告警检查失败: %v", err)
		}
	}
}

// checkErrorAlerts 对每个 notify_enabled 且带 notify_email 的应用，聚合审计中近 1h 的
// 授权成功/失败事件（按 client_id），当 fail≥5 且 fail_rate≥30% 时发送「应用错误率告警」邮件。
// 每应用每天最多 1 封（记录在 client.LastErrorAlertAt，跨 day 后允许再次告警）。
func checkErrorAlerts(st *store.Store, mail *mailer.Mailer, aud *audit.Logger) error {
	clients, err := st.ListClients()
	if err != nil {
		return err
	}
	evs, err := aud.ReadRecent(20000)
	if err != nil {
		return err
	}
	// 近 1h 的窗口起点。
	windowStart := time.Now().Add(-time.Hour)
	// 按 client_id 聚合近 1h 的 ok / fail。
	type agg struct {
		ok   int
		fail int
	}
	byClient := map[string]*agg{}
	for _, e := range evs {
		if e.ClientID == "" {
			continue
		}
		t, terr := time.Parse(time.RFC3339Nano, e.TS)
		if terr != nil || t.Before(windowStart) {
			continue
		}
		switch e.Event {
		case "authorize.code", "token.exchange.ok":
			a := byClient[e.ClientID]
			if a == nil {
				a = &agg{}
				byClient[e.ClientID] = a
			}
			a.ok++
		case "gate.block", "token.exchange.fail":
			a := byClient[e.ClientID]
			if a == nil {
				a = &agg{}
				byClient[e.ClientID] = a
			}
			a.fail++
		}
	}

	today := time.Now().Format("2006-01-02")
	for i := range clients {
		c := &clients[i]
		if !c.NotifyEnabled || c.NotifyEmail == "" {
			continue
		}
		a := byClient[c.ClientID]
		if a == nil {
			continue
		}
		total := a.ok + a.fail
		if total == 0 || a.fail < 5 {
			continue
		}
		rate := float64(a.fail) / float64(total) * 100.0
		if rate < 30.0 {
			continue
		}
		// 每日去重：LastErrorAlertAt 同日则跳过。
		if c.LastErrorAlertAt != "" {
			if t, terr := time.Parse(time.RFC3339, c.LastErrorAlertAt); terr == nil && t.Format("2006-01-02") == today {
				continue
			}
		}
		subject := "【NSAuth2】应用错误率告警"
		body := fmt.Sprintf("应用「%s」最近 1 小时授权错误率升高。\n\n应用ID: %s\n近 1 小时失败次数: %d\n近 1 小时总请求: %d\n错误率: %.1f%%\n\n建议: 检查回调地址、密钥配置，或联系管理者排查。",
			c.ClientName, c.ClientID, a.fail, total, rate)
		var ev string
		if err := mail.SendTo(c.NotifyEmail, subject, body); err != nil {
			log.Printf("应用 %s 错误率告警邮件发送失败: %v", c.ClientID, err)
			ev = "mail.send_fail"
		} else {
			ev = "system.error_alert"
		}
		aud.Eventf(ev, "", "", c.ClientID, fmt.Sprintf("fail=%d rate=%.1f%%", a.fail, rate))
		// 记录告警时间以做每日去重。
		now := time.Now().UTC().Format(time.RFC3339)
		if _, uerr := st.UpdateClient(c.ClientID, func(u *store.Client) {
			u.LastErrorAlertAt = now
		}); uerr != nil {
			log.Printf("记录应用 %s 告警时间失败: %v", c.ClientID, uerr)
		}
	}
	return nil
}

// nextReportTime 计算下一个 NS_REPORT_TIME（本地时区 HH:MM）。
func nextReportTime(hhmm string) time.Time {
	now := time.Now()
	h, m, err := parseHHMM(hhmm)
	if err != nil {
		h, m = 20, 0
	}
	next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

// parseHHMM 解析 HH:MM。
func parseHHMM(s string) (int, int, error) {
	parts := strings.SplitN(strings.TrimSpace(s), ":", 2)
	if len(parts) != 2 {
		return 0, 0, errors.New("invalid HH:MM")
	}
	h, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, errors.New("invalid HH:MM")
	}
	return h, m, nil
}

// buildDailyReport 组装日报正文（纯文本）。
func buildDailyReport(cfg *config.Config, st *store.Store, cnt *stats.Stats, startedAt time.Time) string {
	snap := cnt.Snapshot()
	clients, _ := st.ListClients()

	now := time.Now()
	uptime := now.Sub(startedAt)
	days := int(uptime.Hours()) / 24
	hours := int(uptime.Hours()) % 24

	// 逐账号状态（正常/未设置/可能失效）。
	accounts, _ := st.ListAccounts()
	var acctLines []string
	for _, ac := range accounts {
		acctLines = append(acctLines, fmt.Sprintf("  - %s (%s): %s", ac.AccountName, ac.AccountID, accountState(ac)))
	}
	acctSummary := strings.Join(acctLines, "\n")
	if acctSummary == "" {
		acctSummary = "  （无账号）"
	}

	mockLine := "关"
	if cfg.MockMode {
		mockLine = "开"
	}

	return fmt.Sprintf(
		"NSAuth2 系统日报\n生成时间: %s\n运行时长: %d 天 %d 小时\n系统账号:\n%s\nMock 模式: %s\n已注册应用: %d\n本周期统计: 验证码生成 %d · 登录成功 %d · 登录失败 %d · 门槛拦截 %d · Cookie 告警 %d",
		now.Format("2006-01-02 15:04:05"),
		days, hours,
		acctSummary,
		mockLine,
		len(clients),
		snap.Verifies, snap.LoginsOK, snap.LoginsFail, snap.GateBlocks, snap.CookieAlerts,
	)
}

// accountState 返回账号状态（正常/未设置/可能失效，age>24h 启发式）。
func accountState(ac store.Account) string {
	if ac.CookieEncrypted == "" {
		return "未设置"
	}
	if t, err := time.Parse(time.RFC3339, ac.UpdatedAt); err == nil && time.Since(t).Hours() > 24 {
		return "可能失效"
	}
	return "正常"
}
