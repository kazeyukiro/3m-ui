import client from './client';

export interface ListenerRuntime {
  id: number;
  state: 'disabled' | 'listening' | 'not_listening' | 'unknown';
  reason: string;
  checked_at: string;
  endpoints: { network: string; address: string; port: number; bound: boolean }[];
  connection_check?: {
    state: 'available' | 'unavailable' | 'unknown' | 'disabled';
    reason: string; source: 'local'; checked_at: string; expires_at: string;
    address?: string; target?: string; delay_ms?: number;
    steps: { name: string; state: 'passed' | 'failed' | 'unknown'; reason: string }[];
  };
}

export const fetchListenerRuntime = (signal?: AbortSignal) =>
  client.get<ListenerRuntime[]>('/nodes/runtime-status', { params: { summary: true }, signal }).then(r => r.data);

export const checkListenerRuntime = (id: number, signal?: AbortSignal) =>
  client.post<ListenerRuntime>(`/nodes/${id}/check`, undefined, { signal }).then(r => r.data);

export const testListenerConnection = (id: number, signal?: AbortSignal, reuse = false) =>
  client.post<ListenerRuntime>(`/nodes/${id}/connection-check`, undefined, { signal, params: { reuse }, timeout: 45000 }).then(r => r.data);
