import client from './client';

export interface ProxyUser {
  id: number;
  username: string;
  password?: string;
  enabled: boolean;
  traffic_limit?: number;
  traffic_used?: number;
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
export const createUser = (payload: Partial<ProxyUser>) =>
  client.post<ProxyUser>('/users', payload).then((r) => r.data);
export const updateUser = (id: number, payload: Partial<ProxyUser>) =>
  client.put<ProxyUser>(`/users/${id}`, payload).then((r) => r.data);
export const deleteUser = (id: number) => client.delete(`/users/${id}`);

/** List nodes currently bound to a proxy user. */
export const fetchUserNodes = (userId: number) =>
  client.get<BoundNode[]>(`/users/${userId}/nodes`).then((r) => r.data);

/** Replace the set of nodes bound to a proxy user. */
export const bindUserNodes = (userId: number, listenerIds: number[]) =>
  client.post(`/users/${userId}/nodes`, { listener_ids: listenerIds }).then((r) => r.data);
