import client from './client';

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

export interface DashboardResponse {
  mihomo: MihomoStatus;
  system: SystemStatus;
  listeners: { total: number; enabled: number; disabled: number };
  traffic: {
    uploadRate: number;
    downloadRate: number;
    totalUpload: number;
    totalDownload: number;
    onlineUsers: number;
    activeConnections: number;
  };
}

export const fetchDashboard = () => client.get<DashboardResponse>('/dashboard').then(r => r.data);
export const startMihomo = () => client.post('/mihomo/start');
export const stopMihomo = () => client.post('/mihomo/stop');
export const restartMihomo = () => client.post('/mihomo/restart');
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
  return client.post('/system/backup/restore-db', fd, {
    headers: { 'Content-Type': 'multipart/form-data' },
  });
};

export const openApiUrl = '/api/v1/openapi.yaml';
