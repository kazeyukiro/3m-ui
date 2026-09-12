import assert from 'node:assert/strict';
import fs from 'node:fs';
import { createRequire } from 'node:module';
import test from 'node:test';
import vm from 'node:vm';
import ts from 'typescript';

const require = createRequire(import.meta.url);
const source = fs.readFileSync(new URL('../api/client.ts', import.meta.url), 'utf8');
const javascript = ts.transpileModule(source, {
  compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS, esModuleInterop: true },
}).outputText;

function panelClient() {
  const module = { exports: {} };
  vm.runInNewContext(javascript, {
    module, exports: module.exports,
    require: name => name === '../stores/authStore'
      ? { useAuthStore: { getState: () => ({ token: null }) } }
      : require(name),
  });
  const client = module.exports.default;
  // Observe the effective timeout after real Axios merging and interceptors.
  client.defaults.adapter = async config => ({
    data: config.timeout, status: 200, statusText: 'OK', headers: {}, config,
  });
  return client;
}

test('read requests default to 45 seconds and mutations default to 120 seconds', async () => {
  const client = panelClient();
  for (const method of ['get', 'head', 'post', 'put', 'patch', 'delete']) {
    const response = await client.request({ method, url: '/nodes' });
    assert.equal(response.data, method === 'get' || method === 'head' ? 45000 : 120000, method);
  }
});

test('explicit request timeouts survive defaults and request interceptors', async () => {
  const client = panelClient();
  for (const method of ['get', 'post', 'delete']) {
    for (const timeout of [0, 25000, 40000, 45000, 120000, 180000]) {
      const response = await client.request({ method, url: '/nodes', timeout });
      assert.equal(response.data, timeout, `${method} with timeout ${timeout}`);
    }
  }
});
