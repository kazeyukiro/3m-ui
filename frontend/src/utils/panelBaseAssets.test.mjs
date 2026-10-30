import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import test from 'node:test';

// The panel is served under an operator-chosen path prefix (server.web_path),
// which first install randomises — so every static asset URL has to resolve
// relative to that prefix. Vite's base is './' and the backend injects the
// prefix as <meta name="x-web-path">, and the SPA reads it through webPath().
// A root-absolute reference ("/logo.png") bypasses all of that: it resolves to
// the domain root and 404s, which is exactly what shipped — the login card,
// the desktop sidebar and the mobile header all rendered a broken logo on every
// default install, invisible in dev because vite dev serves from the root.
//
// Custom themes are unaffected: this only inspects the bundled SPA sources.
const srcDir = new URL('../', import.meta.url);
const publicDir = new URL('../../public/', import.meta.url);

// Image/font assets only. Route paths ("/listeners"), API prefixes ("/api/v1")
// and anchors ("https://...") are intentionally out of scope — they are not
// fetched relative to the document and are resolved by the router/client.
const ASSET = String.raw`\.(?:png|jpe?g|gif|svg|ico|webp|avif|woff2?|ttf|otf|eot|webmanifest)`;
const PATTERNS = [
  { name: 'src attribute', re: new RegExp(String.raw`\bsrc\s*=\s*["'\`](/[^"'\`]*)`, 'g') },
  { name: 'href attribute', re: new RegExp(String.raw`\bhref\s*=\s*["'\`](/[^"'\`]*)`, 'g') },
  { name: 'css url()', re: /url\(\s*["']?(\/[^"')]*)/g },
];

function sourceFiles(dir) {
  const out = [];
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) out.push(...sourceFiles(full));
    else if (/\.(tsx?|css)$/.test(entry.name) && !entry.name.endsWith('.test.mjs')) out.push(full);
  }
  return out;
}

test('no static asset is referenced by a root-absolute path', () => {
  const offenders = [];
  for (const file of sourceFiles(srcDir.pathname)) {
    const text = fs.readFileSync(file, 'utf8');
    for (const { name, re } of PATTERNS) {
      for (const match of text.matchAll(re)) {
        const value = match[1];
        if (!new RegExp(ASSET + '$').test(value)) continue;
        const line = text.slice(0, match.index).split('\n').length;
        offenders.push(`${path.relative(srcDir.pathname, file)}:${line} ${name}="${value}"`);
      }
    }
  }
  assert.deepEqual(
    offenders,
    [],
    `static assets must be prefixed with webPath() so they resolve under the panel base:\n  ${offenders.join('\n  ')}`,
  );
});

test('the web app manifest uses relative asset URLs', () => {
  const manifest = JSON.parse(fs.readFileSync(new URL('manifest.webmanifest', publicDir), 'utf8'));
  // A manifest's relative URLs resolve against the manifest URL itself, so
  // "logo-192.png" lands under the panel base while "/logo-192.png" would not.
  const bad = [];
  if (manifest.start_url?.startsWith('/')) bad.push(`start_url="${manifest.start_url}"`);
  for (const icon of manifest.icons ?? []) {
    if (icon.src?.startsWith('/')) bad.push(`icon src="${icon.src}"`);
  }
  assert.deepEqual(bad, [], `manifest URLs must be relative: ${bad.join(', ')}`);
});
