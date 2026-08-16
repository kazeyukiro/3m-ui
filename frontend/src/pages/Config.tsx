import React, { useEffect, useState } from 'react';
import { Card, Table, Button, Space, Modal, Form, Input, Select, message, Popconfirm, Tabs } from 'antd';
import { PlusOutlined, DeleteOutlined, EditOutlined, DownloadOutlined, CheckOutlined, FileTextOutlined } from '@ant-design/icons';
import Editor from '@monaco-editor/react';
import {
  fetchProxies, createProxy, updateProxy, deleteProxy,
  fetchConfigYAML, generateConfig, validateConfigYAML,
  ProxyEntry,
} from '../api/config';
import { useI18n } from '../i18n';
import { useThemeStore } from '../stores/themeStore';

const { TabPane } = Tabs;

const PROXY_TYPES = ['shadowsocks', 'vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'snell'];

const ConfigPage: React.FC = () => {
  const { t } = useI18n();
  const { isDark } = useThemeStore();
  const [proxies, setProxies] = useState<ProxyEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [form] = Form.useForm();
  const [yaml, setYaml] = useState('');
  const [activeTab, setActiveTab] = useState('visual');

  const load = async () => {
    setLoading(true);
    try {
      const [p, y] = await Promise.all([fetchProxies(), fetchConfigYAML()]);
      setProxies(p);
      setYaml(y.config);
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const onSubmit = async (values: ProxyEntry) => {
    try {
      if (editingIndex !== null) {
        await updateProxy(editingIndex, values);
        message.success(t('config.editProxy') + ' ' + t('common.success'));
      } else {
        await createProxy(values);
        message.success(t('config.addProxy') + ' ' + t('common.success'));
      }
      setModalOpen(false); setEditingIndex(null); form.resetFields(); load();
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    }
  };

  const onDelete = async (index: number) => {
    try { await deleteProxy(index); message.success(t('config.deleteProxy') + ' ' + t('common.success')); load(); }
    catch (e: any) { message.error(e.message || t('common.error')); }
  };

  const handleGenerate = async () => {
    try { await generateConfig(); message.success(t('config.generateSuccess')); const y = await fetchConfigYAML(); setYaml(y.config); }
    catch (e: any) { message.error(e.message || t('common.error')); }
  };

  const handleValidate = async () => {
    try {
      const res = await validateConfigYAML(yaml);
      if (res.valid) message.success(t('config.validateSuccess'));
      else message.error(res.error || t('config.validateFailed'));
    } catch (e: any) { message.error(e.message || t('common.error')); }
  };

  const columns = [
    { title: t('config.proxyName'), dataIndex: 'name', key: 'name' },
    { title: t('config.proxyType'), dataIndex: 'type', key: 'type' },
    { title: t('config.proxyServer'), dataIndex: 'server', key: 'server' },
    { title: t('config.proxyPort'), dataIndex: 'port', key: 'port' },
    {
      title: t('common.actions'), key: 'actions',
      render: (_: any, record: ProxyEntry, index: number) => (
        <Space>
          <Button icon={<EditOutlined />} onClick={() => { setEditingIndex(index); form.setFieldsValue(record); setModalOpen(true); }} />
          <Popconfirm title={t('config.deleteConfirm')} onConfirm={() => onDelete(index)}><Button icon={<DeleteOutlined />} danger /></Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <h2>{t('config.title')}</h2>
      <Tabs activeKey={activeTab} onChange={setActiveTab}>
        <TabPane tab={t('config.visual')} key="visual">
          <Card title={t('config.proxies')} extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditingIndex(null); form.resetFields(); setModalOpen(true); }}>{t('config.addProxy')}</Button>}>
            <Table dataSource={proxies} columns={columns} rowKey="name" loading={loading} pagination={false} scroll={{ x: 640 }} size="middle" />
          </Card>
          <Card style={{ marginTop: 16 }} title={t('config.yamlPreview')}>
            <Editor height={300} language="yaml" value={yaml} onChange={(v) => setYaml(v || '')} theme={isDark ? 'vs-dark' : 'light'} options={{ readOnly: false, minimap: { enabled: false } }} />
            <Space style={{ marginTop: 12 }}>
              <Button icon={<CheckOutlined />} onClick={handleValidate}>{t('config.validate')}</Button>
              <Button type="primary" icon={<FileTextOutlined />} onClick={handleGenerate}>{t('config.generate')}</Button>
              <Button icon={<DownloadOutlined />} onClick={() => { const blob = new Blob([yaml], { type: 'text/yaml' }); const url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href = url; a.download = 'config.yaml'; a.click(); URL.revokeObjectURL(url); }}>{t('common.download')}</Button>
            </Space>
          </Card>
        </TabPane>
        <TabPane tab={t('config.yaml')} key="yaml">
          <Editor height={600} language="yaml" value={yaml} onChange={(v) => setYaml(v || '')} theme={isDark ? 'vs-dark' : 'light'} options={{ minimap: { enabled: false } }} />
          <Space style={{ marginTop: 12 }}>
            <Button icon={<CheckOutlined />} onClick={handleValidate}>{t('config.validate')}</Button>
            <Button type="primary" icon={<FileTextOutlined />} onClick={handleGenerate}>{t('config.generate')}</Button>
          </Space>
        </TabPane>
      </Tabs>
      <Modal open={modalOpen} title={editingIndex !== null ? t('config.editProxy') : t('config.addProxy')} onCancel={() => { setModalOpen(false); setEditingIndex(null); form.resetFields(); }} onOk={() => form.submit()} destroyOnClose>
        <Form form={form} layout="vertical" onFinish={onSubmit}>
          <Form.Item name="name" label={t('config.proxyName')} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="type" label={t('config.proxyType')} rules={[{ required: true }]}>
            <Select>{PROXY_TYPES.map((t) => <Select.Option key={t} value={t}>{t}</Select.Option>)}</Select>
          </Form.Item>
          <Form.Item name="server" label={t('config.proxyServer')} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="port" label={t('config.proxyPort')} rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="password" label={t('config.proxyPassword')}><Input.Password /></Form.Item>
          <Form.Item name="uuid" label={t('config.proxyUUID')}><Input /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ConfigPage;
