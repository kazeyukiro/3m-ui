import client from './client';

/** GORM historically serialized primary keys as "ID"; accept both shapes. */
export function normalizeId(row: { id?: number; ID?: number } | number | null | undefined): number {
  if (typeof row === 'number' && Number.isFinite(row) && row > 0) return row;
  if (row && typeof row === 'object') {
    const v = (row as any).id ?? (row as any).ID;
    const n = typeof v === 'string' ? parseInt(v, 10) : Number(v);
    if (Number.isFinite(n) && n > 0) return n;
  }
  return 0;
}

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

function mapListener(raw: any): Listener {
  return {
    ...raw,
    id: normalizeId(raw),
  };
}

export const fetchListeners = () =>
  client.get<any[]>('/nodes').then((r) => (r.data || []).map(mapListener));
export const createListener = (payload: Partial<Listener>) =>
  client.post<any>('/nodes', payload).then((r) => mapListener(r.data));
export const updateListener = (id: number, payload: Partial<Listener>) => {
  const nid = normalizeId(id);
  if (!nid) return Promise.reject(new Error('invalid node id'));
  return client.put<any>(`/nodes/${nid}`, payload).then((r) => mapListener(r.data));
};
export const deleteListener = (id: number) => {
  const nid = normalizeId(id);
  if (!nid) return Promise.reject(new Error('invalid node id'));
  return client.delete(`/nodes/${nid}`).then((r) => r.data);
};
export const reloadListener = (id: number) => {
  const nid = normalizeId(id);
  if (!nid) return Promise.reject(new Error('invalid node id'));
  return client.post(`/nodes/${nid}/reload`).then((r) => r.data);
};
export const exportNodeURI = (id: number) => {
  const nid = normalizeId(id);
  if (!nid) return Promise.reject(new Error('invalid node id'));
  return client.get<{ name: string; protocol: string; uris: string[]; hint?: string }>(`/nodes/${nid}/uri`).then((r) => r.data);
};

export type ClientAccess = {
  id: number;
  name: string;
  token: string;
  listener_id: number;
  mihomo_link?: string;
  clash_link?: string;
  singbox_link?: string;
  shadowrocket_link?: string;
};

/** Create or reuse listener-level subscription access token + client links. */
export const createClientAccess = (id: number) => {
  const nid = normalizeId(id);
  if (!nid) return Promise.reject(new Error('invalid node id'));
  return client.post<ClientAccess>(`/nodes/${nid}/client-access`).then((r) => r.data);
};
