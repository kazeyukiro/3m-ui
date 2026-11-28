import assert from 'node:assert/strict';
import fs from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import ts from 'typescript';
import vm from 'node:vm';

const source = fs.readFileSync(new URL('./logFilter.ts', import.meta.url), 'utf8');
const javascript = ts.transpileModule(source, {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, esModuleInterop: true },
}).outputText;
const module = { exports: {} };
vm.runInNewContext(javascript, {
  module,
  exports: module.exports,
  require: createRequire(import.meta.url),
});
const { filterLogs } = module.exports;

const logs = [
  { timestamp: 't1', level: 'info', payload: 'core started' },
  { timestamp: 't2', level: 'error', payload: 'connection refused' },
  { timestamp: 't3', level: 'warn', payload: 'high latency' },
  { timestamp: 't4', level: 'debug', payload: 'heartbeat sent' },
  { timestamp: 't5', level: 'fatal', payload: 'panic occurred' },
];

test('returns everything with no search and level=all', () => {
  assert.equal(filterLogs(logs, '', 'all').length, 5);
});

test('search matches case-insensitively on payload substring', () => {
  const r = filterLogs(logs, 'LATENCY', 'all');
  assert.equal(r.length, 1);
  assert.equal(r[0].timestamp, 't3');
});

test('level=info matches info and debug', () => {
  const r = filterLogs(logs, '', 'info');
  assert.deepEqual(r.map((l) => l.timestamp).sort(), ['t1', 't4']);
});

test('level=warning matches warn', () => {
  const r = filterLogs(logs, '', 'warning');
  assert.deepEqual(r.map((l) => l.timestamp), ['t3']);
});

test('level=error matches error and fatal', () => {
  const r = filterLogs(logs, '', 'error');
  assert.deepEqual(r.map((l) => l.timestamp).sort(), ['t2', 't5']);
});

test('search and level combine with AND', () => {
  const r = filterLogs(logs, 'refus', 'error');
  assert.deepEqual(r.map((l) => l.timestamp), ['t2']);
});

test('empty result when nothing matches', () => {
  assert.equal(filterLogs(logs, 'zzz', 'all').length, 0);
});
