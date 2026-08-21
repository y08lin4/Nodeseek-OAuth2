// Package ratelimit 提供内存滑动窗口限流（SPEC §3.11）。
// 键由调用方构造（如 "ip:1.2.3.4"、"uid:9037"、"cid:demo-app"），
// 同一键每分钟最多 limit 次；窗口按时间戳列表滑动，写时清理过期条目。
package ratelimit

import (
	"sync"
	"time"
)

// Limiter 单键限流器（每个键独立窗口）。
type Limiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	disabled bool
	hits    map[string][]time.Time
}

// New 创建限流器：limit 次 / window 时长。
func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		limit:  limit,
		window: window,
		hits:   make(map[string][]time.Time),
	}
}

// SetDisabled 全局关闭（NS_RATE_LIMIT_DISABLED=1）。
func (l *Limiter) SetDisabled(d bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.disabled = d
}

// Allow 记录一次访问并判断是否放行。
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.disabled {
		return true
	}
	now := time.Now()
	cutoff := now.Add(-l.window)
	// 清理该键过期条目
	ts := l.hits[key]
	i := 0
	for i < len(ts) && ts[i].Before(cutoff) {
		i++
	}
	ts = ts[i:]
	if len(ts) >= l.limit {
		l.hits[key] = ts
		return false
	}
	l.hits[key] = append(ts, now)
	return true
}
