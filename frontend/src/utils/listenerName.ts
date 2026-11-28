/** Suggest the first unused listener name, reusing gaps in the sequence. */
export function suggestListenerName(names: string[]): string {
  const usedNames = new Set(
    names
      .map((name) => name.trim())
      .filter(Boolean)
      .filter((name) => !/__deleted_/i.test(name)),
  );
  let number = 1;
  while (usedNames.has(`节点${number}`)) number += 1;
  return `节点${number}`;
}

/** Next candidate after a UNIQUE name conflict from the server. */
export function nextListenerNameAfterConflict(current: string, names: string[]): string {
  const base = (current || '').replace(/__deleted_.*$/, '').trim() || '节点';
  const used = new Set(names.map((n) => n.trim()).filter(Boolean));
  if (current.trim()) used.add(current.trim());
  const match = /^(.*?)(\d+)$/.exec(base);
  if (match) {
    let n = Number(match[2]) || 1;
    const prefix = match[1];
    for (let i = 0; i < 1000; i += 1) {
      n += 1;
      const candidate = `${prefix}${n}`;
      if (!used.has(candidate)) return candidate;
    }
  }
  for (let n = 1; n < 10000; n += 1) {
    const candidate = `节点${n}`;
    if (!used.has(candidate)) return candidate;
  }
  return `节点${Date.now()}`;
}
