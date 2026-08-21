# 管理后台详细设计方案（v2 细化，审批中）

> 目标：/admin 升级为**完整管理后台**——登录页 + 浅色侧栏导航 + 仪表盘/管理/设置/日志四区。
> **已确认**：服务端会话登录（httpOnly Cookie）· 浅色侧栏 · 审计独立导航「日志」· 内容区 900px 居中。
> **本版细化**：布局线框、组件级规格、视觉 token、交互与状态、响应式、验收清单。所有图标一律 **lucide**，全站无 emoji。

---

## 1. 设计 Token（后台专用，延续门户 NS 绿体系）

| Token | 值 | 用途 |
|-------|-----|------|
| 主色 | `#2ea44f` | 激活态/主按钮/关键数字 |
| 主色浅底 | `#e6f4ea` | 激活项背景/标签底 |
| 侧栏背景 | `#ffffff` | 侧栏 |
| 内容区背景 | `#fbfbfb` | 页面底 |
| 卡片 | `#ffffff` + 1px `#e5e7eb` + 6px 圆角 | 所有内容卡 |
| 主文字/次级/弱化 | `#333` / `#555` / `#888` | 层级 |
| 危险/成功/警告 | `#cf222e` / `#1a7f37` / `#9a6700` | 语义 |
| 侧栏宽度 | 220px（≥768px），抽屉（<768px） | 布局 |
| 内容区 | max-width 900px 居中，padding 24px | 布局 |
| 顶栏高 | 56px，白底 + 1px 底边框 | 布局 |
| 字体 | 系统栈（门户一致） | 排版 |
| 数字 | tabular-nums | 统计/ID/时长 |

---

## 2. 整体布局（线框）

```
┌─────────────────────────────────────────────────────────────┐
│ 顶栏 56px 白底                                              │
│  [Logo 28] NSAuth2 管理后台                [👤 退出登录]    │
├──────────────┬──────────────────────────────────────────────┤
│ 侧栏 220px   │ 内容区 max-900 居中 padding 24               │
│ 白底右边框    │                                              │
│ ─ 分组标题    │  <router-view>                               │
│ ▸ 仪表盘      │                                              │
│   管理        │                                              │
│   · 应用      │                                              │
│   · 审核      │                                              │
│   · 账号      │                                              │
│   日志        │                                              │
│   设置        │                                              │
└──────────────┴──────────────────────────────────────────────┘
```

**顶栏**：左 = NSLogo 28px + 「NSAuth2 管理后台」（15px/600）；右 = 退出登录（text 按钮，点击 → dialog 确认「确定退出管理后台？」→ POST /api/admin/logout → 清会话 → 跳 /admin/login）。
**侧栏**：
- 分组标题：11px/500 `#888`，上边距 20px（「概览」「管理」「系统」）
- 导航项：40px 高，图标 18px + 文字 14px `#555`；hover 浅灰底；**激活** = 2px 绿左条 + `#e6f4ea` 底 + 文字 `#333` 加粗 + 图标绿
- 图标（lucide）：仪表盘 `LayoutDashboard` · 应用 `Blocks` · 审核 `ClipboardCheck` · 账号 `Users` · 日志 `ScrollText` · 设置 `Settings` · 退出 `LogOut`
- 分组：**概览**（仪表盘）/ **管理**（应用·审核·账号）/ **系统**（日志·设置）

---

## 3. 登录页 `/admin/login`（详细）

```
        ┌──────────────────────────────┐
        │      [Logo 40]               │
        │      NSAuth2 管理后台        │
        │  ┌──────────────────────┐    │
        │  │ 管理令牌             │    │
        │  │ ＊＊＊＊＊＊＊＊      │    │
        │  └──────────────────────┘    │
        │  [      登录      ]（绿主按钮）│
        │  ──────────────────────      │
        │  · 令牌在服务端 .env 配置     │
        │  · 会话有效期 24h             │
        └──────────────────────────────┘
```
- 页面：内容区 900px 居中垂直居中（min-height 计算），白卡片 320px 宽居中
- 输入：NInput type="password"（可切换明文），Enter 键提交；提交中按钮 loading + disabled
- 错误：卡片内 NAlert（error，如「管理令牌错误」）或 429「请求过于频繁」
- 成功：跳 `?next=<原路径>`（默认 /admin/dashboard）；已登录访问 → 直接跳 dashboard
- 视觉：Logo 40px、标题 16px/600、卡片 6px 圆角、底部说明 12px `#888`

---

## 4. 仪表盘 `/admin/dashboard`（详细）

