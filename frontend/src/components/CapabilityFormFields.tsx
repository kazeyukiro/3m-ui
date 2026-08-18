import React, { useMemo } from 'react';
import { Form, Input, InputNumber, Select, Switch, Divider, Radio, Space, Typography } from 'antd';
import type { ProtocolCapability, FieldCapability, ComponentCapability } from '../api/capabilities';
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
 * Dynamic node form driven by backend capability JSON (m-ui style).
 * Renders transport/security layers + protocol fields from the manifest.
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

/** Map capability-driven form values into Mihomo listener config JSON keys. */
export function capabilityFormToConfig(
  protocol: string,
  values: Record<string, any>,
  capability?: ProtocolCapability,
): Record<string, any> {
  const cfg: Record<string, any> = {};
  const set = (k: string, v: any) => {
    if (v === undefined || v === null || v === '') return;
    cfg[k] = v;
  };

  // Transport exclusivity
  const transport = values.transport_layer || 'raw';
  if (transport === 'ws') set('ws-path', values['ws-path'] || values.ws_path);
  if (transport === 'grpc') set('grpc-service-name', values['grpc-service-name'] || values.grpc_service_name);
  if (transport === 'xhttp') {
    const x: Record<string, any> = {};
    if (values.xhttp_path) x.path = values.xhttp_path;
    if (values.xhttp_host) x.host = values.xhttp_host;
    if (values.xhttp_mode) x.mode = values.xhttp_mode;
    if (Object.keys(x).length) cfg['xhttp-config'] = x;
  }

  // Security exclusivity
  const security = values.security_layer || 'none';
  if (security === 'reality') {
    const r: Record<string, any> = {};
    if (values.reality_dest) r.dest = values.reality_dest;
    if (values.reality_private_key) r['private-key'] = values.reality_private_key;
    if (values.reality_short_id) r['short-id'] = values.reality_short_id;
    if (values.reality_server_names) r['server-names'] = values.reality_server_names;
    if (Object.keys(r).length) cfg['reality-config'] = r;
  } else if (security === 'tls') {
    set('certificate', values.certificate);
    set('private-key', values['private-key'] || values.private_key);
    set('alpn', values.alpn);
    if (values['allow-insecure'] === true) cfg['allow-insecure'] = true;
  }

  // Protocol fields by path (from capability)
  const skip = new Set([
    'transport_layer', 'security_layer', 'name', 'protocol', 'port', 'bind_address', 'enabled', 'udp',
    'public_host', 'public_port', 'access_sni', 'client_fingerprint', 'access_alpn',
    'ws-path', 'grpc-service-name', 'xhttp_path', 'xhttp_host', 'xhttp_mode',
    'reality_dest', 'reality_private_key', 'reality_short_id', 'reality_server_names',
    'certificate', 'private-key', 'private_key', 'allow-insecure',
  ]);
  for (const [k, v] of Object.entries(values)) {
    if (skip.has(k)) continue;
    if (v === undefined || v === null || v === '') continue;
    // capability paths already use mihomo keys where possible
    cfg[k] = v;
  }

  // VLESS flow
  if (protocol === 'vless' && values.flow) set('flow', values.flow);

  return cfg;
}
