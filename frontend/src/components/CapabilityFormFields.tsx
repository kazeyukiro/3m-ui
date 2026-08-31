import React, { useMemo } from 'react';
import { Form, Input, InputNumber, Select, Switch, Divider, Radio, Space, Typography } from 'antd';
import type { ProtocolCapability, FieldCapability } from '../api/capabilities';
import { useI18n } from '../i18n';

const { Text } = Typography;

function FieldInput({ field }: { field: FieldCapability }) {
  switch (field.type) {
    case 'boolean': return <Switch />;
    case 'integer': return <InputNumber style={{ width: '100%' }} />;
    case 'secret': return <Input.Password placeholder={field.description} />;
    case 'text': return <Input.TextArea rows={2} placeholder={field.description} />;
    case 'string-list': return <Select mode="tags" tokenSeparators={[',']} placeholder={field.description} />;
    case 'string':
    default:
      if (field.options?.length) return <Select allowClear options={field.options.map((o) => ({ value: o, label: o }))} placeholder={field.description} />;
      return <Input placeholder={field.description} />;
  }
}

function renderFields(fields: FieldCapability[] | undefined, showAdvanced: boolean) {
  if (!fields?.length) return null;
  return fields.filter((f) => showAdvanced || !f.advanced).map((f) => {
    // Component fields are rendered conditionally inside shouldUpdate. AntD can
    // validate a stale/unmounted required item from the previous component and
    // report it as empty even though the currently selected component has a
    // value. Required checks are therefore performed by the submit handler for
    // protocol-specific critical fields (not by these transient Form.Items).
    const transientRequired = f.path === 'reality_dest' || f.path === 'reality_private_key';
    return (
      <Form.Item
        key={f.path}
        name={f.path}
        label={f.label}
        tooltip={f.description}
        rules={!transientRequired && f.required ? [{ required: true, whitespace: f.type === 'string' || f.type === 'text' || f.type === 'secret', message: `${f.label} is required` }] : undefined}
        valuePropName={f.type === 'boolean' ? 'checked' : 'value'}
      >
        <FieldInput field={f} />
      </Form.Item>
    );
  });
}

type Props = { protocol?: string; capability?: ProtocolCapability; showAdvanced?: boolean };

const CapabilityFormFields: React.FC<Props> = ({ protocol, capability, showAdvanced = false }) => {
  const { t } = useI18n();
  const transportComps = useMemo(() => {
    const list = (capability?.components || []).filter((c) => c.group === 'transport');
    // XHTTP is VLESS-only even if an older capability payload still lists it.
    if (protocol !== 'vless') return list.filter((c) => c.kind !== 'xhttp');
    return list;
  }, [capability, protocol]);
  const securityComps = useMemo(() => (capability?.components || []).filter((c) => c.group === 'security'), [capability]);
  if (!protocol || !capability) return <Text type="secondary">{t('listeners.selectProtocolFirst')}</Text>;
  const defaultTransport = capability.layers?.find((l) => l.group === 'transport')?.default_component || 'raw';
  const defaultSecurity = capability.layers?.find((l) => l.group === 'security')?.default_component || 'none';
  return (
    <Space direction="vertical" style={{ width: '100%' }} size="middle">
      {transportComps.length > 0 && <>
        <Divider titlePlacement="start" plain>{t('listeners.sectionTransport')}</Divider>
        <Form.Item name="transport_layer" label={t('listeners.sectionTransport')} initialValue={defaultTransport}>
          <Radio.Group optionType="button" buttonStyle="solid">{transportComps.map((c) => <Radio.Button key={c.kind} value={c.kind}>{c.label}</Radio.Button>)}</Radio.Group>
        </Form.Item>
        <Form.Item noStyle shouldUpdate={(a, b) => a.transport_layer !== b.transport_layer}>
          {({ getFieldValue }) => renderFields(transportComps.find((c) => c.kind === (getFieldValue('transport_layer') || defaultTransport))?.fields, showAdvanced)}
        </Form.Item>
      </>}
      {securityComps.length > 0 && <>
        <Divider titlePlacement="start" plain>{t('listeners.sectionTLS')}</Divider>
        <Form.Item name="security_layer" label={t('listeners.tls')} initialValue={defaultSecurity}>
          <Radio.Group optionType="button" buttonStyle="solid">{securityComps.map((c) => <Radio.Button key={c.kind} value={c.kind}>{c.label}</Radio.Button>)}</Radio.Group>
        </Form.Item>
        <Form.Item noStyle shouldUpdate={(a, b) => a.security_layer !== b.security_layer}>
          {({ getFieldValue }) => renderFields(securityComps.find((c) => c.kind === (getFieldValue('security_layer') || defaultSecurity))?.fields, showAdvanced)}
        </Form.Item>
      </>}
      {capability.fields?.length ? <><Divider titlePlacement="start" plain>{t('listeners.protocol')}</Divider>{renderFields(capability.fields, showAdvanced)}</> : null}
    </Space>
  );
};

