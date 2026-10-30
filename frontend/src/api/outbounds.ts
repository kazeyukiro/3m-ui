import client from './client';

export interface OutboundProxy {
  name: string;
  type: string;
  server?: string;
  port?: number;
  [key: string]: any;
}

export interface ServerOutbound {
  proxy: OutboundProxy;
  enabled: boolean;
  managed: boolean;
}

const endpoint = '/config/server-outbounds';
export const fetchOutbounds = () => client.get<ServerOutbound[]>(endpoint).then(r => r.data);
export const saveOutbound = (entry: ServerOutbound, previous?: string) => previous
  ? client.put<ServerOutbound>(endpoint, entry, { params: { name: previous } }).then(r => r.data)
  : client.post<ServerOutbound>(endpoint, entry).then(r => r.data);
export const deleteOutbound = (name: string) => client.delete(endpoint, { params: { name } });
export const setOutboundsEnabled = (names: string[], enabled: boolean) => client.post(`${endpoint}/enabled`, { names, enabled });
