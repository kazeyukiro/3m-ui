import client, { withNetworkRetry } from './client';

export interface GroupEntry {
  name: string;
  type: string;
  proxies: string[];
  url?: string;
  interval?: number;
}

export const fetchGroups = () =>
  withNetworkRetry(() => client.get<GroupEntry[]>('/config/groups').then((r) => r.data));
export const saveGroups = (groups: GroupEntry[]) =>
  withNetworkRetry(() => client.put<GroupEntry[]>('/config/groups', groups).then((r) => r.data));
export const fetchRules = () =>
  withNetworkRetry(() => client.get<string[]>('/config/rules').then((r) => r.data));
export const saveRules = (rules: string[]) =>
  withNetworkRetry(() => client.put<string[]>('/config/rules', rules).then((r) => r.data));

export interface ServerRoutingConfig {
  proxies: Array<Record<string, unknown> & { name: string; type: string; server?: string; port?: number | string }>;
  proxyGroups: GroupEntry[];
  rules: string[];
  warpDomains?: string[];
}

export const fetchServerRouting = () =>
  withNetworkRetry(() =>
    client.get<ServerRoutingConfig>('/config/server-routing').then((r) => r.data),
  );
export const saveServerRouting = (cfg: ServerRoutingConfig) =>
  withNetworkRetry(() =>
    client.put<ServerRoutingConfig>('/config/server-routing', cfg).then((r) => r.data),
  );
