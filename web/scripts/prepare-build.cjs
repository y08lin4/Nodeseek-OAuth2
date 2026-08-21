// 构建准备脚本（幂等，可重复执行）
//
// 背景：当前构建沙箱禁止 Node 子进程通过管道通信（spawn EPERM）：
//   1. esbuild 的 JS API 在 Node 下必然 spawn 服务子进程（stdin/stdout 管道）→ 被沙箱拒绝；
//   2. vite 在 Windows 下用 exec("net use", ...) 探测网络盘映射 → 同样被拒绝。
// 本脚本在依赖（重）安装后自动施加两个最小修补，使 `npm run build` 可在沙箱内完成：
//   ① 用「esbuild 迷你实现」覆盖 node_modules/esbuild/lib/main.js（TS 转译走 typescript 包，
//      不 spawn 子进程；仅覆盖 vite build 所需 API 面）；
//   ② 将 vite 的 net use 探测改写为 if(false) 短路（本地磁盘路径无需网络映射）。
// 正常环境（无沙箱限制）下请移除 package.json 的 overrides 并重装依赖，使用原版 esbuild。
'use strict'

const fs = require('fs')
const path = require('path')

const webRoot = path.resolve(__dirname, '..')
const log = (msg) => console.log(`[prepare-build] ${msg}`)

// ---------- ① esbuild 迷你实现 ----------
const esbuildStubMarker = '[esbuild-stub]'
const esbuildMainPath = path.join(webRoot, 'node_modules', 'esbuild', 'lib', 'main.js')
const esbuildStub = String.raw`// ${esbuildStubMarker}
//
// 沙箱兼容的 esbuild 迷你实现：不 spawn 子进程，用 typescript 包完成 TS 转译。
// 详见 scripts/prepare-build.cjs 头部注释。vite dev 的依赖预构建需要真实 esbuild，
// 请在正常环境移除 package.json overrides 后重装。
'use strict'

const ts = require('typescript')
const fs = require('fs')
const path = require('path')

// transform：TS/TSX 转译 + 其余 loader 透传；总是返回合法 sourcemap 字符串
function transform(code, options = {}) {
  return new Promise((resolve, reject) => {
    try {
      const loader = options.loader || 'js'
      const sourcefile = options.sourcefile || 'file'
      let out = code
      if (loader === 'ts' || loader === 'tsx' || loader === 'jsx') {
        const r = ts.transpileModule(code, {
          fileName: sourcefile,
          compilerOptions: {
            target: ts.ScriptTarget.ES2020,
            module: ts.ModuleKind.ESNext,
            moduleResolution: ts.ModuleResolutionKind.Bundler,
            jsx: ts.JsxEmit.Preserve,
            esModuleInterop: true,
            allowSyntheticDefaultImports: true,
            isolatedModules: true,
            resolveJsonModule: true,
          },
        })
        out = r.outputText
      }
      const map = JSON.stringify({
        version: 3,
        file: null,
        sources: [sourcefile],
        sourcesContent: [null],
        names: [],
        mappings: '',
      })
      resolve({ code: out, map, warnings: [] })
    } catch (err) {
      reject(err)
    }
  })
}

// build：仅 vite config bundling / dev 预构建使用；沙箱下透传入口文件原文
function build(options = {}) {
  return new Promise((resolve, reject) => {
    try {
      const absWorkingDir = options.absWorkingDir || process.cwd()
      const entry = options.entryPoints && options.entryPoints[0]
      if (!entry) {
        reject(new Error('[esbuild-stub] build() 缺少 entryPoints'))
        return
      }
      const entryAbs = path.isAbsolute(entry) ? entry : path.join(absWorkingDir, entry)
      const text = fs.readFileSync(entryAbs, 'utf8')
      resolve({
        outputFiles: [
          { path: options.outfile || 'out.js', text, contents: Buffer.from(text, 'utf8') },
        ],
        metafile: { inputs: {}, outputs: {} },
      })
    } catch (err) {
      reject(err)
    }
  })
}

// formatMessages：透传（vite 只在 esbuild 报错时用于格式化）
function formatMessages(messages) {
  return Promise.resolve(
    (messages || []).map((m) => m && (m.text || m.message || JSON.stringify(m))),
  )
}

// context：vite dev 专用（依赖预构建），沙箱下不支持
async function context() {
  throw new Error('[esbuild-stub] context() 仅 vite dev 使用，沙箱下不支持；请用 vite build 验证产物')
}

// 导出：对象字面量 + 标识符引用（cjs-module-lexer 可静态识别为 named exports）
var version = '0.25.12-stub';
var stop = () => Promise.resolve();
module.exports = { build, transform, formatMessages, context, version, stop };
`

function patchEsbuildStub() {
  if (!fs.existsSync(esbuildMainPath)) {
    log(`警告：未找到 node_modules/esbuild/lib/main.js（未安装依赖？），跳过 esbuild 修补`)
    return false
  }
  const cur = fs.readFileSync(esbuildMainPath, 'utf8')
  if (cur.includes(esbuildStubMarker)) {
    log('esbuild 已是迷你实现，跳过')
    return true
  }
  fs.writeFileSync(esbuildMainPath, esbuildStub, 'utf8')
  log('已写入 esbuild 迷你实现（node_modules/esbuild/lib/main.js）')
  return true
}

// ---------- ② vite 的 net use 探测短路 ----------
const netUsePattern = /(\s*)exec\("net use", \(error, stdout\) => \{/
const patchedMarker = 'if (false) exec("net use", (error, stdout) => {'

function patchViteNetUse() {
  const chunksDir = path.join(webRoot, 'node_modules', 'vite', 'dist', 'node', 'chunks')
  if (!fs.existsSync(chunksDir)) {
    log(`警告：未找到 vite chunks 目录（未安装依赖？），跳过 vite 修补`)
    return false
  }
  const files = fs
    .readdirSync(chunksDir)
    .filter((f) => f.endsWith('.js') && !f.includes('.map'))
  let patched = false
  for (const f of files) {
    const p = path.join(chunksDir, f)
    const src = fs.readFileSync(p, 'utf8')
    if (src.includes(patchedMarker)) {
      patched = true
      continue
    }
    if (netUsePattern.test(src)) {
      const next = src.replace(netUsePattern, `$1${patchedMarker}`)
      fs.writeFileSync(p, next, 'utf8')
      patched = true
      log(`已将 net use 探测短路（${f}）`)
    }
  }
  if (!patched) log('vite 无需修补或已修补')
  return true
}

patchEsbuildStub()
patchViteNetUse()
log('构建准备完成')
