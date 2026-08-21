package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// SessionCookieName 会话 Cookie 名称。
const SessionCookieName = "ns_oauth_session"

// Session 会话载荷。
type Session struct {
	UserID string `json:"user_id"`
	Exp    int64  `json:"exp"` // 过期时间，Unix 秒
}

// SignSession 签发会话令牌。
//
// 格式：base64url(载荷JSON) + "." + base64url(HMAC-SHA256(派生密钥, 载荷JSON))。
func SignSession(key [32]byte, userID string, ttl time.Duration) (string, error) {
	payload := Session{UserID: userID, Exp: time.Now().Add(ttl).Unix()}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key[:])
	mac.Write(raw)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// VerifySession 校验会话令牌，成功返回载荷，失败返回 error。
func VerifySession(key [32]byte, token string) (*Session, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("会话格式非法")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("会话载荷解码失败")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("会话签名解码失败")
	}
	mac := hmac.New(sha256.New, key[:])
	mac.Write(raw)
	if !hmac.Equal(mac.Sum(nil), sig) {
		return nil, errors.New("会话签名校验失败")
	}
	var s Session
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, errors.New("会话载荷解析失败")
	}
	if s.UserID == "" {
		return nil, errors.New("会话缺少 user_id")
	}
	if time.Now().Unix() > s.Exp {
		return nil, errors.New("会话已过期")
	}
	return &s, nil
}
