// Package stats 提供进程内内存计数器（用于每日日报，跨日报周期清零）。
package stats

import (
	"sync"
	"time"
)

// Stats 内存计数器。
type Stats struct {
	mu           sync.Mutex
	verifies     int       // 验证码生成次数
	loginsOK     int       // 登录成功次数
	loginsFail   int       // 登录失败次数
	gateBlocks   int       // 授权门槛拦截次数
	cookieAlerts int       // Cookie 失效告警次数
	resetAt      time.Time // 最近一次 Reset 时间（New 时初始化为当前时间）
}

// Snapshot 计数快照。
type Snapshot struct {
	Verifies     int
	LoginsOK     int
	LoginsFail   int
	GateBlocks   int
	CookieAlerts int
}

// New 创建计数器。
func New() *Stats { return &Stats{resetAt: time.Now()} }

// IncVerify 验证码生成 +1。
func (s *Stats) IncVerify() { s.mu.Lock(); s.verifies++; s.mu.Unlock() }

// IncLoginOK 登录成功 +1。
func (s *Stats) IncLoginOK() { s.mu.Lock(); s.loginsOK++; s.mu.Unlock() }

// IncLoginFail 登录失败 +1。
func (s *Stats) IncLoginFail() { s.mu.Lock(); s.loginsFail++; s.mu.Unlock() }

// IncGateBlock 授权门槛拦截 +1。
func (s *Stats) IncGateBlock() { s.mu.Lock(); s.gateBlocks++; s.mu.Unlock() }

// IncCookieAlert Cookie 失效告警 +1。
func (s *Stats) IncCookieAlert() { s.mu.Lock(); s.cookieAlerts++; s.mu.Unlock() }

// Snapshot 返回当前计数快照（不重置）。
func (s *Stats) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Snapshot{
		Verifies:     s.verifies,
		LoginsOK:     s.loginsOK,
		LoginsFail:   s.loginsFail,
		GateBlocks:   s.gateBlocks,
		CookieAlerts: s.cookieAlerts,
	}
}

// Reset 清零（日报周期切换时调用）。
func (s *Stats) Reset() {
	s.mu.Lock()
	s.verifies, s.loginsOK, s.loginsFail, s.gateBlocks, s.cookieAlerts = 0, 0, 0, 0, 0
	s.resetAt = time.Now()
	s.mu.Unlock()
}

// SnapshotResetAt 返回最近一次 Reset 的 UTC 时间（RFC3339，New 时为进程启动时间）。
func (s *Stats) SnapshotResetAt() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resetAt
}
