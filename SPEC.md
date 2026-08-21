# Nodeseek-OAuth2 规格书（SPEC）

> 三方开发契约：`server/`（Go 后端）、`web/`（Vue3 前端）、`extension/`（MV3 扩展）均以本文件为唯一事实来源。子代理只修改自己目录内的文件。

## 1. 项目定位

复刻 ns.idamie.com 的「Nodeseek OAuth2 授权服务」，三方角色：

1. **授权服务提供方**（本项目，OAuth2 服务端）：签发会话、发放 client 凭证、执行授权与门槛校验；
2. **需要授权服务的服务端**（第三方应用）：在授权服务里**注册应用**（应用名唯一、主页、描述、回调地址、图标、最低等级），拿到 `client_id`/`client_secret` 后接入 OAuth；
3. **使用授权服务的用户**（NodeSeek 账号持有者）：通过**私信验证码**确认账号归属后授权应用。

核心流程：
- 登录：输入 NS 数字 ID → 服务端生成验证码（10 分钟、一次性）→ 用户把验证码私信发给系统账号（`idamie`，NS ID `9037`）→ 服务端用系统账号 Cookie 读取私信核验 → 签发会话 Cookie。
- 授权：应用接入前用户查看应用名/主页/描述/图标并确认；授权前校验用户 NodeSeek 等级 / 加入天数门槛，不满足拒绝。
- 保活：浏览器扩展自动推送/更新系统账号 Cookie，防失效（系统 Cookie 失效 = 服务不可用）。
- 仓库：https://github.com/y08lin4/Nodeseek-OAuth2 （**公开仓库：严禁提交任何真实 Cookie、密钥、token**）

## 2. 仓库结构

```
Nodeseek-OAuth2/
├── SPEC.md            # 本文件（契约，只读）
├── server/            # Go 后端（纯标准库，零第三方依赖）
├── web/               # Vue3 + Vite + TS 前端 SPA
├── extension/         # MV3 TypeScript 浏览器扩展
├── README.md          # 由主代理最后编写（子代理不要动）
├── LICENSE            # 由主代理最后编写（子代理不要动）
└── .gitignore         # 由主代理最后编写（子代理不要动）
```

- 子代理不要执行任何 git 命令；只写自己的目录。
- 本机沙箱注意：Go 构建需设 `$env:GOCACHE="C:\Users\lin\Desktop\deepseek\.gocache"`、`$env:GOPATH="C:\Users\lin\Desktop\deepseek\.gopath"`、`$env:GOTELEMETRY="off"`；npm 需 `--cache "C:\Users\lin\Desktop\deepseek\.npm-cache"`。文件用 write/edit 工具创建，不用 PowerShell 重定向。

## 3. 后端规格（server/）

### 3.1 技术约束

- Go 1.22+，**仅标准库**（proxy.golang.org 不可达，禁止任何第三方依赖，包括 SQLite——存储用 JSON 文件）。
- 构建验证（在 server/ 目录）：设置上方 GOCACHE/GOPATH/GOTELEMETRY 后执行 `go build ./...` 必须成功。

### 3.2 配置（环境变量，带默认值）

| 变量 | 默认 | 说明 |
|---|---|---|
| `PORT` | `8080` | 监听端口 |
| `NS_SECRET_KEY` | （必设，未设用固定开发密钥并启动打印警告） | base64 编码的 32 字节密钥，用于 AES-GCM 与 HMAC 派生 |
| `NS_ADMIN_TOKEN` | 空 | 管理接口令牌（`X-Admin-Token` 头，常量时间比较）；未设置时管理接口返回 403 |
| `NS_MOCK_MODE` | `0` | `1` 时 confirm 跳过真实私信核验、用户信息返回固定样例、应用创建等级门槛放行（开发测试用） |
| `NS_AUTH_ACCOUNT_ID` | `9037` | 默认系统账号 NS ID（首次启动播种到 accounts.json） |
| `NS_AUTH_ACCOUNT_NAME` | `idamie` | 默认系统账号用户名（播种用） |
| `NS_COOKIE_AUTO_DETECT` | `1` | `1` 时推送 Cookie 自动识别账号：**优先解析 pjwt Cookie（base64 JWT 载荷 `{id,name}`）**，失败回退 NS_NS_API_WHOAMI_URL；`0` 时用手动绑定（请求须带 account_id） |
| `NS_NS_API_WHOAMI_URL` | `https://www.nodeseek.com/api/account/getInfo/{user_id}` | 探测本人备用端点（自动识别回退；user_id 由 pjwt 解析提供，pjwt 缺失 → 400 无法识别） |
| `NS_NS_BASE_URL` | `https://www.nodeseek.com` | NodeSeek 站点基址 |
| `NS_NS_API_MESSAGE_URL` | `https://www.nodeseek.com/api/notification/message/list` | 私信列表 API（**已实测校准 2026-08**：GET 返回 `{"success":true,"msgArray":[{"sender_id","receiver_id","content","created_at","viewed","sender_name","receiver_name","max_id"}]}`） |
| `NS_NS_API_USER_URL` | `https://www.nodeseek.com/api/account/getInfo/{user_id}` | 用户信息 API（**已实测校准 2026-08**：返回 `{"success":true,"detail":{"member_id","member_name","rank","coin","created_at","nPost","nComment","fans","follows","roles"}}`；等级=rank、鸡腿=coin、主题=nPost、评论=nComment、加入天数=created_at 推算；`{user_id}` 为占位符需替换） |
| `NS_GATE_MIN_RANK` | `0` | 全局授权门槛：最低等级，`0` 关闭（应用未显式设置 min_rank 时的兜底） |
| `NS_GATE_MIN_JOIN_DAYS` | `0` | 全局授权门槛：最低加入天数，`0` 关闭 |
| `NS_MIN_CLIENT_CREATION_RANK` | `6` | 创建应用所需的最低 NodeSeek 等级（原服务同款业务规则） |
| `NS_SMTP_HOST` | 空 | SMTP 服务器（空 = 邮件功能禁用，仅日志） |
| `NS_SMTP_PORT` | `587` | SMTP 端口 |
| `NS_SMTP_TLS` | `starttls` | `starttls`（587）/ `ssl`（465 隐式 TLS）/ `none` |
| `NS_SMTP_USER` | 空 | SMTP 用户名 |
| `NS_SMTP_PASS` | 空 | SMTP 密码/授权码 |
| `NS_SMTP_FROM` | 空 | 发件人地址 |
| `NS_MAIL_TO` | 空 | 收件人（逗号分隔多个） |
| `NS_REPORT_TIME` | `20:00` | 每日日报发送时间（本地时区 HH:MM） |
| `NS_MAIL_COOLDOWN_MIN` | `60` | Cookie 失效告警最小间隔（分钟），防刷屏 |
| `NS_REVIEW_EMAIL_NOTIFY` | `0` | `1` 时新应用提交（pending_review）发送审核通知邮件 |
| `NS_SESSION_TTL_MIN` | `10` | 授权会话有效期（分钟）；会话只服务授权流程，超时未授权即过期，不做「记住我」 |
| `NS_RATE_LIMIT_DISABLED` | `0` | `1` 时关闭限流（仅测试） |
| `NS_ALLOW_ORIGIN` | `http://localhost:5173` | CORS 允许来源（逗号分隔） |

