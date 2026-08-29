import client from './client';

/** GORM historically serialized primary keys as "ID"; accept both shapes. */
export function normalizeId(row: { id?: number; ID?: number } | number | null | undefined): number {
  if (typeof row === 'number' && Number.isFinite(row) && row > 0) return row;
  if (!row || typeof row !== 'object') return 0;
  const n = Number((row as any).id ?? (row as any).ID ?? 0);
  return Number.isFinite(n) && n > 0 ? n : 0;
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
  /** Per-node Access Profile (m-ui) */
  public_host?: string;
  public_port?: string;
  access_sni?: string;
  client_fingerprint?: string;
  access_alpn?: string;
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
  return client.delete(`/nodes/${nid}`);
};
export const reloadListener = (id: number) => {
  const nid = normalizeId(id);
  if (!nid) return Promise.reject(new Error('invalid node id'));
  return client.post(`/nodes/${nid}/reload`);
};
export const exportNodeURI = (id: number) => {
  const nid = normalizeId(id);
  if (!nid) return Promise.reject(new Error('invalid node id'));
  return client.get<{ uri?: string; uris?: string[]; name?: string; protocol?: string; hint?: string; client_yaml?: string }>(`/nodes/${nid}/uri`).then((r) => r.data);
};
