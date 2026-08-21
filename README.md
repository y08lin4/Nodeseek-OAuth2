# NSAuth2 — NodeSeek 非官方 OAuth2 授权服务

> 为 NodeSeek 生态打造的身份授权服务：第三方应用通过 OAuth2 接入，用户用 **私信验证码** 确认身份（全程不碰密码），系统账号 Cookie 由浏览器扩展自动保活。

[![CI](https://github.com/y08lin4/Nodeseek-OAuth2/actions/workflows/ci.yml/badge.svg)](https://github.com/y08lin4/Nodeseek-OAuth2/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go&logoColor=white)](server/go.mod)
[![Vue](https://img.shields.io/badge/Vue-3-42b883?logo=vuedotjs&logoColor=white)](web/package.json)
[![Chrome MV3](https://img.shields.io/badge/Chrome-MV3-4285F4?logo=googlechrome&logoColor=white)](extension/manifest.json)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

## 项目状态 · v0.1（可部署测试）

| 环节 | 状态 | 说明 |
|---|---|---|
| 规格契约 | ✅ | `SPEC.md` 为唯一契约（接口/存储/安全/部署） |
| Go 后端 | ✅ | 纯标准库零依赖，11 轮开发+单元测试全绿 |
| Vue3 前端 | ✅ | 门户/登录向导/授权页/控制台/管理端/我的授权，构建通过 |
| Chrome 扩展 | ✅ | MV3 多槽位 Cookie 保活 + 私信填充桥，构建通过 |
| NodeSeek API | ✅ | **真实 Cookie 实测校准**（2026-08-21）：用户信息、私信列表、pjwt 识别、CF 请求头 |
| 集成验收 | ✅ | 三端构建 + mock 全流程冒烟 **21/21 全绿**（登录→授权→token→撤销→审核→多账号→限流→审计） |
| 真实模式端到端 | ⚠️ | 上线前按 [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) §3.1 验证真实私信链路 |
| 发布 | ✅ | v0.1 已推送 GitHub（public） |

## 为什么需要它

NodeSeek 没有官方 OAuth2。本项目让第三方应用安全接入 NodeSeek 身份：

- **私信验证码确认归属**：服务端生成一次性验证码，用户通过 NodeSeek 私信发给系统账号，服务端读私信核验——**不触碰密码**
- **OAuth2 标准流程**：授权码模式（state/scope/.well-known 元数据），授权即授权码，一次性防重放，令牌可随时撤销
- **门槛控制**：等级 0-6 + 加入天数，全局与应用级双重门槛（AND 逻辑、默认关闭=0、fail-closed）
- **应用审核制**：注册→审核→通过/拒绝，暂停/恢复/删除全走审核；提交后不可改
- **多系统账号冗余**：多 Cookie 自动识别（pjwt）归位、优先级轮询、单账号故障自动跳过 + 告警
- **Cookie 保活**：浏览器扩展定时推送 Cookie，杜绝"登录静默瘫痪"
- **本站自举**：本服务自己的登录也跑在自己的 OAuth 上（内置应用 `nsauth2-web`）

## 工作原理

```
┌────────────┐   推送系统 Cookie     ┌─────────────────────┐   读私信核验    ┌───────────────┐
│ 浏览器扩展   │ ───────────────────▶ │ Go 服务端 (server/)  │ ─────────────▶ │ nodeseek.com  │
│ Cookie 保活 │   (X-Admin-Token)    │ verify/confirm/     │  系统账号 Cookie │ (系统账号收信)  │
└────────────┘                      │ authorize + 管理 API │ ◀───────────── │               │
        ▲                           └─────────────────────┘                └───────────────┘
        │  用户浏览器 (web/ SPA：登录/授权/控制台/管理端)
        └────────────────────────────────────┘
```

**三步私信验证（登录）**：

```
① 输入 NS 数字 ID ──▶ ② 服务端生成一次性验证码（NS_AUTH_XXXXXXXX，10 分钟有效）
                          │
                          ▼
        用户把验证码私信发给系统账号（如 idamie/9037）
                          │
                          ▼
③ 服务端用系统账号 Cookie 读私信 ──▶ 核验通过 ──▶ 签发 10 分钟会话
```

**应用生命周期（审核制）**：

```
注册提交 ─▶ 审核中 ─审核─▶ 已通过 / 未通过
已通过 ─▶ 申请暂停/恢复/删除 ─审核─▶ 生效 / 驳回回原状态
```

## 快速开始：本地 mock 模式（5 分钟跑通全流程）

无需真实网络，离线体验完整业务。

```bash
# 1. 构建并启动后端（mock 模式：验证码自动核验，注册自动通过）
cd server
$env:NS_SECRET_KEY = "dGVzdC1rZXktMzJieXRlcy1iYXNlNjQta2V5IQ=="  # 32 字节 base64，随意
$env:NS_ADMIN_TOKEN = "dev-admin-token"
$env:NS_MOCK_MODE = "1"
go run .

# 2. （可选）构建前端，由后端直接托管 http://localhost:8080
cd ../web && npm install && npm run build
#    不构建也行：后端返回占位页，API 照常可用

# 3. 打开 http://localhost:8080
#    登录向导：输入任意数字 ID → 验证码自动通过（mock）
#    /console 注册应用 → 自动 approved → 拿到 client_id/secret
#    /admin 用 X-Admin-Token 体验审核队列与账号管理（mock 下注册即通过，队列为空属正常）
```

## 部署到服务器

```bash
cd deploy
cp .env.example .env          # 填 NS_SECRET_KEY / NS_ADMIN_TOKEN / 邮件配置
docker compose up -d --build  # 构建镜像（含前端）+ 启动 :8080
```

- 前置：反向代理 + HTTPS（`Caddyfile.example` / `nginx.conf.example`）、SMTP 邮件、扩展 `host_permissions` 域名替换
- **NodeSeek API 已实测校准**，环境变量默认值即正确值；数据中心 IP 可能被 Cloudflare 风控，详见
- 完整步骤见 [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)

## 仓库结构

```
├── SPEC.md        接口契约与规格（改接口先改它）
├── server/        Go 后端（纯标准库，零第三方依赖）
├── web/           Vue3 + Vite + TS 前端 SPA
├── extension/     Chrome MV3 扩展「NSAuth2 Cookie Keeper」
├── deploy/        Dockerfile / docker-compose / 反代示例 / .env 模板
├── docs/          部署与运维指南（含 NodeSeek API 校准记录）
└── tools/         calibrate_ns.mjs（部署前快速验证 API 连通性）
```

## API 一览

| 端点 | 说明 |
|---|---|
| `POST /oauth/verify` / `POST /oauth/confirm` | 私信验证码三步登录 |
| `GET /oauth/authorize` / `POST /oauth/authorize/decision` | OAuth2 授权入口 / 同意或拒绝（服务端复检门槛） |
| `POST /oauth/token` / `GET /oauth/userinfo` | 授权码换 token（一次性防重放）/ 消费身份（含 `sub`） |
| `GET /.well-known/oauth-authorization-server` | RFC 8414 服务发现元数据 |
| `POST /api/client/register` + `pause/resume/delete-request` | 应用注册（审核制）与生命周期申请 |
| `GET /api/grants` / `POST /api/grants/{id}/revoke` | 我的授权 / 撤销（令牌即刻作废） |
| `GET /api/admin/reviews` / `POST /api/admin/review` | 审核队列 / 审核处理 |
| `POST /api/admin/cookie` / `GET /api/admin/accounts` | 系统账号 Cookie 推送（多账号自动识别）/ 账号管理 |
| `GET /api/admin/status` / `POST /api/admin/test-mail` | 状态面板 / 测试邮件 |
| `GET /healthz` | 探活 |

完整契约见 [SPEC.md](SPEC.md)。

## 环境变量（要点）

| 变量 | 默认 | 说明 |
|---|---|---|
| `NS_SECRET_KEY` / `NS_ADMIN_TOKEN` | 无（必设） | 数据加密密钥 / 管理令牌 |
| `NS_MOCK_MODE` | `0` | `1` 跳过真实私信核验（开发用） |
| `NS_NS_API_MESSAGE_URL` / `NS_NS_API_USER_URL` | 已校准默认值 | NodeSeek 私信/用户信息 API（**已实测**） |
| `NS_COOKIE_AUTO_DETECT` | `1` | Cookie 自动识别账号（解析 pjwt） |
| `NS_GATE_MIN_RANK` / `NS_GATE_MIN_JOIN_DAYS` | `0` | 全局授权门槛（0=关闭；应用可各自设置） |
| `NS_MIN_CLIENT_CREATION_RANK` | `6` | 创建应用所需最低等级 |
| `NS_SMTP_*` / `NS_MAIL_TO` | 空 | 邮件（日报/告警/审核通知），留空禁用 |
| `HTTPS_PROXY` | — | **访问 nodeseek 的代理**（Go 不读系统代理，需显式设置） |

全部变量与默认值见 [SPEC.md](SPEC.md) §3.2。

## 安全设计

- 不触碰 NodeSeek 密码；验证码一次性、10 分钟过期、与 user_id 绑定
- 系统账号 Cookie AES-256-GCM 加密存储（密钥由 `NS_SECRET_KEY` 派生）
- 授权码一次性防重放、redirect_uri 白名单校验、`state` 回显
- 安全响应头（CSP / `X-Frame-Options: DENY` / HSTS）+ 双重限流（登录链路 IP+uid，应用链路 cid+IP）+ JSONL 审计日志
- **公开仓库警示**：严禁提交真实 Cookie/密钥/令牌；部署时替换扩展 `host_permissions` 占位域名

## 参与贡献

欢迎 Issue 与 PR！请先读 [CONTRIBUTING.md](CONTRIBUTING.md)（开发环境/测试/PR 流程），接口契约以 [SPEC.md](SPEC.md) 为准。安全漏洞请走 [SECURITY.md](SECURITY.md)，社区准则见 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)。

## 免责声明

本项目仅用于**账号本人 / 自有服务**的身份验证，禁止仿冒、钓鱼或未授权访问他人账号。使用者需遵守 NodeSeek 服务条款与当地法律法规。

## License

[MIT](LICENSE) © 2026 y08lin4
