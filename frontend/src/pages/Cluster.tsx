import client from '../api/client';
import React, { useEffect, useState } from 'react';
import { Card, Table, Button, Space, Modal, Form, Input, Switch, message, Popconfirm, Tag, Typography } from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  EditOutlined,
  MedicineBoxOutlined,
  CloudSyncOutlined,
  SendOutlined,
  DesktopOutlined,
  TeamOutlined,
  HddOutlined,
  LoginOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import {
  fetchCluster,
  createClusterNode,
  updateClusterNode,
  deleteClusterNode,
  healthClusterNode,
  syncRemoteNodes,
  pushClusterNode,
  loginClusterRemote,
  healthAllCluster,
  fetchRemoteDashboard,
  fetchRemoteUsers,
  remoteStartCore,
  remoteStopCore,
  remoteRestartCore,
  RemoteServer,
} from '../api/cluster';
import { useI18n } from '../i18n';
import PageHeader from '../components/PageHeader';
import useIsMobile from '../hooks/useIsMobile';

const errMsg = (e: any) => e?.message || e?.response?.data?.error || 'failed';

const ClusterPage: React.FC = () => {
  const { t } = useI18n();
  const isMobile = useIsMobile();
  const [data, setData] = useState<RemoteServer[]>([]);
  const [loading, setLoading] = useState(false);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<RemoteServer | null>(null);
  const [form] = Form.useForm();

  const [remoteNodes, setRemoteNodes] = useState<any[] | null>(null);
  const [remoteServerId, setRemoteServerId] = useState<number | null>(null);
  const [remoteForm] = Form.useForm();
  const [dashOpen, setDashOpen] = useState(false);
  const [dashData, setDashData] = useState<any>(null);
  const [usersOpen, setUsersOpen] = useState(false);
  const [remoteUsers, setRemoteUsers] = useState<any[]>([]);
  const [ctrlId, setCtrlId] = useState<number | null>(null);

  const [loginOpen, setLoginOpen] = useState(false);
  const [loginId, setLoginId] = useState<number | null>(null);
  const [loginForm] = Form.useForm();

  const load = async () => {
    setLoading(true);
    try {
      setData(await fetchCluster());
    } catch (e: any) {
      message.error(errMsg(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const loadRemoteNodes = async (id: number) => {
    try {
      const r = await client.get(`/cluster/${id}/nodes`);
      setRemoteNodes(Array.isArray(r.data) ? r.data : []);
      setRemoteServerId(id);
      message.success(t('cluster.nodesLoaded') || 'Loaded remote nodes');
    } catch (e: any) {
      message.error(errMsg(e));
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
      message.error(errMsg(e));
    }
  };

  const deleteRemoteNode = async (nodeId: number) => {
    if (!remoteServerId) return;
    try {
      await client.delete(`/cluster/${remoteServerId}/nodes/${nodeId}`);
      message.success(t('cluster.remoteDeleted') || 'Deleted');
      await loadRemoteNodes(remoteServerId);
    } catch (e: any) {
      message.error(errMsg(e));
    }
  };

  const openLogin = (id: number) => {
    setLoginId(id);
    loginForm.resetFields();
    setLoginOpen(true);
  };

  const doLoginRemote = async () => {
    if (!loginId) return;
    try {
      const v = await loginForm.validateFields();
      await loginClusterRemote(loginId, v.username, v.password);
      message.success(t('cluster.loginOk') || 'Token saved from remote login');
      setLoginOpen(false);
      load();
    } catch (e: any) {
      if (e?.errorFields) return;
      message.error(errMsg(e));
    }
  };

  const onSave = async () => {
    try {
      const v = await form.validateFields();
      if (editing) {
        await updateClusterNode(editing.id, {
          name: v.name,
          base_url: v.base_url,
          api_token: v.api_token || '',
          keep_token: !v.api_token,
          enabled: v.enabled,
          remark: v.remark || '',
        });
        message.success(t('cluster.updated') || 'Updated');
      } else {
        await createClusterNode({
          name: v.name,
          base_url: v.base_url,
          api_token: v.api_token || '',
          enabled: v.enabled !== false,
          remark: v.remark || '',
        });
        message.success(t('cluster.created') || 'Created');
      }
      setOpen(false);
      setEditing(null);
      form.resetFields();
      load();
    } catch (e: any) {
      if (e?.errorFields) return;
      message.error(errMsg(e));
    }
  };

  const columns = [
    { title: t('cluster.colName') || 'Name', dataIndex: 'name', ellipsis: true },
    { title: t('cluster.colBaseURL') || 'Panel URL', dataIndex: 'base_url', ellipsis: true },
    {
      title: t('cluster.colStatus') || 'Status',
      dataIndex: 'last_status',
      width: 90,
      render: (s: string) => {
        const color = s === 'up' ? 'green' : s === 'down' || s === 'error' ? 'red' : 'default';
        return <Tag color={color}>{s || '-'}</Tag>;
      },
    },
    {
      title: t('cluster.colToken') || 'Token',
      dataIndex: 'api_token_set',
      width: 80,
      render: (v: boolean) => (v ? <Tag color="blue">JWT</Tag> : <Tag>{t('cluster.tokenMissing') || 'none'}</Tag>),
    },
    {
      title: t('cluster.colEnabled') || 'Enabled',
      dataIndex: 'enabled',
      width: 80,
      render: (v: boolean) => (v ? t('common.enabled') || 'On' : t('common.disabled') || 'Off'),
    },
    {
      title: t('cluster.colError') || 'Last error',
      dataIndex: 'last_error',
      ellipsis: true,
      render: (s: string) => (s ? <Typography.Text type="danger" style={{ fontSize: 12 }}>{s}</Typography.Text> : '-'),
    },
    {
      title: t('common.actions') || 'Actions',
      key: 'actions',
      width: isMobile ? 200 : 420,
      render: (_: unknown, r: RemoteServer) => (
        <Space size={[4, 4]} wrap>
          <Button
            size="small"
            icon={<MedicineBoxOutlined />}
            onClick={async () => {
              try {
                await healthClusterNode(r.id);
                message.success(t('cluster.healthDone') || 'Health checked');
                load();
              } catch (e: any) {
                message.error(errMsg(e));
              }
            }}
          >
            {t('cluster.health') || 'Health'}
          </Button>
          <Button size="small" icon={<LoginOutlined />} onClick={() => openLogin(r.id)}>
            {t('cluster.login') || 'Login'}
          </Button>
          <Button
            size="small"
            icon={<DesktopOutlined />}
            onClick={async () => {
              try {
                setCtrlId(r.id);
                setDashData(await fetchRemoteDashboard(r.id));
                setDashOpen(true);
              } catch (e: any) {
                message.error(errMsg(e));
              }
            }}
          >
            {t('cluster.dashboard') || 'Dashboard'}
          </Button>
          <Button size="small" icon={<HddOutlined />} onClick={() => loadRemoteNodes(r.id)}>
            {t('cluster.remoteNodes') || 'Nodes'}
          </Button>
          <Button
            size="small"
            icon={<CloudSyncOutlined />}
            onClick={async () => {
              try {
                await syncRemoteNodes(r.id);
                message.success(t('cluster.syncDone') || 'Synced');
              } catch (e: any) {
                message.error(errMsg(e));
              }
            }}
          >
            {t('cluster.syncNodes') || 'Sync'}
          </Button>
          <Button
            size="small"
            icon={<SendOutlined />}
            onClick={async () => {
              const idStr = window.prompt(t('cluster.pushNodePrompt') || 'Local node ID to push (disabled on remote)?', '');
              const localId = Number(idStr || 0);
              if (!localId) return;
              try {
                const dry = await pushClusterNode(r.id, { local_node_id: localId, dry_run: true });
                if (
                  !window.confirm(
                    (t('cluster.pushNodeConfirm') || 'Push this node to remote?') +
                      '\n' +
                      JSON.stringify(dry?.payload || dry, null, 2).slice(0, 800),
                  )
                ) {
                  return;
                }
                await pushClusterNode(r.id, { local_node_id: localId, dry_run: false });
                message.success(t('cluster.pushNodeDone') || 'Pushed (created disabled on remote)');
              } catch (e: any) {
                message.error(errMsg(e));
              }
            }}
          >
            {t('cluster.pushNode') || 'Push'}
          </Button>
          <Button
            size="small"
            icon={<TeamOutlined />}
            onClick={async () => {
              try {
                setCtrlId(r.id);
                const u = await fetchRemoteUsers(r.id);
                setRemoteUsers(Array.isArray(u) ? u : u?.items || []);
                setUsersOpen(true);
              } catch (e: any) {
                message.error(errMsg(e));
              }
            }}
          >
            {t('cluster.users') || 'Users'}
          </Button>
          <Button
            size="small"
            icon={<ReloadOutlined />}
            onClick={async () => {
              try {
                await remoteRestartCore(r.id);
                message.success(t('cluster.restarted') || 'Core restarted');
              } catch (e: any) {
                message.error(errMsg(e));
              }
            }}
          >
            {t('cluster.restartCore') || 'Restart'}
          </Button>
          <Button
            size="small"
            icon={<EditOutlined />}
            onClick={() => {
              setEditing(r);
              form.setFieldsValue({ ...r, api_token: undefined });
              setOpen(true);
            }}
          />
          <Popconfirm
            title={t('common.confirmDelete')}
            onConfirm={async () => {
              try {
                await deleteClusterNode(r.id);
                message.success(t('cluster.deleted') || 'Deleted');
                load();
              } catch (e: any) {
                message.error(errMsg(e));
              }
            }}
          >
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <PageHeader title={t('cluster.title')} subtitle={t('cluster.subtitle')} />
      <Card
        extra={
          <Space wrap>
            <Button
              icon={<MedicineBoxOutlined />}
              onClick={async () => {
                try {
                  setData(await healthAllCluster());
                  message.success(t('cluster.healthAllDone') || 'Health check done');
                } catch (e: any) {
                  message.error(errMsg(e));
                }
              }}
            >
              {t('cluster.healthAll') || 'Check all'}
            </Button>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => {
                setEditing(null);
                form.resetFields();
                form.setFieldsValue({ enabled: true });
                setOpen(true);
              }}
            >
              {t('cluster.add') || 'Add node'}
            </Button>
          </Space>
        }
      >
        <Typography.Paragraph type="secondary" style={{ marginTop: 0 }}>
          {t('cluster.tokenHint') ||
            'Remote management needs a JWT from the remote panel (POST /api/v1/auth/login). Health check can work without a token. Use Login on each row to refresh the token when operations fail with 401.'}
        </Typography.Paragraph>
        <Table
          size="small"
          rowKey="id"
          loading={loading}
          dataSource={data}
          columns={columns}
          scroll={{ x: true }}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      <Modal
        open={open}
        title={editing ? t('cluster.edit') || 'Edit' : t('cluster.add') || 'Add'}
        onCancel={() => {
          setOpen(false);
          setEditing(null);
        }}
        onOk={onSave}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label={t('cluster.colName') || 'Name'} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="base_url"
            label={t('cluster.colBaseURL') || 'Panel URL'}
            rules={[{ required: true }]}
            extra={t('cluster.baseURLHint') || 'e.g. http://1.2.3.4:8080 (no trailing path)'}
          >
            <Input placeholder="http://x.x.x.x:8080" />
          </Form.Item>
          <Form.Item
            name="api_token"
            label={t('cluster.apiToken') || 'API JWT'}
            extra={
              editing
                ? t('cluster.tokenKeepHint') || 'Leave empty to keep existing token'
                : t('cluster.tokenCreateHint') || 'Optional now — use Login later to obtain JWT'
            }
          >
            <Input.Password placeholder="eyJhbGciOi..." autoComplete="off" />
          </Form.Item>
          <Form.Item name="enabled" label={t('cluster.colEnabled') || 'Enabled'} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="remark" label={t('cluster.remark') || 'Remark'}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={loginOpen}
        title={t('cluster.loginTitle') || 'Remote panel login'}
        onCancel={() => setLoginOpen(false)}
        onOk={doLoginRemote}
        okText={t('cluster.loginSave') || 'Login & save token'}
        destroyOnClose
      >
        <Typography.Paragraph type="secondary">
          {t('cluster.loginHint') ||
            'Signs in to the remote 3m-ui admin API and stores the JWT for cluster operations.'}
        </Typography.Paragraph>
        <Form form={loginForm} layout="vertical">
          <Form.Item name="username" label={t('login.username') || 'Username'} rules={[{ required: true }]}>
            <Input autoComplete="username" />
          </Form.Item>
          <Form.Item name="password" label={t('login.password') || 'Password'} rules={[{ required: true }]}>
            <Input.Password autoComplete="current-password" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={remoteNodes !== null}
        onCancel={() => {
          setRemoteNodes(null);
          setRemoteServerId(null);
        }}
        footer={null}
        title={t('cluster.remoteNodes') || 'Remote nodes'}
        width={isMobile ? '100%' : 900}
      >
        <Form form={remoteForm} layout={isMobile ? 'vertical' : 'inline'} onFinish={createRemoteNode} style={{ marginBottom: 12 }}>
          <Form.Item name="name" rules={[{ required: true }]}>
            <Input placeholder={t('cluster.colName') || 'Name'} />
          </Form.Item>
          <Form.Item name="protocol" rules={[{ required: true }]} initialValue="vless">
            <Input placeholder="protocol" style={{ width: 100 }} />
          </Form.Item>
          <Form.Item name="port" rules={[{ required: true }]}>
            <Input placeholder="port" style={{ width: 90 }} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" icon={<PlusOutlined />}>
              {t('common.create') || 'Create'}
            </Button>
          </Form.Item>
        </Form>
        <Table
          size="small"
          rowKey={(r: any) => r.id ?? r.ID}
          dataSource={remoteNodes || []}
          pagination={{ pageSize: 8 }}
          columns={[
            { title: t('cluster.colName'), dataIndex: 'name' },
            { title: t('cluster.colProtocol'), dataIndex: 'protocol', width: 100 },
            { title: t('cluster.colPort'), dataIndex: 'port', width: 80 },
            {
              title: t('common.actions'),
              width: 100,
              render: (_: any, r: any) => (
                <Popconfirm title={t('common.confirmDelete')} onConfirm={() => deleteRemoteNode(r.id ?? r.ID)}>
                  <Button size="small" danger>
                    {t('common.delete')}
                  </Button>
                </Popconfirm>
              ),
            },
          ]}
        />
      </Modal>

      <Modal
        open={dashOpen}
        onCancel={() => {
          setDashOpen(false);
          setDashData(null);
        }}
        footer={
          ctrlId ? (
            <Space>
              <Button
                onClick={async () => {
                  try {
                    await remoteStartCore(ctrlId!);
                    message.success(t('cluster.operationOk') || t('common.ok'));
                  } catch (e: any) {
                    message.error(errMsg(e));
                  }
                }}
              >
                {t('cluster.startCore') || 'Start core'}
              </Button>
              <Button
                danger
                onClick={async () => {
                  try {
                    await remoteStopCore(ctrlId!);
                    message.success(t('cluster.operationOk') || t('common.ok'));
                  } catch (e: any) {
                    message.error(errMsg(e));
                  }
                }}
              >
                {t('cluster.stopCore') || 'Stop core'}
              </Button>
              <Button
                type="primary"
                onClick={async () => {
                  try {
                    await remoteRestartCore(ctrlId!);
                    message.success(t('cluster.operationOk') || t('common.ok'));
                  } catch (e: any) {
                    message.error(errMsg(e));
                  }
                }}
              >
                {t('cluster.restartCore') || 'Restart'}
              </Button>
            </Space>
          ) : null
        }
        title={t('cluster.dashboard') || 'Remote dashboard'}
        width={720}
      >
        <pre style={{ maxHeight: 420, overflow: 'auto', fontSize: 12 }}>
          {dashData ? JSON.stringify(dashData, null, 2) : ''}
        </pre>
      </Modal>

      <Modal
        open={usersOpen}
        onCancel={() => {
          setUsersOpen(false);
          setRemoteUsers([]);
        }}
        footer={null}
        title={t('cluster.users') || 'Remote users'}
        width={isMobile ? '100%' : 800}
      >
        <Table
          size="small"
          rowKey={(r: any) => r.id || r.username}
          dataSource={remoteUsers}
          pagination={{ pageSize: 10 }}
          columns={[
            { title: t('cluster.colId'), dataIndex: 'id', width: 60 },
            { title: t('users.username') || 'User', dataIndex: 'username' },
            {
              title: t('users.enabled') || 'Enabled',
              dataIndex: 'enabled',
              width: 80,
              render: (v: boolean) => String(!!v),
            },
            {
              title: t('users.traffic') || 'Traffic',
              key: 'tr',
              render: (_: any, r: any) => `${r.traffic_used ?? 0} / ${r.traffic_limit ?? 0}`,
            },
          ]}
        />
      </Modal>
    </div>
  );
};

export default ClusterPage;
