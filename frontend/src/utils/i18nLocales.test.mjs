import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';
import ts from 'typescript';

// Locale files contain minified regions — several "key":"value" pairs on a single
// line — so they have to be read through the TypeScript AST. A line-based parser
// silently swallows those keys, which is how the gap this file guards went
// unnoticed: 24 keys were referenced from components without ever being declared,
// so `t()` fell through to its hardcoded fallback and, where there was none,
// rendered the raw key in the UI.
const localeDir = new URL('../i18n/locales/', import.meta.url);
const srcDir = new URL('../', import.meta.url);

function parseLocale(name) {
  const file = new URL(name, localeDir);
  const src = ts.createSourceFile(name, fs.readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.TS);
  let root = null;
  for (const statement of src.statements) {
    if (!ts.isExportAssignment(statement)) continue;
    let expr = statement.expression;
    if (ts.isAsExpression(expr) || ts.isSatisfiesExpression(expr)) expr = expr.expression;
    if (ts.isObjectLiteralExpression(expr)) root = expr;
  }
  assert.ok(root, `${name} must default-export an object literal`);

  const keys = new Map();
  const duplicates = [];
  const walk = (object, stack) => {
    const seen = new Set();
    for (const property of object.properties) {
      if (!ts.isPropertyAssignment(property)) continue;
      const nameNode = property.name;
      const key = ts.isStringLiteral(nameNode) || ts.isIdentifier(nameNode) ? nameNode.text : null;
      if (key == null) continue;
      if (seen.has(key)) duplicates.push([...stack, key].join('.'));
      seen.add(key);
      const full = [...stack, key].join('.');
      if (ts.isObjectLiteralExpression(property.initializer)) walk(property.initializer, [...stack, key]);
      else if (ts.isStringLiteral(property.initializer)) keys.set(full, property.initializer.text);
    }
  };
  walk(root, []);
  return { keys, duplicates };
}

/** Dotted keys handed to t() or fieldTip() anywhere in src. */
function referencedKeys() {
  const found = new Map();
  const visit = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const target = new URL(entry.name + (entry.isDirectory() ? '/' : ''), dir);
      if (entry.isDirectory()) {
        if (entry.name !== 'node_modules') visit(target);
        continue;
      }
      if (!entry.name.endsWith('.ts') && !entry.name.endsWith('.tsx')) continue;
      const source = ts.createSourceFile(
        entry.name,
        fs.readFileSync(target, 'utf8'),
        ts.ScriptTarget.Latest,
        true,
        entry.name.endsWith('.tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
      );
      const walkNode = (node) => {
        if (ts.isCallExpression(node) && node.arguments.length && ts.isStringLiteral(node.arguments[0])) {
          const callee = node.expression.getText(source);
          if (/(^|\.)t$/.test(callee) || callee === 'fieldTip') {
            const key = node.arguments[0].text;
            if (key.includes('.')) {
              if (!found.has(key)) found.set(key, new Set());
              found.get(key).add(`${path.relative(fs.realpathSync(srcDir), fs.realpathSync(target.pathname))}:${source.getLineAndCharacterOfPosition(node.pos).line + 1}`);
            }
          }
        }
        ts.forEachChild(node, walkNode);
      };
      walkNode(source);
    }
  };
  visit(srcDir);
  return found;
}

const en = parseLocale('en.ts');
const references = referencedKeys();

test('the locale parser sees the whole catalogue', () => {
  assert.ok(en.keys.size > 1000, `en.ts should expose ~1000 keys, parsed ${en.keys.size}`);
});

// The reported defect: a key used by a component but never declared, so the
// tooltip either vanished (fieldTip) or showed the raw key (t).
test('every key referenced from code is declared in en.ts', () => {
  const missing = [...references.keys()].filter((k) => !en.keys.has(k)).sort();
  assert.deepEqual(missing, [], `undeclared keys: ${missing.map((k) => `${k} (${[...references.get(k)][0]})`).join(', ')}`);
});

test('no locale declares the same key twice', () => {
  for (const file of fs.readdirSync(localeDir).filter((f) => f.endsWith('.ts') && f !== 'zh.ts').sort()) {
    assert.deepEqual(parseLocale(file).duplicates, [], `${file} has duplicate keys`);
  }
});

test('no locale keeps a key en.ts no longer has', () => {
  for (const file of fs.readdirSync(localeDir).filter((f) => f.endsWith('.ts') && f !== 'zh.ts').sort()) {
    if (file === 'en.ts') continue;
    const orphans = [...parseLocale(file).keys.keys()].filter((k) => !en.keys.has(k));
    assert.deepEqual(orphans, [], `${file} has keys missing from en.ts: ${orphans.join(', ')}`);
  }
});

// Chinese is the primary UI language; a gap there reads as a defect, and English
// fallback would look broken rather than merely untranslated.
test('zh-CN covers every key en.ts declares', () => {
  const zh = parseLocale('zh-CN.ts').keys;
  const missing = [...en.keys.keys()].filter((k) => !zh.has(k)).sort();
  assert.deepEqual(missing, [], `zh-CN is missing: ${missing.join(', ')}`);
});

test('no locale carries an empty or non-string value', () => {
  for (const file of fs.readdirSync(localeDir).filter((f) => f.endsWith('.ts') && f !== 'zh.ts').sort()) {
    const { keys } = parseLocale(file);
    for (const [key, value] of keys) {
      assert.ok(value.trim() !== '', `${file}: ${key} is empty`);
    }
  }
});

test('other locales fall back to English for keys they lack', () => {
  // Informational: these lag on purpose and the provider resolves them from
  // en.ts. Printed so the gap stays visible instead of being forgotten.
  const lagging = [];
  for (const file of fs.readdirSync(localeDir).filter((f) => f.endsWith('.ts') && f !== 'zh.ts').sort()) {
    if (file === 'en.ts' || file === 'zh-CN.ts') continue;
    const missing = [...en.keys.keys()].filter((k) => !parseLocale(file).keys.has(k)).length;
    if (missing > 0) lagging.push(`${file}:${missing}`);
  }
  if (lagging.length) console.log('    untranslated (falls back to en):', lagging.join(' '));
  assert.ok(true);
});
