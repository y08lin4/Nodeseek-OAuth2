# NSAuth2 授权助手（UserScript / Tampermonkey）

Nodeseek OAuth2 授权流程的浏览器自动化脚本（油猴脚本）：**自动读取授权页的验证码，并在 nodeseek.com 上自动私信发送给系统账号，全程用户零操作**。

- 语言：纯 JavaScript，无第三方依赖
- 环境：Tampermonkey（其余支持 `@grant` 的 UserScript 管理器亦可）
- 发布状态：可发布到 [Greasy Fork](https://greasyfork.org/)（已含 `@license MIT` 与 `@supportURL` 元数据）

---

## 安装方法

1. 安装浏览器扩展 **Tampermonkey**（Chrome / Edge / Firefox 均支持）。
2. 点击 Tampermonkey 图标 → **管理面板**（Dashboard）→ **加号（新建脚本）**。
3. 清空编辑器默认模板，将 [nsauth2-helper.user.js](./nsauth2-helper.user.js) 的**全部内容**粘贴进去。
4. `Ctrl+S` 保存。脚本即生效，无需其他设置。

> 提示：也可先安装 Greasy Fork 或本仓库的新建脚本流程，把文件内容导入即可。

---

## 功能说明

### 1. 自动读取验证码（授权服务页）
当你登录授权服务页（`nodeseek-ouath.ailinyu.de`）并提交 NS ID 后，页面会显示 6 位验证码。脚本通过 `MutationObserver` 监测 `.oauth-code-display` 元素的出现：

- 出现后读取验证码文本；
- 同时读取 `#oauth-system-account` 的 `data-uid`、`data-name`（目标系统账号）；
- 存入本地 `GM_getValue/GM_setValue`（键 `pending_code`）；
- 右上角弹出绿色小卡片「已读取验证码，正在私信发送…」；
- **自动跳转**到 nodeseek.com 私信页（验证码已在存储中，跳走不丢）。

### 2. 自动发送私信（nodeseek.com 页）
进入 nodeseek 私信页后，脚本检测到 `pending_code` 存在即自动：

- 向私信内容输入框填入**纯验证码**（服务端按 `content` 含验证码匹配，填纯码最稳）；
- 通过 URL 的 `to` 参数（数字 UID）定位收件人；若页面存在收件人输入框则优先填 UID、失败回退昵称；
- 点击「发送」按钮；
- 成功后清除 `pending_code` 并提示「已发送私信」。

**私信页 URL**：经项目源码确认，NodeSeek 官方私信入口为
`https://www.nodeseek.com/notification#/message?mode=talk&to={id}`，
收件人由 `to` 参数定位（数字 UID）。脚本运行时还会对 `/message`、`/inbox`、`/pm` 等候选 URL 做 fetch 探测，取含「私信」关键字且可访问的那个，避免 URL 硬编码失效。

### 3. 容错行为（不静默失败）
- 找不到输入框 / 发送按钮 / 收件人输入框 → `console` 与页面卡片双重提示；
- 发送失败 → 提示用户**手动发送**，不阻塞流程；
- 所有选择器均宽松适配，任一环节失败都给出明确提示。

### 4. 油猴菜单
脚本注册了两个菜单命令（Tampermonkey → 脚本图标）：

- **重新发送验证码**：重读 `pending_code` 再次尝试发送；
- **清除待发送码**：清掉本地缓存的待发送验证码。

---

## 工作流程总览

```
用户在授权页提交 NS ID
        │
        ▼
授权页显示验证码 (.oauth-code-display) + 系统账号 (#oauth-system-account)
        │  MutationObserver 捕获 → 写入 GM 存储 (pending_code)
        ▼
自动跳转 nodeseek.com 私信页 (/notification#/message?mode=talk&to={uid})
        │ 读取 pending_code → 填内容(纯验证码) → 点发送
        ▼
服务端检测到私信 => 登录成功（清除 pending_code）
```

---

## 已知限制

- **私信页 DOM 可能变更失效**：NodeSeek 私信页为前端 SPA 动态渲染，本脚本与仓库浏览器扩展（`extension/src/pm-fill.ts`）均标注其输入框/发送按钮结构「未校准」。脚本采用宽松选择器（`textarea` / `[contenteditable="true"]` / `.ql-editor` / 含「发送」的按钮），若 NodeSeek 改版导致选择器失效，脚本会提示手动发送，需按新 DOM 调整 `CONTENT_SELECTORS` / 发送按钮判断逻辑。
- **依赖前端钩子**：脚本依赖授权页渲染 `.oauth-code-display` 与 `#oauth-system-account[data-uid][data-name]` 两个钩子。当前 `web/src/views/LoginView.vue` 尚未合并这两个钩子（前端批次 2 并行添加中）；钩子到位后脚本自动生效。
- **未登录无法发送**：若浏览器在 nodeseek.com 未登录，私信页不会出现输入框/发送按钮，脚本会提示手动处理——请先在 nodeseek.com 登录。
- **仅本机使用**：`pending_code` 存于浏览器本地，换浏览器/设备不会同步。

---

## 服务地址更换说明

脚本通过 `@match` 限定生效域名，默认两块：

```text
https://www.nodeseek.com/*
https://nodeseek-ouath.ailinyu.de/*
```

- 若授权服务部署域名变更（例如换成你自建的服务地址），需**同步修改 `@match`**，否则脚本在新授权页不生效。
- 若 nodeseek 站点主域不变（仍是 `www.nodeseek.com`），私信部分无需改动；如站点基址变化，还需同步脚本内 `NS_HOST` 与 `PM_URL_CANDIDATES` 中的域名。
- 元数据中的 `@supportURL`、`@namespace` 指向仓库地址，发布/使用前请按实际仓库 URL 更新。

---

## 发布到 Greasy Fork

1. 在 Greasy Fork 登录并进入「把脚本拖到/粘贴到此」。
2. 粘贴 `nsauth2-helper.user.js` 的完整内容，填写标题与说明。
3. 脚本头部已含 `@license MIT`、`@supportURL`、`@namespace`、`@version 0.1.0` 等 Greasy Fork 所需元数据。建议把 `@namespace` 与 `@supportURL` 更新为真实仓库地址后再发布。

---

## 代码结构速览（nsauth2-helper.user.js）

| 模块 | 作用 |
| --- | --- |
| 元数据块 | `@match` 生效域、`@grant` 授权、版本声明 |
| `watchAuthPage` | 授权页 MutationObserver，捕获验证码 + 系统账号 |
| `captureCodeAndGo` | 校验钩子、写 `pending_code`、自动跳私信页 |
| `runOnNodeseek` / `trySendPm` | 私信页自动填码 + 点发送 + 成功后清除 |
| `registerMenu` | 「重新发送验证码」「清除待发送码」菜单 |
| 工具函数 | `showToast`、`isVisible`、`setNativeValue`（兼容 Vue v-model） |

---

## License

MIT © 本仓库（见根目录 [LICENSE](../LICENSE)）。
