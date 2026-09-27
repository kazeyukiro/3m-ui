#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const localesDir = path.join(root, 'frontend/src/i18n/locales');

function load(filePath) {
  let src = fs.readFileSync(filePath, 'utf8');
  src = src.replace(/^\s*export\s+default\s+/, 'module.exports = ');
  src = src.replace(/\s+as\s+const\s*;?\s*$/m, ';');
  const sandbox = { module: { exports: {} }, exports: {} };
  vm.runInNewContext(src, sandbox, { filename: filePath, timeout: 5000 });
  return sandbox.module.exports;
}

function flatten(obj, prefix = '', out = {}) {
  if (obj == null || typeof obj !== 'object' || Array.isArray(obj)) {
    out[prefix] = obj;
    return out;
  }
  for (const [k, v] of Object.entries(obj)) {
    const p = prefix ? `${prefix}.${k}` : k;
    if (v != null && typeof v === 'object' && !Array.isArray(v)) flatten(v, p, out);
    else out[p] = v;
  }
  return out;
}

const enKeys = Object.keys(flatten(load(path.join(localesDir, 'en.ts')))).sort();
let failed = false;
for (const f of fs.readdirSync(localesDir).filter((x) => x.endsWith('.ts') && x !== 'en.ts' && x !== 'zh.ts')) {
  const keys = new Set(Object.keys(flatten(load(path.join(localesDir, f)))));
  const missing = enKeys.filter((k) => !keys.has(k));
  const extra = [...keys].filter((k) => !(k in Object.fromEntries(enKeys.map((x) => [x, 1]))));
  // fix extra check
  const enSet = new Set(enKeys);
  const extra2 = [...keys].filter((k) => !enSet.has(k));
  if (missing.length || extra2.length) {
    failed = true;
    console.log(`\n${f}: missing ${missing.length}, extra ${extra2.length}`);
    missing.slice(0, 15).forEach((k) => console.log('  -', k));
  } else {
    console.log(`${f}: OK (${enKeys.length} keys)`);
  }
}
process.exit(failed ? 1 : 0);
