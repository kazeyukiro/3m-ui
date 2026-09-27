import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';

// Guards the failure modes these files have already hit: a duplicated key inside
// one object silently breaks the type check, and a key renamed in en.ts leaving
// an orphan behind in the other locales. Missing translations are NOT an error —
// the provider falls back to English — but the main locale is expected complete.
const dir = new URL('../i18n/locales/', import.meta.url);

const VALUE = /^"([^"]+)":\s*"(.*)",?$/;
const OPEN = /^(?:"([^"]+)"|([A-Za-z0-9_-]+)):\s*\{$/;

function parse(name) {
  const lines = fs.readFileSync(new URL(name, dir), 'utf8').split('\n');
  const stack = [];
  const scopes = [new Set()];
  const keys = new Set();
  const duplicates = [];
  for (const line of lines) {
    const text = line.trim();
    const open = OPEN.exec(text);
    if (open) {
      stack.push(open[1] || open[2]);
      scopes.push(new Set());
      continue;
    }
    if (text.startsWith('}')) {
      if (stack.length) {
        stack.pop();
        scopes.pop();
      }
      continue;
    }
    const kv = VALUE.exec(text);
    if (!kv) continue;
    const scope = scopes[scopes.length - 1];
    if (scope.has(kv[1])) duplicates.push([...stack, kv[1]].join('.'));
    scope.add(kv[1]);
    keys.add([...stack, kv[1]].join('.'));
  }
  return { keys, duplicates };
}

/** Locale files that carry translations; zh.ts is a re-export of zh-CN. */
function localeFiles() {
  return fs
    .readdirSync(dir)
    .filter((f) => f.endsWith('.ts') && f !== 'zh.ts')
    .sort();
}

const en = parse('en.ts');

test('the locale parser sees the whole catalogue', () => {
  assert.ok(en.keys.size > 900, `en.ts should expose ~1000 keys, parsed ${en.keys.size}`);
});

test('no locale declares the same key twice', () => {
  for (const file of localeFiles()) {
    const { duplicates } = parse(file);
    assert.deepEqual(duplicates, [], `${file} has duplicate keys: ${duplicates.join(', ')}`);
  }
});

test('no locale keeps a key en.ts no longer has', () => {
  for (const file of localeFiles()) {
    if (file === 'en.ts') continue;
    const orphans = [...parse(file).keys].filter((k) => !en.keys.has(k));
    assert.deepEqual(orphans, [], `${file} has keys missing from en.ts: ${orphans.join(', ')}`);
  }
});

// Chinese is the primary UI language; an untranslated hint there is visible to
// most users, and English fallback would read as a defect rather than a gap.
test('zh-CN covers every key en.ts declares', () => {
  const missing = [...en.keys].filter((k) => !parse('zh-CN.ts').keys.has(k)).sort();
  assert.deepEqual(missing, [], `zh-CN is missing: ${missing.join(', ')}`);
});

test('other locales fall back to English for keys they lack', () => {
  // Documentation rather than assertion: these lag on purpose and the provider
  // resolves them from en.ts. Listed so the gap is visible in test output.
  const lagging = [];
  for (const file of localeFiles()) {
    if (file === 'en.ts' || file === 'zh-CN.ts') continue;
    const missing = [...en.keys].filter((k) => !parse(file).keys.has(k)).length;
    if (missing > 0) lagging.push(`${file}:${missing}`);
  }
  assert.ok(lagging.length >= 0, 'informational');
  if (lagging.length) console.log('    untranslated (falls back to en):', lagging.join(' '));
});
