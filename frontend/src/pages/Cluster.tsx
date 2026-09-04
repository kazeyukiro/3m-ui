import client from '../api/client';
import React, { useEffect, useState } from 'react';
import { Card, Table, Button, Space, Modal, Form, Input, Switch, message, Popconfirm, Tag } from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined, HeartOutlined } from '@ant-design/icons';
import {
  fetchCluster, createClusterNode, updateClusterNode, deleteClusterNode, healthClusterNode,
  syncRemoteNodes,
  healthAllCluster, fetchRemoteDashboard, fetchRemoteUsers, remoteStartCore, remoteStopCore, remoteRestartCore,
  RemoteServer,
} from '../api/cluster';
import { useI18n } from '../i18n';

const ClusterPage: React.FC = () => {
  const { t } = useI18n();
  const [remoteNodes, setRemoteNodes] = useState<any[] | null>(null);
  const [remoteServerId, setRemoteServerId] = useState<number | null>(null);
  const [remoteForm] = Form.useForm();
  const [dashOpen, setDashOpen] = useState(false);
  const [dashData, setDashData] = useState<any>(null);
  const [usersOpen, setUsersOpen] = useState(false);
  const [remoteUsers, setRemoteUsers] = useState<any[]>([]);
  const [ctrlId, setCtrlId] = useState<number | null>(null);

  const loadRemoteNodes = async (id: number) => {
    try {
      const r = await client.get(`/cluster/${id}/nodes`);
      setRemoteNodes(Array.isArray(r.data) ? r.data : []);
      setRemoteServerId(id);
      message.success(t('cluster.nodesLoaded') || 'Loaded remote nodes');
    } catch (e: any) {
      message.error(e.message || 'failed');
    }
  };

  const createRemoteNode = async (values: any) => {
    if (!remoteServerId) return;
    try {
      await client.post(`/cluster/${remoteServerId}/nodes`, {
        name: values.name,
        protocol: values.protocol,
        port: String(values.port),
        bind_address: values.bind_address || '0.0.0.0',
        enabled: true,
        config: values.config || '{}',
      });
      message.success(t('cluster.remoteCreated') || 'Remote node created');
      remoteForm.resetFields();
      await loadRemoteNodes(remoteServerId);
    } catch (e: any) {
      message.error(e.response?.data?.error || e.message || 'failed');
    }
  };

  const deleteRemoteNode = async (nodeId: number) => {
    if (!remoteServerId) return;
    try {
      await client.delete(`/cluster/${remoteServerId}/nodes/${nodeId}`);
      message.success(t('cluster.remoteDeleted') || 'Remote node deleted');
      await loadRemoteNodes(remoteServerId);
    } catch (e: any) {
      message.error(e.response?.data?.error || e.message || 'failed');
    }
  };

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
        <Space wrap size={4}>
          <Button size="small" icon={<HeartOutlined />} onClick={async () => {
            try { await healthClusterNode(r.id); message.success(t('cluster.healthDone')); load(); }
            catch (e: any) { message.error(e.message || t('common.error')); }
          }}>{t('cluster.health')}</Button>
          <Button size="small" onClick={async () => {
            try {
              setCtrlId(r.id);
              setDashData(await fetchRemoteDashboard(r.id));
              setDashOpen(true);
            } catch (e: any) { message.error(e.message || t('common.error')); }
          }}>{t('cluster.dashboard') || 'Dashboard'}</Button>
          <Button size="small" onClick={() => loadRemoteNodes(r.id)}>{t('cluster.remoteNodes') || 'Nodes'}</Button>
          <Button size="small" onClick={async () => {
            try {
              const rows = await syncRemoteNodes(r.id);
              message.success((t('cluster.syncDone') || 'Synced') + `: ${rows?.length ?? 0}`);
              load();
            } catch (e: any) { message.error(e.message || t('common.error')); }
          }}>{t('cluster.syncNodes') || 'Sync nodes'}</Button>
          <Button size="small" onClick={async () => {
            try {
              setCtrlId(r.id);
              const u = await fetchRemoteUsers(r.id);
              setRemoteUsers(Array.isArray(u) ? u : (u?.items || []));
              setUsersOpen(true);
            } catch (e: any) { message.error(e.message || t('common.error')); }
          }}>{t('cluster.users') || 'Users'}</Button>
          <Button size="small" onClick={async () => {
            try { await remoteRestartCore(r.id); message.success(t('cluster.restarted') || 'Core restarted'); }
            catch (e: any) { message.error(e.message || t('common.error')); }
          }}>{t('cluster.restartCore') || 'Restart'}</Button>
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
      <Card extra={<Space>
        <Button onClick={async () => {
          try { setData(await healthAllCluster()); message.success(t('cluster.healthAllDone') || 'Health check done'); }
          catch (e: any) { message.error(e.message || t('common.error')); }
        }} icon={<HeartOutlined />}>{t('cluster.healthAll') || 'Check all'}</Button>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); form.setFieldsValue({ enabled: true }); setOpen(true); }}>{t('cluster.create')}</Button>
      </Space>}>
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
      <Modal open={!!remoteNodes} onCancel={() => { setRemoteNodes(null); setRemoteServerId(null); }} footer={null} title={t('cluster.remoteNodes') || 'Remote nodes'} width={800}>
        <Form form={remoteForm} layout="inline" onFinish={createRemoteNode} style={{ marginBottom: 16, flexWrap: 'wrap', gap: 8 }}>
          <Form.Item name="name" rules={[{ required: true }]}><Input placeholder="name" /></Form.Item>
          <Form.Item name="protocol" initialValue="vless" rules={[{ required: true }]}>
            <Input placeholder="protocol" style={{ width: 100 }} />
          </Form.Item>
          <Form.Item name="port" rules={[{ required: true }]}><Input placeholder="port" style={{ width: 90 }} /></Form.Item>
          <Form.Item name="bind_address" initialValue="0.0.0.0"><Input placeholder="bind" style={{ width: 110 }} /></Form.Item>
          <Button type="primary" htmlType="submit">{t('cluster.remoteCreate') || 'Create'}</Button>
        </Form>
        <Table
          size="small"
          rowKey={(r: any) => r.id || r.ID}
          dataSource={remoteNodes || []}
          pagination={{ pageSize: 8 }}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 60, render: (_: any, r: any) => r.id ?? r.ID },
            { title: 'Name', dataIndex: 'name' },
            { title: 'Protocol', dataIndex: 'protocol', width: 100 },
            { title: 'Port', dataIndex: 'port', width: 80 },
            {
              title: t('common.actions'),
              width: 100,
              render: (_: any, r: any) => (
                <Popconfirm title={t('common.confirmDelete')} onConfirm={() => deleteRemoteNode(r.id ?? r.ID)}>
                  <Button size="small" danger>{t('common.delete')}</Button>
                </Popconfirm>
              ),
            },
          ]}
        />
      </Modal>
      <Modal open={dashOpen} onCancel={() => { setDashOpen(false); setDashData(null); }} footer={ctrlId ? (
        <Space>
          <Button onClick={async () => { try { await remoteStartCore(ctrlId!); message.success('OK'); } catch (e: any) { message.error(e.message); } }}>{t('cluster.startCore') || 'Start core'}</Button>
          <Button danger onClick={async () => { try { await remoteStopCore(ctrlId!); message.success('OK'); } catch (e: any) { message.error(e.message); } }}>{t('cluster.stopCore') || 'Stop core'}</Button>
          <Button type="primary" onClick={async () => { try { await remoteRestartCore(ctrlId!); message.success('OK'); } catch (e: any) { message.error(e.message); } }}>{t('cluster.restartCore') || 'Restart'}</Button>
        </Space>
      ) : null} title={t('cluster.dashboard') || 'Remote dashboard'} width={720}>
        <pre style={{ maxHeight: 420, overflow: 'auto', fontSize: 12 }}>{dashData ? JSON.stringify(dashData, null, 2) : ''}</pre>
      </Modal>
      <Modal open={usersOpen} onCancel={() => { setUsersOpen(false); setRemoteUsers([]); }} footer={null} title={t('cluster.users') || 'Remote users'} width={800}>
        <Table size="small" rowKey={(r: any) => r.id || r.username} dataSource={remoteUsers} pagination={{ pageSize: 10 }}
          columns={[
            { title: 'ID', dataIndex: 'id', width: 60 },
            { title: t('users.username') || 'User', dataIndex: 'username' },
            { title: t('users.enabled') || 'Enabled', dataIndex: 'enabled', width: 80, render: (v: boolean) => String(!!v) },
            { title: t('users.traffic') || 'Traffic', key: 'tr', render: (_: any, r: any) => `${r.traffic_used ?? 0} / ${r.traffic_limit ?? 0}` },
          ]}
        />
      </Modal>
    </div>
  );
};

export default ClusterPage;
