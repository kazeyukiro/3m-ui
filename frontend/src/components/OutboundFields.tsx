import React from 'react';
import { Button, Card, Collapse, Form, Input, InputNumber, Select, Space, Switch, message } from 'antd';
import { IconAddNode, IconDelete } from '../icons';
import { useI18n } from '../i18n';
import { OUTBOUND_OBJECTS, type OutboundField } from '../utils/outboundSchema';

type MapKind = 'stringMap' | 'stringListMap' | 'map';

function ValueMap({ value = {}, onChange, kind }: { value?: Record<string, any>; onChange?: (value: Record<string, any>) => void; kind: MapKind }) {
  const { t } = useI18n();
  const entries = Object.entries(value);
  const update = (index: number, key: string, v: any) => {
    if (key !== entries[index][0] && Object.hasOwn(value, key)) { message.error(t('outbounds.duplicateKey')); return; }
    onChange?.(Object.fromEntries(entries.map((entry, i) => i === index ? [key, v] : entry)));
  };
  return <Space orientation="vertical" style={{ width: '100%' }}>
    {entries.map(([key, v], index) => <Card size="small" key={index}>
      <Space wrap style={{ width: '100%' }}>
        <Input aria-label={t('outbounds.key')} placeholder={t('outbounds.key')} value={key} onChange={e => update(index, e.target.value, v)} />
        <Button aria-label={t('common.delete')} icon={<IconDelete />} onClick={() => onChange?.(Object.fromEntries(entries.filter((_, i) => i !== index)))} />
      </Space>
      <div style={{ marginTop: 8 }}>
        {kind === 'map' ? <FlexibleValue value={v} onChange={next => update(index, key, next)} />
          : kind === 'stringListMap' ? <Select mode="tags" style={{ width: '100%' }} value={v} onChange={next => update(index, key, next)} />
          : <Input value={v} onChange={e => update(index, key, e.target.value)} />}
      </div>
    </Card>)}
    <Button icon={<IconAddNode />} onClick={() => onChange?.({ ...value, '': kind === 'stringListMap' ? [] : '' })} disabled={Object.hasOwn(value, '')}>{t('outbounds.addEntry')}</Button>
  </Space>;
}

function FlexibleValue({ value, onChange }: { value: any; onChange: (value: any) => void }) {
  const kind = Array.isArray(value) ? 'list' : value !== null && typeof value === 'object' ? 'object' : typeof value;
  const defaults: Record<string, any> = { string: '', number: 0, boolean: false, list: [], object: {} };
  return <Space orientation="vertical" style={{ width: '100%' }}>
    <Select aria-label="Value type" value={kind} options={Object.keys(defaults).map(v => ({ value: v, label: v }))} onChange={v => onChange(defaults[v])} style={{ width: 130 }} />
    {kind === 'object' ? <ValueMap kind="map" value={value} onChange={onChange} />
      : kind === 'list' ? <Select mode="tags" style={{ width: '100%' }} value={value.map(String)} onChange={v => onChange(v)} />
      : kind === 'boolean' ? <Switch checked={value} onChange={onChange} />
      : kind === 'number' ? <InputNumber value={value} onChange={onChange} style={{ width: '100%' }} />
      : <Input value={value} onChange={e => onChange(e.target.value)} />}
  </Space>;
}

const isSecret = (key: string) => /password|private-key|^psk$|^token$|^key$|primary-key|pre-shared-key/.test(key);

export default function OutboundFields({ fields, prefix = [], required = [], protocol }: { fields: readonly OutboundField[]; prefix?: (string | number)[]; required?: string[]; protocol?: string }) {
  const { t } = useI18n();
  return <>{fields.map(([key, kind, object]) => {
    const path = [...prefix, key];
    const label = t(`outbounds.fields.${key}`, key);
    if (kind === 'object') return <Collapse key={key} style={{ marginBottom: 16 }} items={[{ key, label, children: <OutboundFields fields={OUTBOUND_OBJECTS[object!]} prefix={path} protocol={protocol} /> }]} />;
    if (kind === 'objects') return <Form.Item key={key} label={label}>
      <Form.List name={path}>{(rows, { add, remove }) => <Space orientation="vertical" style={{ width: '100%' }}>
        {rows.map(row => <Card size="small" key={row.key} extra={<Button aria-label={t('common.delete')} icon={<IconDelete />} onClick={() => remove(row.name)} />}>
          <OutboundFields fields={OUTBOUND_OBJECTS[object!]} prefix={[row.name]} protocol={protocol} required={object === 'WireGuardPeerOption' ? ['server', 'port', 'public-key'] : []} />
        </Card>)}
        <Button icon={<IconAddNode />} onClick={() => add({})}>{t('outbounds.addEntry')}</Button>
      </Space>}</Form.List>
    </Form.Item>;
    const input = kind === 'boolean' ? <Select allowClear placeholder={t('outbounds.defaultValue')} options={[{ value: 'true', label: t('common.enabled') }, { value: 'false', label: t('common.disabled') }]} />
      : kind === 'number' ? <InputNumber style={{ width: '100%' }} precision={0} min={key === 'port' ? 1 : undefined} max={key === 'port' ? 65535 : undefined} />
      : kind === 'strings' || kind === 'numbers' ? <Select mode="tags" style={{ width: '100%' }} />
      : kind === 'stringMap' || kind === 'stringListMap' || kind === 'map' ? <ValueMap kind={kind} />
      : key === 'certificate' || (key === 'private-key' && protocol !== 'wireguard') ? <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} spellCheck={false} />
      : isSecret(key) ? <Input.Password autoComplete="new-password" />
      : <Input />;
    return <Form.Item key={key} name={path} label={label} tooltip={label === key ? undefined : key}
      getValueProps={kind === 'boolean' ? (value: boolean) => ({ value: value === undefined ? undefined : String(value) }) : kind === 'numbers' ? (value: number[]) => ({ value: value?.map(String) }) : undefined}
      getValueFromEvent={kind === 'boolean' ? (value: string) => value === undefined ? undefined : value === 'true' : kind === 'numbers' ? (values: string[]) => values.map(Number) : undefined}
      rules={[
        ...(required.includes(key) ? [{ required: true, message: t('outbounds.requiredField') }] : []),
        ...(kind === 'numbers' ? [{ validator: async (_, values: number[]) => { if (values?.some(v => !Number.isInteger(v) || (key === 'reserved' && (v < 0 || v > 255)))) throw new Error(t('outbounds.invalidNumbers')); } }] : []),
      ]}>{input}</Form.Item>;
  })}</>;
}
