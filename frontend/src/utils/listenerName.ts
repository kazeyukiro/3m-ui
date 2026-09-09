/** Suggest the first unused listener name, reusing gaps in the sequence. */
export function suggestListenerName(names: string[]): string {
  const usedNames = new Set(names.map((name) => name.trim()));
  let number = 1;
  while (usedNames.has(`节点${number}`)) number++;
  return `节点${number}`;
}
