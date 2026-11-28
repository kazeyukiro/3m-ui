import assert from 'node:assert/strict';
import fs from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import vm from 'node:vm';
import ts from 'typescript';

const require = createRequire(import.meta.url);
const source = fs.readFileSync(new URL('./routingRules.ts', import.meta.url), 'utf8');
const javascript = ts.transpileModule(source, {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, esModuleInterop: true },
}).outputText;

function load() {
  const module = { exports: {} };
  vm.runInNewContext(
    `${javascript}\nmodule.exports = { parseRuleLine, serializeRule, parseRulesText, serializeRules, PARAM_RULE_TYPES: exports.PARAM_RULE_TYPES, COMMA_PAYLOAD_TYPES: exports.COMMA_PAYLOAD_TYPES };`,
    { module, exports: module.exports, require },
  );
  return module.exports;
}

const { parseRuleLine, serializeRule, parseRulesText, serializeRules } = load();

test('comma-payload rule types round-trip as raw lines (never mangled)', () => {
  const lines = [
    'AND,((DOMAIN,baidu.com),(NETWORK,UDP)),DIRECT',
    'OR,((NETWORK,UDP),(DOMAIN,baidu.com)),REJECT',
    'NOT,((DOMAIN,baidu.com)),PROXY',
    'SUB-RULE,(NETWORK,tcp),sub-rule',
    'DOMAIN-REGEX,^abc.*com,PROXY',
    'PROCESS-NAME-REGEX,(?i)Telegram,PROXY',
    'PROCESS-PATH-REGEX,.*bin/wget,PROXY',
  ];
  for (const line of lines) {
    const row = parseRuleLine(line);
    assert.equal(row.raw, line, `${line} must be kept raw`);
    assert.equal(serializeRule(row), line, `${line} must round-trip unchanged`);
  }
});

test('no-resolve round-trips on every param rule type', () => {
  for (const type of ['IP-CIDR', 'IP-CIDR6', 'SRC-IP-CIDR', 'IP-SUFFIX', 'SRC-IP-SUFFIX', 'GEOIP', 'SRC-GEOIP', 'IP-ASN', 'SRC-IP-ASN', 'RULE-SET']) {
    const payload = type.startsWith('IP-') || type === 'RULE-SET' ? 'x' : 'CN';
    const line = type === 'RULE-SET' ? 'RULE-SET,gfw,PROXY,no-resolve' : `${type},${payload},DIRECT,no-resolve`;
    const row = parseRuleLine(line);
    assert.equal(row.noResolve, true, `${line} must set noResolve`);
    assert.equal(serializeRule(row), line, `${line} must round-trip`);
  }
});

test('src param round-trips (mihomo treats src as implying no-resolve)', () => {
  const line = 'IP-CIDR,10.0.0.0/8,DIRECT,src';
  const row = parseRuleLine(line);
  assert.equal(row.param, 'src');
  assert.equal(row.noResolve, false);
  assert.equal(serializeRule(row), line);
});

test('structured rules still round-trip', () => {
  const lines = [
    'DOMAIN-SUFFIX,example.com,PROXY',
    'GEOIP,CN,DIRECT,no-resolve',
    'MATCH,PROXY',
    'DST-PORT,80/443,DIRECT',
    'IN-TYPE,SOCKS/HTTP,PROXY',
    'NETWORK,UDP,DIRECT',
  ];
  for (const line of lines) {
    assert.equal(serializeRule(parseRuleLine(line)), line, `${line} must round-trip`);
  }
});

test('parseRulesText/serializeRules preserve an imported batch', () => {
  const text = [
    '# comment',
    '',
    'DOMAIN-SUFFIX,example.com,PROXY',
    'AND,((DOMAIN,a.com),(NETWORK,UDP)),DIRECT',
    'GEOIP,CN,DIRECT,no-resolve',
  ].join('\n');
  const rows = parseRulesText(text);
  assert.equal(rows.length, 3);
  // Spread into this realm: values created inside the vm context carry a
  // different Array prototype and fail strict deepEqual on that alone.
  assert.deepEqual([...serializeRules(rows)], [
    'DOMAIN-SUFFIX,example.com,PROXY',
    'AND,((DOMAIN,a.com),(NETWORK,UDP)),DIRECT',
    'GEOIP,CN,DIRECT,no-resolve',
  ]);
});
