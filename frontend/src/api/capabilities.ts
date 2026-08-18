import client from './client';

export type FieldType = 'string' | 'text' | 'secret' | 'boolean' | 'integer' | 'string-list';

export interface FieldCapability {
  path: string;
  label: string;
  type: FieldType;
  required?: boolean;
  advanced?: boolean;
  options?: string[];
  description?: string;
}

export interface LayerCapability {
  group: 'transport' | 'security' | 'extension';
  required: boolean;
  multiple: boolean;
  default_component?: string;
}

export interface ComponentCapability {
  group: string;
  kind: string;
  label: string;
  selection_path?: string;
  enabled_path?: string;
  fields?: FieldCapability[];
  conflicts?: string[];
}

export interface ProtocolCapability {
  kind: string;
  label: string;
  layers: LayerCapability[];
  components: ComponentCapability[];
  fields?: FieldCapability[];
  user_fields?: FieldCapability[];
  features?: string[];
}

export interface CapabilityManifest {
  schema_version: number;
  node_schema_version: number;
  node_fields: FieldCapability[];
  access_profile_fields: FieldCapability[];
  protocols: ProtocolCapability[];
}

let cache: CapabilityManifest | null = null;

export async function fetchCapabilities(force = false): Promise<CapabilityManifest> {
  if (cache && !force) return cache;
  const { data } = await client.get<CapabilityManifest>('/capabilities');
  cache = data;
  return data;
}

export function protocolCapability(manifest: CapabilityManifest, kind: string): ProtocolCapability | undefined {
  return manifest.protocols.find((p) => p.kind === kind);
}
