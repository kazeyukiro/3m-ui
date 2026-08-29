import client from './client';
import { fetchListeners, createListener, updateListener, deleteListener, reloadListener, exportNodeURI, Listener } from './nodes';

export { fetchListeners, createListener, updateListener, deleteListener, reloadListener, exportNodeURI };
export type { Listener };

export interface ListenerTemplate {
  id: number;
  name: string;
  protocol: string;
  config: string;
  created_at?: string;
  updated_at?: string;
}

export interface ListenerVersion {
  id: number;
  listener_id: number;
  version: number;
  created_at: string;
  reason?: string;
  snapshot: string;
}

export const listListenerTemplates = () => client.get<ListenerTemplate[]>('/nodes/templates').then(r => r.data);
export const createListenerTemplate = (payload: Partial<ListenerTemplate>) => client.post<ListenerTemplate>('/nodes/templates', payload).then(r => r.data);
export const deleteListenerTemplate = (id: number) => client.delete(`/nodes/templates/${id}`).then(r => r.data);
export const instantiateListenerTemplate = (id: number, payload: { name: string; port: string }) => client.post(`/nodes/templates/${id}/instantiate`, payload).then(r => r.data);
export const cloneListener = (id: number, payload: { name: string; port: string }) => client.post(`/nodes/${id}/clone`, payload).then(r => r.data);
export const batchSetListenersEnabled = (ids: number[], enabled: boolean) => client.post('/nodes/batch/enabled', { ids, enabled }).then(r => r.data);
export const listListenerVersions = (id: number) => client.get<ListenerVersion[]>(`/nodes/${id}/versions`).then(r => r.data);
export const diffListenerVersion = (id: number, version: number) => client.get<string>(`/nodes/${id}/versions/${version}/diff`, { responseType: 'text' }).then(r => r.data);
export const rollbackListenerVersion = (id: number, version: number) => client.post(`/nodes/${id}/versions/${version}/rollback`).then(r => r.data);

export const generateMaterial = (payload: { kind: string; cipher?: string }) =>
  client.post('/listeners/generate', payload).then((r) => r.data as Record<string, string>);
