// Package store 提供基于 JSON 文件的持久化存储（互斥锁 + 原子写）。
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"nodeseek-oauth2/server/internal/auth"
)

// Client OAuth2 客户端（data/clients.json）。
type Client struct {
	ClientID         string      `json:"client_id"`
	ClientSecretHash string      `json:"client_secret_hash"`
	ClientName       string      `json:"client_name"`
	OwnerUserID      string      `json:"owner_user_id"`
	HomepageURL      string      `json:"homepage_url"`
	Description      string      `json:"description"`
	RedirectURIs     []string    `json:"redirect_uris"`
	IconURL          string      `json:"icon_url"`
	MinRank          int         `json:"min_rank"`
	TokenTTL         int         `json:"token_ttl"`             // access_token 有效期（秒），默认 3600，范围 60-86400
	Status           string      `json:"status"`                // approved|pending_review|rejected|paused|pause_request|resume_request|delete_request
	PrevStatus       string      `json:"prev_status,omitempty"` // delete_request 前的状态（用于 reject 回退）
	Stats            ClientStats `json:"stats"`                 // 授权统计
	Builtin          bool        `json:"builtin"`               // 内置应用（不可被 owner 暂停/删除）
	Scopes           []string    `json:"scopes"`
	CreatedAt        string      `json:"created_at"`
}

// ClientStats 应用授权统计（data/clients.json 的 stats 字段）。
type ClientStats struct {
	AuthOKToday   int    `json:"auth_ok_today"`
	AuthFailToday int    `json:"auth_fail_today"`
	AuthOKTotal   int    `json:"auth_ok_total"`
	AuthFailTotal int    `json:"auth_fail_total"`
	StatsDate     string `json:"stats_date"` // 今日计数所属日期（YYYY-MM-DD），跨日清零
}

// Code 验证码与授权码共用（data/codes.json）。
type Code struct {
	Code        string `json:"code"`
	UserID      string `json:"user_id"`
	ClientID    string `json:"client_id,omitempty"`
	RedirectURI string `json:"redirect_uri,omitempty"`
	Scope       string `json:"scope,omitempty"` // 授权码记录的 scope（缺省 "user"）
	ExpiresAt   int64  `json:"expires_at"`
	Used        bool   `json:"used"`
}

// Token access_token（data/tokens.json）。
type Token struct {
	Token     string `json:"token"`
	UserID    string `json:"user_id"`
	ClientID  string `json:"client_id"`
	ExpiresAt int64  `json:"expires_at"`
}

// Grant 用户→应用授权记录（data/grants.json）。
type Grant struct {
	UserID    string `json:"user_id"`
	ClientID  string `json:"client_id"`
	GrantedAt string `json:"granted_at"`
	Status    string `json:"status"` // active|revoked
	RevokedAt string `json:"revoked_at,omitempty"`
}

// Account 系统账号（data/accounts.json）。
type Account struct {
	AccountID       string `json:"account_id"`
	AccountName     string `json:"account_name"`
	CookieEncrypted string `json:"cookie_encrypted"` // AES-GCM 密文（ciphertextB64.nonceB64）
	UpdatedAt       string `json:"updated_at"`
	Priority        int    `json:"priority"`
	Enabled         bool   `json:"enabled"`
	LastError       string `json:"last_error"`
	FailCount       int    `json:"fail_count"`
	AutoDetected    bool   `json:"auto_detected"`
}

// Store 线程安全的 JSON 文件存储。
type Store struct {
	mu                 sync.RWMutex
	dir                string
	key                [32]byte
	defaultAccountID   string
	defaultAccountName string
}

// New 创建存储实例，确保目录存在并预置骨架期种子数据。
// defaultAccountID/Name 用于首次启动时播种默认系统账号（NS_AUTH_ACCOUNT_ID/NAME）。
func New(dir string, key [32]byte, defaultAccountID, defaultAccountName string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, key: key, defaultAccountID: defaultAccountID, defaultAccountName: defaultAccountName}
	if err := s.seed(); err != nil {
		return nil, err
	}
	return s, nil
}

