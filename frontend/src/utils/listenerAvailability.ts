import type { ListenerRuntime } from '../api/listenerRuntime';

export function listenerAvailability(status: ListenerRuntime | undefined, enabled: boolean, testing = false) {
  if (!enabled) return 'disabled';
  if (testing) return 'checking';
  if (status?.state === 'not_listening') return 'unavailable';
  if (status?.reason === 'status_unavailable') return 'unknown';
  const check = status?.connection_check;
  // The last known result stays on the badge until a new test or an
  // invalidation (process/config/client change) replaces it. The reuse TTL
  // alone must not send the badge back to "not tested".
  // A check taken while the listener was disabled says nothing about the
  // now-enabled listener, so it does not count as a result.
  if (check && check.state !== 'disabled') return check.state;
  return 'pending';
}

// Explain the same failure that drives the badge. A secondary client-check
// limitation must not hide a definite failure of the serving listener.
export function listenerDiagnosticReason(status: ListenerRuntime | undefined) {
  if (!status) return 'pending';
  if (status.reason === 'status_unavailable') return status.reason;
  if (status.state === 'not_listening') {
    const specific = status.connection_check?.reason;
    if (status.reason === 'config_not_applied' &&
        (specific === 'listener_config_invalid' || specific === 'listener_sni_unsupported')) return specific;
    return status.reason;
  }
  return status.connection_check?.reason || (status.state === 'listening' ? 'pending' : status.reason);
}