```
┌ 统计行（5 卡 grid: repeat(auto-fit, minmax(150px,1fr)) gap 12）┐
│ [验证码生成 12] [登录成功 8] [登录失败 2] [门槛拦截 0] [Cookie告警 1] │
│  数值 24px/700 绿 · 标题 12px #888 · tabular-nums · 标注"自 20:00 起" │
└──────────────────────────────────────────────────────────────┘
┌ 左（2/3）──────────────┬ 右（1/3）───────────────┐
│ Cookie 状态卡           │ 审核待办卡               │
│  状态徽章（已设置/未设置）│  app ×2 · pause ×1      │
│  更新时间 + 距今时长     │  「去处理 →」链接        │
│  告警提示（NAlert）      │                          │
│ ─────────────────────  │                          │
│ 系统账号卡              │  最近审计（5 条）         │
│  数量/启用/失效计数      │  事件徽章 + 时间 + 对象   │
│  失效账号 NAlert 警告    │  顶部「查看全部 →」       │
└─────────────────────────┴──────────────────────────┘
```
- 数据：`/api/admin/stats`（5 计数 + reset_at）、`/api/admin/status`（cookie/mail/mock）、`/api/admin/accounts`、`/api/admin/reviews`、`/api/admin/audit?limit=5`
- 刷新：页面进入拉取 + 顶栏刷新按钮（同步全部）
- 空态：待办为 0 → 绿 NEmpty「暂无待处理」；审计无 → NEmpty
- 告警：Cookie 未设置 → NAlert warning「系统账号 Cookie 未设置，登录不可用」+「去推送 →」跳账号页；失效账号 >0 → NAlert error 列出账号名

---

## 5. 应用管理 `/admin/apps`（详细）

```
┌ 应用表格（白卡）──────────────────────────────────────────┐
│ 表头：应用/ID · Owner · 状态 · 门槛 · Token 时长 · 今日/累计 · 操作 │
│ 行（斑马纹 #fbfbfb 隔行）                                  │
│ Demo App / demo-app · 1 · [已通过] · 0级 · 60min · 3/1 · 8/2 │
│     [暂停] [调整时长] [删除]                                │
└──────────────────────────────────────────────────────────┘
```
- 状态徽章 NTag：pending_review=审核中(info) · approved=已通过(success) · paused=已暂停(warning) · rejected=未通过(error) · pause_request=暂停申请中(warning) · resume_request=恢复申请中(info) · delete_request=删除申请中(error)；disabled=true 叠加灰标签「已禁用」
- 操作交互：
  - **暂停/恢复**：approved→「暂停」（PATCH status:"paused"）；paused→「恢复」（PATCH status:"approved"）；操作前 dialog 确认
  - **调整时长**：行内 NInputNumber（分钟 1-1440）+ 确认按钮 → PATCH token_ttl（秒）
  - **删除**：红「删除」→ dialog 确认（输入应用名确认？——简单 dialog 即可）→ DELETE
- 每次操作后刷新列表；失败 useMessage error（ApiError message）
- 搜索框（可选 v1 不做）：顶部 NInput 按名称过滤

---

## 6. 审核队列 `/admin/reviews`（详细）

```
┌ 待审核列表（白卡）────────────────────────────────────────┐
│ [应用申请] Demo App · owner 1 · 3小时前   [通过] [拒绝]  │
│ [暂停申请] Foo · owner 2 · 1天前          [通过] [拒绝]  │
└──────────────────────────────────────────────────────────┘
```
- 类型徽章：app=应用申请(info) · pause=暂停申请(warning) · resume=恢复申请(success) · delete=删除申请(error)
- 行：类型徽章 + 应用名（+client_id 小字）+ owner（点击可复制）+ 相对时间（或本地化）+ 操作
- 通过：dialog 确认（显示「将把应用状态置为 X」）→ POST /api/admin/review {type, client_id, action:"approve"}
- 拒绝：dialog 含理由输入框（可空）→ action:"reject" + reason
- 空态：NEmpty「暂无待审核申请」+ 图标 `ClipboardCheck`
- 队列顶部刷新按钮 + 计数摘要（各类型数量）

---

## 7. 系统账号 `/admin/accounts`（详细）

```
┌ 账号列表（白卡）──────────────────────────────────────────┐
│ 名称/ID · 优先级 · 状态 · 更新时间 · 最近错误 · 操作         │
│ idamie/9037 · 0 · [启用] · 3小时前 · 无 · [启停][调优][删] │
└──────────────────────────────────────────────────────────┘
┌ 更新 Cookie 表单（白卡）──────────────────────────────────┐
│ [账号选择：自动识别 ▼] [Cookie 粘贴 多行] [推送更新]       │
└──────────────────────────────────────────────────────────┘
```
- 列表：启用状态 NTag（启用=success/禁用=default）；last_error 非空显示为红色小字（可截断 + title）
- 操作：启停（PATCH enabled 反转，dialog 确认）、优先级调整（行内数字）、删除（红，dialog 确认——**最后 1 个账号不可删**：提示「至少保留 1 个系统账号」）
- 新增账号：顶部「+ 新增」→ dialog（ID 纯数字校验 + 名称）→ POST /api/admin/accounts
- Cookie 表单：账号下拉（现有账号 + 自动识别）+ 多行文本 + 「推送」按钮（loading）→ POST /api/admin/cookie → 成功提示识别到的账号名
- 空态：无账号 → NEmpty「暂无系统账号，推送 Cookie 后自动创建」

