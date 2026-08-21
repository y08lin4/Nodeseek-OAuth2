package api

import (
	"time"

	"nodeseek-oauth2/server/internal/ratelimit"
)

// RateLimits 持有各链路的内存滑动窗口限流器（SPEC §3.11，每分钟阈值）。
//
// 登录链路用 ip + uid 双键；应用链路用 cid + ip 双键；任一键超限即 429。
type RateLimits struct {
	verifyIP     *ratelimit.Limiter // /oauth/verify 的 ip 键：10/min
	verifyUID    *ratelimit.Limiter // /oauth/verify 的 uid 键：5/min
	confirmIP    *ratelimit.Limiter // /oauth/confirm 的 ip 键：20/min
	confirmUID   *ratelimit.Limiter // /oauth/confirm 的 uid 键：10/min
	authorizeCID *ratelimit.Limiter // /oauth/authorize 的 cid 键：20/min
	decisionCID  *ratelimit.Limiter // /oauth/authorize/decision 的 cid 键：30/min
	tokenCID     *ratelimit.Limiter // /oauth/token 的 cid 键：60/min
	appIP        *ratelimit.Limiter // 应用链路的 ip 键：SPEC 未给阈值，取 120/min 兜底（TODO）
	adminLoginIP *ratelimit.Limiter // /api/admin/login 的 ip 键：10/min（登录失败限流）
}

// NewRateLimits 创建全部限流器；disabled=true 时全部放行（NS_RATE_LIMIT_DISABLED=1）。
func NewRateLimits(disabled bool) *RateLimits {
	rl := &RateLimits{
		verifyIP:     ratelimit.New(10, time.Minute),
		verifyUID:    ratelimit.New(5, time.Minute),
		confirmIP:    ratelimit.New(20, time.Minute),
		confirmUID:   ratelimit.New(10, time.Minute),
		authorizeCID: ratelimit.New(20, time.Minute),
		decisionCID:  ratelimit.New(30, time.Minute),
		tokenCID:     ratelimit.New(60, time.Minute),
		appIP:        ratelimit.New(120, time.Minute),
		adminLoginIP: ratelimit.New(10, time.Minute),
	}
	if disabled {
		for _, l := range []*ratelimit.Limiter{
			rl.verifyIP, rl.verifyUID, rl.confirmIP, rl.confirmUID,
			rl.authorizeCID, rl.decisionCID, rl.tokenCID, rl.appIP, rl.adminLoginIP,
		} {
			l.SetDisabled(true)
		}
	}
	return rl
}
