import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

const src = new URL('../apps/web/src/', import.meta.url).pathname;
const vue = (dir) =>
  readdirSync(join(src, dir))
    .filter((name) => name.endsWith('.vue'))
    .map((name) => join(dir, name));
const files = ['style.css', 'App.vue', ...vue('components'), ...vue('components/ui')];
// Translucent shadows and backdrops read the same on both themes; opaque colors must be tokens.
const opaque = /(?<!&)#(?:[0-9a-fA-F]{6}|[0-9a-fA-F]{3})\b/g;

test('interface colors resolve through theme tokens', () => {
  const literals = [];
  for (const file of files) {
    const text = readFileSync(join(src, file), 'utf8');
    text.split('\n').forEach((line, index) => {
      // Email previews render in their own light document.
      if (line.includes('<!doctype html>')) return;
      for (const match of line.matchAll(opaque))
        if (!/url\(#|href="#/.test(line)) literals.push(`${file}:${index + 1} ${match[0]}`);
    });
  }
  assert.deepEqual(literals, []);
});

test('every referenced color token is defined with a light and a dark value', () => {
  const tokens = readFileSync(join(src, 'tokens.css'), 'utf8');
  const defined = new Set([...tokens.matchAll(/^\s*(--[a-z0-9-]+):/gm)].map((m) => m[1]));
  const families = /^--(gray|violet|blue|teal|green|amber|red|pink)-\d/;
  for (const [, name, value] of tokens.matchAll(/^\s*(--[a-z0-9-]+):\s*([^;]+);/gm))
    if (families.test(name))
      assert.match(value, /^light-dark\(#[0-9a-f]{6}, #[0-9a-f]{6}\)$/, name);
  const missing = new Set();
  for (const file of files)
    for (const [, name] of readFileSync(join(src, file), 'utf8').matchAll(/var\((--[a-z0-9-]+)/g))
      if (
        (families.test(name) || /^--(page|surface|on-solid|accent|edge|chart|canvas)/.test(name)) &&
        !defined.has(name)
      )
        missing.add(name);
  assert.deepEqual([...missing], []);
});