### 3.3 API 契约

统一约定：成功 `{"success":true, ...}`；错误用对应 HTTP 状态码（400/401/403/404/422）+ `{"success":false,"message":"..."}`。

| 方法/路径 | 请求 | 成功响应 | 说明 |
|---|---|---|---|
| `GET /api/config` | — | `{"nodeseek":{"base_url","space_url_template":"{base}/space/{id}","message_url":"{base}/notification#/message?mode=talk&to={id}","auth_account_id","auth_account_username","accounts":[{"account_id","account_name","enabled","priority"}]},"business":{"min_client_creation_rank":6},"verification":{"code_expiry_seconds":600},"gate":{"min_rank":0,"min_join_days":0}}` | 前端配置（business.min_client_creation_rank 来自 NS_MIN_CLIENT_CREATION_RANK；gate 来自全局门槛环境变量；**accounts 为启用系统账号列表（按 priority 升序，不含 Cookie）**） |
| `POST /oauth/verify` | `{"user_id":"<纯数字>"}` | `{"success":true,"verification_code":"NS_AUTH_XXXXXXXX","expires_in":600,"accounts":[{"account_id","account_name","priority"}]}` | 不校验 ID 是否存在（与原服务一致）；user_id 非纯数字返回 422；**accounts = 启用系统账号（按 priority 升序），用户发给任一账号，confirm 按序轮询** |
| `POST /oauth/confirm` | `{"user_id":"<纯数字>","verification_code":"NS_AUTH_XXXX"}` | `{"success":true,"redirect_to":"/...","stats":{"rank":3,"join_days":360,"chicken":1494,"topics":86,"comments":1418}}` + Set-Cookie `ns_oauth_session`（HttpOnly; Path=/; SameSite=Lax） | 核验私信并顺带拉取用户信息（拉取失败时 `stats` 为 `null`，不阻塞登录）；系统 Cookie 缺失/失效时 message 须含 "Cookie" 字样（前端据此提示运维） |
| `GET /oauth/authorize?client_id&redirect_uri&response_type&scope&state` | — | 未登录：302 `/login?next=<当前完整URL>`；已登录：200 SPA HTML | 校验 client_id 存在且 redirect_uri 在白名单；**scope 仅支持 `user`**（缺省按 user；其他值 → 400 `{"error":"invalid_scope","error_description":"..."}`）；**state 可选、≤256 字符，随 code 存储并在重定向回显**；**status 非 approved → 403（按状态：审核中/未通过审核/已暂停/暂停申请处理中/删除申请处理中）**；否则 400/422 JSON |
| `POST /oauth/authorize/decision` | `{"approve":bool,"client_id","redirect_uri","response_type","state":可选}`（需会话） | approve：302 `redirect_uri?code=<32hex>&state=<原样>`；deny：302 `redirect_uri?error=access_denied&state=<原样>`（无 state 则不拼） | **服务端复检授权门槛（权威，fail-closed，按应用生效门槛）**，不满足 → 403 JSON 不重定向；**status 非 approved → 403（按状态文案）**；**统计：approve → 该应用 auth_ok_total+1、auth_ok_today+1；其余（拒绝/门槛拦截/状态拦截/错误）→ auth_fail_total+1、auth_fail_today+1（今日计数跨日自动清零）**；授权码写入 codes 存储（code、user_id、client_id、redirect_uri、scope、expires_at=10min、used=false，**在 /oauth/token 兑换成功后标记 used**）；approve 同时 upsert grants 记录（active） |
| `GET /api/oauth/client?client_id=` | —（需会话） | `{"success":true,"client":{"client_id","client_name","owner_user_id","homepage_url","description","redirect_uris":[...],"icon_url","min_rank","status","disabled":false},"stats":{"rank","join_days","chicken","topics","comments"},"gate":{"min_rank":0,"min_join_days":0,"ok":true}}` | 授权页展示用；**实时拉取用户信息并校验按应用生效的门槛**：门槛不满足 → 403 `{"success":false,"message":"等级不足：需要 ≥ 2，当前 1"}`（或「加入天数不足：需要 ≥ 30 天，当前 12 天」）；信息拉取失败 → 403 fail-closed「无法获取用户信息，请稍后重试」；**status 非 approved → 403（按状态文案）**；client 不存在 404 |
| `POST /api/client/register` | （需会话）`{"name":"应用名","homepage_url":"https://linux.do","description":"...","redirect_uris":["https://app.example.com/callback"],"icon_url":"https://example.com/logo.png","min_rank":2,"token_ttl":3600}` | `{"success":true,"client":{"client_id":"<32hex>","client_secret":"<32hex 仅此一次>","client_name","owner_user_id","homepage_url","description","redirect_uris":[...],"icon_url","min_rank","token_ttl":3600,"status":"pending_review","created_at"}}` | **创建应用（提交后不可编辑，走审核）**：应用名非空且**唯一（大小写不敏感）**；redirect_uris 至少 1 个且均为合法 http(s) URL；icon_url 可选（合法 URL 或空串）；**min_rank 为 0-6 整数（NodeSeek 最高 6 级，0=不限）**；**token_ttl 为 access_token 有效期（秒，默认 3600，范围 60-86400，应用自控）**；**创建人等级 ≥ NS_MIN_CLIENT_CREATION_RANK**（mock 模式放行；等级拉取失败 fail-closed 403「无法获取用户信息」）；client_secret 只明文返回一次，存储只存 SHA-256 哈希；**status=pending_review（mock 模式自动 approved）**；`NS_REVIEW_EMAIL_NOTIFY=1` 时异步发送审核通知邮件 |
| `GET /api/client/list` | —（需会话） | `{"success":true,"clients":[{"client_id","client_name","owner_user_id","homepage_url","description","redirect_uris":[...],"icon_url","min_rank","token_ttl","status","stats":{"auth_ok_today","auth_fail_today","auth_ok_total","auth_fail_total"},"created_at"}]}` | 当前用户的应用列表（不含 secret；**含授权统计：今日/累计成功失败**） |
| `PATCH /api/client/{client_id}` | Header `X-Admin-Token`（**管理端专用，前端不暴露**）；`{"disabled":bool,"token_ttl":可选,"status":可选}` | `{"success":true,"client":{...}}` | 管理端调整应用（应用方编辑一律走申请流程）；非管理端 403 |
| `DELETE /api/client/{client_id}` | Header `X-Admin-Token`（**管理端专用**） | `{"success":true}` | 管理端强制删除应用（应用方删除走 delete-request 审核） |
| `POST /api/client/{client_id}/pause` | （需会话，owner，approved 态） | `{"success":true,"status":"pause_request"}` | 申请暂停 → pause_request；非 owner 403 |
| `POST /api/client/{client_id}/resume` | （需会话，owner，paused 态） | `{"success":true,"status":"resume_request"}` | 申请恢复 → resume_request |
| `POST /api/client/{client_id}/delete-request` | （需会话，owner，approved/paused 态） | `{"success":true,"status":"delete_request"}` | 申请删除 → delete_request |
| `GET /api/admin/reviews` | Header `X-Admin-Token` | `{"success":true,"reviews":[{"type":"app\|pause\|resume\|delete","client_id","client_name","owner_user_id","detail":"","created_at"}]}` | 待审核队列（pending_review / pause_request / resume_request / delete_request 的应用） |
| `POST /api/admin/review` | Header `X-Admin-Token`；`{"type":"app\|pause\|resume\|delete","client_id","action":"approve\|reject","reason":可选}` | `{"success":true,"client":{...}}` | 审核：app approve→approved、reject→rejected；pause approve→paused、reject→approved；resume approve→approved、reject→paused；delete approve→**删除应用**、reject→回原状态；审计 review.approve / review.reject |
| `GET /api/grants` | —（需会话） | `{"success":true,"grants":[{"user_id","client_id","client_name","icon_url","min_rank","granted_at","status":"active\|revoked"}]}` | 我的授权列表（**用户可暂停自己给应用的授权**） |
| `POST /api/grants/{client_id}/revoke` | —（需会话） | `{"success":true}` | 撤销对该应用的授权：grant 标记 revoked + **删除该 user+client 的所有 tokens（已签发 token 即刻失效）**；审计 grant.revoke |
| `POST /api/logout` | —（需会话） | `{"success":true}` | 清除 `ns_oauth_session` 会话 |
| `GET /api/me` | —（需会话） | `{"success":true,"user_id":"<数字ID>"}` | 当前登录用户（导航/控制台登录态） |
| `POST /oauth/token` | form-encoded：`grant_type=authorization_code`、`code`、`client_id`、`client_secret`、`redirect_uri` | `{"access_token":"<32hex>","token_type":"Bearer","expires_in":3600,"scope":"user"}` | **授权码兑换 access_token**：校验 code 未用/未过期/client_id 匹配/redirect_uri 匹配/secret SHA-256 常量时间比较；成功后 code 标记 used（防重放）；access_token 写入 tokens.json（**有效期 = client.token_ttl，默认 3600s，范围 60-86400**）；**响应回显 code 记录的 scope**；错误按 OAuth2 规范 `{"error":"invalid_grant|invalid_client|invalid_request","error_description":"..."}`（400/401） |
| `GET /oauth/userinfo` | Header `Authorization: Bearer <access_token>` | `{"success":true,"user_id":"...","sub":"...","client_id":"...","stats":{"rank","join_days","chicken","topics","comments"}\|null}` | **access_token 消费端点（一次性身份授权）**：校验 token 有效未过期 → 返回绑定的 user_id/**sub（=user_id，OIDC 习惯别名）**/client_id + 尝试实时拉取 stats（失败 stats=null 不阻塞）；无效/过期 → 401 `{"success":false,"message":"token 无效或已过期"}`；审计 `userinfo.access` |
| `GET /healthz` | — | `{"status":"ok"}` | 探活端点（Docker healthcheck/负载均衡；无需鉴权，不含敏感信息） |
| `GET /.well-known/oauth-authorization-server` | — | `{"issuer":"<origin>","authorization_endpoint":"<origin>/oauth/authorize","token_endpoint":"<origin>/oauth/token","userinfo_endpoint":"<origin>/oauth/userinfo","response_types_supported":["code"],"grant_types_supported":["authorization_code"],"scopes_supported":["user"],"token_endpoint_auth_methods_supported":["client_secret_post"],"code_challenge_methods_supported":[]}` | **RFC 8414 元数据**（无需鉴权；origin 按请求动态生成）——第三方免文档自动发现端点 |
| `POST /api/admin/cookie` | Header `X-Admin-Token`；`{"cookie":"name=value; ...","account_id":可选}` | `{"success":true,"account_id":"...","account_name":"...","updated_at":"RFC3339"}` | 更新/新增系统账号 Cookie（**多账号**）：`NS_COOKIE_AUTO_DETECT=1`（默认）时服务端调 whoami 端点自动识别 Cookie 归属账号并归位（识别失败 → 400「无法识别 Cookie 对应账号」，`account_id` 参数忽略）；`NS_COOKIE_AUTO_DETECT=0` 时必须带 `account_id` 手动绑定；识别成功同时更新该账号的 account_name/updated_at、清空 last_error/fail_count；加密存储（AES-256-GCM） |
| `GET /api/admin/accounts` | Header `X-Admin-Token` | `{"success":true,"accounts":[{"account_id","account_name","priority","enabled","updated_at","last_error","fail_count","auto_detected"}]}` | 系统账号列表（不含 Cookie 明文） |
| `POST /api/admin/accounts` | Header `X-Admin-Token`；`{"account_id":"<纯数字>","account_name":"...","priority":0,"enabled":true}` | `{"success":true,"account":{...}}` | 手动新增系统账号（Cookie 由扩展推送或 /admin Cookie 表单按账号更新）；account_id 唯一 |
| `PATCH /api/admin/accounts/{account_id}` | Header `X-Admin-Token`；`{"priority":可选,"enabled":可选}` | `{"success":true,"account":{...}}` | 调整优先级/启用状态 |
| `DELETE /api/admin/accounts/{account_id}` | Header `X-Admin-Token` | `{"success":true}` | 删除系统账号（至少保留 1 个，否则 400） |
| `GET /api/admin/status` | Header `X-Admin-Token` | `{"success":true,"cookie":{"set":bool,"updated_at":str|"","age_seconds":int},"mock_mode":bool,"mail":{"configured":bool,"report_time":"20:00","last_test_at":str|""}}` | 管理页/扩展状态展示 |
| `POST /api/admin/test-mail` | Header `X-Admin-Token` | `{"success":true,"message":"测试邮件已发送"}` | 发送测试邮件（SMTP 未配置 → 400「SMTP 未配置」；mock 模式仅打日志并返回成功） |
| `GET /` 及静态资源 | — | SPA | 优先服务 `../web/dist/`（磁盘 FileServer），不存在时返回简单占位 HTML |

