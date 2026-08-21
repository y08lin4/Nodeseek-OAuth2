# Changelog

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 与 [语义化版本](https://semver.org/lang/zh-CN/)。

## [0.1.0] - 2026-08-21

首个可部署版本。

### 新增

- **OAuth2 授权码流程**：`/oauth/authorize`（state/scope 支持）、`/oauth/authorize/decision`、`/oauth/token`（一次性防重放）、`/oauth/userinfo`（含 `sub`）、`/.well-known/oauth-authorization-server`（RFC 8414）
- **私信验证码三步登录**：`/oauth/verify` + `/oauth/confirm`，验证码一次性、10 分钟过期、与 user_id 绑定，全程不触碰 NodeSeek 密码
- **应用审核制生命周期**：注册 → pending_review → approved/rejected；暂停/恢复/删除均走审核；提交后不可修改；管理端审核队列 + 邮件通知
- **多系统账号冗余**：`/api/admin/accounts` CRUD、Cookie 自动识别（pjwt 解析优先，getInfo 回退）、优先级轮询、单账号故障自动跳过 + 独立告警冷却
- **授权管理**：`/api/grants` + 撤销（已签发令牌即刻作废）
- **门槛控制**：等级 0-6 + 加入天数，全局与应用级双重门槛（AND、fail-closed、0=关闭）
- **门户与站点**：首页介绍、/docs 接入文档、/dashboard 数据面板、/console 应用控制台、/admin 管理端、/grants 我的授权
- **本站自举**：内置应用 `nsauth2-web`，本站登录运行在自己的 OAuth 之上
- **浏览器扩展**（MV3）：多槽位 Cookie 保活推送、一键私信填充桥
- **运维能力**：每日状态日报、Cookie 失效告警邮件、双重限流、JSONL 审计日志、安全响应头、`/healthz`、Docker 部署套件
- **NodeSeek API 实测校准**（2026-08-21 真实 Cookie 验证）：`/api/account/getInfo/{user_id}`、`/api/notification/message/list`、pjwt 识别、CF 请求头约定

### 技术栈

- 后端：Go 1.22 纯标准库，零第三方依赖，JSON 文件存储（AES-256-GCM 加密 Cookie）
- 前端：Vue 3 + Vite + TypeScript
- 扩展：Chrome MV3 + TypeScript

[0.1.0]: https://github.com/y08lin4/Nodeseek-OAuth2/releases/tag/v0.1.0
