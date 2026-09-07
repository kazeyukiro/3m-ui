/** Default high-port suggestion range (inclusive). */
export const LISTENER_PORT_MIN = 10000;
export const LISTENER_PORT_MAX = 60000;

const MAX_ATTEMPTS = 48;

/**
 * Parse listener port specs (single, comma lists, ranges).
 * Inverted ranges (e.g. 50000-40000) are normalized.
 * Only ports intersecting [rangeMin, rangeMax] are collected.
 */
export function collectUsedPorts(
  portSpecs: string[],
  rangeMin = LISTENER_PORT_MIN,
  rangeMax = LISTENER_PORT_MAX,
): Set<number> {
  const used = new Set<number>();
  for (const spec of portSpecs) {
    for (const part of String(spec ?? '').replace(/\s/g, '').split(',')) {
      if (!part) continue;
      const match = /^(\d+)(?:-(\d+))?$/.exec(part);
      if (!match) continue;
      let start = Number(match[1]);
      let end = Number(match[2] ?? match[1]);
      if (!Number.isFinite(start) || !Number.isFinite(end)) continue;
      if (start > end) {
        const tmp = start;
        start = end;
        end = tmp;
      }
      const from = Math.max(rangeMin, start);
      const to = Math.min(rangeMax, end);
      for (let port = from; port <= to; port++) used.add(port);
    }
  }
  return used;
}

function pickIndex(length: number): number {
  if (length <= 0) return 0;
  const random = new Uint32Array(1);
  if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
    crypto.getRandomValues(random);
    return Math.floor((random[0] / 0x100000000) * length);
  }
  return Math.floor(Math.random() * length);
}

/**
 * Suggest a free port in [min,max] that is not listed in portSpecs.
 * Uses rejection sampling (lightweight) instead of materializing the full free list.
 */
export function randomListenerPort(
  portSpecs: string[],
  rangeMin = LISTENER_PORT_MIN,
  rangeMax = LISTENER_PORT_MAX,
): string | undefined {
  if (rangeMin > rangeMax) return undefined;
  const used = collectUsedPorts(portSpecs, rangeMin, rangeMax);
  const span = rangeMax - rangeMin + 1;
  if (used.size >= span) return undefined;

  for (let attempt = 0; attempt < MAX_ATTEMPTS; attempt++) {
    const candidate = rangeMin + pickIndex(span);
    if (!used.has(candidate)) return String(candidate);
  }

  // Fallback: sparse scan from a random offset (still O(span) worst case, rare).
  const offset = pickIndex(span);
  for (let i = 0; i < span; i++) {
    const candidate = rangeMin + ((offset + i) % span);
    if (!used.has(candidate)) return String(candidate);
  }
  return undefined;
}