### 3.4 存储（data/ 目录 JSON 文件，互斥锁 + 原子写：写 tmp 再 rename）

- `data/clients.json`：`[{"client_id","client_secret_hash","client_name","owner_user_id","homepage_url","description","redirect_uris":[],"icon_url","min_rank","token_ttl":3600,"status":"approved|pending_review|rejected|paused|pause_request|resume_request|delete_request","stats":{"auth_ok_today":0,"auth_fail_today":0,"auth_ok_total":0,"auth_fail_total":0,"stats_date":""},"builtin":false,"scopes":[],"created_at"}]`
  - 预置示例：`demo-app`（name `Demo App`、redirect_uris `["http://localhost:5173/callback"]`、min_rank 0、token_ttl 3600、status approved、owner 示例用户）；**内置应用 `nsauth2-web`（name「本站 NSAuth2」、builtin:true、status approved、owner_user_id "0"、不可被暂停/删除，本站自身登录同样走本服务 OAuth，自举）**
- `data/codes.json`：`[{"code","user_id","client_id?","redirect_uri?","expires_at","used"}]`（验证码与授权码共用；授权码在 /oauth/token 兑换后标记 used）
- `data/tokens.json`：`[{"token","user_id","client_id","expires_at"}]`（access_token 32 位小写 hex，有效期 = client.token_ttl；**一次性身份授权，无 refresh_token**）
- `data/accounts.json`：`[{"account_id","account_name","cookie_encrypted","updated_at","priority":0,"enabled":true,"last_error":"","fail_count":0,"auto_detected":true}]` —— **多系统账号**；首次启动为空时用 `NS_AUTH_ACCOUNT_ID/NAME` 播种默认账号（enabled、priority 0）；Cookie 为 AES-256-GCM 密文；`confirm` 轮询失败或 whoami 识别失败时记 last_error/fail_count
- `data/grants.json`：`[{"user_id","client_id","granted_at","status":"active|revoked","revoked_at?"}]`（用户→应用授权记录；decision approve 时 upsert active；revoke 时标记 revoked 并删除该 user+client 全部 tokens）
- AES-256-GCM；密钥 = `SHA-256(NS_SECRET_KEY)` 派生（任意长度输入均可）。加密算法放 `internal/auth/crypto.go`。

