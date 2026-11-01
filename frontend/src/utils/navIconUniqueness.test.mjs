import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';

// The sidebar once rendered 出站管理 and 路由规则 with the very same component
// (both pointed at IconNavRouting), so two unrelated destinations looked
// identical. A missing glyph is easy to spot; a *shared* glyph is not, so it
// needs a guard: every top-level destination must own a distinct semantic icon.

const srcDir = new URL('../', import.meta.url);

function read(rel) {
  return fs.readFileSync(new URL(rel, srcDir), 'utf8');
}

/** Extract `{ key: '...', icon: <IconX /> ... }` pairs from a menu definition. */
function menuIcons(rel) {
  const src = read(rel);
  const pairs = [...src.matchAll(/\{\s*key:\s*'([^']+)',\s*icon:\s*<(Icon[A-Za-z0-9]+)\s*\/>/g)];
  return pairs.map(([, key, icon]) => ({ key, icon }));
}

test('desktop sidebar gives every destination its own icon', () => {
  const items = menuIcons('components/Sidebar.tsx');
  assert.ok(items.length >= 12, `expected the full nav list, parsed ${items.length}`);

  const byIcon = new Map();
  for (const { key, icon } of items) {
    const seen = byIcon.get(icon);
    assert.equal(seen, undefined, `${icon} is reused by both ${seen} and ${key}`);
    byIcon.set(icon, key);
  }
});

test('mobile bottom nav gives every destination its own icon', () => {
  const items = menuIcons('components/MobileBottomNav.tsx');
  assert.ok(items.length >= 10, `expected the mobile nav list, parsed ${items.length}`);

  const byIcon = new Map();
  for (const { key, icon } of items) {
    const seen = byIcon.get(icon);
    assert.equal(seen, undefined, `${icon} is reused by both ${seen} and ${key}`);
    byIcon.set(icon, key);
  }
});

test('desktop and mobile nav agree on the icon for the same destination', () => {
  const desktop = new Map(menuIcons('components/Sidebar.tsx').map((i) => [i.key, i.icon]));
  const mobile = new Map(menuIcons('components/MobileBottomNav.tsx').map((i) => [i.key, i.icon]));

  for (const [key, icon] of mobile) {
    if (!desktop.has(key)) continue;
    assert.equal(desktop.get(key), icon, `${key} uses ${desktop.get(key)} on desktop but ${icon} on mobile`);
  }
});

test('no two semantic icons wrap the same underlying glyph', () => {
  const src = read('icons.tsx');
  const byGlyph = new Map();
  for (const m of src.matchAll(/export const (Icon[A-Za-z0-9]+) = wrap\(([A-Za-z0-9]+),/g)) {
    const [, name, glyph] = m;
    const seen = byGlyph.get(glyph);
    assert.equal(seen, undefined, `${glyph} is wrapped twice (${seen} and ${name})`);
    byGlyph.set(glyph, name);
  }
  assert.ok(byGlyph.size > 50, `expected the icon table, parsed ${byGlyph.size}`);
});
