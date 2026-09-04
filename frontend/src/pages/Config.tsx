import React, { useEffect, useState } from 'react';
import { Card, Table, Button, Space, Modal, Form, Input, Select, message, Popconfirm, Tabs } from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined, DownloadOutlined, CheckOutlined, FileTextOutlined } from '@ant-design/icons';
import {
  fetchProxies, createProxy, updateProxy, deleteProxy,
  fetchConfigYAML, generateConfig, validateConfigYAML,
  ProxyEntry,
} from '../api/config';
import { useI18n } from '../i18n';

const { TabPane } = Tabs;
const { TextArea } = Input;

const PROXY_TYPES = ['shadowsocks', 'vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'snell'];

const ConfigPage: React.FC = () => {
  const { t } = useI18n();
  const [proxies, setProxies] = useState<ProxyEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [form] = Form.useForm();
  const [yaml, setYaml] = useState('');
  const [activeTab, setActiveTab] = useState('visual');
  const [yamlLoading, setYamlLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    setYamlLoading(true);
    try {
      const [p, y] = await Promise.all([fetchProxies(), fetchConfigYAML()]);
      setProxies(p || []);
      setYaml(typeof y?.config === 'string' ? y.config : '');
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setLoading(false);
      setYamlLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const onSubmit = async (values: ProxyEntry) => {
    try {
      if (editingIndex !== null) {
        await updateProxy(editingIndex, values);
        message.success(t('config.editProxy') + ' ' + t('common.success'));
      } else {
        await createProxy(values);
        message.success(t('config.addProxy') + ' ' + t('common.success'));
      }
      setModalOpen(false);
      setEditingIndex(null);
      form.resetFields();
      load();
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    }
  };

  const onDelete = async (index: number) => {
    try {
      await deleteProxy(index);
      message.success(t('config.deleteProxy') + ' ' + t('common.success'));
      load();
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    }
  };

  const handleGenerate = async () => {
    setYamlLoading(true);
    try {
      await generateConfig();
      message.success(t('config.generateSuccess'));
      const y = await fetchConfigYAML();
      setYaml(typeof y?.config === 'string' ? y.config : '');
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setYamlLoading(false);
    }
  };

  const handleValidate = async () => {
    try {
      const res = await validateConfigYAML(yaml);
      if (res.valid) message.success(t('config.validateOk') || 'Valid');
      else message.error(res.error || t('config.validateFail') || 'Invalid');
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    }
  };

  const columns = [
    { title: t('config.proxyName'), dataIndex: 'name', key: 'name' },
    { title: t('config.proxyType'), dataIndex: 'type', key: 'type' },
    { title: t('config.proxyServer'), dataIndex: 'server', key: 'server' },
    { title: t('config.proxyPort'), dataIndex: 'port', key: 'port' },
    {
      title: t('common.actions'),
      key: 'actions',
      render: (_: unknown, __: ProxyEntry, index: number) => (
        <Space>
          <Button
            size="small"
            icon={<EditOutlined />}
            onClick={() => {
              setEditingIndex(index);
              form.setFieldsValue(proxies[index]);
              setModalOpen(true);
            }}
          />
          <Popconfirm title={t('common.confirm')} onConfirm={() => onDelete(index)}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const yamlEditor = (
    <TextArea
      value={yaml}
      onChange={(e) => setYaml(e.target.value)}
      placeholder={yamlLoading ? (t('common.loading') || 'Loading…') : 'proxies:\n  - name: ...'}
      disabled={yamlLoading}
      style={{
        width: '100%',
        minHeight: activeTab === 'yaml' ? 560 : 300,
        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
        fontSize: 13,
        lineHeight: 1.45,
      }}
    />
  );

  return (
    <div>
      <h2>{t('config.title') || 'Config engine'}</h2>
      <Tabs activeKey={activeTab} onChange={setActiveTab}>
        <TabPane tab={t('config.visual') || 'Visual'} key="visual">
          <Card
            title={t('config.proxies') || 'Proxies'}
            extra={
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => {
                  setEditingIndex(null);
                  form.resetFields();
                  setModalOpen(true);
                }}
              >
                {t('config.addProxy')}
              </Button>
            }
          >
            <Table rowKey={(_, i) => String(i)} loading={loading} dataSource={proxies} columns={columns} pagination={false} />
          </Card>
          <Card style={{ marginTop: 16 }} title={t('config.yamlPreview') || 'YAML preview'} loading={yamlLoading}>
            {yamlEditor}
            <Space style={{ marginTop: 12 }} wrap>
              <Button type="primary" icon={<FileTextOutlined />} loading={yamlLoading} onClick={handleGenerate}>
                {t('config.generate')}
              </Button>
              <Button icon={<CheckOutlined />} onClick={handleValidate}>
                {t('config.validate') || 'Validate'}
              </Button>
              <Button
                icon={<DownloadOutlined />}
                onClick={() => {
                  const blob = new Blob([yaml], { type: 'text/yaml' });
                  const url = URL.createObjectURL(blob);
                  const a = document.createElement('a');
                  a.href = url;
                  a.download = 'config.yaml';
                  a.click();
                  URL.revokeObjectURL(url);
                }}
              >
                {t('common.download')}
              </Button>
            </Space>
          </Card>
        </TabPane>
        <TabPane tab={t('config.yaml') || 'YAML'} key="yaml">
          <Card loading={yamlLoading}>{yamlEditor}</Card>
          <Space style={{ marginTop: 12 }} wrap>
            <Button type="primary" icon={<FileTextOutlined />} loading={yamlLoading} onClick={handleGenerate}>
              {t('config.generate')}
            </Button>
            <Button icon={<CheckOutlined />} onClick={handleValidate}>
              {t('config.validate') || 'Validate'}
            </Button>
            <Button
              icon={<DownloadOutlined />}
              onClick={() => {
                const blob = new Blob([yaml], { type: 'text/yaml' });
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = 'config.yaml';
                a.click();
                URL.revokeObjectURL(url);
              }}
            >
              {t('common.download')}
            </Button>
          </Space>
        </TabPane>
      </Tabs>

      <Modal
        open={modalOpen}
        title={editingIndex !== null ? t('config.editProxy') : t('config.addProxy')}
        onCancel={() => {
          setModalOpen(false);
          setEditingIndex(null);
          form.resetFields();
        }}
        onOk={() => form.submit()}
        destroyOnClose
      >
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item name="name" label={t('config.proxyName')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="type" label={t('config.proxyType')} rules={[{ required: true }]}>
            <Select>
              {PROXY_TYPES.map((tp) => (
                <Select.Option key={tp} value={tp}>
                  {tp}
                </Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="server" label={t('config.proxyServer')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="port" label={t('config.proxyPort')} rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="password" label={t('config.proxyPassword')}>
            <Input.Password />
          </Form.Item>
          <Form.Item name="uuid" label={t('config.proxyUUID')}>
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ConfigPage;