### 3.5 会话

- Cookie 名 `ns_oauth_session`；载荷 `{"user_id","exp"}` JSON + HMAC-SHA256 签名（密钥同上派生，签名附在载荷后 base64url 编码）；有效期 **`NS_SESSION_TTL_MIN`（默认 10 分钟）**；HttpOnly、Path=/、SameSite=Lax。见 `internal/auth/session.go`。
- **设计原则：会话只覆盖授权流程（登录 → 同意授权），不做「记住我」；授权成功后的持续访问由应用侧 access_token 控制（时长 = 应用的 token_ttl）。**

### 3.6 验证码与授权码

- 验证码：格式 `NS_AUTH_` + 8 位大写 HEX（crypto/rand）；600s 过期；一次性（confirm 成功后标记 used）；与 user_id 绑定（不匹配返回 400）。
- 授权码：32 位小写 hex（crypto/rand）；600s 过期；一次性；绑定 user_id + client_id + redirect_uri。

### 3.7 NodeSeek 客户端（internal/nodeseek/client.go）

- `CheckCodeReceived(systemCookie, userID, code string) (bool, error)`：
  - `NS_MOCK_MODE=1` → 直接返回 `(true, nil)`；
  - 否则 GET `NS_NS_API_MESSAGE_URL`，Header `Cookie: <systemCookie>`；
  - 解析 JSON，找「发送者 == userID 且内容含 code」的消息；
  - 解析失败（空/非 JSON/登录跳转）→ 返回可读错误，错误文本包含「系统账号 Cookie 可能已失效」提示（对齐原服务文案）；
  - 真实字段名不确定：对常见字段名做宽松匹配（发送者：`sender_id`/`user_id`/`from_uid`；内容：`content`/`message`/`body`），留 TODO 注释「部署后按 NodeSeek 实际 API 校准」。