---

## 8. 审计日志 `/admin/audit`（详细，独立导航）

```
┌ 日志表格（白卡）──────────────────────────────────────────┐
│ 表头：时间 · 事件 · IP · 用户 · 应用 · 详情               │
│ 行：08-21 10:47 · admin.stats.view · 172.x · — · — · —   │
│ 顶部：[limit 50 ▼] [刷新] [自动刷新开关(可选)]             │
└──────────────────────────────────────────────────────────┘
```
- 事件徽章：login.* = info · admin.* = warning · authorize/token/userinfo = default · gate.block = error · mail.* = success
- 时间本地化（toLocaleString）；IP/ID 等宽字体
- limit：NSelect（50/100/200）+ 刷新按钮；空态 NEmpty
- 行 hover 高亮；详情列可长文本截断（title 全量）

---

## 9. 设置 `/admin/settings`（详细）

```
┌ 邮件（白卡）──────────────────────────────────────────────┐
│ SMTP 配置：未配置/已配置（badge）· 报告时间 20:00 · 上次测试 08-21 10:14 │
│ 新应用通知：[已开启/未开启]（review_email_notify，只读+说明"环境变量控制"） │
│ [发送测试邮件]（loading，成功/失败 message）                 │
└──────────────────────────────────────────────────────────┘
┌ 系统信息（白卡）──────────────────────────────────────────┐
│ Mock 模式：关 · 会话类型：服务端 Cookie · 服务启动时间（或略）│
└──────────────────────────────────────────────────────────┘
```

---

## 10. 响应式与动效

- **≥768px**：侧栏固定 220px 展示
- **<768px**：侧栏隐藏，顶栏汉堡按钮（`Menu` lucide）→ 抽屉（遮罩 + 侧栏滑入，点击遮罩/导航项关闭）
- 表格窄屏：横向滚动（overflow-x: auto，min-width 表格）
- 动效：卡片/区块进入 150ms fade；侧栏抽屉 200ms 滑入；尊重 prefers-reduced-motion
- 登录页移动端：卡片全宽（padding 16px）

---

## 11. 全站图标规范（lucide 全覆盖，无 emoji）

| 位置 | lucide 图标 |
|------|-------------|
| 侧栏导航 | LayoutDashboard / Blocks / ClipboardCheck / Users / ScrollText / Settings |
| 顶栏 | LogOut（退出）· Menu（移动端汉堡） |
| 登录页 | Lock（令牌输入前缀，可选） |
| 仪表盘卡 | KeyRound / CheckCircle2 / XCircle / ShieldBan / AlertTriangle（5 统计卡） |
| 应用管理 | Pause / Play / Clock / Trash2（操作按钮） |
| 审核 | Check / X（通过/拒绝） |
| 账号 | Plus / RefreshCw / Trash2 |
| 设置 | Mail / Server / Send |
| 空态 | NEmpty 默认图标（naive 自带，非 emoji） |

**全站现有 emoji 替换清单（本次一并执行）**：
- DashboardView 快捷入口 `🧩🔑📖` → `Blocks / KeyRound / BookOpen`
- AuthorizeView 门槛错误 `⛔` → `ShieldBan`
- ConsoleView secret 提示 `⚠` → `TriangleAlert`
- DocsView `💡` → `Lightbulb`、`⚠️` → `TriangleAlert`（正文内嵌小图标）

---

## 12. 验收清单

1. 未登录访问 /admin/* → 跳 /admin/login；登录成功回原页
2. 顶栏退出 → 会话清除 → 回登录页；刷新后仍退出态
3. 仪表盘 5 统计 + Cookie/账号/待办/审计全部有数据（无数据显空态）
4. 应用管理：暂停/恢复/时长/删除全部生效且刷新
5. 审核：通过/拒绝（含理由）生效
6. 账号：启停/优先级/删除/新增/Cookie 推送全通；最后 1 个账号删除被拒
7. 日志：limit 切换 + 刷新 + 徽章配色
8. 设置：邮件测试按钮 + 通知开关状态显示
9. 窄屏（<768px）抽屉导航可用
10. 全站无 emoji 图标（grep 校验）
