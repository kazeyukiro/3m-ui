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
  traffic_multiplier?: number;
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

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** True when the HTTP response never arrived (core reload / proxy drop / timeout). */
export function isTransientNetworkError(err: unknown): boolean {
  const msg = String((err as any)?.message || err || '');
  const code = String((err as any)?.code || '');
  return (
    /Cannot reach the panel API|Network Error|ECONNABORTED|ETIMEDOUT|timed out|timeout|ERR_NETWORK|ERR_EMPTY_RESPONSE|Failed to fetch/i.test(
      msg,
    ) ||
    /ECONNABORTED|ETIMEDOUT|ERR_NETWORK/i.test(code)
  );
}

/**
 * Create/delete can finish on the server while the browser sees a dropped
 * connection: ApplyConfig restarts Mihomo under the request lock, and some
 * proxies close the panel response before 201 is written. Recover by re-listing.
 */
async function recoverCreatedByName(name: string): Promise<Listener | null> {
  const want = String(name || '').trim();
  if (!want) return null;
  for (const delay of [800, 1600, 3200]) {
    await sleep(delay);
    try {
      const list = await fetchListeners();
      const found = list.find((l) => String(l.name).trim() === want);
      if (found) return found;
    } catch {
      /* keep trying while the panel settles */
    }
  }
  return null;
}

async function recoverDeleted(id: number): Promise<boolean> {
  const nid = normalizeId(id);
  if (!nid) return true;
  for (const delay of [800, 1600, 3200]) {
    await sleep(delay);
    try {
      const list = await fetchListeners();
      if (!list.some((l) => normalizeId(l) === nid)) return true;
    } catch {
      /* keep trying */
    }
  }
  return false;
}

export const fetchListeners = () =>
  client.get<any[]>('/nodes').then((r) => (r.data || []).map(mapListener));

export const quickCreateListener = async (payload: { name: string; protocol: string }) => {
  try {
    const r = await client.post<any>('/nodes/quick', payload);
    return mapListener(r.data);
  } catch (err) {
    if (isTransientNetworkError(err)) {
      const recovered = await recoverCreatedByName(payload.name);
      if (recovered) return recovered;
    }
    throw err;
  }
};

export const createListener = async (payload: Partial<Listener>) => {
  try {
    const r = await client.post<any>('/nodes', payload);
    return mapListener(r.data);
  } catch (err) {
    if (isTransientNetworkError(err)) {
      const recovered = await recoverCreatedByName(String(payload.name || ''));
      if (recovered) return recovered;
    }
    throw err;
  }
};

export const updateListener = async (id: number, payload: Partial<Listener>) => {
  const nid = normalizeId(id);
  if (!nid) return Promise.reject(new Error('invalid node id'));
  try {
    const r = await client.put<any>(`/nodes/${nid}`, payload);
    return mapListener(r.data);
  } catch (err) {
    if (isTransientNetworkError(err)) {
      await sleep(1200);
      try {
        const r = await client.get<any>(`/nodes/${nid}`);
        return mapListener(r.data);
      } catch {
        /* fall through */
      }
    }
    throw err;
  }
};

export const deleteListener = async (id: number) => {
  const nid = normalizeId(id);
  if (!nid) return Promise.reject(new Error('invalid node id'));
  try {
    await client.delete(`/nodes/${nid}`);
  } catch (err) {
    if (isTransientNetworkError(err)) {
      if (await recoverDeleted(nid)) return;
    }
    throw err;
  }
};

export const reloadListener = (id: number) => {
  const nid = normalizeId(id);
  if (!nid) return Promise.reject(new Error('invalid node id'));
  return client.post(`/nodes/${nid}/reload`);
};

export const exportNodeURI = (id: number) => {
  const nid = normalizeId(id);
  if (!nid) return Promise.reject(new Error('invalid node id'));
  return client
    .get<{ uri?: string; uris?: string[]; name?: string; protocol?: string; hint?: string; client_yaml?: string }>(
      `/nodes/${nid}/uri`,
    )
    .then((r) => r.data);
};