// seed 首次启动时预置示例客户端与空 codes 文件。
func (s *Store) seed() error {
	cp := filepath.Join(s.dir, "clients.json")
	if _, err := os.Stat(cp); errors.Is(err, os.ErrNotExist) {
		demo := []Client{
			{
				ClientID:         "demo-app",
				ClientSecretHash: "",
				ClientName:       "Demo App",
				OwnerUserID:      "1",
				HomepageURL:      "",
				Description:      "",
				RedirectURIs:     []string{"http://localhost:5173/callback"},
				IconURL:          "",
				MinRank:          0,
				TokenTTL:         3600,
				Status:           "approved",
				Stats:            ClientStats{},
				Builtin:          false,
				Scopes:           []string{},
				CreatedAt:        time.Now().UTC().Format(time.RFC3339),
			},
			{
				ClientID:         "nsauth2-web",
				ClientSecretHash: "",
				ClientName:       "本站 NSAuth2",
				OwnerUserID:      "0",
				HomepageURL:      "",
				Description:      "本站自身 OAuth2 登录（内置应用，自举）",
				RedirectURIs:     []string{},
				IconURL:          "",
				MinRank:          0,
				TokenTTL:         3600,
				Status:           "approved",
				Stats:            ClientStats{},
				Builtin:          true,
				Scopes:           []string{},
				CreatedAt:        time.Now().UTC().Format(time.RFC3339),
			},
		}
		if err := s.writeJSON("clients.json", demo); err != nil {
			return err
		}
	}
	cp = filepath.Join(s.dir, "codes.json")
	if _, err := os.Stat(cp); errors.Is(err, os.ErrNotExist) {
		if err := s.writeJSON("codes.json", []Code{}); err != nil {
			return err
		}
	}
	tp := filepath.Join(s.dir, "tokens.json")
	if _, err := os.Stat(tp); errors.Is(err, os.ErrNotExist) {
		if err := s.writeJSON("tokens.json", []Token{}); err != nil {
			return err
		}
	}
	gp := filepath.Join(s.dir, "grants.json")
	if _, err := os.Stat(gp); errors.Is(err, os.ErrNotExist) {
		if err := s.writeJSON("grants.json", []Grant{}); err != nil {
			return err
		}
	}
	// accounts.json 播种 + 旧 settings.json 迁移（详见 migrateLegacyCookie）。
	// 仅当配置了默认系统账号（NS_AUTH_ACCOUNT_ID）时才播种；未配置则保持空列表，
	// 由管理员通过 /api/admin/accounts 或推 Cookie 自行添加。
	ap := filepath.Join(s.dir, "accounts.json")
	if s.defaultAccountID != "" {
		if _, err := os.Stat(ap); errors.Is(err, os.ErrNotExist) {
			acc := Account{
				AccountID:    s.defaultAccountID,
				AccountName:  s.defaultAccountName,
				Priority:     0,
				Enabled:      true,
				AutoDetected: false,
			}
			if migrated, ts := s.migrateLegacyCookie(); migrated != "" {
				acc.CookieEncrypted = migrated
				acc.UpdatedAt = ts
			}
			if err := s.writeJSON("accounts.json", []Account{acc}); err != nil {
				return err
			}
		}
	}
	return nil
}

// migrateLegacyCookie 从旧 settings.json 读取 Cookie 密文并拼接为 accounts.json 的
// "ciphertextB64.nonceB64" 格式；无旧数据返回空串（seed 阶段无并发，直接无锁读）。
func (s *Store) migrateLegacyCookie() (blob, updatedAt string) {
	var legacy struct {
		SystemCookie *struct {
			CiphertextB64 string `json:"ciphertext_b64"`
			NonceB64      string `json:"nonce_b64"`
			UpdatedAt     string `json:"updated_at"`
		} `json:"system_cookie"`
	}
	if err := s.readJSON("settings.json", &legacy); err != nil {
		return "", ""
	}
	if legacy.SystemCookie == nil || legacy.SystemCookie.CiphertextB64 == "" {
		return "", ""
	}
	return legacy.SystemCookie.CiphertextB64 + "." + legacy.SystemCookie.NonceB64, legacy.SystemCookie.UpdatedAt
}

