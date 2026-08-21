<script setup lang="ts">
// 接入文档页（公开）：完整第三方接入教程
// ① 注册应用 ② 构造授权 URL ③ 用户授权 ④ callback 收 code + 校验 state
// ⑤ POST /oauth/token 换 access_token ⑥ GET /oauth/userinfo 取用户信息 ⑦ 应用自签会话
// 含 curl 与 Node/JS 示例、端点一览表、.well-known 自动发现提示
import { NCard, NAlert, NTable } from 'naive-ui'
</script>

<template>
  <n-card class="page-card">
    <template #header>
      <span class="page-title">接入文档</span>
    </template>
    <p class="ns-card-sub">标准 OAuth2 授权码流程，接入约需 10 分钟</p>

    <n-alert type="info" :show-icon="true" class="ns-mb-3">
      💡 本服务发布 RFC 8414 元数据，第三方可通过
      <code>/.well-known/oauth-authorization-server</code> 自动发现端点，免手写配置。
    </n-alert>

    <!-- 接入步骤 -->
    <h2 class="ns-h5 ns-mt-4 ns-mb-3">接入步骤</h2>
    <ol class="docs-steps">
      <li>
        <strong>注册应用</strong>：在
        <RouterLink to="/console">应用管理</RouterLink>
        提交应用申请（名称 / 主页 / 回调地址 / 最低等级 / token 有效期），提交后不可修改，
        管理员审核通过后状态变为「已通过」，获得 <code>client_id</code> 与
        <code>client_secret</code>（secret 仅展示一次，请立即保存）。
      </li>
      <li>
        <strong>构造授权 URL</strong>：将用户引导至授权端点，携带应用信息与随机
        <code>state</code>（防 CSRF）：
        <pre class="code-block"><code>/oauth/authorize?client_id=&lt;client_id&gt;&amp;redirect_uri=&lt;回调地址&gt;&amp;response_type=code&amp;scope=user&amp;state=&lt;随机串&gt;</code></pre>
      </li>
      <li>
        <strong>用户授权</strong>：用户登录本服务并确认授权（或拒绝）；未登录会先跳到登录页。
      </li>
      <li>
        <strong>callback 收 code</strong>：授权后浏览器 302 回
        <code>redirect_uri?code=NS_AUTH_XXXX&amp;state=&lt;原样&gt;</code>；
        服务端会把 <code>state</code> 原样回显，<strong>务必校验 state 与发起时一致</strong>，否则拒绝本次回调。
      </li>
      <li>
        <strong>换 access_token</strong>：用授权码调用 <code>POST /oauth/token</code>
        （form-encoded），成功后按需销毁授权码（一次性，防重放）。
      </li>
      <li>
        <strong>取用户信息</strong>：用 <code>access_token</code>（Bearer）调用
        <code>GET /oauth/userinfo</code> 获取 <code>user_id</code> / <code>sub</code> /
        等级等 stats。
      </li>
      <li>
        <strong>应用自签会话</strong>：<code>access_token</code> 是一次性身份授权凭证，
        应用应据此签发自己的会话 Cookie，token 用完即弃，不要长期保存。
      </li>
    </ol>

    <!-- 代码示例 -->
    <h2 class="ns-h5 ns-mt-4 ns-mb-3">代码示例</h2>

    <h3 class="ns-h6 ns-mt-3">curl（换 token + 取用户信息）</h3>
    <pre class="code-block"><code># 1) 授权码换 access_token
curl -X POST https://auth.example.com/oauth/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=authorization_code" \
  -d "code=NS_AUTH_XXXX" \
  -d "client_id=&lt;client_id&gt;" \
  -d "client_secret=&lt;client_secret&gt;" \
  -d "redirect_uri=https://app.example.com/callback"

# 2) 用 access_token 取用户信息
curl https://auth.example.com/oauth/userinfo \
  -H "Authorization: Bearer &lt;access_token&gt;"</code></pre>

    <h3 class="ns-h6 ns-mt-3">Node.js（fetch form-encoded 示例）</h3>
    <pre class="code-block"><code>// 1) 换 token（form-encoded）
const body = new URLSearchParams({
  grant_type: 'authorization_code',
  code: 'NS_AUTH_XXXX',
  client_id: 'your_client_id',
  client_secret: 'your_client_secret',
  redirect_uri: 'https://app.example.com/callback',
})
const resp = await fetch('https://auth.example.com/oauth/token', {
  method: 'POST',
  headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
  body,
})
const { access_token, token_type, expires_in, scope } = await resp.json()

// 2) 取用户信息（Bearer）
const user = await fetch('https://auth.example.com/oauth/userinfo', {
  headers: { Authorization: `Bearer ${access_token}` },
}).then((r) => r.json())
console.log(user.user_id, user.sub, user.stats)</code></pre>

    <n-alert type="warning" :show-icon="true" class="ns-mt-2 ns-mb-3">
      ⚠️ <code>state</code> 校验提示：回调时请比对 <code>state</code> 与发起授权时的一致，
      不一致视为 CSRF 攻击，直接拒绝。
    </n-alert>

    <!-- 端点一览表 -->
    <h2 class="ns-h5 ns-mt-4 ns-mb-3">端点一览</h2>
    <n-table :bordered="true" size="small" class="docs-table">
      <thead>
        <tr>
          <th>端点</th>
          <th>方法</th>
          <th>说明</th>
        </tr>
      </thead>
      <tbody>
        <tr><td><code>/oauth/authorize</code></td><td>GET</td><td>授权确认页（浏览器跳转）</td></tr>
        <tr><td><code>/oauth/authorize/decision</code></td><td>POST</td><td>用户同意 / 拒绝授权</td></tr>
        <tr><td><code>/oauth/token</code></td><td>POST</td><td>授权码换 access_token（form-encoded）</td></tr>
        <tr><td><code>/oauth/userinfo</code></td><td>GET</td><td>Bearer 取用户信息（user_id/sub/stats）</td></tr>
        <tr>
          <td><code>/.well-known/oauth-authorization-server</code></td>
          <td>GET</td>
          <td>RFC 8414 元数据（自动发现）</td>
        </tr>
        <tr><td><code>/api/config</code></td><td>GET</td><td>前端全局配置（公开）</td></tr>
      </tbody>
    </n-table>
  </n-card>
</template>
