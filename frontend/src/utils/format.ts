/**
 * Human-readable byte formatter (binary / 1024-based).
 *
 * Unified implementation used across Dashboard, Users, Traffic, etc.
 * Handles: undefined / null / NaN (-> "0 B"), negative (clamped to "0 B"),
 * 0, sub-KiB values, and scales up to PB.
 */
export function formatBytes(bytes?: number | null): string {
  const n = Number(bytes ?? 0);
  if (!Number.isFinite(n) || n <= 0) return '0 B';
  if (n < 1024) return `${n} B`;
  const units = ['KB', 'MB', 'GB', 'TB', 'PB'];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(2)} ${units[i]}`;
}

export function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  return `${h}h ${m}m ${s}s`;
}

export function formatPercent(value: number): string {
  return value.toFixed(1) + '%';
}