// ---- 内部无锁读写 ----

func (s *Store) readJSON(filename string, v any) error {
	data, err := os.ReadFile(filepath.Join(s.dir, filename))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (s *Store) writeJSON(filename string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(s.dir, filename), data)
}

// writeFileAtomic 先写临时文件再 rename，避免写入中断损坏数据。
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ---- 客户端 ----

func (s *Store) readClients() ([]Client, error) {
	var cs []Client
	if err := s.readJSON("clients.json", &cs); err != nil {
		return nil, err
	}
	return cs, nil
}

// ListClients 返回全部客户端。
func (s *Store) ListClients() ([]Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cs, err := s.readClients()
	if err != nil {
		return []Client{}, nil
	}
	return cs, nil
}

// GetClient 按 ID 查找客户端，不存在返回 (nil, nil)。
func (s *Store) GetClient(id string) (*Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cs, err := s.readClients()
	if err != nil {
		return nil, err
	}
	for i := range cs {
		if cs[i].ClientID == id {
			return &cs[i], nil
		}
	}
	return nil, nil
}

// AddClient 追加一个客户端。
func (s *Store) AddClient(c Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cs, err := s.readClients()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if errors.Is(err, os.ErrNotExist) {
		cs = []Client{}
	}
	cs = append(cs, c)
	return s.writeJSON("clients.json", cs)
}

// ClientNameExists 判断应用名是否已存在（大小写不敏感）。
func (s *Store) ClientNameExists(name string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cs, err := s.readClients()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	for i := range cs {
		if strings.EqualFold(cs[i].ClientName, name) {
			return true, nil
		}
	}
	return false, nil
}

// UpdateClient 查找并原地修改客户端，返回修改后的客户端；不存在返回 (nil, nil)。
func (s *Store) UpdateClient(id string, mutate func(*Client)) (*Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cs, err := s.readClients()
	if err != nil {
		return nil, err
	}
	for i := range cs {
		if cs[i].ClientID == id {
			mutate(&cs[i])
			if err := s.writeJSON("clients.json", cs); err != nil {
				return nil, err
			}
			return &cs[i], nil
		}
	}
	return nil, nil
}

// DeleteClient 删除指定 client_id 的客户端；不存在返回 (false, nil)。
func (s *Store) DeleteClient(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cs, err := s.readClients()
	if err != nil {
		return false, err
	}
	for i := range cs {
		if cs[i].ClientID == id {
			cs = append(cs[:i], cs[i+1:]...)
			if err := s.writeJSON("clients.json", cs); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

// ---- 验证码 / 授权码 ----

func (s *Store) readCodes() ([]Code, error) {
	var cs []Code
	if err := s.readJSON("codes.json", &cs); err != nil {
		return nil, err
	}
	return cs, nil
}

// AddCode 追加一条验证码或授权码。
func (s *Store) AddCode(c Code) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cs, err := s.readCodes()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if errors.Is(err, os.ErrNotExist) {
		cs = []Code{}
	}
	cs = append(cs, c)
	return s.writeJSON("codes.json", cs)
}

// GetCode 按码值查找，不存在返回 (nil, nil)。
func (s *Store) GetCode(code string) (*Code, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cs, err := s.readCodes()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	for i := range cs {
		if cs[i].Code == code {
			return &cs[i], nil
		}
	}
	return nil, nil
}

// MarkCodeUsed 将某条码标记为已使用（一次性）。
func (s *Store) MarkCodeUsed(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cs, err := s.readCodes()
	if err != nil {
		return err
	}
	for i := range cs {
		if cs[i].Code == code {
			cs[i].Used = true
			return s.writeJSON("codes.json", cs)
		}
	}
	return nil
}

// ---- access_token ----

func (s *Store) readTokens() ([]Token, error) {
	var ts []Token
	if err := s.readJSON("tokens.json", &ts); err != nil {
		return nil, err
	}
	return ts, nil
}

// AddToken 追加一条 access_token。
func (s *Store) AddToken(t Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts, err := s.readTokens()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if errors.Is(err, os.ErrNotExist) {
		ts = []Token{}
	}
	ts = append(ts, t)
	return s.writeJSON("tokens.json", ts)
}

// GetToken 按 token 值查找，不存在返回 (nil, nil)。
func (s *Store) GetToken(token string) (*Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ts, err := s.readTokens()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	for i := range ts {
		if ts[i].Token == token {
			return &ts[i], nil
		}
	}
	return nil, nil
}

// ---- 授权记录（grants）----

func (s *Store) readGrants() ([]Grant, error) {
	var gs []Grant
	if err := s.readJSON("grants.json", &gs); err != nil {
		return nil, err
	}
	return gs, nil
}

// AddGrant 追加一条授权记录。
func (s *Store) AddGrant(g Grant) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	gs, err := s.readGrants()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if errors.Is(err, os.ErrNotExist) {
		gs = []Grant{}
	}
	gs = append(gs, g)
	return s.writeJSON("grants.json", gs)
}

// GetGrants 返回某用户的全部授权记录。
func (s *Store) GetGrants(userID string) ([]Grant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	gs, err := s.readGrants()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Grant{}, nil
		}
		return nil, err
	}
	var out []Grant
	for i := range gs {
		if gs[i].UserID == userID {
			out = append(out, gs[i])
		}
	}
	return out, nil
}

