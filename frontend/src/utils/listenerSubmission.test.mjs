import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
import ts from 'typescript';

// Execute the page's real submit handler with controlled API/form boundaries.
const page = ts.createSourceFile('Listeners.tsx', fs.readFileSync(new URL('../pages/Listeners.tsx', import.meta.url), 'utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
let handler;
function findHandler(node) {
  if (ts.isVariableDeclaration(node) && node.name.getText(page) === 'onSubmit') handler = node.initializer.getText(page);
  ts.forEachChild(node, findHandler);
}
findHandler(page);
assert.ok(handler, 'listener page must expose its submit handler');
const javascript = ts.transpileModule(`globalThis.submit = ${handler};`, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;

function harness(create) {
  const existing = { id: 1, name: 'existing-node', protocol: 'shadowsocks', port: '1080', enabled: true };
  const values = { name: existing.name, protocol: existing.protocol, port: '2080', enabled: true };
  const observed = { requests: 0, successes: [], errors: [], modalOpen: true, resets: 0, submitError: '' };
  const context = {
    submittingRef: { current: false },
    setSubmitting() {},
    setSubmitError: error => { observed.submitError = error; },
    editing: null, capabilities: null, useCapabilityForm: false,
    REALITY_PROTOCOLS: new Set(),
    form: {
      getFieldsValue: () => ({ ...values }),
      getFieldValue: key => values[key],
      resetFields: () => { observed.resets++; },
    },
    formValuesToConfig: () => ({ cipher: 'aes-128-gcm', password: 'test-password' }),
    protocolSupportsUDP: () => true,
    createListener: async payload => { observed.requests++; return create(payload); },
    fetchListeners: async () => [existing],
    setData() {}, setEditing() {}, load: async () => true,
    setModalOpen: open => { observed.modalOpen = open; },
    runtimeText: { saved: 'Saved', savedDisabled: 'Saved disabled' },
    message: {
      success: text => observed.successes.push(text),
      error: text => observed.errors.push(text),
    },
    t: key => key,
  };
  vm.createContext(context);
  vm.runInContext(javascript, context);
  return { submit: context.submit, observed };
}

test('a genuine listener name conflict keeps the form open and reports the error', async () => {
  const error = 'listener name "existing-node" already exists';
  const { submit, observed } = harness(async payload => {
    assert.equal(payload.port, '2080');
    throw new Error(error);
  });
  await submit();
  assert.equal(observed.requests, 1);
  assert.deepEqual(observed.successes, []);
  assert.deepEqual(observed.errors, [error]);
  assert.equal(observed.submitError, error);
  assert.equal(observed.modalOpen, true);
  assert.equal(observed.resets, 0);
});

test('a successful listener creation closes and clears the form', async () => {
  const { submit, observed } = harness(async payload => ({ ...payload, id: 2 }));
  await submit();
  assert.equal(observed.requests, 1);
  assert.deepEqual(observed.successes, ['Saved']);
  assert.deepEqual(observed.errors, []);
  assert.equal(observed.modalOpen, false);
  assert.equal(observed.resets, 1);
});

test('overlapping submits still create only one listener', async () => {
  let finish;
  const result = new Promise(resolve => { finish = resolve; });
  const { submit, observed } = harness(() => result);
  const first = submit();
  await submit();
  assert.equal(observed.requests, 1);
  finish({ id: 2, enabled: true });
  await first;
  assert.deepEqual(observed.successes, ['Saved']);
  assert.equal(observed.resets, 1);
});
