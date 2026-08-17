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
  created_at?: string;
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
