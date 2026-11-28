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

/** One rejected line from a routing batch import (backend single source of truth). */
export interface RuleImportIssue {
  line: number;
  text: string;
  reason: string;
}

/** Backend parse result: normalized rules + per-line diagnostics. */
export interface RuleImportResult {
  rules: string[];
  duplicates: number;
  issues: RuleImportIssue[];
  truncated: number;
}

/**
 * Validate + normalize a batch-import blob against the mihomo rule syntax
 * (wiki.metacubex.one/config/rules). Never persists anything — the editor
 * previews the result and appends the accepted lines on confirm.
 */
export const parseRulesImport = (text: string) =>
  withNetworkRetry(() =>
    client.post<RuleImportResult>('/config/rules/parse', { text }).then((r) => r.data),
  );

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
  warpGlobal?: boolean;
  ruleProviders?: RuleProvider[];
}

export const fetchServerRouting = () =>
  withNetworkRetry(() =>
    client.get<ServerRoutingConfig>('/config/server-routing').then((r) => r.data),
  );
export const saveServerRouting = (cfg: Partial<Omit<ServerRoutingConfig, 'proxies'>>) =>
  withNetworkRetry(() =>
    client.patch<ServerRoutingConfig>('/config/server-routing', cfg).then((r) => r.data),
  );

export const fetchRuleProviderStatus = () =>
  withNetworkRetry(() => client.get('/config/rule-providers/status').then((r) => r.data));

/** Hot-reload one rule-set via Mihomo PUT /providers/rules/{name}. */
export const updateRuleProvider = (name: string) =>
  withNetworkRetry(() =>
    client.put(`/config/rule-providers/${encodeURIComponent(name)}/update`).then((r) => r.data),
  );
