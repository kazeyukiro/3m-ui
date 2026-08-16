import client from './client';

export interface Listener {
  id: number;
  name: string;
  protocol: string;
  port: string;
  bind_address: string;
  enabled: boolean;
  udp: boolean;
  tls: boolean;
  config: string;
  status: string;
  created_at?: string;
}

export const fetchListeners = () => client.get<Listener[]>('/nodes').then(r => r.data);
export const createListener = (payload: Partial<Listener>) => client.post<Listener>('/nodes', payload).then(r => r.data);
export const updateListener = (id: number, payload: Partial<Listener>) => client.put<Listener>(`/nodes/${id}`, payload).then(r => r.data);
export const deleteListener = (id: number) => client.delete(`/nodes/${id}`).then((r) => r.data);
export const reloadListener = (id: number) => client.post(`/nodes/${id}/reload`).then((r) => r.data);
export const exportNodeURI = (id: number) => client.get<{ name: string; protocol: string; uris: string[] }>(`/nodes/${id}/uri`).then(r => r.data);
