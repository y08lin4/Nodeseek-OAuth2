// copy-static.mjs —— 构建脚本（tsc 之后运行）
// 1) 把 manifest.json / popup.html / options.html 复制到 dist/；
// 2) 校验编译产物：manifest 引用的 JS（background.js / web-bridge.js / pm-fill.js）
//    必须存在于 dist/ 且非空，保证 content_scripts 路径与 dist 产物一致。
// 仅使用 Node 内置 fs，无第三方依赖。
import { cpSync, existsSync, mkdirSync, statSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const dist = join(root, 'dist');
mkdirSync(dist, { recursive: true });

// 1) 复制静态文件
const staticFiles = ['manifest.json', 'popup.html', 'options.html'];
for (const f of staticFiles) {
  cpSync(join(root, f), join(dist, f));
  console.log(`已复制: ${f}`);
}

// 2) 校验 manifest 引用的 JS 编译产物（tsc 已输出到 dist）
const requiredJs = ['background.js', 'web-bridge.js', 'pm-fill.js'];
const missing = requiredJs.filter((f) => {
  const p = join(dist, f);
  return !existsSync(p) || statSync(p).size === 0;
});
if (missing.length > 0) {
  console.error(`构建失败：dist/ 缺少编译产物 ${missing.join(', ')}`);
  process.exit(1);
}
console.log(`校验通过: ${requiredJs.join(', ')}`);
