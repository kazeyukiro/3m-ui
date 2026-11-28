/** Remove empty optional form values without dropping false or zero. */
export function cleanFormValue(value: any): any {
  if (value === '' || value === undefined || value === null) return undefined;
  if (Array.isArray(value)) {
    const items = value.map(cleanFormValue).filter(v => v !== undefined);
    return items.length ? items : undefined;
  }
  if (typeof value === 'object') {
    const entries = Object.entries(value).map(([key, v]) => [key, cleanFormValue(v)] as const).filter(([key, v]) => key !== '' && v !== undefined);
    return entries.length ? Object.fromEntries(entries) : undefined;
  }
  return value;
}
