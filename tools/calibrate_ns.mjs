#!/usr/bin/env node
// NodeSeek API 校准验证脚本：验证已实测的端点并打印响应结构。
// 用法：node tools/calibrate_ns.mjs "<完整Cookie字符串>"
//   或：$env:NS_COOKIE="<Cookie>" ; node tools/calibrate_ns.mjs
// 端点已实测确认（2026-08-21），本脚本用于部署前快速验证 + 查看字段结构。
// 本脚本不写入任何文件、不上传任何数据，仅向 www.nodeseek.com 发起 GET 请求。

const cookie = process.argv[2] || process.env.NS_COOKIE || '';
if (!cookie || !cookie.includes('=')) {
  console.error('用法：node tools/calibrate_ns.mjs "<Cookie字符串>"（或设置环境变量 NS_COOKIE）');
  process.exit(1);
}

const BASE = 'https://www.nodeseek.com';
const UA = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0 Safari/537.36';

// 从 pjwt Cookie 解析用户 id（自动识别逻辑的对照实现）
function parsePjwt() {
  const m = cookie.match(/(?:^|;?\s*)pjwt=([^;]+)/);
  if (!m) return null;
  try {
    const payload = JSON.parse(Buffer.from(m[1].split('.')[1], 'base64url').toString('utf8'));
    return payload; // {id, name, ts}
  } catch { return null; }
}

async function get(path) {
  try {
    const res = await fetch(BASE + path, {
      headers: {
        Cookie: cookie,
        'User-Agent': UA,
        Accept: 'application/json, text/plain, */*',
        'X-Requested-With': 'XMLHttpRequest',
      },
      signal: AbortSignal.timeout(20000),
    });
    const text = await res.text();
    return { status: res.status, text };
  } catch (e) {
    return { error: String(e?.cause?.message || e?.message || e) };
  }
}

console.log('==== 0) pjwt 解析（自动识别依据） ====');
const me = parsePjwt();
console.log(me ? JSON.stringify(me) : '（Cookie 中未找到 pjwt）');
console.log('');

const uid = (me && me.id) || '37384';
console.log(`==== 1) 用户信息 GET /api/account/getInfo/${uid} ====`);
const u = await get(`/api/account/getInfo/${uid}`);
console.log(u.status, u.error || '');
if (u.text) console.log(u.text.slice(0, 800));
console.log('');

console.log('==== 2) 私信列表 GET /api/notification/message/list ====');
const m = await get('/api/notification/message/list');
console.log(m.status, m.error || '');
if (m.text) console.log(m.text.slice(0, 800));
console.log('');

console.log('==== 结论 ====');
console.log('1 正常 → NS_NS_API_USER_URL=https://www.nodeseek.com/api/account/getInfo/{user_id} 可用');
console.log('2 正常 → NS_NS_API_MESSAGE_URL=https://www.nodeseek.com/api/notification/message/list 可用');
console.log('403 "Just a moment" → 当前 IP 被 Cloudflare 挑战（部署机器需干净 IP，见 docs/DEPLOYMENT.md）');
