import React, { useEffect, useState } from 'react';
import { Table, Button, Space, Tag, Modal, Form, Input, Select, Switch, message, Popconfirm, Tooltip, Card } from 'antd';
import { PlusOutlined, ReloadOutlined, QrcodeOutlined, DeleteOutlined, EditOutlined, CopyOutlined } from '@ant-design/icons';
import { fetchListeners, createListener, updateListener, deleteListener, reloadListener, exportNodeURI, Listener } from '../api/nodes';
import { useI18n } from '../i18n';
import ListenerConfigFields, {
  configToFormValues,
  formValuesToConfig,
  protocolSupportsUDP,
} from '../components/ListenerConfigFields';

const PROTOCOLS = [
  'shadowsocks', 'snell', 'vmess', 'vless', 'trojan',
  'hysteria2', 'tuic', 'shadowquic', 'anytls', 'mieru', 'sudoku', 'trusttunnel',
];

const Listeners: React.FC = () => {
  const { t } = useI18n();
  const [data, setData] = useState<Listener[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Listener | null>(null);
  const [form] = Form.useForm();
  const [uris, setUris] = useState<string[]>([]);
  const [uriModal, setUriModal] = useState(false);
  const protocol = Form.useWatch('protocol', form);

  const load = async () => {
    setLoading(true);
    try {
      const res = await fetchListeners();
      setData(res);
    } catch (e: any) {
      message.error(e.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({
      bind_address: '0.0.0.0',
      enabled: true,
      udp: false,
    });
    setModalOpen(true);
  };

  const openEdit = (record: Listener) => {
    setEditing(record);
    form.resetFields();
    form.setFieldsValue({
      name: record.name,
      protocol: record.protocol,
      port: record.port,
      bind_address: record.bind_address || '0.0.0.0',
      enabled: record.enabled,
      udp: record.udp,
      ...configToFormValues(record.config),
    });
    setModalOpen(true);
  };

  const onSubmit = async (values: any) => {
    try {
      const proto = values.protocol as string;
      let previous: Record<string, any> | null = null;
      if (editing?.config) {
        try {
          previous = JSON.parse(editing.config);
        } catch {
          previous = null;
        }
      }
      const config = formValuesToConfig(proto, values, previous);

      const payload: Partial<Listener> = {
        name: values.name,
        protocol: proto,
        port: String(values.port),
        bind_address: values.bind_address || '0.0.0.0',
        enabled: !!values.enabled,
        udp: protocolSupportsUDP(proto) ? !!values.udp : false,
        config: JSON.stringify(config),
      };

      if (editing) {
        await updateListener(editing.id, payload);
        message.success(t('listeners.updated'));
      } else {
        await createListener(payload);
        message.success(t('listeners.created'));
      }
      setModalOpen(false);
      setEditing(null);
      form.resetFields();
      load();
    } catch (e: any) {
      message.error(e.message);
    }
  };

  const onDelete = async (id: number) => {
    try {
      await deleteListener(id);
      message.success(t('listeners.deleted'));
      load();
    } catch (e: any) {
      message.error(e.message);
    }
  };

  const onReload = async (id: number) => {
    try {
      await reloadListener(id);
      message.success(t('listeners.reloaded'));
    } catch (e: any) {
      message.error(e.message);
    }
  };

  const showURIs = async (id: number) => {
    try {
      const res = await exportNodeURI(id);
      setUris(res.uris);
      setUriModal(true);
    } catch (e: any) {
      message.error(e.message);
    }
  };

  const columns = [
    { title: t('listeners.name'), dataIndex: 'name', key: 'name', ellipsis: true, width: 140 },
    {
      title: t('listeners.protocol'),
      dataIndex: 'protocol',
      key: 'protocol',
      width: 110,
      render: (p: string) => <Tag>{p}</Tag>,
    },
    { title: t('listeners.port'), dataIndex: 'port', key: 'port', width: 90 },
    {
      title: t('listeners.status'),
      dataIndex: 'enabled',
      key: 'enabled',
      width: 100,
      render: (v: boolean) => (
        <Tag color={v ? 'success' : 'default'}>{v ? t('common.enabled') : t('common.disabled')}</Tag>
      ),
    },
    {
      title: t('common.actions'),
      key: 'actions',
      fixed: 'right' as const,
      width: 180,
      render: (_: any, record: Listener) => (
        <Space size={4} wrap>
          <Tooltip title={t('listeners.copyURI')}>
            <Button size="small" icon={<QrcodeOutlined />} onClick={() => showURIs(record.id)} />
          </Tooltip>
          <Tooltip title={t('common.refresh')}>
            <Button size="small" icon={<ReloadOutlined />} onClick={() => onReload(record.id)} />
          </Tooltip>
          <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)} />
          <Popconfirm title={t('listeners.deleteConfirm')} onConfirm={() => onDelete(record.id)}>
            <Button size="small" icon={<DeleteOutlined />} danger />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <h2>{t('listeners.title')}</h2>
      <Card
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            {t('listeners.create')}
          </Button>
        }
      >
        <Table dataSource={data} columns={columns} rowKey="id" loading={loading} scroll={{ x: 720 }} size="middle" />
      </Card>

      <Modal
        open={modalOpen}
        title={editing ? t('listeners.edit') : t('listeners.create')}
        onCancel={() => {
          setModalOpen(false);
          setEditing(null);
          form.resetFields();
        }}
        onOk={() => form.submit()}
        width={typeof window !== 'undefined' && window.innerWidth < 768 ? '100%' : 720}
        destroyOnClose
        styles={{ body: { maxHeight: '70vh', overflowY: 'auto' } }}
      >
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item name="name" label={t('listeners.name')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="protocol" label={t('listeners.protocol')} rules={[{ required: true }]}>
            <Select
              options={PROTOCOLS.map((p) => ({ value: p, label: p }))}
              onChange={() => {
                // Clear protocol-specific fields when protocol changes
                const keep = form.getFieldsValue(['name', 'protocol', 'port', 'bind_address', 'enabled', 'udp']);
                form.resetFields();
                form.setFieldsValue(keep);
              }}
            />
          </Form.Item>
          <Form.Item
            name="port"
            label={t('listeners.port')}
            rules={[{ required: true }]}
            tooltip={t('listeners.portHint')}
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

      <Modal open={uriModal} title={t('listeners.urisTitle')} onCancel={() => setUriModal(false)} footer={null}>
        <Space direction="vertical" style={{ width: '100%' }}>
          {uris.map((uri, i) => (
            <Card key={i} size="small">
              <Space>
                <Input value={uri} readOnly style={{ width: 400 }} />
                <Button
                  icon={<CopyOutlined />}
                  onClick={() => {
                    navigator.clipboard.writeText(uri);
                    message.success(t('common.copy'));
                  }}
                />
              </Space>
            </Card>
          ))}
        </Space>
      </Modal>
    </div>
  );
};

export default Listeners;
