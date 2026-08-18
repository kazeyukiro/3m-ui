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

function setConfigPath(cfg: Record<string, any>, path: string, value: any): void {
  const parts = path.split('.');
  let current = cfg;
  for (let i = 0; i < parts.length - 1; i += 1) {
    const part = parts[i];
    if (!current[part] || typeof current[part] !== 'object' || Array.isArray(current[part])) {
      current[part] = {};
    }
    current = current[part];
  }
  current[parts[parts.length - 1]] = value;
}

function setIfPresent(cfg: Record<string, any>, path: string, value: any): void {
  if (value === undefined || value === null || value === '') return;
  setConfigPath(cfg, path, value);
}

/** Map capability-driven form values into Mihomo listener config JSON keys. */
export function capabilityFormToConfig(
  protocol: string,
  values: Record<string, any>,
  capability?: ProtocolCapability,
): Record<string, any> {
  const cfg: Record<string, any> = {};
  const set = (k: string, v: any) => setIfPresent(cfg, k, v);

  // Transport exclusivity. Capability paths may be nested by Ant Design.
  const transport = firstValue(values, 'transport_layer') || 'raw';
  if (transport === 'ws') set('ws-path', firstValue(values, 'ws-path', 'ws_path'));
  if (transport === 'grpc') set('grpc-service-name', firstValue(values, 'grpc-service-name', 'grpc_service_name'));
  if (transport === 'xhttp') {
    const path = firstValue(values, 'xhttp_path', 'xhttp-config.path');
    const host = firstValue(values, 'xhttp_host', 'xhttp-config.host');
    const mode = firstValue(values, 'xhttp_mode', 'xhttp-config.mode');
    setIfPresent(cfg, 'xhttp-config.path', path);
    setIfPresent(cfg, 'xhttp-config.host', host);
    setIfPresent(cfg, 'xhttp-config.mode', mode);
  }

  // Security exclusivity. Reality fields work whether the capability manifest
  // exposes flat names or Mihomo-style nested paths.
  const security = firstValue(values, 'security_layer') || 'none';
  if (security === 'reality') {
    setIfPresent(cfg, 'reality-config.dest', firstValue(values, 'reality_dest', 'reality-config.dest'));
    setIfPresent(cfg, 'reality-config.private-key', firstValue(values, 'reality_private_key', 'reality-config.private-key'));
    setIfPresent(cfg, 'reality-config.short-id', firstValue(values, 'reality_short_id', 'reality-config.short-id'));
    setIfPresent(cfg, 'reality-config.server-names', firstValue(values, 'reality_server_names', 'reality-config.server-names'));
  } else if (security === 'tls') {
    set('certificate', firstValue(values, 'certificate'));
    set('private-key', firstValue(values, 'private-key', 'private_key'));
    set('alpn', firstValue(values, 'alpn'));
    if (firstValue(values, 'allow-insecure') === true) cfg['allow-insecure'] = true;
  }

  // Only serialize fields declared by the capability manifest. Previously this
  // loop copied every Form value, including legacy UI-only aliases such as
  // reality_enabled, reality_dest, shadow_tls_enabled, etc. Those keys are not
  // Mihomo config keys and caused valid configurations to be rejected by the
  // backend schema when an existing listener was edited.
  for (const field of capability?.fields || []) {
    const path = field.path;
    if (!path || path === 'transport_layer' || path === 'security_layer') continue;
    if (path === 'ws-path' || path === 'grpc-service-name' || path.startsWith('xhttp-config.')) continue;
    if (path === 'reality-config' || path.startsWith('reality-config.')) continue;
    if (path === 'certificate' || path === 'private-key' || path === 'private_key' || path === 'allow-insecure') continue;
    const value = firstValue(values, path);
    if (value !== undefined && value !== null && value !== '') setConfigPath(cfg, path, value);
  }

  // VLESS flow may be represented by the legacy visual form even when the
  // capability manifest does not expose it yet.
  const flow = firstValue(values, 'flow');
  if (protocol === 'vless' && flow) set('flow', flow);

  return cfg;
}