// UpsertGrantActive 将某 user+client 的授权记录置为 active（不存在则新建）。
func (s *Store) UpsertGrantActive(userID, clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	gs, err := s.readGrants()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if errors.Is(err, os.ErrNotExist) {
		gs = []Grant{}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range gs {
		if gs[i].UserID == userID && gs[i].ClientID == clientID {
			gs[i].Status = "active"
			gs[i].GrantedAt = now
			gs[i].RevokedAt = ""
			return s.writeJSON("grants.json", gs)
		}
	}
	gs = append(gs, Grant{UserID: userID, ClientID: clientID, GrantedAt: now, Status: "active"})
	return s.writeJSON("grants.json", gs)
}

// RevokeGrant 将某 user+client 的授权记录置为 revoked（不存在则忽略）。
func (s *Store) RevokeGrant(userID, clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	gs, err := s.readGrants()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for i := range gs {
		if gs[i].UserID == userID && gs[i].ClientID == clientID {
			gs[i].Status = "revoked"
			gs[i].RevokedAt = time.Now().UTC().Format(time.RFC3339)
			return s.writeJSON("grants.json", gs)
		}
	}
	return nil
}

// DeleteTokensFor 删除某 user+client 的全部 access_token（撤销授权后即刻失效）。
func (s *Store) DeleteTokensFor(userID, clientID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts, err := s.readTokens()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	kept := make([]Token, 0, len(ts))
	for i := range ts {
		if ts[i].UserID == userID && ts[i].ClientID == clientID {
			continue
		}
		kept = append(kept, ts[i])
	}
	return s.writeJSON("tokens.json", kept)
}

// ---- 系统 Cookie ----

// SetSystemCookie 加密存储系统账号 Cookie，返回更新时间。
// ---- 系统账号（accounts）----

// encryptCookie 加密 Cookie，返回 "ciphertextB64.nonceB64"（base64 不含 "."，可安全分隔）。
func encryptCookie(key [32]byte, plaintext string) (string, error) {
	ct, nonce, err := auth.Encrypt(key, []byte(plaintext))
	if err != nil {
		return "", err
	}
	return ct + "." + nonce, nil
}

// decryptCookie 解密 "ciphertextB64.nonceB64"。
func decryptCookie(key [32]byte, blob string) (string, error) {
	parts := strings.SplitN(blob, ".", 2)
	if len(parts) != 2 || parts[0] == "" {
		return "", errors.New("Cookie 密文格式错误")
	}
	plain, err := auth.Decrypt(key, parts[0], parts[1])
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// readAccounts 无锁读取账号列表。
func (s *Store) readAccounts() ([]Account, error) {
	var as []Account
	if err := s.readJSON("accounts.json", &as); err != nil {
		return nil, err
	}
	return as, nil
}

// ListAccounts 返回全部账号（按 priority 升序，数值小者优先）。
func (s *Store) ListAccounts() ([]Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	as, err := s.readAccounts()
	if err != nil {
		return nil, err
	}
	sort.Slice(as, func(i, j int) bool { return as[i].Priority < as[j].Priority })
	return as, nil
}

// GetAccount 按 account_id 查找；不存在返回 (nil, nil)。
func (s *Store) GetAccount(id string) (*Account, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	as, err := s.readAccounts()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	for i := range as {
		if as[i].AccountID == id {
			return &as[i], nil
		}
	}
	return nil, nil
}

// AddAccount 新增账号。
func (s *Store) AddAccount(a Account) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	as, err := s.readAccounts()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if errors.Is(err, os.ErrNotExist) {
		as = []Account{}
	}
	as = append(as, a)
	return s.writeJSON("accounts.json", as)
}

// UpdateAccount 按 account_id 更新；不存在返回 (nil, nil)。
func (s *Store) UpdateAccount(id string, mutate func(*Account)) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	as, err := s.readAccounts()
	if err != nil {
		return nil, err
	}
	for i := range as {
		if as[i].AccountID == id {
			mutate(&as[i])
			if err := s.writeJSON("accounts.json", as); err != nil {
				return nil, err
			}
			return &as[i], nil
		}
	}
	return nil, nil
}

