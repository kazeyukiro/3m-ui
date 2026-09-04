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
export const healthAllCluster = () =>
  client.post<RemoteServer[]>('/cluster/health-all').then((r) => r.data);
export const fetchRemoteDashboard = (id: number) =>
  client.get(`/cluster/${id}/dashboard`).then((r) => r.data);
export const fetchRemoteUsers = (id: number) =>
  client.get(`/cluster/${id}/users`).then((r) => r.data);
export const fetchRemoteNodes = (id: number) =>
  client.get(`/cluster/${id}/nodes`).then((r) => r.data);
export const remoteStartCore = (id: number) =>
  client.post(`/cluster/${id}/mihomo/start`).then((r) => r.data);
export const remoteStopCore = (id: number) =>
  client.post(`/cluster/${id}/mihomo/stop`).then((r) => r.data);
export const remoteRestartCore = (id: number) =>
  client.post(`/cluster/${id}/mihomo/restart`).then((r) => r.data);

export interface RemoteNodeMirror {
  id: number;
  remote_server_id: number;
  remote_node_id: number;
  name: string;
  protocol?: string;
  port?: string;
  public_host?: string;
  enabled: boolean;
  share_uri?: string;
  client_yaml?: string;
  last_sync_at?: string;
  last_error?: string;
  remote_server_name?: string;
}

export const syncRemoteNodes = (id: number) =>
  client.post<RemoteNodeMirror[]>(`/cluster/${id}/sync-nodes`).then((r) => r.data);
export const fetchMirroredNodes = (remoteServerId?: number) =>
  client
    .get<RemoteNodeMirror[]>('/cluster/mirrored-nodes', {
      params: remoteServerId ? { remote_server_id: remoteServerId } : undefined,
    })
    .then((r) => r.data);
