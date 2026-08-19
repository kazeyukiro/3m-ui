import type { ProtocolCapability, FieldCapability } from '../api/capabilities'

function getNestedValue(values: Record<string, any>, key: string): any {
  if (Object.prototype.hasOwnProperty.call(values, key)) return values[key];
  return key.split('.').reduce((current: any, part: string) => current == null ? undefined : current[part], values);
}

function firstValue(values: Record<string, any>, ...keys: string[]): any {
  for (const key of keys) {
    const value = getNestedValue(values, key);
    if (value !== undefined && value !== null && value !== '') return value;
  }
  return undefined;
}

function setConfigPath(cfg: Record<string, any>, path: string, value: any): void {
  const parts = path.split('.');
  let current = cfg;
  for (let i = 0; i < parts.length - 1; i += 1) {
    const part = parts[i];
    if (!current[part] || typeof current[part] !== 'object' || Array.isArray(current[part])) current[part] = {};
    current = current[part];
  }
  current[parts[parts.length - 1]] = value;
}

function setIfPresent(cfg: Record<string, any>, path: string, value: any): void {
  if (value === undefined || value === null || value === '') return;
  setConfigPath(cfg, path, value);
}

function serializeFields(cfg: Record<string, any>, fields: FieldCapability[] | undefined, values: Record<string, any>, skip: (path: string) => boolean): void {
  for (const field of fields || []) {
    const path = field.path;
    if (!path || skip(path)) continue;
    const value = firstValue(values, path);
    if (value !== undefined && value !== null && value !== '') setConfigPath(cfg, path, value);
  }
}

/** Map capability-driven form values into Mihomo listener config JSON keys. */
export function capabilityFormToConfig(protocol: string, values: Record<string, any>, capability?: ProtocolCapability): Record<string, any> {
  const cfg: Record<string, any> = {};
  const set = (k: string, v: any) => setIfPresent(cfg, k, v);
  const transport = firstValue(values, 'transport_layer') || 'raw';
  if (transport === 'ws') set('ws-path', firstValue(values, 'ws-path', 'ws_path'));
  if (transport === 'grpc') set('grpc-service-name', firstValue(values, 'grpc-service-name', 'grpc_service_name'));
  if (transport === 'xhttp') {
    set('xhttp-config.path', firstValue(values, 'xhttp_path', 'xhttp-config.path'));
    set('xhttp-config.host', firstValue(values, 'xhttp_host', 'xhttp-config.host'));
    set('xhttp-config.mode', firstValue(values, 'xhttp_mode', 'xhttp-config.mode'));
  }

  const security = firstValue(values, 'security_layer') || 'none';
  if (security === 'reality') {
    set('reality-config.dest', firstValue(values, 'reality_dest', 'reality-config.dest'));
    set('reality-config.private-key', firstValue(values, 'reality_private_key', 'reality-config.private-key'));
    set('reality-config.short-id', firstValue(values, 'reality_short_id', 'reality-config.short-id'));
    set('reality-config.server-names', firstValue(values, 'reality_server_names', 'reality-config.server-names'));
  } else if (security === 'tls') {
    set('certificate', firstValue(values, 'certificate'));
    set('private-key', firstValue(values, 'private-key', 'private_key'));
    set('alpn', firstValue(values, 'alpn'));
    if (firstValue(values, 'allow-insecure') === true) cfg['allow-insecure'] = true;
  }

  const skipTransportSecurity = (path: string) => (
    path === 'transport_layer' || path === 'security_layer'
    || path === 'ws-path' || path === 'grpc-service-name'
    || path.startsWith('xhttp-config.') || path === 'xhttp-config'
    || path === 'reality-config' || path.startsWith('reality-config.')
    || path === 'reality_dest' || path === 'reality_private_key'
    || path === 'reality_short_id' || path === 'reality_server_names'
    || path === 'certificate' || path === 'private-key' || path === 'private_key'
    || path === 'allow-insecure'
  );
  const selectedTransport = capability?.components?.find((c) => c.group === 'transport' && c.kind === transport);
  const selectedSecurity = capability?.components?.find((c) => c.group === 'security' && c.kind === security);
  serializeFields(cfg, selectedTransport?.fields, values, skipTransportSecurity);
  serializeFields(cfg, selectedSecurity?.fields, values, skipTransportSecurity);
  serializeFields(cfg, capability?.fields, values, skipTransportSecurity);
  if (protocol === 'vless') {
    const flow = firstValue(values, 'flow');
    if (flow) set('flow', flow);
  }
  return cfg;
}