export default CapabilityFormFields;

function getNestedValue(values: Record<string, any>, key: string): any {
  if (Object.prototype.hasOwnProperty.call(values, key)) return values[key];
  return key.split('.').reduce((current: any, part: string) => current == null ? undefined : current[part], values);
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
    if (!current[part] || typeof current[part] !== 'object' || Array.isArray(current[part])) current[part] = {};
    current = current[part];
  }
  current[parts[parts.length - 1]] = value;
}

function setIfPresent(cfg: Record<string, any>, path: string, value: any): void {
  if (value === undefined || value === null || value === '') return;
  setConfigPath(cfg, path, value);
}

function serializeFields(cfg: Record<string, any>, fields: FieldCapability[] | undefined, values: Record<string, any>, skip: (path: string) => boolean): void {
  for (const field of fields || []) {
    const path = field.path;
    if (!path || skip(path)) continue;
    const value = firstValue(values, path);
    if (value !== undefined && value !== null && value !== '') setConfigPath(cfg, path, value);
  }
}

/** Map capability-driven form values into Mihomo listener config JSON keys. */
export function capabilityFormToConfig(protocol: string, values: Record<string, any>, capability?: ProtocolCapability): Record<string, any> {
  const cfg: Record<string, any> = {};
  const set = (k: string, v: any) => setIfPresent(cfg, k, v);
  const transport = firstValue(values, 'transport_layer') || 'raw';
  if (transport === 'ws') set('ws-path', firstValue(values, 'ws-path', 'ws_path'));
  if (transport === 'grpc') set('grpc-service-name', firstValue(values, 'grpc-service-name', 'grpc_service_name'));
  if (transport === 'xhttp') {
    set('xhttp-config.path', firstValue(values, 'xhttp_path', 'xhttp-config.path'));
    set('xhttp-config.host', firstValue(values, 'xhttp_host', 'xhttp-config.host'));
    set('xhttp-config.mode', firstValue(values, 'xhttp_mode', 'xhttp-config.mode'));
  }

  const security = firstValue(values, 'security_layer') || 'none';
  if (security === 'reality') {
    set('reality-config.dest', firstValue(values, 'reality_dest', 'reality-config.dest'));
    set('reality-config.private-key', firstValue(values, 'reality_private_key', 'reality-config.private-key'));
    set('reality-config.short-id', firstValue(values, 'reality_short_id', 'reality-config.short-id'));
    set('reality-config.server-names', firstValue(values, 'reality_server_names', 'reality-config.server-names'));
  } else if (security === 'tls') {
    set('certificate', firstValue(values, 'certificate'));
    set('private-key', firstValue(values, 'private-key', 'private_key'));
    set('alpn', firstValue(values, 'alpn'));
    if (firstValue(values, 'allow-insecure') === true) cfg['allow-insecure'] = true;
  }

  const skipTransportSecurity = (path: string) => (
    path === 'transport_layer' || path === 'security_layer'
    || path === 'ws-path' || path === 'grpc-service-name'
    || path.startsWith('xhttp-config.') || path === 'xhttp-config'
    || path === 'reality-config' || path.startsWith('reality-config.')
    || path === 'reality_dest' || path === 'reality_private_key'
    || path === 'reality_short_id' || path === 'reality_server_names'
    || path === 'certificate' || path === 'private-key' || path === 'private_key'
    || path === 'allow-insecure'
  );
  const selectedTransport = capability?.components?.find((c) => c.group === 'transport' && c.kind === transport);
  const selectedSecurity = capability?.components?.find((c) => c.group === 'security' && c.kind === security);
  serializeFields(cfg, selectedTransport?.fields, values, skipTransportSecurity);
  serializeFields(cfg, selectedSecurity?.fields, values, skipTransportSecurity);
  serializeFields(cfg, capability?.fields, values, skipTransportSecurity);
  if (protocol === 'vless') {
    const flow = firstValue(values, 'flow');
    if (flow) set('flow', flow);
  }
  return cfg;
}
