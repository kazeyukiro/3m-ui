import React, { useMemo } from 'react';
import { Form, Input, InputNumber, Select, Switch, Divider, Radio, Space, Typography } from 'antd';
import type { ProtocolCapability, FieldCapability } from '../api/capabilities';
import { useI18n } from '../i18n';

const { Text } = Typography;

function FieldInput({ field }: { field: FieldCapability }) {
  switch (field.type) {
    case 'boolean':
      return <Switch />;
    case 'integer':
      return <InputNumber style={{ width: '100%' }} />;
    case 'secret':
      return <Input.Password placeholder={field.description} />;
    case 'text':
      return <Input.TextArea rows={2} placeholder={field.description} />;
    case 'string-list':
      return <Select mode="tags" tokenSeparators={[',']} placeholder={field.description} />;
    case 'string':
    default:
      if (field.options?.length) {
        return <Select allowClear options={field.options.map((o) => ({ value: o, label: o }))} placeholder={field.description} />;
      }
      return <Input placeholder={field.description} />;
  }
}

function renderFields(fields: FieldCapability[] | undefined, showAdvanced: boolean) {
  if (!fields?.length) return null;
  return fields
    .filter((f) => showAdvanced || !f.advanced)
    .map((f) => (
      <Form.Item
        key={f.path}
        name={f.path}
        label={f.label}
        tooltip={f.description}
        rules={f.required ? [{ required: true }] : undefined}
        valuePropName={f.type === 'boolean' ? 'checked' : 'value'}
      >
        <FieldInput field={f} />
      </Form.Item>
    ));
}

type Props = {
  protocol?: string;
  capability?: ProtocolCapability;
  showAdvanced?: boolean;
};

/**
 * Dynamic listener form driven by the backend capability manifest.
 * Nested Mihomo paths are represented by Ant Design's nested form values.
 */
const CapabilityFormFields: React.FC<Props> = ({ protocol, capability, showAdvanced = false }) => {
  const { t } = useI18n();
  const transportComps = useMemo(
    () => (capability?.components || []).filter((c) => c.group === 'transport'),
    [capability],
  );
  const securityComps = useMemo(
    () => (capability?.components || []).filter((c) => c.group === 'security'),
    [capability],
  );
  const hasTransport = transportComps.length > 0;
  const hasSecurity = securityComps.length > 0;

  if (!protocol || !capability) {
    return <Text type="secondary">{t('listeners.selectProtocolFirst')}</Text>;
  }

  const defaultTransport = capability.layers?.find((l) => l.group === 'transport')?.default_component || 'raw';
  const defaultSecurity = capability.layers?.find((l) => l.group === 'security')?.default_component || 'none';

  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      {hasTransport && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionTransport')}</Divider>
          <Form.Item name="transport_layer" label={t('listeners.sectionTransport')} initialValue={defaultTransport}>
            <Radio.Group optionType="button" buttonStyle="solid">
              {transportComps.map((c) => (
                <Radio.Button key={c.kind} value={c.kind}>{c.label}</Radio.Button>
              ))}
            </Radio.Group>
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(a, b) => a.transport_layer !== b.transport_layer}>
            {({ getFieldValue }) => {
              const kind = getFieldValue('transport_layer') || defaultTransport;
              const comp = transportComps.find((c) => c.kind === kind);
              return renderFields(comp?.fields, showAdvanced);
            }}
          </Form.Item>
        </>
      )}

      {hasSecurity && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.sectionTLS')}</Divider>
          <Form.Item name="security_layer" label={t('listeners.tls')} initialValue={defaultSecurity}>
            <Radio.Group optionType="button" buttonStyle="solid">
              {securityComps.map((c) => (
                <Radio.Button key={c.kind} value={c.kind}>{c.label}</Radio.Button>
              ))}
            </Radio.Group>
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(a, b) => a.security_layer !== b.security_layer}>
            {({ getFieldValue }) => {
              const kind = getFieldValue('security_layer') || defaultSecurity;
              const comp = securityComps.find((c) => c.kind === kind);
              return renderFields(comp?.fields, showAdvanced);
            }}
          </Form.Item>
        </>
      )}

      {capability.fields && capability.fields.length > 0 && (
        <>
          <Divider titlePlacement="start" plain>{t('listeners.protocol')}</Divider>
          {renderFields(capability.fields, showAdvanced)}
        </>
      )}
    </Space>
  );
};

