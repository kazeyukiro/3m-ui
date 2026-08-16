import client from './client';

export interface RemoteServer {
  id: number;
  name: string;
  base_url: string;
  api_token_set?: boolean;
  enabled: boolean;
  remark?: string;
  last_status?: string;
  last_check_at?: string | null;
  last_error?: string;
}

export const fetchCluster = () => client.get<RemoteServer[]>('/cluster').then((r) => r.data);
export const createClusterNode = (payload: Record<string, unknown>) =>
  client.post<RemoteServer>('/cluster', payload).then((r) => r.data);
export const updateClusterNode = (id: number, payload: Record<string, unknown>) =>
  client.put<RemoteServer>(`/cluster/${id}`, payload).then((r) => r.data);
export const deleteClusterNode = (id: number) => client.delete(`/cluster/${id}`);
export const healthClusterNode = (id: number) =>
  client.post<RemoteServer>(`/cluster/${id}/health`).then((r) => r.data);
