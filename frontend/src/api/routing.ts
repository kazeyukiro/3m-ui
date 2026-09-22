import client from './client';

export interface GroupEntry {
  name: string;
  type: string;
  proxies: string[];
  url?: string;
  interval?: number;
}

export const fetchGroups = () => client.get<GroupEntry[]>('/config/groups').then((r) => r.data);
export const saveGroups = (groups: GroupEntry[]) =>
  client.put<GroupEntry[]>('/config/groups', groups).then((r) => r.data);
export const fetchRules = () => client.get<string[]>('/config/rules').then((r) => r.data);
export const saveRules = (rules: string[]) =>
  client.put<string[]>('/config/rules', rules).then((r) => r.data);

export const injectWarpRouting = (payload: {
  mode?: 'wireguard' | 'masque';
  rule_mode?: 'none' | 'match' | 'cn_direct';
  name?: string;
}) =>
  client
    .post<{
      status: string;
      name: string;
      rules: string[];
      proxies: unknown[];
      groups: GroupEntry[];
    }>('/config/routing/inject-warp', payload)
    .then((r) => r.data);
