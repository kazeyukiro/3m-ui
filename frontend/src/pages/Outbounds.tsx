import React, { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { Alert, Button, Card, Checkbox, Collapse, Form, Input, Modal, Popconfirm, Select, Space, Switch, Table, Tag, Typography, message } from 'antd';
import { IconAddNode, IconApplyRules, IconBranch, IconDelete, IconEdit, IconRefreshList } from '../icons';
import PageHeader from '../components/PageHeader';
import OutboundFields from '../components/OutboundFields';
import { OutboundConnectionDrawer, OutboundConnectionResult } from '../components/OutboundConnectionTest';
import { useOutboundConnectionMessages } from '../i18n/outboundConnection';
import useIsMobile from '../hooks/useIsMobile';
import { useI18n } from '../i18n';
import type { ServerOutbound } from '../api/outbounds';
import { useOutboundStore } from '../stores/outboundStore';
import { OUTBOUND_PROTOCOLS, OUTBOUND_REQUIRED, outboundDefaults, outboundFields, outboundFromForm, outboundToForm } from '../utils/outboundForm';

export default function Outbounds() {
  const { t } = useI18n();
  const mobile = useIsMobile();
  const store = useOutboundStore();
  const connectionText = useOutboundConnectionMessages();
  const [testName, setTestName] = useState<string | null>(null);
  const busy = store.saving || store.testing !== null;
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState<React.Key[]>([]);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<ServerOutbound>();
  const [source, setSource] = useState<ServerOutbound>();
  const [formError, setFormError] = useState('');
  const [form] = Form.useForm();
  const protocol = Form.useWatch('protocol', form) || 'vless';
  const peers = Form.useWatch('peers', form);
  const ports = Form.useWatch('ports', form);
  const portRange = Form.useWatch('port-range', form);
  const notifyError = (e: any) => message.error(e?.response?.data?.error || e?.message || String(e));
  const load = store.load;
  useEffect(() => { void load().catch(notifyError); }, [load]);

  const start = (entry?: ServerOutbound, clone = false) => {
    setEditing(clone ? undefined : entry);
    setSource(entry);
    setFormError('');
    form.resetFields();
    const values: Record<string, any> = entry ? outboundToForm(entry) : outboundDefaults('vless');
    if (clone && entry) {
      let name = `${entry.proxy.name}-copy`;
      for (let n = 2; store.entries.some(e => e.proxy.name === name); n++) name = `${entry.proxy.name}-copy-${n}`;
      values.name = name;
    }
    form.setFieldsValue(values);
    setOpen(true);
  };
  const submit = async () => {
    try {
      await form.validateFields();
      await store.save(outboundFromForm(form.getFieldsValue(true), source), editing?.proxy.name);
      setOpen(false);
      message.success(t('outbounds.saved'));
    } catch (e: any) {
      if (e?.errorFields) return;
      const text = e?.response?.data?.error || e?.message || String(e);
      setFormError(text);
    }
  };
  const setEnabled = async (names: string[], enabled: boolean) => {
    try { await store.setEnabled(names, enabled); setSelected([]); message.success(t('outbounds.saved')); } catch (e) { notifyError(e); }
  };
  const test = (name: string) => { setTestName(name); void store.test(name).catch(notifyError); };
  const connectionTest = (entry: ServerOutbound) => <Space wrap>
    <Button size="small" icon={<IconRefreshList />} loading={store.testing === entry.proxy.name} disabled={!entry.enabled || busy || store.loading} onClick={() => test(entry.proxy.name)}>{connectionText.check}</Button>
    {store.results[entry.proxy.name] && <Button type="text" size="small" onClick={() => setTestName(entry.proxy.name)}><OutboundConnectionResult result={store.results[entry.proxy.name]} /></Button>}
  </Space>;
  const actions = (entry: ServerOutbound) => entry.managed ? <Link to="/settings?section=network">{t('outbounds.managed')}</Link> : <Space wrap>
    <Button size="small" title={t('common.edit')} icon={<IconEdit />} onClick={() => start(entry)} disabled={busy} />
    <Button size="small" title={t('listeners.clone')} icon={<IconBranch />} onClick={() => start(entry, true)} disabled={busy} />
    <Popconfirm title={t('outbounds.deleteConfirm')} onConfirm={async () => { try { await store.remove(entry.proxy.name); setSelected(keys => keys.filter(k => k !== entry.proxy.name)); message.success(t('outbounds.saved')); } catch (e) { notifyError(e); } }}>
      <Button size="small" danger title={t('common.delete')} icon={<IconDelete />} disabled={busy} />
    </Popconfirm>
  </Space>;
  const enabledSwitch = (entry: ServerOutbound) => <Switch checked={entry.enabled} disabled={entry.managed || busy} onChange={enabled => void setEnabled([entry.proxy.name], enabled)} />;
  const rows = store.entries.filter(e => `${e.proxy.name} ${e.proxy.type} ${e.proxy.server || ''}`.toLowerCase().includes(search.toLowerCase()));
  const alternatePorts = (protocol === 'hysteria2' && ports) || (protocol === 'mieru' && portRange);
  const required = ['name', 'server', 'port', ...(OUTBOUND_REQUIRED[protocol] || []), ...(protocol === 'wireguard' && !peers?.length ? ['public-key'] : [])].filter(key => !(protocol === 'wireguard' && peers?.length && ['server', 'port'].includes(key)) && !(key === 'port' && alternatePorts));
  const basic = protocol === 'wireguard' ? [...required, 'ip', 'ipv6', 'peers', 'server', 'port', 'public-key']
    : [...required, 'port', ...(protocol === 'hysteria2' ? ['ports'] : protocol === 'mieru' ? ['port-range'] : [])];
  const fields = outboundFields(protocol);
  const toolbar = <Space wrap className="page-toolbar">
    {selected.length > 0 && <><Button disabled={busy} onClick={() => void setEnabled(selected.map(String), true)}>{t('listeners.enableSelected')}</Button><Button disabled={busy} onClick={() => void setEnabled(selected.map(String), false)}>{t('listeners.disableSelected')}</Button></>}
    <Input.Search allowClear placeholder={t('common.search')} value={search} onChange={e => setSearch(e.target.value)} style={{ width: mobile ? '100%' : 180 }} />
    <Button icon={<IconRefreshList />} loading={store.loading} disabled={busy} onClick={() => void store.load().catch(notifyError)}>{t('common.refresh')}</Button>
    <Button icon={<IconApplyRules />} loading={store.applying} disabled={store.saving} onClick={async () => { try { await store.apply(); message.success(t('outbounds.applied')); } catch (e) { notifyError(e); } }}>{t('routing.applyNow')}</Button>
    <Button type="primary" icon={<IconAddNode />} disabled={busy} onClick={() => start()}>{t('outbounds.create')}</Button>
  </Space>;
  return <>
    <PageHeader title={t('nav.outbounds')} subtitle={t('outbounds.description')} />
    <Alert type="info" showIcon title={t('outbounds.applyHint')} style={{ marginBottom: 16 }} />
    <Card title={t('nav.outbounds')} extra={mobile ? undefined : toolbar}>
      {mobile && <div style={{ marginBottom: 16 }}>{toolbar}</div>}
      {mobile ? <Space orientation="vertical" style={{ width: '100%' }}>
        {rows.length === 0 && <Typography.Text type="secondary">{t('outbounds.empty')}</Typography.Text>}
        {rows.map(entry => <Card key={entry.proxy.name} size="small" title={<Space>{!entry.managed && <Checkbox checked={selected.includes(entry.proxy.name)} onChange={e => setSelected(keys => e.target.checked ? [...keys, entry.proxy.name] : keys.filter(k => k !== entry.proxy.name))} />}<span>{entry.proxy.name}</span></Space>} extra={enabledSwitch(entry)}>
          <Space orientation="vertical"><Tag>{entry.proxy.type}</Tag><Typography.Text>{entry.proxy.server || '—'}{entry.proxy.port ? `:${entry.proxy.port}` : ''}</Typography.Text>{connectionTest(entry)}{actions(entry)}</Space>
        </Card>)}
      </Space> : <Table rowKey={entry => entry.proxy.name} loading={store.loading} dataSource={rows} scroll={{ x: 720 }}
        rowSelection={{ selectedRowKeys: selected, onChange: setSelected, getCheckboxProps: entry => ({ disabled: entry.managed }) }}
        columns={[
          { title: t('outbounds.fields.name'), render: (_, e) => e.proxy.name },
          { title: t('listeners.protocol'), render: (_, e) => <Tag>{e.proxy.type}</Tag> },
          { title: t('outbounds.fields.server'), render: (_, e) => e.proxy.server || '—' },
          { title: t('outbounds.fields.port'), render: (_, e) => e.proxy.port || '—' },
          { title: t('common.enabled'), render: (_, e) => enabledSwitch(e) },
          { title: connectionText.title, render: (_, e) => connectionTest(e) },
          { title: t('common.actions'), render: (_, e) => actions(e) },
        ]} />}
    </Card>
    <OutboundConnectionDrawer name={testName} result={testName !== null ? store.results[testName] : undefined} checking={testName !== null && store.testing === testName} disabled={busy || store.loading || !store.entries.some(entry => entry.proxy.name === testName && entry.enabled)} onClose={() => setTestName(null)} onCheck={() => { if (testName !== null) test(testName); }} />
    <Modal title={editing ? t('outbounds.edit') : t('outbounds.create')} open={open} onCancel={() => setOpen(false)} onOk={submit} confirmLoading={store.saving} width={760} destroyOnHidden>
      {formError && <Alert type="error" showIcon title={formError} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={submit}>
        <Form.Item name="protocol" label={t('listeners.protocol')} rules={[{ required: true }]}>
          <Select options={OUTBOUND_PROTOCOLS.map(value => ({ value, label: value === 'wireguard' ? 'WireGuard' : value }))} onChange={next => {
            const common = form.getFieldsValue(['name', 'server', 'port', 'enabled']);
            form.resetFields(); form.setFieldsValue({ ...outboundDefaults(next), ...common });
          }} />
        </Form.Item>
        <Form.Item name="enabled" label={t('common.enabled')} valuePropName="checked"><Switch /></Form.Item>
        <Typography.Paragraph strong>{t('outbounds.required')}</Typography.Paragraph>
        {protocol === 'wireguard' && <Alert type="info" title={t('outbounds.wireguardHint')} style={{ marginBottom: 16 }} />}
        <OutboundFields fields={fields.filter(([key]) => basic.includes(key))} required={required} protocol={protocol} />
        <Collapse items={[{ key: 'optional', label: t('outbounds.optional'), forceRender: true, children: <OutboundFields fields={fields.filter(([key]) => !basic.includes(key))} protocol={protocol} /> }]} />
      </Form>
    </Modal>
  </>;
}
