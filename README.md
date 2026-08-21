# Nodeseek OAuth2 授权服务

基于 **Cookie 与私信验证**的 NodeSeek 账号 OAuth2 授权服务：第三方应用用 NodeSeek 账号登录时，通过**私信验证码**确认账号归属；配套浏览器扩展自动**保活系统账号 Cookie**，杜绝因 Cookie 过期导致的登录瘫痪。

## 工作原理

### 三方角色

1. **授权服务提供方**（本项目）：签发会话、审核应用、发放应用凭证、执行授权与门槛校验、统计授权数据；
2. **需要授权服务的服务端**（第三方应用）：在 `/console` **注册应用**（应用名唯一、主页、描述、回调地址、图标、最低等级 0-6、token 有效期），**提交后不可修改，需管理端审核通过后可用**（创建需 NodeSeek 等级 ≥ 6）；通过后可**申请暂停/恢复/删除**（同样走审核）；在 `/console` 查看**授权统计**（今日/累计成功失败）；
3. **使用授权服务的用户**（NodeSeek 账号持有者）：私信验证码确认账号归属后授权应用；在 `/grants`「我的授权」查看自己授权过哪些应用，**随时撤销（已签发令牌即刻作废）**。

> **本站自举**：预置内置应用 `nsauth2-web`（不可暂停/删除），本站自身登录同样运行在本服务的 OAuth 之上。

### 应用生命周期（审核制）

```
注册提交 ──▶ 审核中(pending_review) ──审核──▶ 已通过(approved) / 未通过(rejected)
已通过 ──申请暂停──▶ 审核 ──▶ 已暂停(paused) / 驳回回已通过
已暂停 ──申请恢复──▶ 审核 ──▶ 已通过 / 驳回回已暂停
已通过/已暂停 ──申请删除──▶ 审核 ──▶ 删除 / 驳回回原状态
```

- 提交后不可编辑；管理端在 `/admin` 审核队列处理（新提交可邮件通知，`NS_REVIEW_EMAIL_NOTIFY=1`）
- `NS_MOCK_MODE=1` 时注册自动通过（开发不卡审核）

### 三步私信验证（登录）

```
① 输入 NS 数字 ID ──▶ ② 服务端生成一次性验证码（NS_AUTH_XXXXXXXX，10 分钟有效）
                          │
                          ▼
        用户把验证码通过 NodeSeek 私信发给系统账号 idamie(9037)
                          │
                          ▼
③ 服务端用系统账号 Cookie 读取私信 ──▶ 核验通过 ──▶ 签发会话 Cookie
```

- **不触碰 NodeSeek 密码**：归属证明 = 「能把验证码私信发给系统账号」这一事实
- 验证码一次性、10 分钟过期、与 user_id 绑定
- 系统账号 Cookie **AES-256-GCM 加密存储**于服务端，仅用于读取私信核验

### Cookie 保活（浏览器扩展）

服务端唯一单点故障 = 系统账号 Cookie 过期。`extension/`（NSAuth2 Cookie Keeper）监听 nodeseek.com 的 Cookie 变化并定时推送至服务端 `/api/admin/cookie`，Cookie 失效前自动续上。

## 架构

```
┌────────────────┐   推送系统 Cookie    ┌──────────────────────┐   私信核验    ┌──────────────┐
│ 浏览器扩展       │ ─────────────────▶ │ Go 服务端 (server/)    │ ───────────▶ │ nodeseek.com  │
│ Cookie Keeper   │                     │ verify/confirm/       │  ◀─────────── │ (系统账号收信) │
│ (extension/)    │                     │ authorize + 管理 API   │   系统 Cookie │              │
└────────────────┘                     └──────────────────────┘               └──────────────┘
        ▲                                        │
        └──────────── 用户浏览器 ◀────────────────┘
                     (web/ SPA：登录向导/授权页/管理页)
```

## 仓库结构

