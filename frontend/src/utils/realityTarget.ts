export interface RealityNameValues {
  reality_server_names?: string[];
  access_sni?: string;
}

/** Clear names still owned by the last scan when its destination is edited.
 * Empty names are derived from the new destination by the save/export path.
 * Independently customized names must remain under the user's control.
 */
export function clearScannedRealityNames(
  scannedName: string | null,
  values: RealityNameValues,
): Partial<RealityNameValues> {
  const changes: Partial<RealityNameValues> = {};
  if (!scannedName) return changes;
  if (values.access_sni === scannedName) changes.access_sni = '';
  if (values.reality_server_names?.length === 1 && values.reality_server_names[0] === scannedName) {
    changes.reality_server_names = [];
  }
  return changes;
}
