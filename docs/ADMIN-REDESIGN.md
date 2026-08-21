# 管理后台 v2 详细设计（v4 最终版，待审批）

> 全部确认功能整合：布局 1200px · 对齐规范 · SMTP 配置 · 用户管理（黑名单/详情/社区主页）· 授权记录（搜索/导出）· Secret 重置 · 应用排行 · 审核显示申请者。
> 审批后并行实施。

---

## 1. 布局（适中宽度）

- 顶栏 56px 全宽；侧栏 220px（浅色/lucide/绿激活态）
- 内容区：`margin-left: 220px` + `max-width: 1200px` + 剩余区域居中 + `padding: 24px`（视口 <1200 自动 100%）
- 响应式：<1024px 侧栏抽屉

## 2. 对齐规范（全站强制，防突出/扭曲）

- 间距：内容 padding 24px、卡片间距 16px、卡片内 padding 20px
- 表格：行高 44px（表头 48px）、表头 12px `#888`、斑马纹 `#fbfbfb`；**文本左对齐/数字右对齐/操作列右对齐固定宽**；长文本省略+title；容器 overflow-x auto 防变形
- 统计卡：`auto-fit minmax(200px,1fr)` 等宽等高、数值 24px/700 右对齐 tabular-nums
- 按钮组：统一 small、间距 8px、不换行；表单 label 上置 12px `#555`
- 时间统一 `toLocaleString('zh-CN')`；徽章 NTag small round

---

## 3. 导航结构（最终 9 项）

```
概览   仪表盘 LayoutDashboard
管理   用户 Users · 应用 Blocks · 审核 ClipboardCheck · 账号 ServerCog · 授权记录 KeyRound
系统   日志 ScrollText · 设置 Settings
```

---

## 4. SMTP 运行时配置

- 存储 `data/smtp.json`（pass AES-256-GCM 加密）；`GET/POST /api/admin/smtp`（pass 脱敏、空=保留旧值）；保存即 mailer 热更新 + 审计
- 前端：设置页 SMTP 表单（host/port/tls/user/pass/enabled + 保存 + 测试邮件）

---

## 5. 用户管理

### 5.1 列表（「用户」页）
- 表格：ID（等宽）/昵称/等级（列表不实时拉——显示 users 表缓存或"—"）/注册天数/登录次数/授权数/黑名单徽章/操作
- **操作列**：社区主页链接（`https://www.nodeseek.com/space/{id}`，新窗口，`ExternalLink` 图标）· 详情 · 拉黑/解禁（`Ban`/`ShieldOff`，dialog 确认，**拉黑时吊销该用户全部 token + 授权**）
- 顶部：摘要卡（总用户/今日活跃/今日登录）+ 搜索框（ID/昵称过滤）
- CSV 导出按钮（导出当前列表）

### 5.2 用户详情页 `/admin/users/:id`（新增路由）
- 头部卡：头像区（ID 大字）+ 昵称 + ID + 等级/注册天数（**实时拉取**：管理端 `GET /api/admin/users/{id}/detail` 调 NS getInfo 返回 rank/join_days/co 等 + users 表记录 + 黑名单状态）+ 社区主页按钮（新窗口）
- 统计卡（**今日 / 累计**）：登录成功 · 登录失败 · 授权成功（grants 数）· 授权失败（gate.block/deny 计数）——审计聚合（`GET /api/admin/users/{id}/stats`）
- 授权记录区：该用户全部授权记录（复用授权记录表格，按 user_id 过滤）
- 黑名单操作：拉黑/解禁（同列表）

---

## 6. 授权记录页（「授权记录」）

- 表格：用户 ID/昵称（users 表 join）/应用名/scope/授权时间/状态徽章/撤销时间/token 兑换次数
- **搜索框**：输入**用户名/ID** → 过滤授权记录（`?user_id=` 或昵称匹配）
- 过滤：应用下拉 + 状态下拉；分页（50/页）
- **CSV 导出**（导出当前过滤结果）
- 用户列点击 → 跳用户详情页

---

## 7. 应用管理增强

- **Secret 重置**：行内「重置密钥」按钮（`RefreshCw`）→ dialog 确认「重置后旧 secret 立即失效」→ POST `/api/admin/clients/{id}/reset-secret` → 一次性展示新 secret（复制按钮，提示仅显示一次）+ 审计
- **应用详情**：行点击/「详情」→ 模态框：完整信息（回调地址列表/图标 URL/描述/创建者昵称+社区链接/门槛/token 时长/统计/状态变更记录）
- 表格列：创建者昵称（users 表 join；无则 ID）

---

## 8. 审核队列增强

- 每行显示**申请者昵称**（users 表 join，无则 ID）+ 社区主页小链接 + **等级**（拉取缓存或"—"）
- 通过/拒绝不变（拒绝理由 dialog）

---

## 9. 仪表盘增强

- 现有：5 统计卡 + Cookie 状态 + 账号概览 + 审核待办 + 最近审计
- 新增：**应用排行 Top5**（今日授权数，clients stats 数据，`Blocks` 图标卡，点击跳应用页）

---

## 10. 审计日志增强

- 现有：limit/刷新/表格
- 新增：**过滤**（事件类型下拉 + 用户 ID 输入）

---

## 11. CSV 导出（通用）

- 端点：`GET /api/admin/export/users.csv`、`GET /api/admin/export/grants.csv`（支持与列表相同的过滤参数；UTF-8 BOM 防 Excel 乱码）
- 前端：用户页/授权记录页「导出 CSV」按钮（下载链接）

---

## 12. 后端端点汇总（新增）

| 端点 | 说明 |
|------|------|
| `GET/POST /api/admin/smtp` | SMTP 配置读写（pass 加密/脱敏） |
| `GET /api/admin/users` | 用户列表（join 授权数/黑名单） |
| `GET /api/admin/users/{id}/detail` | 实时拉 NS stats + 用户记录 |
| `GET /api/admin/users/{id}/stats` | 今日/累计 登录/授权 成功失败（审计聚合） |
| `PATCH /api/admin/users/{id}` | 黑名单开关（拉黑=吊销 token+授权） |
| `GET /api/admin/grants` | 授权记录（过滤 user_id/client_id/status + 昵称 join） |
| `POST /api/admin/clients/{id}/reset-secret` | 重置 secret（旧失效） |
| `GET /api/admin/export/users.csv` / `grants.csv` | CSV 导出 |

---

## 13. 实施批次

| 批次 | 内容 | 线 |
|------|------|-----|
| A | 后端全部端点（SMTP/用户/黑名单/详情/统计/grants/secret/导出） | 后端 |
| B | 后台页面（用户页+详情+黑名单/授权记录+搜索+导出/应用 secret+详情/审核昵称/仪表盘排行/审计过滤/布局 1200px+对齐规范） | 前端 |
| C | 扩展重构（EXTENSION-REDESIGN.md，独立） | 扩展 |

---

## 14. 验收清单

1. 布局 1200px、对齐规范全站（表格/卡片/按钮/表单）
2. SMTP 保存即生效、pass 脱敏
3. 用户列表：搜索/黑名单（拉黑即吊销）/社区主页/CSV 导出
4. 用户详情：实时等级/今日累计统计/授权记录
5. 授权记录：用户名搜索/过滤/分页/导出
6. Secret 重置：新 secret 一次性展示、旧失效
7. 应用详情模态：完整信息+创建者
8. 审核行显示昵称+等级+社区链接
9. 仪表盘应用排行 Top5
10. 审计过滤
