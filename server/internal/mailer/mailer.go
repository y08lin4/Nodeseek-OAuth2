// Package mailer 提供 SMTP 邮件发送能力（支持 starttls / ssl / none 三种模式）。
package mailer

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
	"sync"
)

// Mailer SMTP 邮件发送器。
type Mailer struct {
	mu       sync.RWMutex
	Host     string
	Port     int
	TLSMode  string // starttls | ssl | none
	User     string
	Pass     string
	From     string
	To       []string
	MockMode bool
}

// New 创建发送器。
func New(host string, port int, tlsMode, user, pass, from string, to []string, mockMode bool) *Mailer {
	return &Mailer{
		Host:     host,
		Port:     port,
		TLSMode:  tlsMode,
		User:     user,
		Pass:     pass,
		From:     from,
		To:       to,
		MockMode: mockMode,
	}
}

// UpdateConfig 热更新 SMTP 配置（保存后立即生效，后台发信共用同一实例）。
func (m *Mailer) UpdateConfig(host string, port int, tlsMode, user, pass string, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if enabled {
		// enabled=true 时采用新配置；false 时清空 host 以禁用发送（Configured 返回 false）。
		m.Host = host
		m.Port = port
		m.TLSMode = tlsMode
		m.User = user
		m.Pass = pass
	} else {
		m.Host = ""
	}
}

// Configured 判断 SMTP 是否已配置（host 非空即视为已配置）。
func (m *Mailer) Configured() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return strings.TrimSpace(m.Host) != ""
}

// Summary 返回 SMTP 配置摘要（不含密码），供测试邮件正文使用。
func (m *Mailer) Summary() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return fmt.Sprintf("SMTP 配置摘要:\nHost: %s\nPort: %d\nTLS: %s\nUser: %s\nFrom: %s\nTo: %s\nMock: %v",
		m.Host, m.Port, m.TLSMode, m.User, m.From, strings.Join(m.To, ", "), m.MockMode)
}

// Send 发送邮件。
//
// mock 模式仅打印日志（前缀 [MAIL-MOCK]）；未配置时打印 [MAIL-DISABLED]；
// 已配置时走真实 SMTP。返回 error 表示真实发送失败（mock/未配置返回 nil）。
func (m *Mailer) Send(subject, body string) error {
	if m.MockMode {
		log.Printf("[MAIL-MOCK] 主题: %s", subject)
		log.Printf("[MAIL-MOCK] 正文: %s", body)
		return nil
	}
	m.mu.RLock()
	host, port, tlsMode, user, pass, from := m.Host, m.Port, m.TLSMode, m.User, m.Pass, m.From
	to := append([]string(nil), m.To...)
	m.mu.RUnlock()
	if strings.TrimSpace(host) == "" {
		log.Printf("[MAIL-DISABLED] 主题: %s", subject)
		return nil
	}
	return m.send(host, port, tlsMode, user, pass, from, to, subject, body)
}

// SendTo 发送邮件到指定收件人（单地址，用于应用通知邮件）。
func (m *Mailer) SendTo(to, subject, body string) error {
	if m.MockMode {
		log.Printf("[MAIL-MOCK] 收件人: %s 主题: %s", to, subject)
		log.Printf("[MAIL-MOCK] 正文: %s", body)
		return nil
	}
	m.mu.RLock()
	host, port, tlsMode, user, pass, from := m.Host, m.Port, m.TLSMode, m.User, m.Pass, m.From
	m.mu.RUnlock()
	if strings.TrimSpace(host) == "" {
		log.Printf("[MAIL-DISABLED] 主题: %s", subject)
		return nil
	}
	return m.send(host, port, tlsMode, user, pass, from, []string{to}, subject, body)
}

func (m *Mailer) send(host string, port int, tlsMode, user, pass, from string, to []string, subject, body string) error {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	msg := m.buildMessage(from, to, subject, body)

	switch tlsMode {
	case "ssl":
		// 隐式 TLS（通常 465 端口）
		return m.sendSSL(addr, host, user, pass, from, to, msg)
	case "none":
		// 明文（仅测试环境）
		return smtp.SendMail(addr, nil, from, to, msg)
	default:
		// starttls：smtp.SendMail 在提供 auth 时会自动协商 STARTTLS。
		return smtp.SendMail(addr, m.auth(user, pass, host), from, to, msg)
	}
}

// auth 构造 SMTP 认证（无用户名时返回 nil）。
func (m *Mailer) auth(user, pass, host string) smtp.Auth {
	if user == "" {
		return nil
	}
	return smtp.PlainAuth("", user, pass, host)
}

// sendSSL 走隐式 TLS 直连发送。
func (m *Mailer) sendSSL(addr, host, user, pass, from string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return err
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()

	if a := m.auth(user, pass, host); a != nil {
		if err := c.Auth(a); err != nil {
			return err
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, t := range to {
		if err := c.Rcpt(t); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// buildMessage 构造 RFC822 邮件消息。
func (m *Mailer) buildMessage(from string, to []string, subject, body string) []byte {
	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n",
		from, strings.Join(to, ", "), subject,
	)
	return []byte(headers + body)
}
