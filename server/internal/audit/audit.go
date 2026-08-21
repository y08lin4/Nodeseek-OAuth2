// Package audit 提供审计日志：JSONL 追加写 data/audit.log。
// 同步写 + 互斥锁，失败仅记日志不阻塞请求（低流量场景足够）。
// TODO: 按大小轮转（如 50MB 后改名 audit.log.1）。
package audit

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event 审计事件行（SPEC §3.12）
type Event struct {
	TS       string `json:"ts"`
	Event    string `json:"event"`
	IP       string `json:"ip,omitempty"`
	UserID   string `json:"user_id,omitempty"`
	ClientID string `json:"client_id,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// Logger 审计日志写入器
type Logger struct {
	mu   sync.Mutex
	file *os.File
}

// New 打开（或创建）dataDir/audit.log 追加模式。
func New(dataDir string) (*Logger, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dataDir, "audit.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Logger{file: f}, nil
}

// Close 关闭文件。
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// Log 写入一行事件（JSON），失败仅打印日志。
func (l *Logger) Log(ev Event) {
	if l == nil {
		return
	}
	if ev.TS == "" {
		ev.TS = time.Now().Format(time.RFC3339Nano)
	}
	b, err := json.Marshal(ev)
	if err != nil {
		log.Printf("[audit] marshal error: %v", err)
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.file.Write(append(b, '\n')); err != nil {
		log.Printf("[audit] write error: %v", err)
	}
}

// Eventf 便捷方法：Event 名 + IP + 用户 + 应用 + 详情。
func (l *Logger) Eventf(name, ip, userID, clientID, detail string) {
	l.Log(Event{Event: name, IP: ip, UserID: userID, ClientID: clientID, Detail: detail})
}
