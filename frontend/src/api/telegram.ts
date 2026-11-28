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
  schedule?: string;
  attach_backup?: boolean;
  language?: string;
  enabled_events?: string;
  expiry_warn_days?: number;
  traffic_warn_gb?: number;
  proxy_url?: string;
  api_server?: string;
}

export const fetchTelegramSettings = () =>
  client.get<TelegramSettings>('/telegram/settings').then((r) => r.data);
export const saveTelegramSettings = (payload: Partial<TelegramSettings> & { keep_token?: boolean }) =>
  client.put<TelegramSettings>('/telegram/settings', payload).then((r) => r.data);
export const testTelegram = () => client.post('/telegram/test').then((r) => r.data);
export const setTelegramCommands = () => client.post('/telegram/set-my-commands').then((r) => r.data);
export const fetchTelegramBotInfo = () => client.get('/telegram/bot-info').then((r) => r.data);
