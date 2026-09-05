import client from './client';

export interface ProxyEntry {
  name: string;
  type: string;
  server: string;
  port: string;
  password?: string;
  uuid?: string;
  [key: string]: any;
}

export interface VisualConfig {
  proxies: ProxyEntry[];
  proxyGroups?: any[];
  rules?: any[];
  [key: string]: any;
}

export const fetchProxies = () => client.get<ProxyEntry[]>('/config/proxies').then((r) => r.data);
export const createProxy = (payload: ProxyEntry) =>
  client.post<ProxyEntry>('/config/proxies', payload).then((r) => r.data);
export const updateProxy = (index: number, payload: ProxyEntry) =>
  client.put<ProxyEntry>(`/config/proxies/${index}`, payload).then((r) => r.data);
export const deleteProxy = (index: number) => client.delete(`/config/proxies/${index}`);
export const fetchVisualConfig = () =>
  client.get<VisualConfig>('/config/visual').then((r) => r.data);
export const saveVisualConfig = (payload: VisualConfig) =>
  client.post('/config/visual', payload).then((r) => r.data);
export const fetchConfigYAML = () => client.get<{ config: string }>('/config').then((r) => r.data);
export const generateConfig = () =>
  client.post<{ status: string; config?: string; message?: string }>('/config/generate').then((r) => r.data);
export const applyConfigYAML = (config?: string) =>
  client.post('/config/apply', config != null ? { config } : {}).then((r) => r.data);
export const rollbackConfig = () => client.post('/config/rollback').then((r) => r.data);
export const validateConfigYAML = (config: string) =>
  client.post<{ valid: boolean; error?: string }>('/config/validate', { config }).then((r) => r.data);
