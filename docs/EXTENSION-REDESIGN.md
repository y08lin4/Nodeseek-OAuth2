# 浏览器扩展重构方案（v2 简化版，待审批）

> 极简原则：初始化向导（4 步）→ 保存后静默推送。失败走邮件告警（服务端已有），扩展不做复杂通知/分析。
> 纯 TS + 原生 DOM；图标内联 SVG（无 emoji）；视觉 NS 绿。

---

## 1. 初始化向导（首次打开 options / 未配置时，4 步）

```
第 1 步  服务地址          [nodeseek-ouath.ailinyu.de]   ← 只输域名（自动补 https://，校验格式）
                          ───────────── 下一步 ─────────────
第 2 步  管理令牌          [****************]  [验证]
                          → POST {url}/api/admin/status（带 X-Admin-Token）
                          → 成功：绿勾「连接正常 · 服务可用」自动下一步
                          → 失败：红字原因（令牌错误/无法连接）留在本步
第 3 步  识别信息          [开始识别]
                          → 扩展抓 nodeseek.com Cookie → POST /api/admin/cookie
                          → 显示：ID 37384 · 用户名 萧炎 · 等级 3（来自服务端响应扩展的 rank 字段）
                          → 识别失败：红字原因（Cookie 无效/无法识别）可重试
第 4 步  保存并开启        [保存并开始推送]
                          → 写入 storage（serverUrl/token/interval=30min/账号信息）
                          → 完成页：✓ 配置完成，将每 30 分钟静默推送 Cookie
```
- 步骤指示条（1-4 圆点 + 当前高亮）；上一步/下一步按钮；第 2 步验证通过才允许下一步
- **服务地址只输域名**：自动 `https://` 前缀 + 去尾部 `/`；与 manifest host_permissions 匹配校验（不一致提示"域名需与扩展权限一致"，见 §4）

## 2. 设置页（已配置时）

- 当前配置卡：服务器域名 · 账号（ID/用户名/等级）· 推送间隔（分钟，1-1440）· 启用开关
- 操作：「重新配置」（重走向导，保留旧值预填）·「立即推送一次」·「停用/启用」
- 多槽位：**默认单槽位**（简化）；多槽位结构保留在 storage（slots 数组），但 UI 不主动暴露——需要多 profile 时再启用（README 说明）
- 不做的：未保存变更提示、删除确认、targetAccountId 手动绑定（自动识别模式下无需）

## 3. popup

```
┌──────────────────────────────┐
│ [Logo] NSAuth2 Cookie Keeper  │
│ 服务器：nodeseek-ouath.ailinyu.de │
│ 账号：萧炎 (37384) · 等级 3     │
│ 上次推送：3 分钟前 · 成功       │  ← 失败显示红字原因（401/网络/识别失败）
│ [立即推送]        [设置]       │
└──────────────────────────────┘
```
- 未配置：引导「点击配置」→ 打开 options 向导
- 无复杂总览/异常条/图表——一眼状态即可

## 4. 配置与权限

- manifest host_permissions：`https://www.nodeseek.com/*` + `https://*/*`（自建服务端任意域名——**权限校验放宽**：向导输的域名不需要预先声明（MV3 host_permissions 用 `<all_urls>` 或 `https://*/*` 即可推送任意服务器）。**采用 `https://*/*`**（配合 fetch 权限），向导不再校验域名一致（简化）
- 推送：chrome.cookies.getAll({domain:'.nodeseek.com'}) → POST {url}/api/admin/cookie（X-Admin-Token）

## 5. 后端小增强（配合识别信息）

- `POST /api/admin/cookie` 响应增加 `"stats": {"rank":3,"join_days":359,"coin":1508}`（识别成功后顺带拉 getInfo，失败 stats=null 不阻塞）——向导第 3 步一次性拿到 ID/用户名/等级

## 6. 实施范围

| 阶段 | 内容 |
|------|------|
| 1 | 后端 /api/admin/cookie 响应加 stats（小改） |
| 2 | options 重构：4 步向导 + 已配置摘要页 |
| 3 | popup 重构：状态简洁版 |
| 4 | background：推送结果记录（账号名/失败分类）+ 定时推送保留 |

构建：`cd extension && npm run build` 成功；发布 v0.4.0 zip。

## 7. 验收清单

1. 首次打开 → 4 步向导；域名只输主机名自动补 https
2. 令牌验证失败 → 本步停留 + 原因；成功 → 自动下一步
3. 识别信息显示 ID/用户名/等级；失败可重试
4. 保存后定时推送（30min）静默执行；popup 显示账号与上次结果
5. 邮件告警生效（服务端已有，扩展不重复）
6. build 成功 + zip 发布