- `FetchUserStats(userID string) (UserStats, error)`：
  - `UserStats` 结构：`{Rank, JoinDays, Chicken, Topics, Comments int}`；
  - `NS_MOCK_MODE=1` → 返回固定样例 `{Rank:3, JoinDays:360, Chicken:1494, Topics:86, Comments:1418}`；
  - 否则 GET `NS_NS_API_USER_URL`（`{user_id}` 替换为实际 ID）；
  - 解析 JSON：等级字段候选 `rank`/`level`/`grade`/`等级`；加入天数 `join_days`/`joined_days`/`days`/`加入天数`；鸡腿 `chicken`/`chicken_count`/`鸡腿`；主题帖 `topics`/`topic_count`/`主题帖数`；评论 `comments`/`comment_count`/`评论数目`；对常见嵌套结构做宽松匹配（递归找键），留 TODO 注释「部署后按 NodeSeek 实际 API 校准」；
  - 解析失败返回可读错误。

### 3.8 授权门槛（Gate）

- 语义：配置的每个门槛都必须满足（AND）；`0` = 该门槛不启用。
- **按应用生效的等级门槛**：`app.min_rank`（0-6，0=不限）> 0 时以它为准（应用可显式选 0 级 = 不限）；`app.min_rank == 0` 且未设置（老数据）时回退全局 `NS_GATE_MIN_RANK`。
- **加入天数门槛**：仅全局 `NS_GATE_MIN_JOIN_DAYS`。
- 校验位置（两处，均服务端）：
  1. `GET /api/oauth/client`：实时拉取用户信息 → 按生效门槛校验 → 不满足返回 403（文案见 3.3）；
  2. `POST /oauth/authorize/decision`：复检（权威、fail-closed），防绕过。
- 门槛只卡 OAuth 授权，**不卡登录**。
- 信息拉取失败一律 fail-closed（403），不静默放行。

### 3.9 文件布局建议

```
server/
├── go.mod                    # module nodeseek-oauth2/server, go 1.22
├── main.go                   # 装配：配置、存储、路由、静态服务、日报调度
└── internal/
    ├── config/config.go      # 环境变量解析
    ├── store/store.go        # JSON 文件存储（clients/codes/settings/tokens）
    ├── auth/crypto.go        # AES-GCM + HMAC 密钥派生
    ├── auth/session.go       # 会话签发/校验
    ├── nodeseek/client.go    # 私信核验 + 用户信息拉取
    ├── mailer/mailer.go      # SMTP 邮件发送（net/smtp，starttls/ssl/none）
    ├── stats/stats.go        # 内存计数器（日报用：verify/confirm/门槛拦截等）
    ├── ratelimit/limiter.go  # 内存滑动窗口限流（IP/user_id/client_id 双键）
    ├── audit/audit.go        # 审计日志（JSONL 追加写 data/audit.log）
    ├── middleware/security.go# 安全头中间件（CSP/X-Frame-Options 等）
    └── api/router.go         # 路由 + handler（按 3.3 契约）
```

### 3.10 邮件通知（可选，SMTP 未配置时自动禁用）

- **发送器** `internal/mailer`：`Send(subject, body string) error`；`NS_SMTP_TLS` 支持 `starttls`（默认，EHLO 后 STARTTLS）/ `ssl`（tls.Dial 后直连）/ `none`（明文，仅测试）；`NS_MOCK_MODE=1` 时不真发，把主题+正文打印到日志（前缀 `[MAIL-MOCK]`）。
- **Cookie 失效告警**：`/oauth/confirm` 检测到系统 Cookie 缺失/失效（错误含 "Cookie"）时，触发异步告警邮件（主题如 `【NSAuth2】系统账号 Cookie 失效`，正文含错误原文与更新时间）；**冷却**：距上次告警 < `NS_MAIL_COOLDOWN_MIN` 分钟则跳过（防刷屏）；告警事件记入 stats。
- **每日日报**：`main.go` 启动时计算下一个 `NS_REPORT_TIME`（本地时区 HH:MM），到点触发并排下一个；内容（纯文本）：
  ```
  NSAuth2 系统日报
  生成时间: <本地时间>
  运行时长: <天/小时>
  系统 Cookie: 正常（最后更新 <time>，已 <age> 小时）/ 未设置 / 可能失效
  Mock 模式: 开/关
  已注册应用: N
  本周期统计: 验证码生成 X · 登录成功 Y · 登录失败 Z · 门槛拦截 W · Cookie 告警 C
  ```
  统计来自 `internal/stats`（内存计数，跨日报周期自动清零；TODO：后续持久化）；邮件失败记日志不阻塞服务。
