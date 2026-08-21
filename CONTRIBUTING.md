# 参与贡献

感谢你愿意花时间改进 NSAuth2 🎉 本文件是贡献指南，请先读一遍再动手。

## 项目约定（重要）

- **[SPEC.md](SPEC.md) 是唯一契约**：接口、存储结构、环境变量、安全策略都以它为准。**改接口先改 SPEC，再改代码**，保持一致。
- 子目录之间松耦合：`server/`（Go 后端）、`web/`（Vue3 前端）、`extension/`（Chrome MV3 扩展）各自独立构建，互不依赖源码。
- 语言：代码注释与文档使用中文，标识符使用英文。

## 本地开发环境

### 后端（server/）

```bash
cd server
# 开发模式：mock 跳过真实私信核验、注册自动通过，离线可跑全流程
$env:NS_MOCK_MODE = "1"        # PowerShell
$env:NS_SECRET_KEY = "<base64 32 字节>"
$env:NS_ADMIN_TOKEN = "<管理令牌>"
go run .
```

### 前端（web/）

```bash
cd web
npm install
npm run dev       # http://localhost:5173，/api 与 /oauth 代理到 :8080
npm run build     # 产物 web/dist/，由后端直接托管
```

### 扩展（extension/）

```bash
cd extension
npm install
npm run build     # 产物 extension/dist/
```

Chrome `chrome://extensions` → 开发者模式 → 加载已解压的扩展。注意 `manifest.json` 的 `host_permissions` 是占位域名，本地调试改成自己的。

### 测试

```bash
cd server
go vet ./...
go test ./...
# 冒烟：起 mock 服务后跑 tools 下的脚本 / 手工走一遍登录→授权→token 流程
```

## 提交 PR

1. Fork 本仓库，从 `main` 切分支：`git checkout -b feat/xxx`（命名建议：`feat/`、`fix/`、`docs/`、`chore/`）。
2. 小步提交，信息用中文或英文均可，但要点明改动（如 `feat: 支持自定义验证码有效期`）。
3. 运行对应模块的构建与测试，确保绿。
4. 提 PR：说明**动机、改动点、验证方式**；如涉及接口变更，同步更新 SPEC.md 并在 PR 描述中标注。

## 安全相关

- 涉及安全（认证、Cookie 加密、限流、审计）的改动请**额外注明测试路径**。
- 任何 PR 不得包含真实 Cookie、密钥、令牌或 `.env` 文件（CI 会扫描，勿存侥幸）。
- 发现安全漏洞请走 [SECURITY.md](SECURITY.md) 的流程，不要在 Issue 里贴真实凭据。

## 行为准则

参与本项目即视为同意 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)。
