import client, { withNetworkRetry, isTransientNetworkError } from './client';

export interface SystemStatus {
  cpu: { percent: number };
  memory: { used: number; total: number; percent: number };
  disk: { used: number; total: number; percent: number };
  network: { upload: number; download: number };
}

export interface MihomoStatus {
  running: boolean;
  version: string;
  pid: number;
  uptime: string;
}

export interface ProcessUsageSample {
  pid?: number;
  cpu_percent?: number;
  memory_used?: number;
  memory_percent?: number;
}

export interface DashboardResponse {
  mihomo: MihomoStatus;
  system: SystemStatus;
  panel?: ProcessUsageSample;
  core?: ProcessUsageSample;
  listeners: { total: number; enabled: number; disabled: number };
  users?: { total: number; enabled: number; online: number };
  traffic: {
    uploadRate: number;
    downloadRate: number;
    totalUpload: number;
    totalDownload: number;
    onlineUsers: number;
    activeConnections: number;
    tcpConnections?: number;
    udpConnections?: number;
  };
}

export const fetchDashboard = (signal?: AbortSignal) =>
  client.get<DashboardResponse>('/dashboard', { signal }).then(r => r.data);
export const startMihomo = () => client.post('/mihomo/start');
export const stopMihomo = () => client.post('/mihomo/stop');
export const restartMihomo = () =>
  withNetworkRetry(
    () => client.post('/mihomo/restart', null, { timeout: 120_000 }),
    4,
    1200,
  );

/** After core restart, transient network errors often mean the HTTP response was dropped while Mihomo restarted — panel is still up. */
export { isTransientNetworkError };
export interface LogResponse {
  timestamp: string;
  level: string;
  payload: string;
}

export const fetchLogs = () => client.get<LogResponse[]>('/mihomo/logs').then(r => r.data);

export const downloadBackup = async () => {
  const res = await client.get('/system/backup', { responseType: 'blob' });
  const blob = new Blob([res.data], { type: 'application/zip' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `3m-ui-backup-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.zip`;
  a.click();
  URL.revokeObjectURL(url);
};

export const restoreDatabase = (file: File) => {
  const fd = new FormData();
  fd.append('database', file);
  // Let Axios/browser set the multipart boundary automatically. Manually
  // setting Content-Type can omit the boundary and make Gin reject the upload.
  return client.post('/system/backup/restore-db', fd);
};

// Migration import reads a backup exported by another panel (3x-ui / s-ui) and
// appends its users and nodes without replacing the current database. Preview
// is a dry-run; import writes in a single transaction after taking a backup.
export interface MigrationPreviewUser {
  username: string;
  uuid?: string;
  password?: string;
  flow?: string;
  traffic_limit: number;
  upload_bytes: number;
  download_bytes: number;
  ip_limit: number;
  expire_at?: string;
  enabled: boolean;
  remark?: string;
  group?: string;
  telegram_id?: number;
}

export interface MigrationPreviewNode {
  name: string;
  protocol: string;
  port: string;
  listen?: string;
  config?: string;
  tls?: boolean;
  udp?: boolean;
  public_host?: string;
  access_sni?: string;
  access_alpn?: string;
  client_fingerprint?: string;
  source_protocol?: string;
  notes?: string[];
}

export interface MigrationPreview {
  source: string;
  counts: { users: number; nodes: number; bindings: number; warnings: number; skipped: number };
  users: MigrationPreviewUser[];
  nodes: MigrationPreviewNode[];
  warnings?: string[];
  skipped?: string[];
  tables?: string[];
  backup_file?: string;
}

export interface MigrationResult {
  source: string;
  users_created: number;
  nodes_created: number;
  bindings_created: number;
  renamed?: string[];
  skipped?: string[];
  warnings?: string[];
  backup_file?: string;
}

export const previewMigration = (file: File) => {
  const fd = new FormData();
  fd.append('database', file);
  return client.post<MigrationPreview>('/system/backup/migrate/preview', fd).then((r) => r.data);
};

export const importMigration = (file: File) => {
  const fd = new FormData();
  fd.append('database', file);
  return client.post<MigrationResult>('/system/backup/migrate/import', fd).then((r) => r.data);
};

export const openApiUrl = '/api/v1/openapi.yaml';



export interface LocalBackupItem {
  name: string;
  size: number;
  mod_time: string;
  is_dir?: boolean;
}

export const listLocalBackups = () =>
  client.get<{ items: LocalBackupItem[]; total_bytes: number; dir: string }>('/system/backups').then((r) => r.data);

export const deleteLocalBackup = (name: string) =>
  client.delete(`/system/backups/${encodeURIComponent(name)}`);

export const cleanupLocalBackups = (body: { keep?: number; older_than_days?: number }) =>
  client.post<{ ok: boolean; deleted: string[]; deleted_count: number; kept: number; freed_bytes: number }>(
    '/system/backups/cleanup',
    body,
  ).then((r) => r.data);

export const registerWarp = (mode: 'wireguard' | 'masque' | 'both' = 'wireguard') =>
  client.post<{ yaml?: string; masque_yaml?: string; address?: string; ipv6?: string; reserved?: number[] }>(
    `/system/templates/warp/register?mode=${mode}`,
  ).then((r) => r.data);

export interface UpdateInfo {
  current_version: string;
  current_channel: string;   // 'stable' | 'pre'
  target_channel: string;    // the channel you're NOT on (for switch prompt)
  latest_version: string;    // latest version in target_channel
  latest_stable: string;     // latest stable release tag
  latest_pre: string;         // latest pre release tag
  update_available: boolean;
  release_url?: string;
  release_notes?: string;
  error?: string;
}

export const restartPanel = () =>
  client.post('/system/restart').then((r) => r.data);

export const checkUpdate = () =>
  client.get<UpdateInfo>('/system/update-info').then((r) => r.data);

export const runUpdate = (channel?: 'stable' | 'pre') =>
  client.post('/system/update', channel ? { channel } : undefined).then((r) => r.data);

export type WarpAccountView = {
  configured: boolean;
  device_id?: string;
  license_key?: string;
  address_v4?: string;
  address_v6?: string;
  endpoint_host?: string;
  endpoint_port?: number;
  peer_public_key?: string;
  updated_at?: string;
  proxy_name?: string;
};

export const fetchWarpAccount = () =>
  client.get<WarpAccountView>('/system/warp').then((r) => r.data);
export const createWarpAccount = (mode?: string) =>
  client
    .post<{ account: WarpAccountView; yaml?: string; proxy_name?: string }>(
      `/system/warp${mode ? `?mode=${mode}` : ''}`,
    )
    .then((r) => r.data);
export const deleteWarpAccount = () =>
  client.delete('/system/warp').then((r) => r.data);