- **`POST /api/admin/test-mail`**：发测试邮件（`NSAuth2 测试邮件` + SMTP 配置摘要），成功时间记录供 `/api/admin/status` 的 `mail.last_test_at` 展示。

### 3.11 限流（双重键，内存滑动窗口）

- `internal/ratelimit`：内存滑动窗口（`map[key][]time.Time` + 互斥锁，写时清理过期条目）。
- **双重键设计**：
  - 登录链路（`/oauth/verify`、`/oauth/confirm`）：键 = `ip:<remoteIP>` + `uid:<user_id>`（双键各自计数，任一超限即 429）；
  - 应用链路（`/oauth/authorize`、`/oauth/authorize/decision`、`/oauth/token`）：键 = `cid:<client_id>` + `ip:<remoteIP>`。
- 默认阈值（每分钟）：verify ip 10 / uid 5；confirm ip 20 / uid 10；authorize cid 20；decision cid 30；token cid 60。
- 超限响应：429 `{"success":false,"message":"请求过于频繁，请稍后再试"}`，并记审计 `rate.limit`。
- `NS_RATE_LIMIT_DISABLED=1` 时全部放行（仅测试）。

### 3.12 审计日志

- `internal/audit`：JSONL 追加写 `data/audit.log`（互斥锁同步写，失败仅记日志不阻塞请求；TODO：按大小轮转）。
- 行格式：`{"ts":"RFC3339Nano","event":"...","ip":"...","user_id":"...","client_id":"...","detail":"..."}`。
- 事件清单：`login.verify`、`login.confirm.ok`、`login.confirm.fail`、`gate.block`、`authorize.code`、`token.exchange.ok`、`token.exchange.fail`、`userinfo.access`、`client.register`、`client.pause_request`、`client.resume_request`、`client.delete_request`、`review.approve`、`review.reject`、`client.delete`、`grant.revoke`、`admin.cookie.update`、`admin.test_mail`、`mail.sent`、`mail.cookie_alert`、`rate.limit`。

### 3.13 安全头中间件

- `internal/middleware`：所有响应附加：`X-Content-Type-Options: nosniff`、`X-Frame-Options: DENY`（防授权页点击劫持）、`Referrer-Policy: no-referrer`、`Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; img-src 'self' data: https:; connect-src 'self'`；请求为 HTTPS（`r.TLS` 或 `X-Forwarded-Proto: https`）时附加 `Strict-Transport-Security: max-age=31536000`。

### 3.14 多系统账号（多 Cookie）

- **目标**：消除「系统账号 Cookie」单点故障，支持多账号冗余、优先级与故障转移。
- **存储**：`data/accounts.json`（见 §3.4），每个账号一条记录；首次启动空表时用 `NS_AUTH_ACCOUNT_ID/NAME` 播种默认账号。
- **自动识别**：`POST /api/admin/cookie` 默认走 `NS_COOKIE_AUTO_DETECT=1`——服务端**优先解析推送 Cookie 中的 pjwt（base64 JWT 载荷 `{id,name}`）**获得归属账号，再用 `NS_NS_API_WHOAMI_URL`（getInfo）校验确认；命中已有账号则更新其 Cookie，否则**自动新建账号记录**（account_name 取自探测结果，priority = 当前最大值 +1）；pjwt 缺失且校验失败 → 400「无法识别 Cookie 对应账号」。`NS_COOKIE_AUTO_DETECT=0` 时请求必须带 `account_id` 手动绑定。
- **优先级**：`priority` 数值小者优先；verify 返回按 priority 升序的启用账号列表；confirm 按 priority 升序轮询各账号私信直到命中。
- **故障转移**：confirm 轮询中某账号读私信失败（Cookie 失效/网络错误）→ 记该账号 `last_error`/`fail_count`、跳过并继续下一账号，**每个失败账号独立触发告警邮件（冷却 60min，不阻塞登录）**；全部账号失败才报错。
- **负载均衡**：用户可把验证码发给列表中的任意账号（前端账号 chips），自然分散；管理端可调整 priority 引导流量。
- **管理端**：账号 CRUD（§3.3），删除时至少保留 1 个；日报逐账号状态（正常/未设置/可能失效，age>24h 启发式）。
- 审计：`admin.cookie.update` 的 detail 记目标 account_id；新增 `admin.account.create/patch/delete`。

## 4. 前端规格（web/）

