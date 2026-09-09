import assert from 'node:assert/strict';
import test from 'node:test';
import { listenerAvailability, listenerDiagnosticReason } from './listenerAvailability.ts';

const now = Date.now();
const listening = { state: 'listening' };
const passed = { ...listening, connection_check: { state: 'available', expires_at: new Date(now + 60000).toISOString() } };
test('listening alone never claims usability', () => {
  assert.equal(listenerAvailability(listening, true, false, now), 'pending');
  assert.equal(listenerAvailability(passed, true, false, now), 'available');
});

test('diagnostics explain the unavailable badge rather than a secondary test limitation', () => {
  const missing = { state: 'not_listening', reason: 'config_not_applied', connection_check: { reason: 'unsupported_local_check' } };
  assert.equal(listenerDiagnosticReason(missing), 'config_not_applied');
  assert.equal(listenerDiagnosticReason({ ...missing, connection_check: { reason: 'listener_sni_unsupported' } }), 'listener_sni_unsupported');
  assert.equal(listenerDiagnosticReason({ ...missing, reason: 'core_stopped' }), 'core_stopped');
  assert.equal(listenerDiagnosticReason({ ...passed, reason: 'status_unavailable' }), 'status_unavailable');
});
test('expired, invalid, failed and disabled results cannot remain green', () => {
  assert.equal(listenerAvailability(passed, true, false, now + 60001), 'pending');
  assert.equal(listenerAvailability(passed, false, false, now), 'disabled');
  assert.equal(listenerAvailability({ ...passed, state: 'not_listening' }, true, false, now), 'unavailable');
  assert.equal(listenerAvailability({ ...passed, reason: 'status_unavailable' }, true, false, now), 'unknown');
  assert.equal(listenerAvailability(passed, true, true, now), 'checking');
});
