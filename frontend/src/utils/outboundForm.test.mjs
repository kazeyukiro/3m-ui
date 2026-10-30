import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import { outboundToForm, outboundFromForm } from './outboundForm.ts';

test('all protocol fixtures survive form round-trip', () => {
  const fixtures = JSON.parse(fs.readFileSync(new URL('../../../backend/internal/mihomo/config/testdata/server_outbounds.json', import.meta.url), 'utf8'));
  for (const { protocol, proxy } of fixtures) {
    const entry = { proxy, enabled: true, managed: false };
    assert.equal(outboundToForm(entry).protocol, protocol);
    assert.deepEqual(outboundFromForm(outboundToForm(entry)).proxy, proxy, proxy.name);
  }
});

test('editing an outbound preserves nested transport options and explicit false', () => {
  const original = { enabled: true, managed: false, proxy: { name: 'edge', type: 'vless', server: 'example.com', port: 443, uuid: 'id', tls: true, 'ws-opts': { path: '/old', headers: { Host: 'cdn.example.com' } }, 'future-option': 12 } };
  const values = outboundToForm(original);
  values.tls = false;
  values['ws-opts'].path = '/new';
  const result = outboundFromForm(values, original);
  assert.equal(result.proxy.tls, false);
  assert.deepEqual(result.proxy['ws-opts'], { path: '/new', headers: { Host: 'cdn.example.com' } });
  assert.equal(result.proxy['future-option'], 12);
  assert.equal(original.proxy['ws-opts'].path, '/old');
  assert.equal(result.proxy.protocol, undefined);
  assert.equal(result.proxy.enabled, undefined);
});

test('TUIC versions and SOCKS use the core outbound names', () => {
  assert.deepEqual(outboundFromForm({ protocol: 'tuic-v4', name: 't', server: 's', port: 443, token: 'secret', enabled: true }).proxy, { type: 'tuic', name: 't', server: 's', port: 443, token: 'secret' });
  assert.equal(outboundFromForm({ protocol: 'socks', name: 's', server: 's', port: 1080 }).proxy.type, 'socks5');
  assert.equal(outboundToForm({ proxy: { type: 'tuic', token: 'secret' }, enabled: true }).protocol, 'tuic-v4');
  assert.equal(outboundToForm({ proxy: { type: 'tuic', uuid: 'id', password: 'p' }, enabled: true }).protocol, 'tuic-v5');
});

test('WireGuard peers remain structured and clearing optional fields removes them', () => {
  const result = outboundFromForm({ protocol: 'wireguard', name: 'wg', 'private-key': 'key', ip: '10.0.0.2', dns: [], 'dialer-proxy': '', peers: [{ server: 'example.com', port: 51820, 'public-key': 'peer', 'allowed-ips': ['0.0.0.0/0'], reserved: [0, 1, 255] }] });
  assert.deepEqual(result.proxy.peers, [{ server: 'example.com', port: 51820, 'public-key': 'peer', 'allowed-ips': ['0.0.0.0/0'], reserved: [0, 1, 255] }]);
  assert.equal(result.proxy.dns, undefined);
  assert.equal(result.proxy['dialer-proxy'], undefined);
});

test('clearing an optional boolean restores the core default', () => {
  const original = { enabled: true, managed: false, proxy: { name: 'edge', type: 'http', server: 'example.com', port: 443, tls: true } };
  const values = outboundToForm(original);
  values.tls = undefined;
  assert.equal(Object.hasOwn(outboundFromForm(values, original).proxy, 'tls'), false);
});