- Vite + Vue3 + TS（npm registry 可达，可安装依赖）。构建验证（web/ 目录）：`npm install --no-audit --no-fund --cache "C:\Users\lin\Desktop\deepseek\.npm-cache"` 后 `npm run build` 必须成功。
- 依赖：`vue`、`vue-router`、`vite`、`typescript`、`vue-tsc`（+ 必要插件）。不引入 UI 框架；样式手写（可引 Bootstrap 5 CDN 于 index.html，观感贴近原服务卡片风）。
- 路由（vue-router，history 模式）：
  - `/` 门户首页（公开）：项目介绍（是什么/三方角色/三步私信验证流程图/特性列表：审核制应用、多系统账号冗余、授权统计、等级门槛）+ CTA 按钮「登录 Nodeseek 账号」「接入文档」（/docs）「申请接入」（/console）；已登录时导航显示「面板」入口（/dashboard）。
  - `/docs` 接入文档页（公开）：完整第三方接入教程——① 注册应用（提交后待审核）② 审核通过后构造授权 URL：`/oauth/authorize?client_id=..&redirect_uri=..&response_type=code&scope=user&state=<随机串>` ③ 用户授权 ④ callback 收 `code` + **校验 state 原样回显** ⑤ `POST /oauth/token`（form：grant_type=authorization_code/code/client_id/client_secret/redirect_uri）换 access_token（响应含 `scope:"user"`）⑥ `GET /oauth/userinfo`（Bearer）拿 user_id/sub/等级 ⑦ 应用自签会话（token 为一次性身份授权，用完即弃）；含 curl 与 Node 代码示例、端点一览表、`.well-known/oauth-authorization-server` 提示。
  - `/dashboard` 登录后面板：欢迎语 + 用户 stats 卡片（等级/加入天数/鸡腿/主题帖/评论）+ 快捷入口卡片（我的应用 → /console、我的授权 → /grants、接入文档 → /docs）+ 我的应用统计摘要（GET /api/client/list 前 3 个应用的今日授权成功数）；未登录跳 `/login?next=/dashboard`。
  - `/login` 三步向导：① 输入 NS 数字 ID（校验纯数字）→ `POST /oauth/verify` ② 展示验证码 + 600s 倒计时 + **账号 chips**（verify 返回的 accounts 列表，默认高亮第一个，可点选；私信链接/「打开私信页」按选中账号生成 message_url）+ 「重新生成」③ 点「我已发送验证码」→ `POST /oauth/confirm` → 成功后跳 `redirect_to`（无则 `/`）。**第 3 步成功后展示 `stats`（等级/加入天数/鸡腿/主题帖/评论），并从 `/api/config` 的 `gate` 展示授权门槛提示（如「本服务授权门槛：等级 ≥ 2」）**。支持 URL 参数 `?next=` 登录后跳转。
  - `/authorize` 授权确认页：从 URL 读 `client_id/redirect_uri/response_type` → `GET /api/oauth/client` 展示应用信息（名称/主页链接/描述/图标）+ 当前用户 stats + 门槛状态 → **403（门槛未满足）时展示错误面板（显示 message 文案），不渲染同意按钮** → 通过则「同意」/「拒绝」→ `POST /oauth/authorize/decision`。
  - `/console` 应用管理页（需登录）：**注册表单**：应用名（唯一）、应用主页（如 https://linux.do）、应用描述、回调地址（一个或多个，逗号分隔，合法 http(s) URL）、应用图标 URL（可选）、最低等级（select：0级/1级/2级/3级/4级/5级/6级，默认 0 级 = 不限，NodeSeek 最高 6 级）、**access_token 有效期（分钟，默认 60，范围 1-1440，提交时转秒）**→ 提交前 **confirm 弹窗「提交后不可修改，确认提交？」** → `POST /api/client/register` → **成功后一次性展示 client_id + client_secret（带复制按钮，提示仅显示一次）**并提示「等待审核」；下方**我的应用列表** `GET /api/client/list`：每卡片展示 client_id/名称/主页/描述/回调地址/图标/最低等级/**状态徽章**（审核中/已通过/未通过/已暂停/申请处理中）/token 有效期/**统计行「今日 成功 X · 失败 Y ｜ 累计 成功 X · 失败 Y」**；按钮按状态呈现：approved →「申请暂停」「申请删除」；paused →「申请恢复」「申请删除」；申请类按钮点击后 confirm 并提示「已提交申请，等待审核」；**不提供编辑与直接删除**。页面顶部提示创建门槛（来自 `/api/config` 的 business.min_client_creation_rank，如「需 NodeSeek 等级 ≥ 6」）。
  - `/grants` 我的授权页（需登录）：`GET /api/grants` 展示**用户自己授权过的所有应用**（名称/图标/最低等级/授权时间/状态），每项「撤销授权」按钮（confirm）→ `POST /api/grants/{client_id}/revoke` → 提示「已撤销，该应用的访问令牌已作废」并刷新。
  - 全局导航：挂载时 `GET /api/me` 探测登录态 → 已登录显示「ID xxx · 退出登录」（`POST /api/logout` 后刷新）；未登录显示「登录」链接。导航含「我的授权」（/grants）入口。
  - `/login` 第 2 步（拿到验证码后）增加**「自动打开私信并填充验证码」按钮**：优先 `window.postMessage({type:'nsauth2-fill-pm', code, toUserId}, location.origin)`（由扩展 web-bridge.js 接收并转发，见 §5）；扩展缺失/失败时降级：`navigator.clipboard.writeText(code)` + `window.open(message_url)` 并提示「已复制验证码，请手动粘贴发送」。
  - `/admin` 管理页：输入并保存 Admin Token（localStorage，key `ns_admin_token`）；展示 `GET /api/admin/status`（Cookie 是否设置、更新时间、mock 模式、**邮件配置状态**：mail.configured / mail.report_time / mail.last_test_at）；表单更新 Cookie → `POST /api/admin/cookie`（**账号选择下拉：现有账号或「自动识别」，自动识别时服务端探测归属**）；**「发送测试邮件」按钮 → `POST /api/admin/test-mail`**（成功/失败提示 message）；**审核队列区块**：`GET /api/admin/reviews` 列出待审核项（类型/应用名/owner/详情），每项「通过」「拒绝」按钮 → `POST /api/admin/review`（提示 reason 可选输入）；**系统账号区块**：`GET /api/admin/accounts` 列表（账号/名称/优先级/启用开关/更新时间/最近错误），支持新增（POST）、调优先级与启停（PATCH）、删除（DELETE）；显示「新提交邮件通知」状态（来自 /api/config 或 status，如 NS_REVIEW_EMAIL_NOTIFY 状态提示）。
