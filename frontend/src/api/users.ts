import client from './client';

export interface ProxyUser {
  id: number;
  username: string;
  password?: string;
  uuid_masked?: string;
  enabled: boolean;
  traffic_limit?: number;
  traffic_used?: number;
  upload_bytes?: number;
  download_bytes?: number;
  last_seen?: string | null;
  online?: boolean;
  expire_time?: string;
  blocked?: boolean;
  ip_limit?: number;
  remark?: string;
  sub_token?: string;
  telegram_id?: number;
  telegram_name?: string;
}

export interface BoundNode {
  id: number;
  name: string;
  protocol: string;
  port: string;
  bind_address: string;
  enabled: boolean;
  tls?: boolean;
  udp?: boolean;
  status?: string;
}

export const fetchUsers = () => client.get<ProxyUser[]>('/users').then((r) => r.data);
export const createUser = (payload: Record<string, unknown>) =>
  client.post<ProxyUser>('/users', payload).then((r) => r.data);
export const updateUser = (id: number, payload: Record<string, unknown>) =>
  client.put<ProxyUser>(`/users/${id}`, payload).then((r) => r.data);
export const deleteUser = (id: number) => client.delete(`/users/${id}`);
export const resetUserTraffic = (id: number) =>
  client.post<ProxyUser>(`/users/${id}/reset-traffic`).then((r) => r.data);

export const fetchUserNodes = (userId: number) =>
  client.get<BoundNode[]>(`/users/${userId}/nodes`).then((r) => r.data);
export const bindUserNodes = (userId: number, listenerIds: number[]) =>
  client.post(`/users/${userId}/nodes`, { listener_ids: listenerIds }).then((r) => r.data);

export const fetchUserSubscription = (id: number) =>
  client.get<{ token: string; url: string }>(`/users/${id}/subscription`).then((r) => r.data);
export const rotateUserSubscription = (id: number) =>
  client.post<{ token: string; url: string }>(`/users/${id}/subscription/rotate`).then((r) => r.data);

/** remove expired or over-quota users — remove expired / over-quota users. */
export const deleteDepletedUsers = () =>
  client.post<{ deleted: number }>('/users/del-depleted').then((r) => r.data);

/** bulk actions: enable | disable | reset-traffic | delete */
export const batchUsers = (action: string, ids: number[]) =>
  client.post<{ affected: number; action: string }>('/users/batch', { action, ids }).then((r) => r.data);

export const fetchUserRemoteNodes = (userId: number) =>
  client.get<{ mirror_ids: number[] }>(`/users/${userId}/remote-nodes`).then((r) => r.data);
export const bindUserRemoteNodes = (userId: number, mirrorIds: number[]) =>
  client.post(`/users/${userId}/remote-nodes`, { mirror_ids: mirrorIds }).then((r) => r.data);
