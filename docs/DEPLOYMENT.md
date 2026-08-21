# NSAuth2 (Nodeseek-OAuth2) 部署与运维指南

> 配套文件：`deploy/Dockerfile`、`deploy/docker-compose.yml`、`deploy/.env.example`、`deploy/Caddyfile.example`、`deploy/nginx.conf.example`。

## 1. 构建

```bash
# 本地
cd server && go build ./... && cd ..
cd web && npm install && npm run build && cd ..

# 或直接 Docker（一键）
cd deploy && docker compose up -d --build
```

## 2. 配置（deploy/.env.example → .env）

必填两项：

```bash
# 生成 NS_SECRET_KEY（32 字节 base64）
python -c "import secrets,base64;print(base64.b64encode(secrets.token_bytes(32)).decode())"

# 生成 NS_ADMIN_TOKEN（至少 16 位随机串）
python -c "import secrets;print(secrets.token_urlsafe(24))"
```

邮件通知（可选）：填写 `NS_SMTP_*` 与 `NS_MAIL_TO`（QQ 邮箱/163 等用授权码而非登录密码，TLS 选 `starttls` 或 `ssl`）。

## 3. ✅ NodeSeek API 校准（已完成实测，2026-08-21）

核心端点已用真实 Cookie 实测确认，环境变量默认值即正确值：

```bash
# 私信列表（系统账号收件箱）：GET，无需 CSRF 头
NS_NS_API_MESSAGE_URL=https://www.nodeseek.com/api/notification/message/list
# 响应：{"success":true,"msgArray":[{"sender_id","receiver_id","content","created_at","viewed","sender_name","receiver_name","max_id"}]}

# 用户信息：GET，{user_id} 替换为实际 ID
NS_NS_API_USER_URL=https://www.nodeseek.com/api/account/getInfo/{user_id}
# 响应：{"success":true,"detail":{"member_id","member_name","rank","coin","created_at","nPost","nComment","fans","follows","roles"}}
# 字段映射：等级=rank、鸡腿=coin、主题帖=nPost、评论=nComment、加入天数=created_at 推算
```

- 自动识别账号（`NS_COOKIE_AUTO_DETECT=1`）：**解析 pjwt Cookie**（base64 JWT 载荷含 `{id,name}`），再用 getInfo 校验；无需 whoami 专用端点。
- 实测样本：`37384`（萧炎）与 `9037`（idamie）均可用作对照数据。
- **请求头要求**（服务端已内置）：GET 带 `X-Requested-With: XMLHttpRequest` 可绕过 CF 对 API 的挑战；POST 类操作需 `x-csrf-challenge: simple-token`（本服务只用 GET，仅探测时用到）。
- **Cloudflare 注意**：数据中心 IP 可能被间歇性 JS 挑战（实测代理出口偶发 403），部署 VPS 若遇到「Just a moment」响应，说明出口 IP 被 CF 风控——建议换住宅/干净 IP 或通过受信代理出口。浏览器正常 IP 不受影响。

本地快速验证（可选）：

```bash
node tools/calibrate_ns.mjs "<你的NodeSeek_Cookie字符串>"
```
- 代码中对字段名做了宽松匹配（rank/level/grade/等级 等多候选、递归找键），但**端点 URL 必须校准**，否则核验与门槛功能不可用。校准后更新 `.env` 并重启。

### 3.1 上线前真实链路验证（推荐，本机即可）

先在本机用真实链路验证一遍，再上服务器：

```bash
cd server
$env:NS_SECRET_KEY = "<你的密钥>"
$env:NS_ADMIN_TOKEN = "<你的管理令牌>"
$env:NS_MOCK_MODE = "0"

# 关键：Go 不读 Windows 系统代理，必须显式设置代理环境变量才能访问 nodeseek.com
$env:HTTPS_PROXY = "http://127.0.0.1:7890"   # 你的 Clash 等代理地址；直连可达的机器可省略
$env:NO_PROXY = "127.0.0.1,localhost"
go run .
```

1. 打开 `http://localhost:8080/admin`，推送**真实系统账号 Cookie**（管理页操作或 `POST /api/admin/cookie`，服务端自动解析 pjwt 识别归属）
2. 打开登录向导，输入你的 **NodeSeek 小号 ID** → 拿到验证码
3. 用该小号把验证码**私信发给系统账号**
4. 回登录向导点「我已发送」→ 服务端真实读私信核验 → 登录成功
5. 走一遍注册应用（真实模式需等级 ≥6 才能创建）→ 授权 → 换取 token → userinfo

> ⚠️ 真实 Cookie 属于敏感凭据：测试完请登出/轮换；绝不提交进仓库（`.gitignore` 已覆盖）。

## 4. 扩展（NSAuth2 Cookie Keeper）部署

1. `cd extension && npm install && npm run build`
2. Chrome → `chrome://extensions` → 开发者模式 → 加载已解压的扩展 → 选 `extension/dist/`
3. 扩展设置中填：服务器地址（如 `https://ns.example.com`）、管理令牌（= `NS_ADMIN_TOKEN`）
4. **修改 `extension/manifest.json` 的 `host_permissions` 与 `content_scripts` 占位**：`https://ns.example.com/*` 改为你的真实域名（改完重新加载扩展）

## 5. 上线检查清单

```bash
# 安全头
curl -sI https://ns.example.com/api/config | grep -iE 'x-frame-options|x-content-type|strict-transport'

# 管理接口鉴权
curl -s https://ns.example.com/api/admin/status                  # 期望 403
curl -s https://ns.example.com/api/admin/status -H "X-Admin-Token: $NS_ADMIN_TOKEN"  # 期望 200

# 测试邮件（管理页「发送测试邮件」按钮或 curl）
curl -s -X POST https://ns.example.com/api/admin/test-mail -H "X-Admin-Token: $NS_ADMIN_TOKEN"

# 限流（1 分钟内连续 11 次 verify 应出现 429）
for i in $(seq 1 11); do curl -s -o /dev/null -w "%{http_code} " -X POST https://ns.example.com/oauth/verify -H 'Content-Type: application/json' -d '{"user_id":"12345"}'; done; echo

# 审计日志
tail -f data/audit.log
```

## 6. 运维要点

- 数据目录 `data/`（clients/codes/settings/tokens/audit.log）**定期备份**；settings.json 中的系统 Cookie 为 AES-256-GCM 密文，密钥 = NS_SECRET_KEY，**密钥丢失 = Cookie 密文无法解密**，需扩展重新推送。
- 日报：`NS_REPORT_TIME`（默认 20:00）收系统状态邮件；Cookie 失效会收到告警邮件（冷却 60 分钟）。
- 升级：`git pull && docker compose up -d --build`；web/dist 变更随镜像重建。
- 反向代理必须设置 `X-Forwarded-Proto`（Caddy/Nginx 示例均已含），否则 HSTS 不生效。