- `src/api.ts`：类型化 API 客户端（Config/VerifyResp/ConfirmResp/ClientInfo/RegisterResp/ClientList/AdminStatus 等类型），错误统一取 `data.message` 抛给组件展示。
- `vite.config.ts`：dev server proxy：`/api`、`/oauth` → `http://localhost:8080`。
- 页面标题「Nodeseek 非官方 OAuth2 授权服务」。组件用 `<script setup lang="ts">`。

## 5. 扩展规格（extension/）

- MV3 + TS。依赖仅 `typescript`（devDependency）。构建验证（extension/ 目录）：`npm install --no-audit --no-fund --cache "C:\Users\lin\Desktop\deepseek\.npm-cache"` 后 `npx tsc` 成功，且 `dist/` 内含 background.js 与 manifest.json（构建脚本把 manifest/popup.html/options.html 复制进 dist）。
- `manifest.json`：name `NSAuth2 Cookie Keeper`，version `0.1.0`，manifest_version 3，permissions `["cookies","storage","alarms","tabs"]`，host_permissions `["https://*.nodeseek.com/*","https://ns.example.com/*"]`（服务器源占位，README 提示改），background `service_worker: "background.js"`，action 默认 popup，options_page，content_scripts：`[{matches:["https://ns.example.com/*"],js:["web-bridge.js"],run_at:"document_start"},{matches:["https://*.nodeseek.com/*"],js:["pm-fill.js"],run_at:"document_idle"}]`（ns.example.com 同占位需改）。
- `src/web-bridge.ts`（编译为 web-bridge.js，注入授权服务页面）：监听 `window` 的 `message` 事件（`data.type === 'nsauth2-fill-pm'`，含 `code`/`toUserId`）→ `chrome.runtime.sendMessage({type:'fill-pm',...})` 转发后台 → 收到结果后 `window.postMessage({type:'nsauth2-fill-pm-result', ok:bool, message:string}, location.origin)` 回传网页。此桥不依赖 serverUrl 配置。
- `src/pm-fill.ts`（编译为 pm-fill.js，注入 nodeseek.com）：监听 `chrome.runtime.onMessage` 的 `fill-pm` → 轮询等待 SPA 私信输入框出现（候选选择器：`textarea`、`[contenteditable="true"]`、`.ql-editor`；TODO 注释：真实 DOM 结构部署后校准）→ 填入 code 并派发 `input` 事件（兼容 Vue 双向绑定）→ 高亮发送按钮（候选：包含「发送」文本的 button）→ 返回 `{ok:bool}`。
- `src/background.ts` 增 `onMessage fill-pm`：`chrome.tabs.query({url:'*://*.nodeseek.com/*'})` → 有则激活并 `tabs.sendMessage` 给该 tab；无则 `chrome.tabs.create({url:'<NS_NS_BASE_URL>/notification#/message?mode=talk&to=<toUserId>'})` 等加载后重试发送（最多 3 次，间隔 1s）；返回填充结果。
- `src/background.ts`：
  - 配置 `chrome.storage.sync`：`{slots:[{id,name,serverUrl,adminToken,intervalMin=30,targetAccountId:可选,enabled:true}], ...}` —— **多槽位**：每个浏览器 profile 只配自己的账号槽位；`pushCookie()` 对每个启用槽位各执行一次：`chrome.cookies.getAll({domain:'.nodeseek.com'})` → 组装 `name=value; ...` → `POST {serverUrl}/api/admin/cookie`，Header `X-Admin-Token`；`targetAccountId` 有值且服务端 `NS_COOKIE_AUTO_DETECT=0` 时请求带 `account_id`；结果按槽位记 `storage.local`（`slotResults:[{id,lastPushAt,lastResult}]`）；
  - 监听 `chrome.cookies.onChanged`（cookie.domain 含 `nodeseek` 时，debounce 10s 后推送全部槽位）；
  - `chrome.alarms` 周期推送（每槽位 intervalMin，取最小值调度，到点只推对应槽位）；
  - 推送失败简单退避重试（1min、5min）。
- `popup.html` + `src/popup.ts`：显示各槽位（name、serverUrl、上次推送时间/结果、成功/失败状态点），按钮「立即推送全部」「打开设置」。
- `options.html` + `src/options.ts`：**槽位列表管理**——新增/删除槽位，每槽位编辑 name/serverUrl/adminToken/intervalMin/targetAccountId（可选，注明「服务端自动识别开启时忽略」）/enabled，保存到 storage.sync。
- 骨架期不放图标（Chrome 用默认）。

## 6. 通用要求

- 代码注释用中文；不确定处留 `TODO` 注释。
- 严禁写入任何真实凭据/密钥。
- 回传格式（JSON）：`{"component":"server|web|extension","files":["相对路径",...],"build":"ok|fail(附错误摘要)","deviations":[{"what","why"}],"notes":["..."], "api_selfcheck":["按契约自检结果",...]}`

## 7. 部署与运维（deploy/ + docs/DEPLOYMENT.md）

- `deploy/Dockerfile`：多阶段构建（golang:1.22-alpine 编译 server → node:20 构建 web → 精简运行时镜像，非 root 用户，静态文件 + 二进制进镜像）。
- `deploy/docker-compose.yml`：单服务编排，`env_file: .env`，数据卷 `./data:/app/data`。
- `deploy/.env.example`：全量环境变量模板（含注释）。
- `deploy/Caddyfile.example` / `deploy/nginx.conf.example`：HTTPS 反代示例（安全头 + HSTS）。
- `docs/DEPLOYMENT.md`：部署步骤、**NodeSeek API 校准清单**（用系统账号 Cookie 实测私信/用户信息端点的真实 URL 与字段，校准 `NS_NS_API_MESSAGE_URL` / `NS_NS_API_USER_URL`）、扩展 host_permissions 修改指引、上线检查清单（test-mail、限流、安全头 curl 验证）。
