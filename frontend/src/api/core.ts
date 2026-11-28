import client from './client';

export type CoreStatus = { running: boolean; version: string; pid: number; uptime: string };
export type CoreRelease = { version: string; published_at: string; url: string; asset: string; sha256: string; size: number; prerelease: boolean };
export type CoreUpdateJob = { id: string; operation: 'install' | 'rollback'; version: string; status: 'running' | 'succeeded' | 'failed'; stage: string; error?: string; rolled_back: boolean };
export type CoreUpdateStatus = { supported: boolean; disabled_reason?: string; busy: boolean; managed: boolean; previous_version?: string; job?: CoreUpdateJob };
export const coreAPI = {
  status: async (signal?: AbortSignal) => (await client.get<CoreStatus>('/mihomo/status', { signal })).data,
  updateStatus: async (signal?: AbortSignal) => (await client.get<CoreUpdateStatus>('/mihomo/update', { signal })).data,
  releases: async () => (await client.get<CoreRelease[]>('/mihomo/releases', { timeout: 25000 })).data,
  install: async (version: string) => (await client.post<CoreUpdateJob>('/mihomo/update', { version })).data,
  rollback: async () => (await client.post<CoreUpdateJob>('/mihomo/update/rollback')).data,
  action: async (action: 'start' | 'stop' | 'restart') => client.post(`/mihomo/${action}`),
};
