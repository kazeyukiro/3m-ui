import type { ServerOutbound } from '../api/outbounds.ts';
import { OUTBOUND_OBJECTS, OUTBOUND_PROTOCOL_SCHEMAS } from './outboundSchema.ts';

export const OUTBOUND_PROTOCOLS = Object.keys(OUTBOUND_PROTOCOL_SCHEMAS);
export const OUTBOUND_REQUIRED: Record<string, string[]> = {
  shadowsocks: ['cipher', 'password'], snell: ['psk'], vmess: ['uuid', 'cipher'], vless: ['uuid'],
  trojan: ['password'], 'tuic-v4': ['token'], 'tuic-v5': ['uuid', 'password'],
  anytls: ['password'], mieru: ['username', 'password', 'transport'], sudoku: ['key'], wireguard: ['private-key'],
};

export const outboundFields = (protocol: string) => [...(OUTBOUND_OBJECTS[OUTBOUND_PROTOCOL_SCHEMAS[protocol]] || []), ['smux', 'object', 'SingMuxOption'] as const]
  .filter(([key]) => !(protocol === 'tuic-v4' && ['uuid', 'password'].includes(key)) && !(protocol === 'tuic-v5' && key === 'token'));

export const outboundDefaults = (protocol: string) => ({
  enabled: true, protocol,
  ...(protocol === 'vmess' ? { cipher: 'auto', alterId: 0 } : {}),
  ...(protocol === 'shadowsocks' ? { cipher: 'aes-128-gcm' } : {}),
  ...(protocol === 'mieru' ? { transport: 'TCP' } : {}),
});

export function outboundToForm(entry: ServerOutbound): Record<string, any> {
  const proxy = structuredClone(entry.proxy);
  const protocol = proxy.type === 'ss' ? 'shadowsocks' : proxy.type === 'socks5' ? 'socks' : proxy.type === 'tuic' ? (proxy.token ? 'tuic-v4' : 'tuic-v5') : proxy.type;
  return { ...proxy, protocol, enabled: entry.enabled };
}

function clean(value: any): any {
  if (value === '' || value === undefined || value === null) return undefined;
  if (Array.isArray(value)) {
    const items = value.map(clean).filter(v => v !== undefined);
    return items.length ? items : undefined;
  }
  if (typeof value === 'object') {
    const entries = Object.entries(value).map(([key, v]) => [key, clean(v)] as const).filter(([key, v]) => key !== '' && v !== undefined);
    return entries.length ? Object.fromEntries(entries) : undefined;
  }
  return value;
}

export function outboundFromForm(values: Record<string, any>, previous?: ServerOutbound): ServerOutbound {
  const protocol = values.protocol;
  const type = protocol === 'shadowsocks' ? 'ss' : protocol === 'socks' ? 'socks5' : protocol?.startsWith('tuic-') ? 'tuic' : protocol;
  const sameProtocol = previous && outboundToForm(previous).protocol === protocol;
  const proxy: Record<string, any> = sameProtocol ? structuredClone(previous.proxy) : {};
  for (const [key] of outboundFields(protocol)) {
    if (Object.hasOwn(values, key)) {
      const value = clean(values[key]);
      if (value === undefined) delete proxy[key]; else proxy[key] = value;
    }
  }
  proxy.type = type;
  delete proxy.enabled;
  delete proxy.managed;
  delete proxy.protocol;
  return { proxy: proxy as ServerOutbound['proxy'], enabled: values.enabled !== false, managed: false };
}
