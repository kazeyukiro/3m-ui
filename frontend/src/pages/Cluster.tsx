import React, { useEffect, useState } from 'react';
import { Card, Table, Button, Space, Modal, Form, Input, Switch, message, Popconfirm, Tag } from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined, HeartOutlined } from '@ant-design/icons';
import {
  fetchCluster, createClusterNode, updateClusterNode, deleteClusterNode, healthClusterNode, RemoteServer,
} from '../api/cluster';
import { useI18n } from '../i18n';

const ClusterPage: React.FC = () => {
  const { t } = useI18n();
  const [data, setData] = useState<RemoteServer[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<RemoteServer | null>(null);
  const [form] = Form.useForm();

  const load = async () => {
    setLoading(true);
    try { setData(await fetchCluster()); }
    catch (e: any) { message.error(e.message || t('common.error')); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, []);

  const onSubmit = async (values: any) => {
    try {
      const payload = {
        name: values.name, base_url: values.base_url, api_token: values.api_token,
        enabled: !!values.enabled, remark: values.remark || '', keep_token: !values.api_token,
      };
      if (editing) { await updateClusterNode(editing.id, payload); message.success(t('cluster.updated')); }
      else { await createClusterNode(payload); message.success(t('cluster.created')); }
      setOpen(false); setEditing(null); form.resetFields(); load();
    } catch (e: any) { message.error(e.message || t('common.error')); }
  };

  const columns = [
    { title: t('cluster.name'), dataIndex: 'name', key: 'name' },
    { title: t('cluster.baseUrl'), dataIndex: 'base_url', key: 'url', ellipsis: true },
    {
      title: t('common.status'), key: 'st',
      render: (_: any, r: RemoteServer) => {
        const s = r.last_status || '-';
        const color = s === 'up' ? 'success' : s === 'down' || s === 'error' ? 'error' : 'default';
        return <Tag color={color}>{s}</Tag>;
      },
    },
    { title: t('common.enabled'), dataIndex: 'enabled', key: 'en', render: (v: boolean) => (v ? t('common.enabled') : t('common.disabled')) },
    {
      title: t('common.actions'), key: 'act',
      render: (_: any, r: RemoteServer) => (
        <Space wrap>
          <Button size="small" icon={<HeartOutlined />} onClick={async () => {
            try { await healthClusterNode(r.id); message.success(t('cluster.healthDone')); load(); }
            catch (e: any) { message.error(e.message || t('common.error')); }
          }}>{t('cluster.health')}</Button>
          <Button size="small" icon={<EditOutlined />} onClick={() => { setEditing(r); form.setFieldsValue({ ...r, api_token: undefined }); setOpen(true); }} />
          <Popconfirm title={t('common.confirmDelete')} onConfirm={async () => {
            try { await deleteClusterNode(r.id); message.success(t('cluster.deleted')); load(); }
            catch (e: any) { message.error(e.message || t('common.error')); }
          }}><Button size="small" danger icon={<DeleteOutlined />} /></Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <h2>{t('cluster.title')}</h2>
      <p style={{ opacity: 0.65 }}>{t('cluster.subtitle')}</p>
      <Card extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); form.setFieldsValue({ enabled: true }); setOpen(true); }}>{t('cluster.create')}</Button>}>
        <Table rowKey="id" loading={loading} dataSource={data} columns={columns} scroll={{ x: 720 }} />
      </Card>
      <Modal open={open} title={editing ? t('cluster.edit') : t('cluster.create')} onCancel={() => setOpen(false)} onOk={() => form.submit()} destroyOnClose>
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item name="name" label={t('cluster.name')} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="base_url" label={t('cluster.baseUrl')} rules={[{ required: true }]}><Input placeholder="https://panel.example.com:8080" /></Form.Item>
          <Form.Item name="api_token" label={t('cluster.apiToken')} tooltip={t('cluster.apiTokenHint')}><Input.Password placeholder={editing?.api_token_set ? '********' : ''} /></Form.Item>
          <Form.Item name="remark" label={t('cluster.remark')}><Input /></Form.Item>
          <Form.Item name="enabled" label={t('common.enabled')} valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ClusterPage;
