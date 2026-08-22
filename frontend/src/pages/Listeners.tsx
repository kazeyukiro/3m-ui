import React, { useEffect, useMemo, useState } from 'react';
import { Table, Button, Space, Tag, Modal, Form, Input, Select, Switch, message, Popconfirm, Tooltip, Card, Tabs, Descriptions, Divider } from 'antd';
import { PlusOutlined, ReloadOutlined, QrcodeOutlined, DeleteOutlined, EditOutlined, CopyOutlined, BranchesOutlined, HistoryOutlined, SaveOutlined, PoweroffOutlined, DiffOutlined } from '@ant-design/icons';
import {
  fetchListeners, createListener, updateListener, deleteListener, reloadListener, exportNodeURI, normalizeId, Listener,
} from '../api/nodes';
import {
  listListenerTemplates, createListenerTemplate, deleteListenerTemplate, instantiateListenerTemplate,
  cloneListener, batchSetListenersEnabled, listListenerVersions, diffListenerVersion, rollbackListenerVersion,
  quickSetupListener,
  ListenerTemplate, ListenerVersion,
} from '../api/listeners';
import { useI18n } from '../i18n';
import { copyText } from '../utils/clipboard';
import ListenerConfigFields, { configToFormValues, formValuesToConfig, protocolSupportsUDP } from '../components/ListenerConfigFields';
import CapabilityFormFields, { capabilityFormToConfig } from '../components/CapabilityFormFields';
import { fetchCapabilities, protocolCapability, CapabilityManifest } from '../api/capabilities';

