// Package nodeseek 封装与 NodeSeek 私信核验、用户信息、账号识别相关的 HTTP 客户端。
package nodeseek

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// browserUA 模拟浏览器 UA（配合 X-Requested-With 绕过 CF 对 API 的挑战）。
const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// Client NodeSeek HTTP 客户端。
type Client struct {
	APIMessageURL string       // 私信列表 API 端点
	APIUserURL    string       // 用户信息 API 端点（含 {user_id} 占位符）
	APIWhoamiURL  string       // 「探测本人」端点（含 {user_id} 占位符，自动识别回退）
	MockMode      bool         // 为 true 时跳过真实请求、返回固定样例
	HTTP          *http.Client // 底层 HTTP 客户端
}

// New 创建客户端。
func New(apiMessageURL, apiUserURL, apiWhoamiURL string, mockMode bool) *Client {
	return &Client{
		APIMessageURL: apiMessageURL,
		APIUserURL:    apiUserURL,
		APIWhoamiURL:  apiWhoamiURL,
		MockMode:      mockMode,
		HTTP:          &http.Client{Timeout: 15 * time.Second},
	}
}

// getJSON GET 请求 NodeSeek API 并返回响应体。
//
// 加 X-Requested-With: XMLHttpRequest 绕过 CF 对 API 的挑战；User-Agent 用浏览器 UA。
// POST 场景（当前暂无）需额外加 x-csrf-challenge: simple-token（NodeSeek 表单约定）。
func (c *Client) getJSON(url, cookie string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", browserUA)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("状态码 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// CheckCodeReceived 判断 userID 是否向系统账号发送了包含 code 的私信。
//
// NS_MOCK_MODE=1 时直接返回 (true, nil)。否则 GET 私信列表 API，解析 msgArray，
// 匹配 sender_id == userID 且 content 含 code（receiver_id 宽松忽略）。
func (c *Client) CheckCodeReceived(systemCookie, userID, code string) (bool, error) {
	if c.MockMode {
		return true, nil
	}
	if strings.TrimSpace(systemCookie) == "" {
		return false, errors.New("系统账号 Cookie 未设置，请先通过管理页或扩展更新 Cookie")
	}

	body, err := c.getJSON(c.APIMessageURL, systemCookie)
	if err != nil {
		return false, fmt.Errorf("系统账号 Cookie 可能已失效：请求私信列表失败 %w", err)
	}
	received, err := parseMessageList(body, userID, code)
	if err != nil {
		return false, fmt.Errorf("系统账号 Cookie 可能已失效：解析私信列表失败 %w", err)
	}
	return received, nil
}

// UserStats 用户信息统计（等级/加入天数/鸡腿/主题帖/评论数）。
type UserStats struct {
	Rank     int
	JoinDays int
	Chicken  int
	Topics   int
	Comments int
}

// FetchUserStats 拉取用户信息统计（getInfo）。
//
// NS_MOCK_MODE=1 时返回固定样例；否则 GET NS_NS_API_USER_URL（替换 {user_id} 占位符），
// 解析 {success,detail}：rank→rank、coin→chicken、nPost→topics、nComment→comments、
// join_days 由 created_at 推算（解析失败置 0）。success=false 或非 2xx → 错误（fail-closed）。
func (c *Client) FetchUserStats(userID string) (UserStats, error) {
	if c.MockMode {
		return UserStats{Rank: 3, JoinDays: 360, Chicken: 1494, Topics: 86, Comments: 1418}, nil
	}

	u := strings.ReplaceAll(c.APIUserURL, "{user_id}", userID)
	body, err := c.getJSON(u, "")
	if err != nil {
		return UserStats{}, fmt.Errorf("获取用户信息失败：请求 %w", err)
	}
	stats, err := parseUserStats(body)
	if err != nil {
		return UserStats{}, fmt.Errorf("获取用户信息失败：解析 %w", err)
	}
	return stats, nil
}

// WhoAmIResult 账号识别结果。
type WhoAmIResult struct {
	UserID   string
	Username string
}

// WhoAmI 从推送 Cookie 解析 pjwt 识别账号，并用 getInfo/{id} 校验。
//
// NS_MOCK_MODE=1 时返回固定样例 {user_id:"9037",username:"idamie"}。否则：
// 优先解析 Cookie 里的 pjwt（JWT 第二段 payload 的 base64url，取 {id,name}），
// pjwt 缺失/解析失败返回错误（调用方映射 400「无法识别 Cookie 对应账号」）；
// 有 id 后用 NS_NS_API_WHOAMI_URL（模板 getInfo/{user_id}，user_id 来自 pjwt）校验，
// member_name 以响应为准（空则回退 pjwt 的 name）。
func (c *Client) WhoAmI(cookie string) (WhoAmIResult, error) {
	if c.MockMode {
		return WhoAmIResult{UserID: "9037", Username: "idamie"}, nil
	}

	id, name, err := parsePjwt(cookie)
	if err != nil {
		return WhoAmIResult{}, err
	}

	u := strings.ReplaceAll(c.APIWhoamiURL, "{user_id}", id)
	body, err := c.getJSON(u, cookie)
	if err != nil {
		return WhoAmIResult{}, fmt.Errorf("探测账号失败：%w", err)
	}
	username, err := parseWhoamiName(body)
	if err != nil {
		return WhoAmIResult{}, fmt.Errorf("探测账号失败：%w", err)
	}
	if username == "" {
		username = name
	}
	return WhoAmIResult{UserID: id, Username: username}, nil
}

// ---- 纯解析函数（供单元测试喂真实样例） ----

// parseUserStats 解析 getInfo 响应 → UserStats。
func parseUserStats(body []byte) (UserStats, error) {
	var resp struct {
		Success bool `json:"success"`
		Detail  struct {
			Rank      int    `json:"rank"`
			Coin      int    `json:"coin"`
			CreatedAt string `json:"created_at"`
			NPost     int    `json:"nPost"`
			NComment  int    `json:"nComment"`
		} `json:"detail"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return UserStats{}, err
	}
	if !resp.Success {
		return UserStats{}, errors.New("success=false")
	}
	return UserStats{
		Rank:     resp.Detail.Rank,
		Chicken:  resp.Detail.Coin,
		Topics:   resp.Detail.NPost,
		Comments: resp.Detail.NComment,
		JoinDays: joinDaysFromCreatedAt(resp.Detail.CreatedAt),
	}, nil
}

// parseMessageList 解析私信列表响应，判断 userID 是否发送了 code。
func parseMessageList(body []byte, userID, code string) (bool, error) {
	var resp struct {
		Success  bool `json:"success"`
		MsgArray []struct {
			SenderID   int    `json:"sender_id"`
			ReceiverID int    `json:"receiver_id"`
			Content    string `json:"content"`
		} `json:"msgArray"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return false, err
	}
	if !resp.Success {
		return false, errors.New("success=false")
	}
	targetID, _ := strconv.Atoi(userID)
	for _, m := range resp.MsgArray {
		if m.SenderID == targetID && strings.Contains(m.Content, code) {
			return true, nil
		}
	}
	return false, nil
}

// parseWhoamiName 解析 getInfo 响应里的 member_name。
func parseWhoamiName(body []byte) (string, error) {
	var resp struct {
		Success bool `json:"success"`
		Detail  struct {
			MemberName string `json:"member_name"`
		} `json:"detail"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if !resp.Success {
		return "", errors.New("success=false")
	}
	return resp.Detail.MemberName, nil
}

// parsePjwt 从 Cookie 串提取 pjwt（JWT），取第二段 payload 解码 {id,name}。
func parsePjwt(cookie string) (id, name string, err error) {
	token := ""
	for _, part := range strings.Split(cookie, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && kv[0] == "pjwt" {
			token = kv[1]
			break
		}
	}
	if token == "" {
		return "", "", errors.New("无法识别 Cookie 对应账号（缺少 pjwt）")
	}
	segs := strings.Split(token, ".")
	if len(segs) < 2 {
		// 直接是 payload 段（无 header/signature）。
		return decodePjwtPayload(token)
	}
	return decodePjwtPayload(segs[1])
}

// decodePjwtPayload base64url 解码 pjwt 载荷段，取 {id,name}。
func decodePjwtPayload(payloadB64 string) (id, name string, err error) {
	raw, err := base64.URLEncoding.DecodeString(padBase64URL(payloadB64))
	if err != nil {
		return "", "", errors.New("无法识别 Cookie 对应账号（pjwt 载荷解码失败）")
	}
	var claims struct {
		ID   json.Number `json:"id"`
		Name string      `json:"name"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return "", "", errors.New("无法识别 Cookie 对应账号（pjwt 载荷 JSON 解析失败）")
	}
	id = claims.ID.String()
	if id == "" || id == "0" {
		return "", "", errors.New("无法识别 Cookie 对应账号（pjwt 缺少 id）")
	}
	return id, claims.Name, nil
}

// padBase64URL 补齐 base64url 的 padding（=）。
func padBase64URL(s string) string {
	switch len(s) % 4 {
	case 2:
		return s + "=="
	case 3:
		return s + "="
	}
	return s
}

// joinDaysFromCreatedAt 由 created_at（RFC3339，含毫秒）推算加入天数，解析失败返回 0。
func joinDaysFromCreatedAt(createdAt string) int {
	if createdAt == "" {
		return 0
	}
	var t time.Time
	var err error
	if t, err = time.Parse(time.RFC3339, createdAt); err != nil {
		if t, err = time.Parse(time.RFC3339Nano, createdAt); err != nil {
			return 0
		}
	}
	days := int(time.Since(t).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return days
}
