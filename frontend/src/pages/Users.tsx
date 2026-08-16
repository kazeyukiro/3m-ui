import React, { useEffect, useState } from 'react';
import { Card, Table, Button, Space, Modal, Form, Input, Switch, message, Popconfirm, Select, Tag } from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined, LinkOutlined } from '@ant-design/icons';
import {
  fetchUsers,
  createUser,
  updateUser,
  deleteUser,
  fetchUserNodes,
  bindUserNodes,
  ProxyUser,
} from '../api/users';
import { fetchListeners, Listener } from '../api/nodes';
import { useI18n } from '../i18n';

const Users: React.FC = () => {
  const { t } = useI18n();
  const [data, setData] = useState<ProxyUser[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<ProxyUser | null>(null);
  const [form] = Form.useForm();

  // Bind nodes modal
  const [bindOpen, setBindOpen] = useState(false);
  const [bindUser, setBindUser] = useState<ProxyUser | null>(null);
  const [allNodes, setAllNodes] = useState<Listener[]>([]);
  const [selectedNodeIds, setSelectedNodeIds] = useState<number[]>([]);
  const [bindLoading, setBindLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const res = await fetchUsers();
      setData(res);
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
      if (editing) {
        await updateUser(editing.id, values);
        message.success(t('users.updated'));
      } else {
        await createUser(values);
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
    { title: t('users.username'), dataIndex: 'username', key: 'username' },
    {
      title: t('common.status'),
      dataIndex: 'enabled',
      key: 'enabled',
      render: (v: boolean) => (v ? t('common.enabled') : t('common.disabled')),
    },
    {
      title: t('common.actions'),
      key: 'actions',
      render: (_: any, record: ProxyUser) => (
        <Space wrap size={4}>
          <Button
            size="small"
            icon={<LinkOutlined />}
            onClick={() => openBind(record)}
            title={t('users.bindNodes')}
          >
            {t('users.bind')}
          </Button>
          <Button
            size="small"
            icon={<EditOutlined />}
            onClick={() => {
              setEditing(record);
              form.setFieldsValue(record);
              setModalOpen(true);
            }}
          />
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
              setModalOpen(true);
            }}
          >
            {t('users.create')}
          </Button>
        }
      >
        <Table
          scroll={{ x: 520 }}
          size="middle"
          dataSource={data}
          columns={columns}
          rowKey="id"
          loading={loading}
        />
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
          <Form.Item
            name="password"
            label={t('users.password')}
            rules={[{ required: !editing }]}
          >
            <Input.Password placeholder={editing ? t('common.empty') : ''} />
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
        <p style={{ marginBottom: 12, color: 'rgba(0,0,0,0.45)' }}>{t('users.bindHint')}</p>
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
          optionRender={(option) => {
            const node = allNodes.find((n) => n.id === option.value);
            return (
              <Space>
                <span>{option.label}</span>
                {node && !node.enabled && <Tag>{t('common.disabled')}</Tag>}
              </Space>
            );
          }}
        />
      </Modal>
    </div>
  );
};

export default Users;
