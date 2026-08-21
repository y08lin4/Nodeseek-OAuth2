# 安全策略

## 报告漏洞

本项目涉及**身份认证与第三方授权**，安全至关重要。发现漏洞请通过以下渠道报告：

1. **GitHub Security Advisory**（推荐，可私密）：仓库首页 → `Security` → `Report a vulnerability`，填写影响与复现步骤。
2. **Issue**：仅在**不包含任何敏感信息**（真实 Cookie、密钥、令牌、账号 ID）的前提下可公开讨论一般性问题。

请**不要**在公开渠道（Issue/讨论/PR）中贴出：真实 NodeSeek Cookie、`NS_SECRET_KEY`、`NS_ADMIN_TOKEN`、客户端密钥等。

## 处理承诺

- 维护者会尽快（目标 72 小时内）确认并回复。
- 修复后发布补丁版本，并在 [CHANGELOG.md](CHANGELOG.md) 与 Release Notes 中披露（若涉及可利用的严重漏洞，按需协调公开时间）。

## 安全边界（项目定位）

- 本项目定位为**账号本人/自有服务**的授权服务，不是面向公众的 IdP。
- 系统账号 Cookie 属于最高敏感资产：仅加密存储于服务端，仅用于读取私信核验；扩展仅应通过 HTTPS 推送。
- 部署要求：反向代理强制 HTTPS、设置强 `NS_SECRET_KEY`/`NS_ADMIN_TOKEN`、及时轮换泄露的 Cookie。
