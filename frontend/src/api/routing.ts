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

/** Mihomo rule-providers entry (wiki.metacubex.one/config/rule-providers). */
export interface RuleProvider {
  name: string;
  type: 'http' | 'file' | 'inline' | string;
  behavior: 'domain' | 'ipcidr' | 'classical' | string;
  format?: 'yaml' | 'text' | 'mrs' | string;
  url?: string;
  path?: string;
  interval?: number;
  proxy?: string;
  payload?: string[];
  sizeLimit?: number;
}

export interface ServerRoutingConfig {
  proxies: Array<Record<string, unknown> & { name: string; type: string; server?: string; port?: number | string }>;
  proxyGroups: GroupEntry[];
  rules: string[];
  warpDomains?: string[];
  ruleProviders?: RuleProvider[];
}

export const fetchServerRouting = () =>
  withNetworkRetry(() =>
    client.get<ServerRoutingConfig>('/config/server-routing').then((r) => r.data),
  );
export const saveServerRouting = (cfg: ServerRoutingConfig) =>
  withNetworkRetry(() =>
    client.put<ServerRoutingConfig>('/config/server-routing', cfg).then((r) => r.data),
  );

export const fetchRuleProviderStatus = () =>
  withNetworkRetry(() => client.get('/config/rule-providers/status').then((r) => r.data));

/** Hot-reload one rule-set via Mihomo PUT /providers/rules/{name}. */
export const updateRuleProvider = (name: string) =>
  withNetworkRetry(() =>
    client.put(`/config/rule-providers/${encodeURIComponent(name)}/update`).then((r) => r.data),
  );
