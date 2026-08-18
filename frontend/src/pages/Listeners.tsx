import React, { useEffect, useState } from 'react';
import { Table, Button, Space, Tag, Modal, Form, Input, Select, Switch, message, Popconfirm, Tooltip, Card, Tabs, Descriptions, Divider } from 'antd';
import { PlusOutlined, ReloadOutlined, QrcodeOutlined, DeleteOutlined, EditOutlined, CopyOutlined, BranchesOutlined, HistoryOutlined, SaveOutlined, PoweroffOutlined, DiffOutlined } from '@ant-design/icons';
import {
  fetchListeners, createListener, updateListener, deleteListener, reloadListener, exportNodeURI, normalizeId, Listener,
} from '../api/nodes';
import {
  listListenerTemplates, createListenerTemplate, deleteListenerTemplate, instantiateListenerTemplate,
  cloneListener, batchSetListenersEnabled, listListenerVersions, diffListenerVersion, rollbackListenerVersion,
  ListenerTemplate, ListenerVersion,
} from '../api/listeners';
import { useI18n } from '../i18n';
import ListenerConfigFields, { configToFormValues, formValuesToConfig, protocolSupportsUDP } from '../components/ListenerConfigFields';

const PROTOCOLS = ['shadowsocks', 'snell', 'vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'shadowquic', 'anytls', 'mieru', 'sudoku', 'trusttunnel'];

const parseConfig = (raw?: string) => {
  try { return raw ? JSON.parse(raw) : {}; } catch { return {}; }
};

const Listeners: React.FC = () => {
  const { t } = useI18n();
  const [data, setData] = useState<Listener[]>([]);
  const [templates, setTemplates] = useState<ListenerTemplate[]>([]);
  const [loading, setLoading] = useState(false);
  const [templateLoading, setTemplateLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
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

  const load = async () => {
    setLoading(true);
    try { setData(await fetchListeners()); } catch (e: any) { message.error(e.message); } finally { setLoading(false); }
  };
  const loadTemplates = async () => {
    setTemplateLoading(true);
    try { setTemplates(await listListenerTemplates()); } catch (e: any) { message.error(e.message); } finally { setTemplateLoading(false); }
  };
  useEffect(() => { load(); loadTemplates(); }, []);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({ bind_address: '0.0.0.0', enabled: true, udp: false, protocol: 'vless' });
    setModalOpen(true);
  };
  const openEdit = (record: Listener) => {
    setEditing(record);
    form.resetFields();
    form.setFieldsValue({ name: record.name, protocol: record.protocol, port: record.port, bind_address: record.bind_address || '0.0.0.0', enabled: record.enabled, udp: record.udp, ...configToFormValues(record.config) });
    setModalOpen(true);
  };
  const onSubmit = async (rawValues?: any) => {
    try {
      const values = { ...(form.getFieldsValue(true) || {}), ...(rawValues || {}) };
      const proto = String(values.protocol || '').trim();
      if (!proto) {
        message.error(t('listeners.selectProtocolFirst'));
        return;
      }
      if (!values.name || !String(values.port || '').trim()) {
        message.error(t('listeners.portHint') || 'Name and port are required');
        return;
      }
      const previous = editing ? parseConfig(editing.config) : null;
      const config = formValuesToConfig(proto, values, previous);
      const payload: Partial<Listener> = {
        name: String(values.name).trim(),
        protocol: proto,
        port: String(values.port).trim(),
        bind_address: values.bind_address || '0.0.0.0',
        enabled: values.enabled !== false,
        udp: protocolSupportsUDP(proto) ? !!values.udp : false,
        config: JSON.stringify(config),
      };
      if (editing) {
        await updateListener(normalizeId(editing), payload);
        message.success(t('listeners.updated'));
      } else {
        await createListener(payload);
        message.success(t('listeners.created'));
      }
      setModalOpen(false);
      setEditing(null);
      form.resetFields();
      await load();
    } catch (e: any) {
      message.error(e.message);
    }
  };
  const onDelete = async (id: number) => { try { await deleteListener(id); message.success(t('listeners.deleted')); await load(); } catch (e: any) { message.error(e.message); } };
  const onReload = async (id: number) => { try { await reloadListener(id); message.success(t('listeners.reloaded')); await load(); } catch (e: any) { message.error(e.message); } };
  const showURIs = async (id: number) => { try { const res = await exportNodeURI(id); setUris(res.uris); setUriModal(true); } catch (e: any) { message.error(e.message); } };

  const openClone = (record: Listener) => { setCloneSource(record); cloneForm.setFieldsValue({ name: `${record.name}-copy`, port: '' }); setCloneModal(true); };
  const doClone = async (values: { name: string; port: string }) => {
    if (!cloneSource) return;
    try { await cloneListener(normalizeId(cloneSource), { name: values.name, port: values.port }); message.success(t('listeners.cloned')); setCloneModal(false); await load(); } catch (e: any) { message.error(e.message); }
  };
  const openSaveTemplate = (record: Listener) => { setTemplateSource(record); templateForm.setFieldsValue({ name: `${record.name} template` }); setTemplateModal(true); };
  const saveTemplate = async (values: { name: string }) => {
    if (!templateSource) return;
    try { await createListenerTemplate({ name: values.name, protocol: templateSource.protocol, config: templateSource.config }); message.success(t('listeners.templateCreated')); setTemplateModal(false); await loadTemplates(); } catch (e: any) { message.error(e.message); }
  };
  const openInstantiate = (template: ListenerTemplate) => { setInstantiateSource(template); instantiateForm.setFieldsValue({ name: template.name.replace(/\s+template$/i, ''), port: '' }); setInstantiateModal(true); };
  const doInstantiate = async (values: { name: string; port: string }) => {
    if (!instantiateSource) return;
    try { await instantiateListenerTemplate(instantiateSource.id, values); message.success(t('listeners.instantiated')); setInstantiateModal(false); await load(); } catch (e: any) { message.error(e.message); }
  };
  const batchEnabled = async (enabled: boolean) => {
    const ids = selectedRowKeys.map(Number); if (!ids.length) return;
    try { await batchSetListenersEnabled(ids, enabled); message.success(t('listeners.batchDone')); setSelectedRowKeys([]); await load(); } catch (e: any) { message.error(e.message); }
  };
  const openVersions = async (record: Listener) => {
    try { setVersionListener(record); setVersions(await listListenerVersions(normalizeId(record))); setVersionsModal(true); } catch (e: any) { message.error(e.message); }
  };
  const showDiff = async (version: number) => {
    if (!versionListener) return;
    try { setDiffText(await diffListenerVersion(normalizeId(versionListener), version)); setDiffModal(true); } catch (e: any) { message.error(e.message); }
  };
  const doRollback = async (version: number) => {
    if (!versionListener) return;
    try { await rollbackListenerVersion(normalizeId(versionListener), version); message.success(t('listeners.rollbackDone')); setVersions(await listListenerVersions(normalizeId(versionListener))); await load(); } catch (e: any) { message.error(e.message); }
  };
  const deleteTemplate = async (id: number) => { try { await deleteListenerTemplate(id); message.success(t('listeners.templateDeleted')); await loadTemplates(); } catch (e: any) { message.error(e.message); } };

  const columns = [
    { title: t('listeners.name'), dataIndex: 'name', key: 'name', ellipsis: true, width: 150 },
    { title: t('listeners.protocol'), dataIndex: 'protocol', key: 'protocol', width: 110, render: (p: string) => <Tag>{p}</Tag> },
    { title: t('listeners.port'), dataIndex: 'port', key: 'port', width: 100 },
    { title: t('listeners.status'), dataIndex: 'enabled', key: 'enabled', width: 100, render: (v: boolean) => <Tag color={v ? 'success' : 'default'}>{v ? t('common.enabled') : t('common.disabled')}</Tag> },
    { title: t('common.actions'), key: 'actions', fixed: 'right' as const, width: 300, render: (_: any, record: Listener) => (
      <Space size={4} wrap>
        <Tooltip title={t('listeners.copyURI')}><Button size="small" icon={<QrcodeOutlined />} onClick={() => showURIs(normalizeId(record))} /></Tooltip>
        <Tooltip title={t('listeners.clone')}><Button size="small" icon={<BranchesOutlined />} onClick={() => openClone(record)} /></Tooltip>
        <Tooltip title={t('listeners.saveTemplate')}><Button size="small" icon={<SaveOutlined />} onClick={() => openSaveTemplate(record)} /></Tooltip>
        <Tooltip title={t('listeners.versions')}><Button size="small" icon={<HistoryOutlined />} onClick={() => openVersions(record)} /></Tooltip>
        <Tooltip title={t('common.refresh')}><Button size="small" icon={<ReloadOutlined />} onClick={() => onReload(normalizeId(record))} /></Tooltip>
        <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)} />
        <Popconfirm title={t('listeners.deleteConfirm')} onConfirm={() => onDelete(normalizeId(record))}><Button size="small" icon={<DeleteOutlined />} danger /></Popconfirm>
      </Space>
    ) },
  ];

  const templateColumns = [
    { title: t('listeners.templateName'), dataIndex: 'name', key: 'name' },
    { title: t('listeners.protocol'), dataIndex: 'protocol', key: 'protocol', render: (p: string) => <Tag>{p}</Tag> },
    { title: t('listeners.createdAt'), dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
    { title: t('common.actions'), key: 'actions', width: 210, render: (_: any, record: ListenerTemplate) => (
      <Space><Button size="small" type="primary" onClick={() => openInstantiate(record)}>{t('listeners.instantiate')}</Button><Popconfirm title={t('listeners.deleteTemplateConfirm')} onConfirm={() => deleteTemplate(record.id)}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm></Space>
    ) },
  ];

  return <div>
    <Tabs defaultActiveKey="listeners" items={[
      {
        key: 'listeners', label: t('listeners.title'), children: <>
          <Card title={t('listeners.title')} extra={<Space>
            {selectedRowKeys.length > 0 && <><Button icon={<PoweroffOutlined />} onClick={() => batchEnabled(true)}>{t('listeners.enableSelected')}</Button><Button icon={<PoweroffOutlined />} onClick={() => batchEnabled(false)}>{t('listeners.disableSelected')}</Button></>}
            <Button onClick={() => { load(); }} icon={<ReloadOutlined />}>{t('common.refresh')}</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>{t('listeners.create')}</Button>
          </Space>}>
            <Table rowSelection={{ selectedRowKeys, onChange: setSelectedRowKeys }} dataSource={data} columns={columns} rowKey="id" loading={loading} scroll={{ x: 1050 }} size="middle" />
          </Card>
        </>,
      },
      {
        key: 'templates', label: t('listeners.templates'), children: <Card title={t('listeners.templates')} extra={<Button icon={<ReloadOutlined />} onClick={loadTemplates}>{t('common.refresh')}</Button>}>
          <Table dataSource={templates} columns={templateColumns} rowKey="id" loading={templateLoading} pagination={{ pageSize: 10 }} />
        </Card>,
      },
    ]} />

    <Modal
      open={modalOpen}
      title={editing ? t('listeners.edit') : t('listeners.create')}
      onCancel={() => { setModalOpen(false); setEditing(null); form.resetFields(); }}
      onOk={() => form.submit()}
      width={typeof window !== 'undefined' && window.innerWidth < 768 ? '100%' : 720}
      destroyOnClose
      styles={{ body: { maxHeight: '70vh', overflowY: 'auto' } }}
    >
      <Form form={form} layout="vertical" onFinish={onSubmit} preserve>
        <Form.Item name="name" label={t('listeners.name')} rules={[{ required: true }]}>
          <Input placeholder="my-vless" />
        </Form.Item>
        <Form.Item name="protocol" label={t('listeners.protocol')} rules={[{ required: true }]}>
          <Select
            options={PROTOCOLS.map(p => ({ value: p, label: p }))}
            onChange={(nextProto: string) => {
              const keep = form.getFieldsValue(['name', 'port', 'bind_address', 'enabled', 'udp']);
              form.resetFields();
              form.setFieldsValue({ ...keep, protocol: nextProto });
            }}
          />
        </Form.Item>
        <Form.Item
          name="port"
          label={t('listeners.port')}
          tooltip={t('listeners.portHint')}
          rules={[
            { required: true, message: t('listeners.portHint') },
            {
              validator: async (_, v) => {
                const s = String(v || '').trim();
                if (!s) return Promise.reject(new Error(t('listeners.portHint')));
                if (!/^\d{1,5}([,-]\d{1,5})*$/.test(s.replace(/\s/g, ''))) {
                  return Promise.reject(new Error(t('listeners.portHint')));
                }
                return Promise.resolve();
              },
            },
          ]}
        >
          <Input placeholder="443" />
        </Form.Item>
        <Form.Item name="bind_address" label={t('listeners.bindAddress')} initialValue="0.0.0.0">
          <Input />
        </Form.Item>
        <Form.Item name="enabled" label={t('listeners.status')} valuePropName="checked" initialValue={true}>
          <Switch />
        </Form.Item>
        {protocolSupportsUDP(protocol) && (
          <Form.Item name="udp" label={t('listeners.udp')} valuePropName="checked" initialValue={false}>
            <Switch />
          </Form.Item>
        )}
        <ListenerConfigFields protocol={protocol} />
      </Form>
    </Modal>

    <Modal open={cloneModal} title={t('listeners.clone')} onCancel={() => setCloneModal(false)} onOk={() => cloneForm.submit()}>
      <Form form={cloneForm} layout="vertical" onFinish={doClone}><Form.Item name="name" label={t('listeners.name')} rules={[{ required: true }]}><Input /></Form.Item><Form.Item name="port" label={t('listeners.newPort')} rules={[{ required: true }]}><Input placeholder="443" /></Form.Item></Form>
    </Modal>

    <Modal open={templateModal} title={t('listeners.saveTemplate')} onCancel={() => setTemplateModal(false)} onOk={() => templateForm.submit()}>
      <Form form={templateForm} layout="vertical" onFinish={saveTemplate}><Form.Item name="name" label={t('listeners.templateName')} rules={[{ required: true }]}><Input /></Form.Item><Descriptions column={1} size="small"><Descriptions.Item label={t('listeners.protocol')}>{templateSource?.protocol}</Descriptions.Item></Descriptions>
      </Form>
    </Modal>

    <Modal open={instantiateModal} title={t('listeners.instantiate')} onCancel={() => setInstantiateModal(false)} onOk={() => instantiateForm.submit()}>
      <Form form={instantiateForm} layout="vertical" onFinish={doInstantiate}><Form.Item name="name" label={t('listeners.name')} rules={[{ required: true }]}><Input /></Form.Item><Form.Item name="port" label={t('listeners.newPort')} rules={[{ required: true }]}><Input placeholder="443" /></Form.Item></Form>
    </Modal>

    <Modal open={versionsModal} title={`${t('listeners.versions')} — ${versionListener?.name || ''}`} onCancel={() => setVersionsModal(false)} footer={null} width={800}>
      <Table dataSource={versions} rowKey="id" pagination={false} columns={[
        { title: t('listeners.version'), dataIndex: 'version', width: 100 },
        { title: t('listeners.reason'), dataIndex: 'reason', render: (v: string) => v || '-' },
        { title: t('listeners.createdAt'), dataIndex: 'created_at', render: (v: string) => new Date(v).toLocaleString() },
        { title: t('common.actions'), render: (_: any, v: ListenerVersion) => <Space><Button size="small" icon={<DiffOutlined />} onClick={() => showDiff(v.version)}>{t('listeners.diff')}</Button><Popconfirm title={t('listeners.rollbackConfirm')} onConfirm={() => doRollback(v.version)}><Button size="small" type="primary">{t('listeners.rollback')}</Button></Popconfirm></Space> },
      ]} />
    </Modal>

    <Modal open={diffModal} title={t('listeners.diff')} onCancel={() => setDiffModal(false)} footer={null} width={900}>
      <pre style={{ maxHeight: '65vh', overflow: 'auto', whiteSpace: 'pre-wrap', wordBreak: 'break-word', margin: 0 }}>{diffText || t('common.empty')}</pre>
    </Modal>

    <Modal open={uriModal} title={t('listeners.urisTitle')} onCancel={() => setUriModal(false)} footer={null}>
      <Space direction="vertical" style={{ width: '100%' }}>{uris.map((uri, i) => <Card key={i} size="small"><Space style={{ width: '100%' }}><Input value={uri} readOnly /><Button icon={<CopyOutlined />} onClick={() => { navigator.clipboard.writeText(uri); message.success(t('common.copy')); }} /></Space></Card>)}</Space>
    </Modal>
  </div>;
};

export default Listeners;