const PROTOCOLS = ['shadowsocks', 'snell', 'vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'shadowquic', 'anytls', 'mieru', 'sudoku', 'trusttunnel'];
const REALITY_PROTOCOLS = new Set(['vmess', 'vless', 'trojan']);
const parseConfig = (raw?: string) => { try { return raw ? JSON.parse(raw) : {}; } catch { return {}; } };
const firstNonEmpty = (...values: any[]) => values.find((v) => v !== undefined && v !== null && String(v).trim() !== '');

const Listeners: React.FC = () => {
  const { t } = useI18n();
  const [data, setData] = useState<Listener[]>([]);
  const [templates, setTemplates] = useState<ListenerTemplate[]>([]);
  const [loading, setLoading] = useState(false);
  const [keyword, setKeyword] = useState('');
  const [templateLoading, setTemplateLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [quickOpen, setQuickOpen] = useState(false);
  const [quickLoading, setQuickLoading] = useState(false);
  const [quickForm] = Form.useForm();
  const [quickHints, setQuickHints] = useState<Record<string, string> | null>(null);
  const [editing, setEditing] = useState<Listener | null>(null);
  const [form] = Form.useForm();
  const [templateForm] = Form.useForm();
  const [cloneForm] = Form.useForm();
  const [instantiateForm] = Form.useForm();
  const [uris, setUris] = useState<string[]>([]);
  const [uriModal, setUriModal] = useState(false);
  const [cloneModal, setCloneModal] = useState(false);
  const [cloneSource, setCloneSource] = useState<Listener | null>(null);
  const [templateModal, setTemplateModal] = useState(false);
  const [templateSource, setTemplateSource] = useState<Listener | null>(null);
  const [instantiateModal, setInstantiateModal] = useState(false);
  const [instantiateSource, setInstantiateSource] = useState<ListenerTemplate | null>(null);
  const [versionsModal, setVersionsModal] = useState(false);
  const [versions, setVersions] = useState<ListenerVersion[]>([]);
  const [versionListener, setVersionListener] = useState<Listener | null>(null);
  const [diffModal, setDiffModal] = useState(false);
  const [diffText, setDiffText] = useState('');
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const protocol = Form.useWatch('protocol', form);
  const [capabilities, setCapabilities] = useState<CapabilityManifest | null>(null);
  const [useCapabilityForm] = useState(true);

  const load = async (showError = true) => {
    setLoading(true);
    try { setData(await fetchListeners()); return true; }
    catch (e: any) { if (showError) message.error(e.message); return false; }
    finally { setLoading(false); }
  };
  const loadTemplates = async () => { setTemplateLoading(true); try { setTemplates(await listListenerTemplates()); } catch (e: any) { message.error(e.message); } finally { setTemplateLoading(false); } };
  useEffect(() => { load(); loadTemplates(); fetchCapabilities().then(setCapabilities).catch(() => setCapabilities(null)); }, []);

  const openCreate = () => { setEditing(null); form.resetFields(); form.setFieldsValue({ bind_address: '0.0.0.0', enabled: true, udp: false, protocol: 'vless', transport_layer: 'raw', security_layer: 'reality', flow: 'xtls-rprx-vision' }); setModalOpen(true); };
  const openEdit = (record: Listener) => { setEditing(record); form.resetFields(); form.setFieldsValue({ name: record.name, protocol: record.protocol, port: record.port, bind_address: record.bind_address || '0.0.0.0', enabled: record.enabled, udp: record.udp, public_host: (record as any).public_host || '', public_port: (record as any).public_port || '', access_sni: (record as any).access_sni || '', client_fingerprint: (record as any).client_fingerprint || 'chrome', access_alpn: (record as any).access_alpn || '', ...configToFormValues(record.config) }); setModalOpen(true); };
  const onSubmit = async (rawValues?: any) => {
    try {
      const values = { ...(form.getFieldsValue(true) || {}), ...(rawValues || {}) };
      const proto = String(values.protocol || '').trim();
      if (!proto) { message.error(t('listeners.selectProtocolFirst')); return; }
      if (!values.name || !String(values.port || '').trim()) { message.error(t('listeners.portHint')); return; }
      if (REALITY_PROTOCOLS.has(proto) && firstNonEmpty(values.security_layer) === 'reality') {
        const dest = firstNonEmpty(values.reality_dest, values['reality-config']?.dest, values['reality-config.dest']);
        const privateKey = firstNonEmpty(values.reality_private_key, values['reality-config']?.['private-key'], values['reality-config.private-key']);
        if (!dest || !privateKey) {
          form.setFields([
            { name: 'reality_dest', errors: dest ? [] : ['Dest is required'] },
            { name: 'reality_private_key', errors: privateKey ? [] : ['Private Key is required'] },
          ]);
          message.error('Reality Dest / Private Key cannot be empty');
          return;
        }
        values.reality_dest = dest;
        values.reality_private_key = privateKey;
      }
      const previous = editing ? parseConfig(editing.config) : null;
      const cap = capabilities ? protocolCapability(capabilities, proto) : undefined;
      const config = useCapabilityForm && cap ? { ...formValuesToConfig(proto, values, previous), ...capabilityFormToConfig(proto, values, cap) } : formValuesToConfig(proto, values, previous);
      const payload: Partial<Listener> = { name: String(values.name).trim(), protocol: proto, port: String(values.port).trim(), bind_address: values.bind_address || '0.0.0.0', enabled: values.enabled !== false, udp: protocolSupportsUDP(proto) ? !!values.udp : false, config: JSON.stringify(config), public_host: values.public_host || '', public_port: values.public_port || '', access_sni: values.access_sni || '', client_fingerprint: values.client_fingerprint || '', access_alpn: values.access_alpn || '' };
      if (editing) { await updateListener(normalizeId(editing), payload); message.success(t('listeners.updated')); } else { await createListener(payload); message.success(t('listeners.created')); }
      setModalOpen(false); setEditing(null); form.resetFields(); if (!(await load(false))) message.warning(t('common.error'));
    } catch (e: any) { message.error(e.message); }
  };
  const onDelete = async (id: number) => { try { await deleteListener(id); message.success(t('listeners.deleted')); if (!(await load(false))) message.warning(t('common.error')); } catch (e: any) { message.error(e.message); } };
  const onReload = async (id: number) => { try { await reloadListener(id); message.success(t('listeners.reloaded')); if (!(await load(false))) message.warning(t('common.error')); } catch (e: any) { message.error(e.message); } };
  const showURIs = async (id: number) => { try { const res = await exportNodeURI(id); setUris(res.uris); setUriModal(true); } catch (e: any) { message.error(e.message); } };
  const openClone = (record: Listener) => { setCloneSource(record); cloneForm.setFieldsValue({ name: `${record.name}-copy`, port: '' }); setCloneModal(true); };
  const doClone = async (values: { name: string; port: string }) => { if (!cloneSource) return; try { await cloneListener(normalizeId(cloneSource), { name: values.name, port: values.port }); message.success(t('listeners.cloned')); setCloneModal(false); if (!(await load(false))) message.warning(t('common.error')); } catch (e: any) { message.error(e.message); } };
  const openSaveTemplate = (record: Listener) => { setTemplateSource(record); templateForm.setFieldsValue({ name: `${record.name} template` }); setTemplateModal(true); };
  const saveTemplate = async (values: { name: string }) => { if (!templateSource) return; try { await createListenerTemplate({ name: values.name, protocol: templateSource.protocol, config: templateSource.config }); message.success(t('listeners.templateCreated')); setTemplateModal(false); await loadTemplates(); } catch (e: any) { message.error(e.message); } };
  const openInstantiate = (template: ListenerTemplate) => { setInstantiateSource(template); instantiateForm.setFieldsValue({ name: template.name.replace(/\s+template$/i, ''), port: '' }); setInstantiateModal(true); };
  const doInstantiate = async (values: { name: string; port: string }) => { if (!instantiateSource) return; try { await instantiateListenerTemplate(instantiateSource.id, values); message.success(t('listeners.instantiated')); setInstantiateModal(false); if (!(await load(false))) message.warning(t('common.error')); } catch (e: any) { message.error(e.message); } };
  const batchEnabled = async (enabled: boolean) => { const ids = selectedRowKeys.map(Number); if (!ids.length) return; try { await batchSetListenersEnabled(ids, enabled); message.success(t('listeners.batchDone')); setSelectedRowKeys([]); if (!(await load(false))) message.warning(t('common.error')); } catch (e: any) { message.error(e.message); } };
  const openVersions = async (record: Listener) => { try { setVersionListener(record); setVersions(await listListenerVersions(normalizeId(record))); setVersionsModal(true); } catch (e: any) { message.error(e.message); } };
  const showDiff = async (version: number) => { if (!versionListener) return; try { setDiffText(await diffListenerVersion(normalizeId(versionListener), version)); setDiffModal(true); } catch (e: any) { message.error(e.message); } };
  const doRollback = async (version: number) => { if (!versionListener) return; try { await rollbackListenerVersion(normalizeId(versionListener), version); message.success(t('listeners.rollbackDone')); setVersions(await listListenerVersions(normalizeId(versionListener))); if (!(await load(false))) message.warning(t('common.error')); } catch (e: any) { message.error(e.message); } };
  const deleteTemplate = async (id: number) => { try { await deleteListenerTemplate(id); message.success(t('listeners.templateDeleted')); await loadTemplates(); } catch (e: any) { message.error(e.message); } };

    const filteredListeners = useMemo(() => {
    const q = keyword.trim().toLowerCase();
    if (!q) return data;
    return data.filter((l) =>
      [l.name, l.protocol, l.port, l.bind_address, l.public_host].filter(Boolean).join(' ').toLowerCase().includes(q)
    );
  }, [data, keyword]);

const columns = [
    { title: t('listeners.name'), dataIndex: 'name', key: 'name', ellipsis: true, width: 150 },
    { title: t('listeners.protocol'), dataIndex: 'protocol', key: 'protocol', width: 110, render: (p: string) => <Tag>{p}</Tag> },
    { title: t('listeners.port'), dataIndex: 'port', key: 'port', width: 100 },
    { title: t('listeners.status'), dataIndex: 'enabled', key: 'enabled', width: 100, render: (v: boolean) => <Tag color={v ? 'success' : 'default'}>{v ? t('common.enabled') : t('common.disabled')}</Tag> },
    { title: t('common.actions'), key: 'actions', fixed: 'right' as const, width: 300, render: (_: any, record: Listener) => <Space size={4} wrap><Tooltip title={t('listeners.copyURI')}><Button size="small" icon={<QrcodeOutlined />} onClick={() => showURIs(normalizeId(record))} /></Tooltip><Tooltip title={t('listeners.clone')}><Button size="small" icon={<BranchesOutlined />} onClick={() => openClone(record)} /></Tooltip><Tooltip title={t('listeners.saveTemplate')}><Button size="small" icon={<SaveOutlined />} onClick={() => openSaveTemplate(record)} /></Tooltip><Tooltip title={t('listeners.versions')}><Button size="small" icon={<HistoryOutlined />} onClick={() => openVersions(record)} /></Tooltip><Tooltip title={t('common.refresh')}><Button size="small" icon={<ReloadOutlined />} onClick={() => onReload(normalizeId(record))} /></Tooltip><Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)} /><Popconfirm title={t('listeners.deleteConfirm')} onConfirm={() => onDelete(normalizeId(record))}><Button size="small" icon={<DeleteOutlined />} danger /></Popconfirm></Space> },
  ];
  const templateColumns = [
    { title: t('listeners.templateName'), dataIndex: 'name', key: 'name' },
    { title: t('listeners.protocol'), dataIndex: 'protocol', key: 'protocol', render: (p: string) => <Tag>{p}</Tag> },
    { title: t('listeners.createdAt'), dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
    { title: t('common.actions'), key: 'actions', width: 210, render: (_: any, record: ListenerTemplate) => <Space><Button size="small" type="primary" onClick={() => openInstantiate(record)}>{t('listeners.instantiate')}</Button><Popconfirm title={t('listeners.deleteTemplateConfirm')} onConfirm={() => deleteTemplate(record.id)}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm></Space> },
  ];
  return <div>
    <Tabs defaultActiveKey="listeners" items={[{ key: 'listeners', label: t('listeners.title'), children: <Card title={t('listeners.title')} extra={<Space>{selectedRowKeys.length > 0 && <><Button icon={<PoweroffOutlined />} onClick={() => batchEnabled(true)}>{t('listeners.enableSelected')}</Button><Button icon={<PoweroffOutlined />} onClick={() => batchEnabled(false)}>{t('listeners.disableSelected')}</Button></>}<Input.Search allowClear placeholder={t('common.search')} onSearch={setKeyword} onChange={(e) => { if (!e.target.value) setKeyword(''); }} style={{ width: 180 }} /><Button onClick={() => { load(); }} icon={<ReloadOutlined />}>{t('common.refresh')}</Button><Button onClick={() => { setQuickHints(null); quickForm.resetFields(); quickForm.setFieldsValue({ preset: 'vless-reality', bind_address: '0.0.0.0', enabled: true, sni: 'www.microsoft.com' }); setQuickOpen(true); }}>{t('listeners.quickSetup') || 'Quick setup'}</Button><Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>{t('listeners.create')}</Button></Space>}><Table rowSelection={{ selectedRowKeys, onChange: setSelectedRowKeys }} dataSource={filteredListeners} columns={columns} rowKey="id" loading={loading} scroll={{ x: 1050 }} size="middle" /></Card> }, { key: 'templates', label: t('listeners.templates'), children: <Card title={t('listeners.templates')} extra={<Button icon={<ReloadOutlined />} onClick={loadTemplates}>{t('common.refresh')}</Button>}><Table dataSource={templates} columns={templateColumns} rowKey="id" loading={templateLoading} pagination={{ pageSize: 10 }} /></Card> }]} />
    <Modal open={modalOpen} title={editing ? t('listeners.edit') : t('listeners.create')} onCancel={() => { setModalOpen(false); setEditing(null); form.resetFields(); }} onOk={() => form.submit()} width={typeof window !== 'undefined' && window.innerWidth < 768 ? '100%' : 720} destroyOnClose styles={{ body: { maxHeight: '70vh', overflowY: 'auto' } }}>
      <Form form={form} layout="vertical" onFinish={onSubmit} preserve={false}>
        <Form.Item name="name" label={t('listeners.name')} rules={[{ required: true }]}><Input placeholder="my-vless" /></Form.Item>
        <Form.Item name="protocol" label={t('listeners.protocol')} rules={[{ required: true }]}><Select options={PROTOCOLS.map(p => ({ value: p, label: p }))} onChange={(nextProto: string) => { const keep = form.getFieldsValue(['name', 'port', 'bind_address', 'enabled', 'udp']); form.resetFields(); const layerDefaults: Record<string, string> = { transport_layer: 'raw', security_layer: 'none' }; if (nextProto === 'vless') layerDefaults.security_layer = 'reality'; form.setFieldsValue({ ...keep, protocol: nextProto, ...layerDefaults }); }} /></Form.Item>
        <Form.Item name="port" label={t('listeners.port')} tooltip={t('listeners.portHint')} rules={[{ required: true, message: t('listeners.portHint') }, { validator: async (_, v) => { const s = String(v || '').trim(); if (!s) return Promise.reject(new Error(t('listeners.portHint'))); if (!/^\d{1,5}([,-]\d{1,5})*$/.test(s.replace(/\s/g, ''))) return Promise.reject(new Error(t('listeners.portHint'))); return Promise.resolve(); } }]}><Input placeholder="443" /></Form.Item>
        <Form.Item name="bind_address" label={t('listeners.bindAddress')} initialValue="0.0.0.0" tooltip="IPv4: 0.0.0.0 · IPv6 dual-stack: :: · specific: 2001:db8::1"><Input placeholder="0.0.0.0 or ::" /></Form.Item>
        <Form.Item name="enabled" label={t('listeners.status')} valuePropName="checked" initialValue={true}><Switch /></Form.Item>
        {protocolSupportsUDP(protocol) && <Form.Item name="udp" label={t('listeners.udp')} valuePropName="checked" initialValue={false}><Switch /></Form.Item>}
        <Divider titlePlacement="start" plain>{t('settings.accessProfile')}</Divider>
        <Form.Item name="public_host" label={t('settings.publicHost')} tooltip={t('settings.accessProfileHint') || 'Domain or IP (IPv6 without brackets)'}><Input placeholder="example.com or 2001:db8::1" /></Form.Item>
        <Form.Item name="public_port" label={t('settings.publicPort')}><Input placeholder="443" /></Form.Item>
        {protocol && ['vmess','vless','trojan','hysteria2','tuic','anytls','trusttunnel'].includes(protocol) && (
          <>
            <Form.Item name="access_sni" label={t('listeners.sni')}><Input /></Form.Item>
            <Form.Item name="client_fingerprint" label={t('settings.clientFingerprint')} initialValue="chrome"><Select options={['chrome','firefox','safari','ios','android','edge','random'].map(v => ({ value: v, label: v }))} /></Form.Item>
            <Form.Item name="access_alpn" label={t('listeners.alpn')}><Input placeholder="h2,http/1.1" /></Form.Item>
          </>
        )}
        {useCapabilityForm && capabilities && protocolCapability(capabilities, protocol || '') ? (
          <CapabilityFormFields key={protocol || 'none'} protocol={protocol} capability={protocolCapability(capabilities, protocol || '')} />
        ) : (
          <ListenerConfigFields key={protocol || 'none'} protocol={protocol} />
        )}
      </Form>
    </Modal>
    <Modal open={cloneModal} title={t('listeners.clone')} onCancel={() => setCloneModal(false)} onOk={() => cloneForm.submit()}><Form form={cloneForm} layout="vertical" onFinish={doClone}><Form.Item name="name" label={t('listeners.name')} rules={[{ required: true }]}><Input /></Form.Item><Form.Item name="port" label={t('listeners.newPort')} rules={[{ required: true }]}><Input placeholder="443" /></Form.Item></Form></Modal>
    <Modal open={templateModal} title={t('listeners.saveTemplate')} onCancel={() => setTemplateModal(false)} onOk={() => templateForm.submit()}><Form form={templateForm} layout="vertical" onFinish={saveTemplate}><Form.Item name="name" label={t('listeners.templateName')} rules={[{ required: true }]}><Input /></Form.Item><Descriptions column={1} size="small"><Descriptions.Item label={t('listeners.protocol')}>{templateSource?.protocol}</Descriptions.Item></Descriptions></Form></Modal>
    <Modal open={instantiateModal} title={t('listeners.instantiate')} onCancel={() => setInstantiateModal(false)} onOk={() => instantiateForm.submit()}><Form form={instantiateForm} layout="vertical" onFinish={doInstantiate}><Form.Item name="name" label={t('listeners.name')} rules={[{ required: true }]}><Input /></Form.Item><Form.Item name="port" label={t('listeners.newPort')} rules={[{ required: true }]}><Input placeholder="443" /></Form.Item></Form></Modal>
    <Modal open={versionsModal} title={`${t('listeners.versions')} — ${versionListener?.name || ''}`} onCancel={() => setVersionsModal(false)} footer={null} width={800}><Table dataSource={versions} rowKey="id" pagination={false} columns={[{ title: t('listeners.version'), dataIndex: 'version', width: 100 }, { title: t('listeners.reason'), dataIndex: 'reason', render: (v: string) => v || '-' }, { title: t('listeners.createdAt'), dataIndex: 'created_at', render: (v: string) => new Date(v).toLocaleString() }, { title: t('common.actions'), render: (_: any, v: ListenerVersion) => <Space><Button size="small" icon={<DiffOutlined />} onClick={() => showDiff(v.version)}>{t('listeners.diff')}</Button><Popconfirm title={t('listeners.rollbackConfirm')} onConfirm={() => doRollback(v.version)}><Button size="small" type="primary">{t('listeners.rollback')}</Button></Popconfirm></Space> }]} /></Modal>
    <Modal open={diffModal} title={t('listeners.diff')} onCancel={() => setDiffModal(false)} footer={null} width={900}><pre style={{ maxHeight: '65vh', overflow: 'auto', whiteSpace: 'pre-wrap', wordBreak: 'break-word', margin: 0 }}>{diffText || t('common.empty')}</pre></Modal>
    <Modal open={uriModal} title={t('listeners.urisTitle')} onCancel={() => setUriModal(false)} footer={null} width={560}>
      <Space direction="vertical" style={{ width: '100%' }}>
        {uris.map((uri, i) => (
          <Card key={i} size="small">
            <Space direction="vertical" style={{ width: '100%' }}>
              <Space style={{ width: '100%' }}>
                <Input value={uri} readOnly />
                <Button icon={<CopyOutlined />} onClick={async () => { const ok = await copyText(uri); if (ok) message.success(t('common.copied') || t('common.copy')); else message.error(t('common.copyFailed') || 'Copy failed'); }} />
              </Space>
              <div style={{ textAlign: 'center' }}>
                <img alt="qr" width={160} height={160} src={`https://api.qrserver.com/v1/create-qr-code/?size=160x160&data=${encodeURIComponent(uri)}`} />
              </div>
            </Space>
          </Card>
        ))}
      </Space>
    </Modal>
<Modal
        open={quickOpen}
        title={t('listeners.quickSetup') || 'Quick setup'}
        onCancel={() => { setQuickOpen(false); setQuickHints(null); }}
        onOk={() => quickForm.submit()}
        confirmLoading={quickLoading}
        width={560}
        destroyOnClose
      >
        <p style={{ opacity: 0.7, marginBottom: 12 }}>
          {t('listeners.quickSetupHint') || 'One-click inbound (VLESS Reality by default). Override fields below to fine-tune.'}
        </p>
        <Form
          form={quickForm}
          layout="vertical"
          onFinish={async (values) => {
            setQuickLoading(true);
            try {
              const res = await quickSetupListener({
                preset: values.preset,
                name: values.name || undefined,
                port: values.port ? Number(values.port) : 0,
                bind_address: values.bind_address || '0.0.0.0',
                public_host: values.public_host || '',
                public_port: values.public_port || '',
                sni: values.sni || '',
                uuid: values.uuid || '',
                password: values.password || '',
                private_key: values.private_key || '',
                public_key: values.public_key || '',
                short_id: values.short_id || '',
                flow: values.flow || '',
                method: values.method || '',
                enabled: values.enabled !== false,
              });
              setQuickHints(res.hints || {});
              message.success(t('listeners.created'));
              await load(false);
            } catch (e: any) {
              message.error(e.response?.data?.error || e.message || t('common.error'));
            } finally {
              setQuickLoading(false);
            }
          }}
        >
          <Form.Item name="preset" label={t('listeners.preset') || 'Preset'} rules={[{ required: true }]}>
            <Select options={[
              { value: 'vless-reality', label: 'VLESS + REALITY + Vision' },
              { value: 'trojan-reality', label: 'Trojan + REALITY' },
              { value: 'shadowsocks', label: 'Shadowsocks 2022' },
              { value: 'hysteria2', label: 'Hysteria2 (skeleton)' },
            ]} />
          </Form.Item>
          <Form.Item name="name" label={t('common.name')}><Input placeholder="quick-vless-reality" /></Form.Item>
          <Form.Item name="port" label={t('common.port')} tooltip={t('listeners.portAuto') || 'Leave empty for auto'}><Input placeholder="auto" /></Form.Item>
          <Form.Item name="bind_address" label={t('listeners.bind')}><Input /></Form.Item>
          <Form.Item name="public_host" label={t('listeners.publicHost') || 'Public host'}><Input placeholder="optional" /></Form.Item>
          <Form.Item name="sni" label="SNI / Reality dest host"><Input placeholder="www.microsoft.com" /></Form.Item>
          <Form.Item name="enabled" label={t('common.enabled')} valuePropName="checked"><Switch /></Form.Item>
          <Divider>{t('common.advanced') || 'Advanced (optional)'}</Divider>
          <Form.Item name="uuid" label="UUID"><Input placeholder="auto" /></Form.Item>
          <Form.Item name="password" label={t('common.password')}><Input placeholder="auto" /></Form.Item>
          <Form.Item name="private_key" label="Reality private key"><Input.Password placeholder="auto" /></Form.Item>
          <Form.Item name="public_key" label="Reality public key"><Input placeholder="auto" /></Form.Item>
          <Form.Item name="short_id" label="Short ID"><Input placeholder="auto" /></Form.Item>
          <Form.Item name="flow" label="Flow"><Input placeholder="xtls-rprx-vision" /></Form.Item>
          <Form.Item name="method" label="SS method"><Input placeholder="2022-blake3-aes-128-gcm" /></Form.Item>
        </Form>
        {quickHints && (
          <Card size="small" title={t('listeners.quickHints') || 'Generated'} style={{ marginTop: 12 }}>
            {Object.entries(quickHints).map(([k, v]) => (
              <div key={k} style={{ fontSize: 12, marginBottom: 4 }}><b>{k}</b>: <code>{String(v)}</code></div>
            ))}
          </Card>
        )}
      </Modal>
  </div>;
};

export default Listeners;
