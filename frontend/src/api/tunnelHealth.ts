import client from './client';

export interface TunnelHealthSettings {
  enabled: boolean;
  url: string;
  proxy_url: string;
  interval: number; // seconds
  timeout: number;  // seconds
  failures: number;
  cooldown: number; // seconds
}

export interface TunnelHealthStatus {
  enabled: boolean;
  interval: number;
  consecutive_failures: number;
  total_restarts: number;
  last_probe: string;
  last_restart: string;
  last_error: string;
}

export const fetchTunnelHealthSettings = () =>
  client.get<TunnelHealthSettings>('/tunnel-health/settings').then((r) => r.data);
export const saveTunnelHealthSettings = (payload: Partial<TunnelHealthSettings>) =>
  client.put('/tunnel-health/settings', payload).then((r) => r.data);
export const fetchTunnelHealthStatus = () =>
  client.get<TunnelHealthStatus>('/tunnel-health/status').then((r) => r.data);
export const testTunnelHealth = (payload?: Partial<TunnelHealthSettings>) =>
  client.post('/tunnel-health/test', payload ?? {}).then((r) => r.data);
