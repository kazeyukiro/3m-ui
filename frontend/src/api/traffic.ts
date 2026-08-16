import client from './client';

export interface TrafficStatus {
  upload_bytes?: number;
  download_bytes?: number;
  upload_rate?: number;
  download_rate?: number;
  connections?: number;
}

export interface UserTraffic {
  user_id: number;
  username: string;
  upload_bytes: number;
  download_bytes: number;
  traffic_used: number;
  traffic_limit: number;
  online: boolean;
  blocked: boolean;
  expire_time?: string;
  last_seen?: string | null;
}

export interface ConnectionView {
  id?: string;
  network?: string;
  type?: string;
  metadata?: Record<string, unknown>;
  upload?: number;
  download?: number;
  start?: string;
  chains?: string[];
  rule?: string;
  rulePayload?: string;
  username?: string;
  listener_id?: number;
  listener_name?: string;
}

export const fetchTrafficStatus = () =>
  client.get<TrafficStatus>('/traffic/status').then((r) => r.data);
export const fetchTrafficUsers = () =>
  client.get<UserTraffic[]>('/traffic/users').then((r) => r.data);
export const fetchConnections = () =>
  client.get<{ connections: ConnectionView[] }>('/traffic/connections').then((r) => r.data.connections || []);
