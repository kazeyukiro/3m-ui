export interface LogEntry {
  timestamp: string;
  level: string;
  payload: string;
}

export type LogLevelFilter = 'all' | 'info' | 'warning' | 'error';

/** Map loose level spellings to the three filter buckets. */
export function normalizeLevel(level: string): string {
  const lv = (level || '').toLowerCase();
  if (lv === 'warn') return 'warning';
  if (lv === 'fatal' || lv === 'critical') return 'error';
  if (lv === 'debug' || lv === 'trace') return 'info';
  return lv;
}

/** Filter a log list by case-insensitive payload substring and level bucket. */
export function filterLogs(
  logs: LogEntry[],
  search: string,
  level: LogLevelFilter,
): LogEntry[] {
  const q = (search || '').trim().toLowerCase();
  return logs.filter((l) => {
    if (level !== 'all' && normalizeLevel(l.level) !== level) return false;
    if (q && !(l.payload || '').toLowerCase().includes(q)) return false;
    return true;
  });
}
