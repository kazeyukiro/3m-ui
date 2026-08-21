import client from './client';

export interface TelegramSettings {
  enabled: boolean;
  bot_token: string;
  chat_ids: string[];
  notify_on_login?: boolean;
  notify_on_block: boolean;
  notify_on_unblock: boolean;
  notify_on_expiry: boolean;
  notify_on_traffic?: boolean;
  notify_daily_digest: boolean;
  notify_on_cpu?: boolean;
  traffic_warn_pct?: number;
  expiry_warn_hours?: number;
  cpu_warn_pct?: number;
}

export const fetchTelegramSettings = () =>
  client.get<TelegramSettings>('/telegram/settings').then((r) => r.data);
export const saveTelegramSettings = (payload: Partial<TelegramSettings> & { keep_token?: boolean }) =>
  client.put<TelegramSettings>('/telegram/settings', payload).then((r) => r.data);
export const testTelegram = () => client.post('/telegram/test').then((r) => r.data);