// DeleteAccount 按 account_id 删除，返回是否删除成功。
func (s *Store) DeleteAccount(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	as, err := s.readAccounts()
	if err != nil {
		return false, err
	}
	for i := range as {
		if as[i].AccountID == id {
			as = append(as[:i], as[i+1:]...)
			if err := s.writeJSON("accounts.json", as); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

// CountAccounts 返回账号总数。
func (s *Store) CountAccounts() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	as, err := s.readAccounts()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	return len(as), nil
}

// UpsertAccountCookie 按 account_id 更新 Cookie（含 account_name/updated_at，并清空错误）；不存在则新建（priority=最大值+1）。
func (s *Store) UpsertAccountCookie(accountID, accountName, plainCookie string, autoDetected bool) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	blob, err := encryptCookie(s.key, plainCookie)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	as, err := s.readAccounts()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if errors.Is(err, os.ErrNotExist) {
		as = []Account{}
	}
	for i := range as {
		if as[i].AccountID == accountID {
			as[i].CookieEncrypted = blob
			as[i].UpdatedAt = now
			as[i].LastError = ""
			as[i].FailCount = 0
			if accountName != "" {
				as[i].AccountName = accountName
			}
			if err := s.writeJSON("accounts.json", as); err != nil {
				return nil, err
			}
			return &as[i], nil
		}
	}
	acc := Account{
		AccountID:       accountID,
		AccountName:     accountName,
		CookieEncrypted: blob,
		UpdatedAt:       now,
		Priority:        nextPriority(as),
		Enabled:         true,
		AutoDetected:    autoDetected,
	}
	as = append(as, acc)
	if err := s.writeJSON("accounts.json", as); err != nil {
		return nil, err
	}
	return &acc, nil
}

// nextPriority 计算新账号的 priority（当前最大值 +1）。
func nextPriority(as []Account) int {
	max := 0
	for i := range as {
		if as[i].Priority > max {
			max = as[i].Priority
		}
	}
	return max + 1
}

// RecordAccountError 记录账号读信失败（last_error + fail_count++）。
func (s *Store) RecordAccountError(accountID, errMsg string) error {
	_, err := s.UpdateAccount(accountID, func(a *Account) {
		a.LastError = errMsg
		a.FailCount++
	})
	return err
}

// GetAccountCookie 解密并返回某账号的明文 Cookie；未设置返回 ("", nil)。
func (s *Store) GetAccountCookie(accountID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	as, err := s.readAccounts()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	for i := range as {
		if as[i].AccountID == accountID {
			if as[i].CookieEncrypted == "" {
				return "", nil
			}
			return decryptCookie(s.key, as[i].CookieEncrypted)
		}
	}
	return "", nil
}