```
├── SPEC.md        接口契约与规格（开发契约，改接口先改它）
├── server/        Go 后端（纯标准库，零第三方依赖）
├── web/           Vue3 + Vite + TS 前端 SPA
├── extension/     Chrome MV3 扩展「NSAuth2 Cookie Keeper」
├── deploy/        Dockerfile / docker-compose / 反代示例 / .env 模板
└── docs/          部署与运维指南（含 NodeSeek API 校准清单）
```

## 快速开始

### 服务端（server/）

```bash
cd server
go build ./...
# 运行（生产环境务必设置 NS_SECRET_KEY 与 NS_ADMIN_TOKEN）
$env:NS_SECRET_KEY="<base64 32字节>"   # PowerShell
$env:NS_ADMIN_TOKEN="<管理令牌>"
go run .
```

开发调试可加 `$env:NS_MOCK_MODE="1"`（跳过真实私信核验，离线跑通全流程）。

### 前端（web/）

```bash
cd web
npm install
npm run dev        # 开发：http://localhost:5173（已代理 /api、/oauth → :8080）
npm run build      # 产物 web/dist/ 由服务端直接托管
```

### 扩展（extension/）

```bash
cd extension
npm install
npm run build      # 产物 extension/dist/
```

Chrome 打开 `chrome://extensions` → 开发者模式 → 加载已解压的扩展 → 选 `extension/dist/`。
在扩展设置中填写：服务器地址（如 `https://ns.yourdomain.com`）、管理令牌（与 `NS_ADMIN_TOKEN` 一致）。**注意**：`manifest.json` 的 `host_permissions` 中 `ns.example.com` 是占位，部署时改成你的真实域名。

## 环境变量（server/）

| 变量 | 默认 | 说明 |
|---|---|---|
| `PORT` | `8080` | 监听端口 |
| `NS_SECRET_KEY` | 无（必设） | base64 的 32 字节密钥，派生 AES-GCM / HMAC 密钥 |
| `NS_ADMIN_TOKEN` | 无 | 管理接口令牌（`X-Admin-Token`），未设置则管理接口 403 |
| `NS_MOCK_MODE` | `0` | `1` 时跳过真实私信核验 |
| `NS_AUTH_ACCOUNT_ID` | `9037` | 系统账号 NS ID |
| `NS_AUTH_ACCOUNT_NAME` | `idamie` | 系统账号用户名 |
| `NS_NS_BASE_URL` | `https://www.nodeseek.com` | NodeSeek 站点基址 |
| `NS_NS_API_MESSAGE_URL` | `https://www.nodeseek.com/api/notification/message/list` | 私信列表 API（**已实测校准**） |
| `NS_NS_API_USER_URL` | `https://www.nodeseek.com/api/account/getInfo/{user_id}` | 用户信息 API（**已实测校准**；等级=rank/鸡腿=coin/主题=nPost/评论=nComment/加入天数=created_at 推算） |
| `NS_COOKIE_AUTO_DETECT` | `1` | `1` 时推送 Cookie 自动识别账号（**解析 pjwt Cookie**，失败回退 getInfo） |
| `NS_NS_API_WHOAMI_URL` | `https://www.nodeseek.com/api/account/getInfo/{user_id}` | 自动识别回退端点（user_id 由 pjwt 提供） |
| `NS_GATE_MIN_RANK` | `0` | 全局授权门槛：最低等级（`0` 关闭；应用可各自设置，见 /console） |
| `NS_GATE_MIN_JOIN_DAYS` | `0` | 全局授权门槛：最低加入天数（`0` 关闭） |
| `NS_MIN_CLIENT_CREATION_RANK` | `6` | 创建应用所需最低 NodeSeek 等级 |
| `NS_SESSION_TTL_MIN` | `10` | 授权会话有效期（分钟，会话只服务授权流程，不做「记住我」） |
| `NS_RATE_LIMIT_DISABLED` | `0` | `1` 时关闭限流（登录链路按 IP+user_id，应用链路按 client_id+IP 双重限流） |
| `NS_SMTP_HOST` / `NS_SMTP_PORT` / `NS_SMTP_TLS` / `NS_SMTP_USER` / `NS_SMTP_PASS` / `NS_SMTP_FROM` | 空 | SMTP 邮件配置（留空禁用邮件；QQ/163 用授权码） |
| `NS_MAIL_TO` | 空 | 日报 / 告警 / 审核通知收件人（逗号分隔） |
| `NS_REPORT_TIME` | `20:00` | 每日系统状态日报时间 |
| `NS_MAIL_COOLDOWN_MIN` | `60` | Cookie 失效告警最小间隔（分钟） |
| `NS_REVIEW_EMAIL_NOTIFY` | `0` | `1` 时新应用提交发送审核通知邮件 |
| `NS_ALLOW_ORIGIN` | `http://localhost:5173` | CORS 允许来源（逗号分隔） |

