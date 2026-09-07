import React, { useEffect, useMemo, useState } from 'react';
import {
  Card, Table, Button, Space, Modal, Form, Input, Switch, message, Popconfirm, Select, Tag,
  InputNumber, DatePicker, Progress, Tooltip, Dropdown, Checkbox, Spin,
} from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined, LinkOutlined, ClearOutlined, ShareAltOutlined, CopyOutlined, MoreOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import {
  fetchUsers, createUser, updateUser, deleteUser, resetUserTraffic, deleteDepletedUsers, batchUsers,
  fetchUserNodes, bindUserNodes, fetchUserRemoteNodes, bindUserRemoteNodes, ProxyUser,
} from '../api/users';
import { fetchListeners, Listener } from '../api/nodes';
import { fetchMirroredNodes, RemoteNodeMirror } from '../api/cluster';
import { useI18n } from '../i18n';
import PageHeader from '../components/PageHeader';
import useIsMobile from '../hooks/useIsMobile';
import { useNavigate } from 'react-router-dom';
import { copyText } from '../utils/clipboard';
import { formatBytes } from '../utils/format';

const Users: React.FC = () => {
  const navigate = useNavigate();
  const { t } = useI18n();
  const isMobile = useIsMobile();
  const [data, setData] = useState<ProxyUser[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<ProxyUser | null>(null);
  const [form] = Form.useForm();
  const [keyword, setKeyword] = useState('');
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);

  const [bindOpen, setBindOpen] = useState(false);
  const [bindUser, setBindUser] = useState<ProxyUser | null>(null);
  const [allNodes, setAllNodes] = useState<Listener[]>([]);
  const [selectedNodeIds, setSelectedNodeIds] = useState<number[]>([]);
  const [remoteMirrors, setRemoteMirrors] = useState<RemoteNodeMirror[]>([]);
  const [selectedRemoteIds, setSelectedRemoteIds] = useState<number[]>([]);
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

  const filtered = useMemo(() => {
    const q = keyword.trim().toLowerCase();
    if (!q) return data;
    return data.filter((u) =>
      (u.username || '').toLowerCase().includes(q) ||
      (u.remark || '').toLowerCase().includes(q)
    );
  }, [data, keyword]);

  const onSubmit = async (values: any) => {
    try {
      const trafficGB = values.traffic_limit_gb;
      const payload: Record<string, unknown> = {
        username: values.username,
        enabled: !!values.enabled,
        traffic_limit: trafficGB && trafficGB > 0 ? Math.round(Number(trafficGB) * 1024 * 1024 * 1024) : 0,
        ip_limit: values.ip_limit != null ? Number(values.ip_limit) : 0,
        hwid_limit: values.hwid_limit != null ? Number(values.hwid_limit) : 0,
        remark: values.remark || '',
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
      ip_limit: record.ip_limit || 0,
      hwid_limit: record.hwid_limit || 0,
      remark: record.remark || '',
    });
    setModalOpen(true);
  };

  const openBind = async (record: ProxyUser) => {
    setBindUser(record);
    setBindOpen(true);
    setBindLoading(true);
    try {
      const [nodes, bound, mirrors, remoteBound] = await Promise.all([
        fetchListeners(),
        fetchUserNodes(record.id),
        fetchMirroredNodes().catch(() => [] as RemoteNodeMirror[]),
        fetchUserRemoteNodes(record.id).catch(() => ({ mirror_ids: [] as number[] })),
      ]);
      setAllNodes(nodes || []);
      setSelectedNodeIds((bound || []).map((n) => n.id));
      setRemoteMirrors(mirrors || []);
      setSelectedRemoteIds(remoteBound?.mirror_ids || []);
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
      await bindUserRemoteNodes(bindUser.id, selectedRemoteIds);
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

  const openShare = async (record: ProxyUser) => {
    navigate(`/share?user=${record.id}`);
  };

  const onDeleteDepleted = async () => {
    try {
      const res = await deleteDepletedUsers();
      message.success((t('users.depletedDeleted') || 'Deleted depleted users') + `: ${res.deleted}`);
      load();
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    }
  };

  const onBatch = async (action: string, extra?: { days?: number; traffic_gb?: number }) => {
    const ids = selectedRowKeys.map((k) => Number(k)).filter((n) => n > 0);
    if (!ids.length) {
      message.warning(t('users.batchNeedSelect') || 'Select users first');
      return;
    }
    try {
      const res = await batchUsers(action, ids, extra);
      message.success((t('users.batchDone') || 'Batch done') + `: ${res.affected}`);
      setSelectedRowKeys([]);
      load();
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    }
  };


  const columns = [
    { title: t('users.username'), dataIndex: 'username', key: 'username', width: 120, ellipsis: true },
    {
      title: t('users.remark') || 'Remark',
      dataIndex: 'remark',
      key: 'remark',
      width: 120,
      ellipsis: true,
      render: (v: string) => v || '-',
    },
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
      title: t('users.ipLimit') || 'IP limit',
      dataIndex: 'ip_limit',
      key: 'ip_limit',
      width: 90,
      render: (v: number) => (v && v > 0 ? v : t('users.unlimited')),
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
      filters: [
        { text: t('users.online'), value: 'online' },
        { text: t('users.offline'), value: 'offline' },
        { text: t('users.enabled') || 'Enabled', value: 'enabled' },
        { text: t('users.disabled') || 'Disabled', value: 'disabled' },
      ],
      onFilter: (value: any, r: ProxyUser) => {
        if (value === 'online') return !!r.online;
        if (value === 'offline') return !r.online;
        if (value === 'enabled') return !!r.enabled;
        if (value === 'disabled') return !r.enabled;
        return true;
      },
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
      width: 260,
      fixed: 'right' as const,
      render: (_: any, record: ProxyUser) => (
        <Space wrap size={4}>
          <Button size="small" icon={<LinkOutlined />} onClick={() => openBind(record)}>
            {t('users.bind')}
          </Button>
          <Tooltip title={t('users.shareTitle') || 'Subscription'}>
            <Button size="small" icon={<ShareAltOutlined />} onClick={() => openShare(record)} />
          </Tooltip>
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
      <PageHeader title={t('users.title')} subtitle={t('users.subtitle', 'Manage accounts, traffic limits, node bindings and subscriptions.')} />
      <Card
        extra={
          <Space wrap style={{ width: isMobile ? '100%' : undefined }}>
            <Input.Search
              allowClear
              placeholder={t('common.search')}
              onSearch={setKeyword}
              onChange={(e) => { if (!e.target.value) setKeyword(''); }}
              style={{ width: isMobile ? '100%' : 200 }}
            />
            {isMobile ? (
              <>
                <Button
                  type="primary"
                  icon={<PlusOutlined />}
                  block
                  onClick={() => {
                    setEditing(null);
                    form.resetFields();
                    form.setFieldsValue({ enabled: true, ip_limit: 0, hwid_limit: 0 });
                    setModalOpen(true);
                  }}
                >
                  {t('users.create')}
                </Button>
                <Dropdown
                  menu={{
                    items: [
                      {
                        key: 'enable',
                        label: t('users.batchEnable') || 'Enable',
                        disabled: !selectedRowKeys.length,
                        onClick: () => onBatch('enable'),
                      },
                      {
                        key: 'disable',
                        label: t('users.batchDisable') || 'Disable',
                        disabled: !selectedRowKeys.length,
                        onClick: () => onBatch('disable'),
                      },
                      {
                        key: 'reset',
                        label: t('users.batchResetTraffic') || 'Reset traffic',
                        disabled: !selectedRowKeys.length,
                        onClick: () => onBatch('reset-traffic'),
                      },
                      {
                        key: 'extend',
                        label: t('users.batchExtend') || 'Extend days',
                        disabled: !selectedRowKeys.length,
                        onClick: () => {
                          const days = Number(window.prompt(t('users.batchExtendPrompt') || 'Extend by how many days?', '30') || 0);
                          if (days > 0) onBatch('extend-days', { days });
                        },
                      },
                      {
                        key: 'addtf',
                        label: t('users.batchAddTraffic') || 'Add traffic',
                        disabled: !selectedRowKeys.length,
                        onClick: () => {
                          const gb = Number(window.prompt(t('users.batchAddTrafficPrompt') || 'Add how many GiB to limit?', '10') || 0);
                          if (gb > 0) onBatch('add-traffic', { traffic_gb: gb });
                        },
                      },
                      { type: 'divider' },
                      {
                        key: 'del',
                        danger: true,
                        label: t('users.batchDelete') || 'Delete selected',
                        disabled: !selectedRowKeys.length,
                        onClick: () => {
                          Modal.confirm({
                            title: t('users.batchDeleteConfirm') || 'Delete selected users?',
                            onOk: () => onBatch('delete'),
                          });
                        },
                      },
                      {
                        key: 'depleted',
                        danger: true,
                        label: t('users.deleteDepleted') || 'Delete depleted',
                        onClick: () => {
                          Modal.confirm({
                            title: t('users.deleteDepletedConfirm') || 'Delete all expired / over-quota users?',
                            onOk: onDeleteDepleted,
                          });
                        },
                      },
                    ],
                  }}
                >
                  <Button block>{t('users.batch') || 'Batch'} ({selectedRowKeys.length})</Button>
                </Dropdown>
              </>
            ) : (
              <>
            <Button disabled={!selectedRowKeys.length} onClick={() => onBatch('enable')}>
              {t('users.batchEnable') || 'Enable'}
            </Button>
            <Button disabled={!selectedRowKeys.length} onClick={() => onBatch('disable')}>
              {t('users.batchDisable') || 'Disable'}
            </Button>
            <Button disabled={!selectedRowKeys.length} onClick={() => onBatch('reset-traffic')}>
              {t('users.batchResetTraffic') || 'Reset traffic'}
            </Button>
            <Button
              disabled={!selectedRowKeys.length}
              onClick={() => {
                const days = Number(window.prompt(t('users.batchExtendPrompt') || 'Extend by how many days?', '30') || 0);
                if (days > 0) onBatch('extend-days', { days });
              }}
            >
              {t('users.batchExtend') || 'Extend days'}
            </Button>
            <Button
              disabled={!selectedRowKeys.length}
              onClick={() => {
                const gb = Number(window.prompt(t('users.batchAddTrafficPrompt') || 'Add how many GiB to limit?', '10') || 0);
                if (gb > 0) onBatch('add-traffic', { traffic_gb: gb });
              }}
            >
              {t('users.batchAddTraffic') || 'Add traffic'}
            </Button>
            <Popconfirm
              title={t('users.batchDeleteConfirm') || 'Delete selected users?'}
              onConfirm={() => onBatch('delete')}
              disabled={!selectedRowKeys.length}
            >
              <Button danger disabled={!selectedRowKeys.length} icon={<DeleteOutlined />}>
                {t('users.batchDelete') || 'Delete selected'}
              </Button>
            </Popconfirm>
            <Popconfirm
              title={t('users.deleteDepletedConfirm') || 'Delete all expired / over-quota users?'}
              onConfirm={onDeleteDepleted}
            >
              <Button danger icon={<DeleteOutlined />}>
                {t('users.deleteDepleted') || 'Delete depleted'}
              </Button>
            </Popconfirm>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => {
                setEditing(null);
                form.resetFields();
                form.setFieldsValue({ enabled: true, ip_limit: 0, hwid_limit: 0 });
                setModalOpen(true);
              }}
            >
              {t('users.create')}
            </Button>
              </>
            )}
          </Space>
        }
      >
        {isMobile ? (
          <Spin spinning={loading}>
            <div className="mobile-entity-list">
              {filtered.length === 0 && !loading ? (
                <div className="mobile-empty">{t('common.empty')}</div>
              ) : filtered.map((record) => {
                const id = record.id;
                const checked = selectedRowKeys.includes(id);
                const used = Number(record.traffic_used || 0);
                const limit = Number(record.traffic_limit || 0);
                return (
                  <div className="mobile-entity-card" key={id}>
                    <div className="mobile-entity-card-main">
                      <Checkbox
                        checked={checked}
                        onChange={(e) => {
                          setSelectedRowKeys((keys) =>
                            e.target.checked ? [...keys, id] : keys.filter((k) => k !== id),
                          );
                        }}
                      />
                      <div
                        className="mobile-entity-card-body"
                        onClick={() => {
                          setEditing(record);
                          form.setFieldsValue({
                            username: record.username,
                            password: undefined,
                            enabled: record.enabled,
                            traffic_limit: record.traffic_limit
                              ? Number((record.traffic_limit / (1024 ** 3)).toFixed(2))
                              : 0,
                            expire_time:
                              record.expire_time && !String(record.expire_time).startsWith('0001')
                                ? dayjs(record.expire_time)
                                : undefined,
                            ip_limit: record.ip_limit || 0,
                            hwid_limit: record.hwid_limit || 0,
                            remark: record.remark,
                          });
                          setModalOpen(true);
                        }}
                      >
                        <div className="mobile-entity-title">
                          {record.username}
                          {record.remark ? (
                            <span className="mobile-entity-sub"> · {record.remark}</span>
                          ) : null}
                        </div>
                        <div className="mobile-entity-meta">
                          <Tag color={record.online ? 'success' : 'default'}>
                            {record.online ? t('users.online') : t('users.offline')}
                          </Tag>
                          <Tag color={record.enabled ? 'processing' : 'default'}>
                            {record.enabled ? t('common.enabled') : t('common.disabled')}
                          </Tag>
                          <span className="mobile-entity-sub">
                            {formatBytes(used)}
                            {limit > 0 ? ` / ${formatBytes(limit)}` : ` / ${t('users.unlimited')}`}
                          </span>
                        </div>
                      </div>
                      <Dropdown
                        menu={{
                          items: [
                            {
                              key: 'edit',
                              icon: <EditOutlined />,
                              label: t('common.edit'),
                              onClick: () => {
                                setEditing(record);
                                form.setFieldsValue({
                                  username: record.username,
                                  password: undefined,
                                  enabled: record.enabled,
                                  traffic_limit: record.traffic_limit
                                    ? Number((record.traffic_limit / (1024 ** 3)).toFixed(2))
                                    : 0,
                                  expire_time:
                                    record.expire_time && !String(record.expire_time).startsWith('0001')
                                      ? dayjs(record.expire_time)
                                      : undefined,
                                  ip_limit: record.ip_limit || 0,
                                  hwid_limit: record.hwid_limit || 0,
                                  remark: record.remark,
                                });
                                setModalOpen(true);
                              },
                            },
                            {
                              key: 'nodes',
                              icon: <LinkOutlined />,
                              label: t('users.bindNodes') || 'Bind nodes',
                              onClick: () => openBind(record),
                            },
                            {
                              key: 'reset',
                              icon: <ClearOutlined />,
                              label: t('users.resetTraffic') || 'Reset traffic',
                              onClick: () => onResetTraffic(record.id),
                            },
                            { type: 'divider' },
                            {
                              key: 'del',
                              icon: <DeleteOutlined />,
                              danger: true,
                              label: t('common.delete'),
                              onClick: () => {
                                Modal.confirm({
                                  title: t('users.deleteConfirm') || t('common.confirmDelete'),
                                  onOk: () => onDelete(record.id),
                                });
                              },
                            },
                          ],
                        }}
                        trigger={['click']}
                      >
                        <Button type="text" icon={<MoreOutlined />} aria-label="actions" />
                      </Dropdown>
                    </div>
                  </div>
                );
              })}
            </div>
          </Spin>
        ) : (
          <Table
            scroll={{ x: 960 }}
            size="middle"
            dataSource={filtered}
            columns={columns}
            rowKey="id"
            loading={loading}
            rowSelection={{ selectedRowKeys, onChange: setSelectedRowKeys }}
          />
        )}
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
        width={isMobile ? '100%' : 520}
        style={isMobile ? { top: 8, maxWidth: '100vw' } : undefined}
        className={isMobile ? 'mobile-full-modal' : undefined}
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
          <Form.Item
            name="ip_limit"
            label={t('users.ipLimit') || 'IP limit'}
            tooltip={t('users.ipLimitHint') || '0 = unlimited concurrent client IPs'}
          >
            <InputNumber min={0} step={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="hwid_limit"
            label={t('users.hwidLimit') || 'HWID device limit'}
            tooltip={t('users.hwidLimitHint') || '0 = unlimited. Compatible with Happ x-hwid import headers (Remnawave-style).'}
          >
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="remark" label={t('users.remark') || 'Remark'}>
            <Input maxLength={255} />
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
          setSelectedRemoteIds([]);
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
        <p style={{ marginTop: 16, marginBottom: 8, opacity: 0.65 }}>
          {t('users.bindRemoteHint') || 'Remote cluster nodes (synced mirrors)'}
        </p>
        <Select
          mode="multiple"
          style={{ width: '100%' }}
          placeholder={t('users.selectRemoteNodes') || 'Select remote nodes'}
          optionFilterProp="label"
          value={selectedRemoteIds}
          onChange={(ids: number[]) => setSelectedRemoteIds(ids)}
          options={(remoteMirrors || []).filter((m) => m.enabled).map((m) => ({
            value: m.id,
            label: `${m.remote_server_name || m.remote_server_id} / ${m.name} (${m.protocol || '?'})`,
          }))}
        />
      </Modal>

    </div>
  );
};

export default Users;
