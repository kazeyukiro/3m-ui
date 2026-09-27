#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const localesDir = path.join(root, 'frontend/src/i18n/locales');
const outDir = path.join(root, 'frontend/src/i18n/crowdin');

function loadTsObject(filePath) {
  let src = fs.readFileSync(filePath, 'utf8');
  src = src.replace(/^\s*export\s+default\s+/, 'module.exports = ');
  src = src.replace(/\s+as\s+const\s*;?\s*$/m, ';');
  src = src.replace(/export\s*\{\s*default\s*\}\s*;?\s*$/m, '');
  const sandbox = { module: { exports: {} }, exports: {} };
  vm.runInNewContext(src, sandbox, { filename: filePath, timeout: 5000 });
  return sandbox.module.exports;
}

fs.mkdirSync(outDir, { recursive: true });
for (const f of fs.readdirSync(localesDir).filter((x) => x.endsWith('.ts') && x !== 'zh.ts')) {
  const locale = f.replace(/\.ts$/, '');
  const obj = loadTsObject(path.join(localesDir, f));
  const out = path.join(outDir, `${locale}.json`);
  fs.writeFileSync(out, JSON.stringify(obj, null, 2) + '\n');
  console.log('wrote', path.relative(root, out));
}