## API 一览

| 端点 | 说明 |
|---|---|
| `GET /api/config` | 前端配置（系统账号、验证码有效期等） |
| `POST /oauth/verify` | 生成私信验证码 |
| `POST /oauth/confirm` | 核验私信并签发会话 |
| `GET /oauth/authorize` | OAuth2 授权入口（需登录；应用状态非通过则 403） |
| `POST /oauth/authorize/decision` | 同意/拒绝授权（服务端复检门槛，记录授权统计） |
| `POST /oauth/token` | 授权码兑换 access_token（一次性防重放，有效期 = 应用 token_ttl） |
| `GET /oauth/userinfo` | access_token 消费端点（Bearer → 用户 ID/等级等身份信息） |
| `GET /api/oauth/client` | 应用信息 + 当前用户等级/加入天数 + 门槛状态（授权页展示） |
| `POST /api/client/register` | 注册第三方应用（提交后不可改，进入审核） |
| `GET /api/client/list` | 我的应用列表（含状态与授权统计） |
| `POST /api/client/{id}/pause` | 申请暂停（审核通过后生效） |
| `POST /api/client/{id}/resume` | 申请恢复 |
| `POST /api/client/{id}/delete-request` | 申请删除 |
| `GET /api/grants` | 我的授权列表（用户视角） |
| `POST /api/grants/{id}/revoke` | 撤销对某应用的授权（token 即刻作废） |
| `GET /api/admin/reviews` | 待审核队列（管理端） |
| `POST /api/admin/review` | 审核：通过/拒绝（管理端） |
| `POST /api/admin/cookie` | 更新系统账号 Cookie（扩展推送） |
| `POST /api/admin/test-mail` | 发送测试邮件（管理端） |
| `GET /api/admin/status` | 系统 Cookie / 邮件状态（扩展/管理页展示） |
| `GET /api/me` / `POST /api/logout` | 登录态 / 登出 |
| `GET /healthz` | 探活端点（Docker healthcheck） |

详细契约见 [SPEC.md](SPEC.md)。

## 安全说明

- 登录全程不经过 NodeSeek 密码；验证码由服务端生成、必须由账号本人私信发出
- 系统账号 Cookie 使用 AES-256-GCM 加密存储，密钥由 `NS_SECRET_KEY` 派生
- 管理接口 `X-Admin-Token` 常量时间比较鉴权；扩展仅应通过 HTTPS 推送
- 全站安全响应头（CSP / X-Frame-Options: DENY / nosniff / HSTS）、双重限流（登录链路 IP+user_id，应用链路 client_id+IP）、审计日志（`data/audit.log` JSONL）
- **公开仓库警示**：严禁提交任何真实 Cookie、密钥或令牌（`.gitignore` 已覆盖常见文件）；部署时修改 `host_permissions` 占位域名

## 部署

Docker 一键部署（`deploy/`），反代/邮件/校准清单见 [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)：**上线第一件事是按清单校准 NodeSeek 私信与用户信息 API 端点**（沙箱环境无法实测，环境变量中为占位）。

## 免责声明

本项目仅用于**账号本人 / 自有服务**的身份验证，禁止用于仿冒、钓鱼或任何未授权访问他人账号的行为。使用者需遵守 NodeSeek 服务条款与当地法律法规。

## License

[MIT](LICENSE) © 2026 y08lin4
