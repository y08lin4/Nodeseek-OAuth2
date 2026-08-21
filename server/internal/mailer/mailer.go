// Package mailer 提供 SMTP 邮件发送能力（支持 starttls / ssl / none 三种模式）。
package mailer

import (
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
)

// Mailer SMTP 邮件发送器。
type Mailer struct {
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

// Configured 判断 SMTP 是否已配置（host 非空即视为已配置）。
func (m *Mailer) Configured() bool {
	return strings.TrimSpace(m.Host) != ""
}

// Summary 返回 SMTP 配置摘要（不含密码），供测试邮件正文使用。
func (m *Mailer) Summary() string {
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
	if !m.Configured() {
		log.Printf("[MAIL-DISABLED] 主题: %s", subject)
		return nil
	}
	return m.send(subject, body)
}

func (m *Mailer) send(subject, body string) error {
	addr := net.JoinHostPort(m.Host, fmt.Sprintf("%d", m.Port))
	msg := m.buildMessage(subject, body)

	switch m.TLSMode {
	case "ssl":
		// 隐式 TLS（通常 465 端口）
		return m.sendSSL(addr, msg)
	case "none":
		// 明文（仅测试环境）
		return smtp.SendMail(addr, nil, m.From, m.To, msg)
	default:
		// starttls：smtp.SendMail 在提供 auth 时会自动协商 STARTTLS。
		// TODO: 无认证但需 STARTTLS 的服务器需显式协商，暂未处理。
		return smtp.SendMail(addr, m.auth(), m.From, m.To, msg)
	}
}

// auth 构造 SMTP 认证（无用户名时返回 nil）。
func (m *Mailer) auth() smtp.Auth {
	if m.User == "" {
		return nil
	}
	return smtp.PlainAuth("", m.User, m.Pass, m.Host)
}

// sendSSL 走隐式 TLS 直连发送。
func (m *Mailer) sendSSL(addr string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: m.Host})
	if err != nil {
		return err
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		return err
	}
	defer c.Close()

	if a := m.auth(); a != nil {
		if err := c.Auth(a); err != nil {
			return err
		}
	}
	if err := c.Mail(m.From); err != nil {
		return err
	}
	for _, to := range m.To {
		if err := c.Rcpt(to); err != nil {
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
func (m *Mailer) buildMessage(subject, body string) []byte {
	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n",
		m.From, strings.Join(m.To, ", "), subject,
	)
	return []byte(headers + body)
}
