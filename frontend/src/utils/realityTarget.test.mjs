import assert from 'node:assert/strict';
import test from 'node:test';
import { clearScannedRealityNames } from './realityTarget.ts';

const scannedName = 'www.debian.org';

test('manual destination replacement clears both names supplied by the scan', () => {
  const values = { access_sni: scannedName, reality_server_names: [scannedName] };
  const changed = { ...values, ...clearScannedRealityNames(scannedName, values) };
  assert.deepEqual(changed, { access_sni: '', reality_server_names: [] });
  assert.deepEqual(values, { access_sni: scannedName, reality_server_names: [scannedName] });
});

test('each independently customized name is preserved', () => {
  assert.deepEqual(clearScannedRealityNames(scannedName, {
    access_sni: 'custom.example.com', reality_server_names: [scannedName],
  }), { reality_server_names: [] });
  assert.deepEqual(clearScannedRealityNames(scannedName, {
    access_sni: scannedName, reality_server_names: ['custom.example.com'],
  }), { access_sni: '' });
  assert.deepEqual(clearScannedRealityNames(scannedName, {
    access_sni: 'custom.example.com', reality_server_names: [scannedName, 'custom.example.com'],
  }), {});
});

test('editing a manual target without a preceding scan never clears names', () => {
  assert.deepEqual(clearScannedRealityNames(null, {
    access_sni: scannedName, reality_server_names: [scannedName],
  }), {});
});

test('cleared names remain empty during subsequent keystrokes', () => {
  assert.deepEqual(clearScannedRealityNames(scannedName, { access_sni: '', reality_server_names: [] }), {});
  assert.deepEqual(clearScannedRealityNames(null, {}), {});
});
