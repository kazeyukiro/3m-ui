import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
import ts from 'typescript';

// Executes the real form builder instead of reimplementing the rule. The bug
// being guarded lived in the ordering inside that function: the flow was written
// into the config before the transport was consulted, so "cleared for
// ws/grpc/xhttp" only ever edited the form state and the saved listener — and
// therefore every share link built from it — kept advertising a flow no client
// can use.
const source = ts.createSourceFile(
  'ListenerConfigFields.tsx',
  fs.readFileSync(new URL('../components/ListenerConfigFields.tsx', import.meta.url), 'utf8'),
  ts.ScriptTarget.Latest,
  true,
  ts.ScriptKind.TSX,
);

// Pure data and helpers only: every top-level `new Set([...])` / array literal
// and every plain helper function. React components, hooks and anything touching
// JSX are deliberately left out of the harness.
const pieces = [];
ts.forEachChild(source, (node) => {
  // Skip React components: capitalised, or containing JSX tags. Type
  // annotations like Record<string, any> also contain '<', so match tags only.
  const isPlainFunction =
    ts.isFunctionDeclaration(node) &&
    !node.name?.getText(source).match(/^[A-Z]/) &&
    !node.getText(source).includes('</') &&
    !node.getText(source).includes('/>');
  const isDataStatement =
    ts.isVariableStatement(node) &&
    (node.getText(source).includes('new Set(') || /^const [A-Z_]+ = \[/.test(node.getText(source)));
  if (!isPlainFunction && !isDataStatement) return;
  // `export function` is only valid in a module; this harness runs a script.
  pieces.push(node.getText(source).replace(/^export\s+/, ''));
});
assert.ok(pieces.length > 10, `expected the builder's dependencies, got ${pieces.length} pieces`);

const javascript = ts.transpileModule(
  `${pieces.join('\n')}\nglobalThis.formValuesToConfig = formValuesToConfig;`,
  { compilerOptions: { target: ts.ScriptTarget.ES2022 } },
).outputText;

const sandbox = {};
vm.createContext(sandbox);
vm.runInContext(javascript, sandbox);
const formValuesToConfig = sandbox.formValuesToConfig;
assert.equal(typeof formValuesToConfig, 'function', 'form builder must load');

const baseValues = (overrides) => ({
  transport_layer: 'raw',
  security_layer: 'tls',
  flow: 'xtls-rprx-vision',
  ...overrides,
});

test('flow survives on a raw/TCP VLESS listener', () => {
  const cfg = formValuesToConfig('vless', baseValues({}));
  assert.equal(cfg.flow, 'xtls-rprx-vision', 'raw transport must keep flow');
});

test('flow is dropped for ws, grpc and xhttp', () => {
  const cases = [
    ['ws', { transport_layer: 'ws', 'ws-path': '/ws' }],
    ['grpc', { transport_layer: 'grpc', 'grpc-service-name': 'svc' }],
    ['xhttp', { transport_layer: 'xhttp', xhttp_config_path: '/x' }],
  ];
  for (const [name, overrides] of cases) {
    const cfg = formValuesToConfig('vless', baseValues(overrides));
    assert.equal(cfg.flow, undefined, `${name} must not carry flow into the saved config`);
    assert.equal('flow' in cfg, false, `${name} must not emit a flow key at all`);
  }
});

test('flow is dropped for mkcp and mekya transports', () => {
  for (const flag of ['mkcp_enabled', 'mekya_enabled']) {
    const cfg = formValuesToConfig('vless', baseValues({ [flag]: true }));
    assert.equal('flow' in cfg, false, `${flag} must not carry flow`);
  }
});

test('a flow arriving from the previous config is dropped for ws', () => {
  const cfg = formValuesToConfig(
    'vless',
    baseValues({ transport_layer: 'ws', 'ws-path': '/ws' }),
    { flow: 'xtls-rprx-vision', 'ws-path': '/ws' },
  );
  assert.equal('flow' in cfg, false, 'a stale flow must not survive a transport change');
});

test('an unknown transport keeps flow, so TCP listeners are unaffected', () => {
  // The form defaults to raw when the layer is not recorded; failing that way
  // is deliberate, since a TCP listener must not lose its Vision flow.
  const cfg = formValuesToConfig('vless', baseValues({ transport_layer: undefined }));
  assert.equal(cfg.flow, 'xtls-rprx-vision');
});