export default CapabilityFormFields;

function getNestedValue(values: Record<string, any>, key: string): any {
  if (Object.prototype.hasOwnProperty.call(values, key)) return values[key];
  return key.split('.').reduce((current: any, part: string) => {
    if (current === undefined || current === null) return undefined;
    return current[part];
  }, values);
}

function firstValue(values: Record<string, any>, ...keys: string[]): any {
  for (const key of keys) {
    const value = getNestedValue(values, key);
    if (value !== undefined && value !== null && value !== '') return value;
  }
  return undefined;
}

/** Map capability-driven form values into Mihomo listener config JSON keys. */
export function capabilityFormToConfig(
  protocol: string,
  values: Record<string, any>,
  _capability?: ProtocolCapability,
): Record<string, any> {
  const cfg: Record<string, any> = {};
  const set = (k: string, v: any) => {
    if (v === undefined || v === null || v === '') return;
    cfg[k] = v;
  };

  // Transport exclusivity. Capability paths may be nested by Ant Design.
  const transport = firstValue(values, 'transport_layer') || 'raw';
  if (transport === 'ws') set('ws-path', firstValue(values, 'ws-path', 'ws_path'));
  if (transport === 'grpc') set('grpc-service-name', firstValue(values, 'grpc-service-name', 'grpc_service_name'));
  if (transport === 'xhttp') {
    const x: Record<string, any> = {};
    const path = firstValue(values, 'xhttp_path', 'xhttp-config.path');
    const host = firstValue(values, 'xhttp_host', 'xhttp-config.host');
    const mode = firstValue(values, 'xhttp_mode', 'xhttp-config.mode');
    if (path !== undefined) x.path = path;
    if (host !== undefined) x.host = host;
    if (mode !== undefined) x.mode = mode;
    if (Object.keys(x).length) cfg['xhttp-config'] = x;
  }

  // Security exclusivity. Reality fields must work whether the capability
  // manifest exposes flat names or Mihomo-style nested paths.
  const security = firstValue(values, 'security_layer') || 'none';
  if (security === 'reality') {
    const r: Record<string, any> = {};
    const dest = firstValue(values, 'reality_dest', 'reality-config.dest');
    const privateKey = firstValue(values, 'reality_private_key', 'reality-config.private-key');
    const shortID = firstValue(values, 'reality_short_id', 'reality-config.short-id');
    const serverNames = firstValue(values, 'reality_server_names', 'reality-config.server-names');
    if (dest !== undefined) r.dest = dest;
    if (privateKey !== undefined) r['private-key'] = privateKey;
    if (shortID !== undefined) r['short-id'] = shortID;
    if (serverNames !== undefined) r['server-names'] = serverNames;
    if (Object.keys(r).length) cfg['reality-config'] = r;
  } else if (security === 'tls') {
    set('certificate', firstValue(values, 'certificate'));
    set('private-key', firstValue(values, 'private-key', 'private_key'));
    set('alpn', firstValue(values, 'alpn'));
    if (firstValue(values, 'allow-insecure') === true) cfg['allow-insecure'] = true;
  }

  // Protocol fields by path (from capability). Exclude all transport/security
  // component fields handled explicitly above, including nested objects.
  const skip = new Set([
    'transport_layer', 'security_layer', 'name', 'protocol', 'port', 'bind_address', 'enabled', 'udp',
    'public_host', 'public_port', 'access_sni', 'client_fingerprint', 'access_alpn',
    'ws-path', 'grpc-service-name', 'xhttp_path', 'xhttp_host', 'xhttp_mode',
    'xhttp-config', 'reality_dest', 'reality_private_key', 'reality_short_id', 'reality_server_names',
    'reality-config', 'certificate', 'private-key', 'private_key', 'allow-insecure',
  ]);
  for (const [k, v] of Object.entries(values)) {
    if (skip.has(k)) continue;
    if (v === undefined || v === null || v === '') continue;
    cfg[k] = v;
  }

  // VLESS flow
  const flow = firstValue(values, 'flow');
  if (protocol === 'vless' && flow) set('flow', flow);

  return cfg;
}
