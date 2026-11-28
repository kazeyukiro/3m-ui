#!/usr/bin/env node
/**
 * 把 Sub-Store 的 parse + produce 核心打成自包含单文件 bundle。
 * 产物暴露全局对象 SubStoreCore: { parse(raw), produce(proxies, target, opts) }
 * 不依赖任何 Node 内置模块 / 文件系统 / 网络，可在 goja 中直接执行。
 */
const path = require('path');
const fs = require('fs');

// 用法: node scripts/substore/build-bundle.js <substore-backend-dir> [output-file]
// 默认从环境变量 SUBSTORE_BACKEND 或 ../sub-store/backend 读取 Sub-Store 后端源码。
const REPO = path.resolve(__dirname, '../..');
const SUBSTORE = process.argv[2] || process.env.SUBSTORE_BACKEND || path.resolve(REPO, '../sub-store/backend');
const OUT = process.argv[3] || path.join(REPO, 'backend/internal/substore/engine/bundle/substore.js');
const SRC = path.join(SUBSTORE, 'src');
if (!fs.existsSync(SRC)) {
  console.error('Sub-Store backend src not found:', SRC);
  console.error('Pass the Sub-Store backend directory as the first argument.');
  process.exit(1);
}
// esbuild 需从 Sub-Store 后端目录解析（依赖安装在那里）。
const esbuild = require(require.resolve('esbuild', { paths: [SUBSTORE] }));

// 转换核心用不到的重依赖：统一 stub 成安全空实现。
// 这些模块是订阅管理/同步/存储相关，parse+produce 纯转换路径不会走到。
const stubModule = (name) => `
  const noop = function(){ return undefined; };
  var cache = {};
  var stub = new Proxy(noop, {
    get: function(t, p){
      if (p === '__esModule') return true;
      if (p === 'default') return stub;
      if (p === 'MMDB') return {};
      if (p === Symbol.toPrimitive || p === 'toString' || p === 'valueOf') return noop;
      if (p in cache) return cache[p];
      cache[p] = noop;
      return noop;
    },
    apply: function(){ return undefined; }
  });
  module.exports = stub;
  module.exports.default = stub;
`;

const nodeBuiltins = require('module').builtinModules;
const builtinNames = new Set(nodeBuiltins.map((m) => m.replace(/^node:/, '')));

const stubPlugin = {
  name: 'substore-stubs',
  setup(build) {
    // 这些 @/ 路径在 parse/produce 路径不会被真正调用，仅因顶层 import 被拉入
    const stubs = [
      /^@\/utils\/database$/,
      /^@\/restful\/sync$/,
      /^@\/utils\/gist$/,
      /^@\/runtime\/fs$/,
      /^@\/utils\/dns$/,
      /^@\/utils\/geo$/,
      /^@\/core\/app$/,
    ];
    for (const re of stubs) {
      build.onResolve({ filter: re }, (args) => ({ path: args.path, namespace: 'ss-stub' }));
    }
    // 所有 Node 内置模块也 stub 掉（buffer 除外，它需要真正的 polyfill 行为）
    build.onResolve({ filter: /.*/ }, (args) => {
      const normalized = args.path.replace(/^node:/, '');
      if (builtinNames.has(normalized) && normalized !== 'buffer') {
        return { path: args.path, namespace: 'ss-stub' };
      }
    });
    build.onLoad({ filter: /.*/, namespace: 'ss-stub' }, (args) => ({
      contents: stubModule(args.path),
      loader: 'js',
    }));
  },
};

// 让 goja 环境无 process / Buffer 也能跑：注入 shim
const shim = `
  var process = (typeof process !== 'undefined') ? process : { env: {}, version: 'v22.0.0', platform: 'linux', nextTick: function(f){ f(); } };
  if (typeof globalThis !== 'undefined') {
    globalThis.process = globalThis.process || process;
  }
`;

!(async () => {
  const result = await esbuild.build({
    entryPoints: [path.join(SRC, 'core/proxy-utils/index.js')],
    bundle: true,
    write: false,
    format: 'iife',
    globalName: 'SubStoreCoreNS',
    platform: 'neutral',
    target: ['es2016'],
    mainFields: ['browser', 'main', 'module'],
    conditions: ['browser'],
    alias: {
      '@': SRC,
      buffer: require.resolve('buffer/', { paths: [SUBSTORE] }),
    },
    plugins: [stubPlugin],
    inject: [require.resolve('core-js/actual/object/has-own', { paths: [SUBSTORE] })],
    define: {
      'process.env.NODE_ENV': '"production"',
    },
    logLevel: 'info',
  });

  const bundle = result.outputFiles[0].text;
  // 追加导出桥接：把 ProxyUtils 的 parse/produce 暴露为全局 SubStoreCore
  const bridge = `
;(function(){
  var core = (typeof SubStoreCoreNS !== 'undefined' ? SubStoreCoreNS : {});
  var P = core.ProxyUtils || core.default || core;
  if (typeof globalThis !== 'undefined') {
    globalThis.SubStoreCore = {
      parse: function(raw){ return P.parse(raw); },
      produce: function(proxies, target, opts){ return P.produce(proxies, target, opts || {}); }
    };
  }
})();
`;
  fs.mkdirSync(path.dirname(OUT), { recursive: true });
  fs.writeFileSync(OUT, shim + '\n' + bundle + '\n' + bridge, 'utf8');
  console.log('bundle written:', OUT, 'bytes=', fs.statSync(OUT).size);
})().catch((e) => {
  console.error(e);
  process.exit(1);
});
