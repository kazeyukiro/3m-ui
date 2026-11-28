import assert from 'node:assert/strict';
import test from 'node:test';
import { listenerAvailability, listenerDiagnosticReason } from './listenerAvailability.ts';

const now = Date.now();
const listening = { state: 'listening' };
const passed = { ...listening, connection_check: { state: 'available', expires_at: new Date(now + 60000).toISOString() } };
const stale = { ...listening, connection_check: { state: 'available', expires_at: new Date(now - 60000).toISOString() } };
test('listening alone never claims usability', () => {
  assert.equal(listenerAvailability(listening, true, false), 'pending');
  assert.equal(listenerAvailability(passed, true, false), 'available');
});

test('diagnostics explain the unavailable badge rather than a secondary test limitation', () => {
  const missing = { state: 'not_listening', reason: 'config_not_applied', connection_check: { reason: 'unsupported_local_check' } };
  assert.equal(listenerDiagnosticReason(missing), 'config_not_applied');
  assert.equal(listenerDiagnosticReason({ ...missing, connection_check: { reason: 'listener_sni_unsupported' } }), 'listener_sni_unsupported');
  assert.equal(listenerDiagnosticReason({ ...missing, reason: 'core_stopped' }), 'core_stopped');
  assert.equal(listenerDiagnosticReason({ ...passed, reason: 'status_unavailable' }), 'status_unavailable');
});

test('the last result survives the reuse TTL until retested or invalidated', () => {
  // An expired TTL alone must not send the badge back to "not tested":
  // the badge keeps the last known result.
  assert.equal(listenerAvailability(stale, true, false), 'available');
  assert.equal(listenerAvailability({ ...listening, connection_check: { state: 'unavailable', expires_at: new Date(now - 60000).toISOString() } }, true, false), 'unavailable');
  // A result taken while the listener was disabled says nothing about the
  // now-enabled listener.
  assert.equal(listenerAvailability({ ...listening, connection_check: { state: 'disabled', expires_at: new Date(now + 60000).toISOString() } }, true, false), 'pending');
});

test('failures and disabled states still win over a stale success', () => {
  assert.equal(listenerAvailability({ ...passed, state: 'not_listening' }, true, false), 'unavailable');
  assert.equal(listenerAvailability({ ...passed, reason: 'status_unavailable' }, true, false), 'unknown');
  assert.equal(listenerAvailability(passed, false, false), 'disabled');
  assert.equal(listenerAvailability(passed, true, true), 'checking');
});
