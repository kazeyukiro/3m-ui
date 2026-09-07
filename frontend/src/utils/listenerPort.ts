/** Suggest a port without reusing any configured port, including lists and ranges. */
export function randomListenerPort(portSpecs: string[]): string | undefined {
  const min = 10000;
  const max = 60000;
  const used = new Set<number>();
  for (const spec of portSpecs) {
    for (const part of String(spec).replace(/\s/g, '').split(',')) {
      const match = /^(\d+)(?:-(\d+))?$/.exec(part);
      if (!match) continue;
      const start = Math.max(min, Number(match[1]));
      const end = Math.min(max, Number(match[2] ?? match[1]));
      for (let port = start; port <= end; port++) used.add(port);
    }
  }
  const available: number[] = [];
  for (let port = min; port <= max; port++) {
    if (!used.has(port)) available.push(port);
  }
  if (!available.length) return undefined;
  const random = new Uint32Array(1);
  crypto.getRandomValues(random);
  return String(available[Math.floor(random[0] / 0x100000000 * available.length)]);
}
