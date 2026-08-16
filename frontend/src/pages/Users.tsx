import React, { useEffect, useState } from 'react';
import {
  Card, Table, Button, Space, Modal, Form, Input, Switch, message, Popconfirm, Select, Tag,
  InputNumber, DatePicker, Progress, Tooltip,
} from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined, LinkOutlined, ClearOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import {
  fetchUsers, createUser, updateUser, deleteUser, resetUserTraffic,
  fetchUserNodes, bindUserNodes, ProxyUser,
} from '../api/users';
import { fetchListeners, Listener } from '../api/nodes';
import { useI18n } from '../i18n';

const formatBytes = (n?: number) => {
  const bytes = n || 0;
  if (bytes < 1024) return `${bytes} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let v = bytes / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(2)} ${units[i]}`;
};

const Users: React.FC = () => {
  const { t } = useI18n();
  const [data, setData] = useState<ProxyUser[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<ProxyUser | null>(null);
  const [form] = Form.useForm();

  const [bindOpen, setBindOpen] = useState(false);
  const [bindUser, setBindUser] = useState<ProxyUser | null>(null);
  const [allNodes, setAllNodes] = useState<Listener[]>([]);
  const [selectedNodeIds, setSelectedNodeIds] = useState<number[]>([]);
  const [bindLoading, setBindLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      setData(await fetchUsers());
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const onSubmit = async (values: any) => {
    try {
      const trafficGB = values.traffic_limit_gb;
      const payload: Record<string, unknown> = {
        username: values.username,
        enabled: !!values.enabled,
        traffic_limit: trafficGB && trafficGB > 0 ? Math.round(Number(trafficGB) * 1024 * 1024 * 1024) : 0,
      };
      if (values.password) payload.password = values.password;
      if (values.expire_time) {
        payload.expire_time = values.expire_time.toISOString();
      } else if (editing) {
        payload.expire_time = '0001-01-01T00:00:00Z';
      }
      if (editing) {
        await updateUser(editing.id, payload);
        message.success(t('users.updated'));
      } else {
        await createUser(payload);
        message.success(t('users.created'));
      }
      setModalOpen(false);
      setEditing(null);
      form.resetFields();
      load();
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    }
  };

  const onDelete = async (id: number) => {
    try {
      await deleteUser(id);
      message.success(t('users.deleted'));
      load();
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    }
  };

  const onResetTraffic = async (id: number) => {
    try {
      await resetUserTraffic(id);
      message.success(t('users.trafficReset'));
      load();
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    }
  };

  const openEdit = (record: ProxyUser) => {
    setEditing(record);
    const limitGB =
      record.traffic_limit && record.traffic_limit > 0
        ? Number((record.traffic_limit / (1024 * 1024 * 1024)).toFixed(3))
        : undefined;
    const exp =
      record.expire_time && !record.expire_time.startsWith('0001')
        ? dayjs(record.expire_time)
        : undefined;
    form.setFieldsValue({
      username: record.username,
      enabled: record.enabled,
      traffic_limit_gb: limitGB,
      expire_time: exp,
      password: undefined,
    });
    setModalOpen(true);
  };

  const openBind = async (record: ProxyUser) => {
    setBindUser(record);
    setBindOpen(true);
    setBindLoading(true);
    try {
      const [nodes, bound] = await Promise.all([fetchListeners(), fetchUserNodes(record.id)]);
      setAllNodes(nodes || []);
      setSelectedNodeIds((bound || []).map((n) => n.id));
    } catch (e: any) {
      message.error(e.message || t('common.error'));
      setBindOpen(false);
      setBindUser(null);
    } finally {
      setBindLoading(false);
    }
  };

  const onBindSave = async () => {
    if (!bindUser) return;
    setBindLoading(true);
    try {
      await bindUserNodes(bindUser.id, selectedNodeIds);
      message.success(t('users.bindSuccess'));
      setBindOpen(false);
      setBindUser(null);
      setSelectedNodeIds([]);
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setBindLoading(false);
    }
  };

  const columns = [
    { title: t('users.username'), dataIndex: 'username', key: 'username', width: 120 },
    {
      title: t('users.traffic'),
      key: 'traffic',
      width: 180,
      render: (_: any, r: ProxyUser) => {
        const used = r.traffic_used || 0;
        const limit = r.traffic_limit || 0;
        const pct = limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : 0;
        return (
          <div>
            <div style={{ fontSize: 12 }}>
              {formatBytes(used)}
              {limit > 0 ? ` / ${formatBytes(limit)}` : ` / ${t('users.unlimited')}`}
            </div>
            {limit > 0 && <Progress percent={pct} size="small" status={pct >= 100 ? 'exception' : 'active'} />}
          </div>
        );
      },
    },
    {
      title: t('users.expire'),
      dataIndex: 'expire_time',
      key: 'expire',
      width: 120,
      render: (v: string) => {
        if (!v || v.startsWith('0001')) return t('users.neverExpire');
        return dayjs(v).format('YYYY-MM-DD');
      },
    },
    {
      title: t('common.status'),
      key: 'status',
      width: 140,
      render: (_: any, r: ProxyUser) => (
        <Space size={4} wrap>
          {r.online ? <Tag color="success">{t('users.online')}</Tag> : <Tag>{t('users.offline')}</Tag>}
          {r.blocked ? <Tag color="error">{t('users.blocked')}</Tag> : null}
          {!r.enabled ? <Tag color="default">{t('common.disabled')}</Tag> : null}
        </Space>
      ),
    },
    {
      title: t('common.actions'),
      key: 'actions',
      width: 220,
      render: (_: any, record: ProxyUser) => (
        <Space wrap size={4}>
          <Button size="small" icon={<LinkOutlined />} onClick={() => openBind(record)}>
            {t('users.bind')}
          </Button>
          <Tooltip title={t('users.resetTraffic')}>
            <Popconfirm title={t('users.resetTrafficConfirm')} onConfirm={() => onResetTraffic(record.id)}>
              <Button size="small" icon={<ClearOutlined />} />
            </Popconfirm>
          </Tooltip>
          <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)} />
          <Popconfirm title={t('users.deleteConfirm')} onConfirm={() => onDelete(record.id)}>
            <Button size="small" icon={<DeleteOutlined />} danger />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <h2>{t('users.title')}</h2>
      <Card
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditing(null);
              form.resetFields();
              form.setFieldsValue({ enabled: true });
              setModalOpen(true);
            }}
          >
            {t('users.create')}
          </Button>
        }
      >
        <Table scroll={{ x: 780 }} size="middle" dataSource={data} columns={columns} rowKey="id" loading={loading} />
      </Card>

      <Modal
        open={modalOpen}
        title={editing ? t('users.edit') : t('users.create')}
        onCancel={() => {
          setModalOpen(false);
          setEditing(null);
          form.resetFields();
        }}
        onOk={() => form.submit()}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item name="username" label={t('users.username')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="password" label={t('users.password')} rules={[{ required: !editing }]}>
            <Input.Password placeholder={editing ? t('users.passwordKeep') : ''} />
          </Form.Item>
          <Form.Item
            name="traffic_limit_gb"
            label={t('users.trafficLimitGB')}
            tooltip={t('users.trafficLimitHint')}
          >
            <InputNumber min={0} step={1} style={{ width: '100%' }} placeholder="0 = unlimited" />
          </Form.Item>
          <Form.Item name="expire_time" label={t('users.expire')} tooltip={t('users.expireHint')}>
            <DatePicker showTime style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="enabled" label={t('users.enabled')} valuePropName="checked" initialValue={true}>
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={bindOpen}
        title={bindUser ? `${t('users.bindNodes')} — ${bindUser.username}` : t('users.bindNodes')}
        onCancel={() => {
          setBindOpen(false);
          setBindUser(null);
          setSelectedNodeIds([]);
        }}
        onOk={onBindSave}
        confirmLoading={bindLoading}
        destroyOnClose
        width={560}
      >
        <p style={{ marginBottom: 12, opacity: 0.65 }}>{t('users.bindHint')}</p>
        <Select
          mode="multiple"
          style={{ width: '100%' }}
          placeholder={t('users.selectNodes')}
          loading={bindLoading}
          value={selectedNodeIds}
          onChange={(ids: number[]) => setSelectedNodeIds(ids)}
          optionFilterProp="label"
          options={allNodes.map((n) => ({
            value: n.id,
            label: `${n.name} (${n.protocol}:${n.port})`,
          }))}
        />
      </Modal>
    </div>
  );
};

export default Users;
