// Package middleware 提供安全响应头中间件（SPEC §3.13）。
package middleware

import (
	"net/http"
	"strings"
)

// SecurityHeaders 为所有响应附加安全头：
// nosniff / DENY 防点击劫持 / no-referrer / CSP（允许 jsdelivr CDN 与内联脚本）；
// HTTPS（TLS 直连或 X-Forwarded-Proto: https）时附加 HSTS。
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; "+
				"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; "+
				"img-src 'self' data: https:; "+
				"connect-src 'self'")
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}
