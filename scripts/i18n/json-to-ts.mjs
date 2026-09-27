#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const crowdinDir = path.join(root, 'frontend/src/i18n/crowdin');
const localesDir = path.join(root, 'frontend/src/i18n/locales');

for (const f of fs.readdirSync(crowdinDir).filter((x) => x.endsWith('.json'))) {
  const locale = f.replace(/\.json$/, '');
  const obj = JSON.parse(fs.readFileSync(path.join(crowdinDir, f), 'utf8'));
  const body = JSON.stringify(obj, null, 2);
  fs.writeFileSync(path.join(localesDir, `${locale}.ts`), `export default ${body} as const;\n`);
  console.log('wrote', path.relative(root, path.join(localesDir, `${locale}.ts`)));
}
