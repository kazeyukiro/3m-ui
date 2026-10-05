import { useSearchParams } from 'react-router-dom';
import React, { useEffect, useState } from 'react';
import { Card, Table, Button, Space, Modal, Form, Input, Select, message, Popconfirm, Tabs, Row, Col, Switch, Alert } from 'antd';
import { IconAddGeneric, IconDelete, IconEdit, IconDownload, IconCheck, IconFile } from '../icons';
import {
  fetchProxies, createProxy, updateProxy, deleteProxy,
  fetchConfigYAML, generateConfig, validateConfigYAML, applyConfigYAML, rollbackConfig,
  fetchVisualConfig, saveVisualConfig,
  ProxyEntry, VisualConfig,
} from '../api/config';
import { useI18n } from '../i18n';
import useIsMobile from '../hooks/useIsMobile';
import PageHeader from '../components/PageHeader';

const { TabPane } = Tabs;
const { TextArea } = Input;

const PROXY_TYPES = ['shadowsocks', 'vmess', 'vless', 'trojan', 'hysteria2', 'tuic', 'snell'];

const ConfigPage: React.FC = () => {
  const { t } = useI18n();
  const isMobile = useIsMobile();
  const [proxies, setProxies] = useState<ProxyEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [form] = Form.useForm();
  const [generalForm] = Form.useForm();
  const [generalCfg, setGeneralCfg] = useState<VisualConfig | null>(null);
  const [genSaving, setGenSaving] = useState(false);
  const [yaml, setYaml] = useState('');
  const [searchParams, setSearchParams] = useSearchParams();
  const tabFromUrl = searchParams.get('tab');
  const [activeTab, setActiveTab] = useState(tabFromUrl === 'yaml' ? 'yaml' : 'visual');
  useEffect(() => {
    const tab = searchParams.get('tab');
    if (tab === 'yaml' || tab === 'visual') setActiveTab(tab);
  }, [searchParams]);
  const selectTab = (key: string) => {
    setActiveTab(key);
    if (key === 'visual') {
      const next = new URLSearchParams(searchParams);
      next.delete('tab');
      setSearchParams(next, { replace: true });
    } else {
      setSearchParams({ tab: key }, { replace: true });
    }
  };
  const [yamlLoading, setYamlLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    setYamlLoading(true);
    try {
      const [p, y] = await Promise.all([fetchProxies(), fetchConfigYAML()]);
      setProxies(p || []);
      setYaml(typeof y?.config === 'string' ? y.config : '');
      // General kernel settings (mode / allow-lan / ipv6 / inbound-tfo / mptcp)
      // live on the same VisualConfig that drives the serving Mihomo process.
      // Load failure here is non-fatal: proxies still work without the card.
      try {
        const vc = await fetchVisualConfig();
        setGeneralCfg(vc ? { ...vc } : null);
        generalForm.setFieldsValue({
          logLevel: vc?.logLevel ?? 'info',
          allowLan: !!vc?.allowLan,
          ipv6: !!vc?.ipv6,
          inboundTfo: !!vc?.inboundTfo,
          inboundMptcp: !!vc?.inboundMptcp,
        });
      } catch (e: any) {
        console.error('failed to load visual config', e);
      }
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setLoading(false);
      setYamlLoading(false);
    }
  };

  const onSaveGeneral = async (values: Record<string, any>) => {
    if (!generalCfg) return;
    setGenSaving(true);
    try {
      const payload: VisualConfig = {
        ...generalCfg,
        // mode is not editable here — keep whatever was already stored
        logLevel: values.logLevel,
        allowLan: !!values.allowLan,
        ipv6: !!values.ipv6,
        inboundTfo: !!values.inboundTfo,
        inboundMptcp: !!values.inboundMptcp,
        proxies: generalCfg.proxies ?? [],
      };
      await saveVisualConfig(payload);
      setGeneralCfg(payload);
      message.success(t('config.generalSaved') || 'General settings saved');
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setGenSaving(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const onSubmit = async (values: any) => {
    try {
      const { tfo, mptcp, ...rest } = values;
      // Client proxy keys per wiki.metacubex.one/config/proxies — only emit when enabled.
      const payload: ProxyEntry = { ...rest };
      if (tfo === true) payload.tfo = true;
      if (mptcp === true) payload.mptcp = true;
      // Never send server-only dial flags on a client proxy object.
      delete (payload as any)['inbound-tfo'];
      delete (payload as any)['inbound-mptcp'];
      delete (payload as any).inboundTfo;
      delete (payload as any).inboundMptcp;
      if (editingIndex !== null) {
        await updateProxy(editingIndex, payload);
        message.success(t('config.editProxy') + ' ' + t('common.success'));
      } else {
        await createProxy(payload);
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
      const res = await generateConfig();
      if (typeof res?.config === 'string' && res.config) {
        setYaml(res.config);
      } else {
        const y = await fetchConfigYAML();
        setYaml(typeof y?.config === 'string' ? y.config : '');
      }
      message.success(t('config.generateSuccess') || 'Generated (not applied)');
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setYamlLoading(false);
    }
  };

  const handleApply = async () => {
    setYamlLoading(true);
    try {
      await applyConfigYAML(yaml || undefined);
      message.success(t('config.applySuccess') || 'Applied');
      const y = await fetchConfigYAML();
      setYaml(typeof y?.config === 'string' ? y.config : yaml);
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setYamlLoading(false);
    }
  };

  const handleRollback = async () => {
    setYamlLoading(true);
    try {
      await rollbackConfig();
      message.success(t('config.rollbackSuccess') || 'Rolled back');
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
            icon={<IconEdit />}
            onClick={() => {
              setEditingIndex(index);
              const record = proxies[index] as any;
      form.setFieldsValue({
        ...record,
        tfo: !!record.tfo,
        mptcp: !!record.mptcp,
      });
              setModalOpen(true);
            }}
          />
          <Popconfirm title={t('common.confirm')} onConfirm={() => onDelete(index)}>
            <Button size="small" danger icon={<IconDelete />} />
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
      <PageHeader title={t('config.title')} subtitle={t('config.subtitle')} />
      <Tabs activeKey={activeTab} onChange={selectTab}>
        <TabPane tab={t('config.visual') || 'Visual'} key="visual">
          <Card title={t('config.general') || 'General'} style={{ marginBottom: 16 }}>
            <Form form={generalForm} layout="vertical" onFinish={onSaveGeneral}>
              <Row gutter={16}>
                <Col xs={24} sm={12} md={8}>
                </Col>
                <Col xs={24} sm={12} md={8}>
                  <Form.Item name="logLevel" label={t('config.logLevel') || 'Log level'}>
                    <Select
                      options={[
                        { value: 'info', label: 'info' },
                        { value: 'warning', label: 'warning' },
                        { value: 'error', label: 'error' },
                        { value: 'debug', label: 'debug' },
                        { value: 'silent', label: 'silent' },
                      ]}
                    />
                  </Form.Item>
                </Col>
              </Row>
              <Space size="large" wrap>
                <Form.Item name="allowLan" label={t('config.allowLan') || 'Allow LAN'} valuePropName="checked">
                  <Switch disabled={!generalCfg} />
                </Form.Item>
                <Form.Item name="ipv6" label={t('config.ipv6') || 'IPv6'} valuePropName="checked">
                  <Switch disabled={!generalCfg} />
                </Form.Item>
                <Form.Item
                  name="inboundTfo"
                  label={t('config.inboundTfo') || 'Inbound TCP Fast Open'}
                  valuePropName="checked"
                  tooltip={t('config.inboundTfoHint')}
                >
                  <Switch disabled={!generalCfg} />
                </Form.Item>
                <Form.Item name="inboundMptcp" label={t('config.inboundMptcp') || 'Inbound MPTCP'} valuePropName="checked">
                  <Switch disabled={!generalCfg} />
                </Form.Item>
              </Space>
              <div>
                <Button type="primary" htmlType="submit" loading={genSaving} disabled={!generalCfg}>
                  {t('config.saveGeneral') || 'Save general settings'}
                </Button>
              </div>
            </Form>
          </Card>
          <Card
            title={t('config.proxies') || 'Proxies'}
            extra={
              <Button
                type="primary"
                icon={<IconAddGeneric />}
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
            <Table size={isMobile ? "small" : "middle"} rowKey={(_, i) => String(i)} loading={loading} dataSource={proxies} columns={columns} pagination={false} />
          </Card>
          <Card style={{ marginTop: 16 }} title={t('config.yamlPreview') || 'YAML preview'} loading={yamlLoading}>
            {yamlEditor}
            <Space style={{ marginTop: 12 }} wrap>
              <Button type="default" icon={<IconFile />} loading={yamlLoading} onClick={handleGenerate}>
                {t('config.generate') || 'Generate'}
              </Button>
              <Button icon={<IconCheck />} onClick={handleValidate}>
                {t('config.validate') || 'Validate'}
              </Button>
              <Button type="primary" loading={yamlLoading} onClick={handleApply}>
                {t('config.apply') || 'Apply'}
              </Button>
              <Button danger loading={yamlLoading} onClick={handleRollback}>
                {t('config.rollback') || 'Rollback'}
              </Button>
              <Button
                icon={<IconDownload />}
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
            <Button type="default" icon={<IconFile />} loading={yamlLoading} onClick={handleGenerate}>
              {t('config.generate') || 'Generate'}
            </Button>
            <Button icon={<IconCheck />} onClick={handleValidate}>
              {t('config.validate') || 'Validate'}
            </Button>
            <Button type="primary" loading={yamlLoading} onClick={handleApply}>
              {t('config.apply') || 'Apply'}
            </Button>
            <Button danger loading={yamlLoading} onClick={handleRollback}>
              {t('config.rollback') || 'Rollback'}
            </Button>
            <Button
              icon={<IconDownload />}
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
        destroyOnClose width={isMobile ? '100%' : 640} style={isMobile ? { top: 8 } : undefined}
        styles={isMobile ? { body: { maxHeight: '78vh', overflowY: 'auto', paddingBottom: 8 } } : undefined}
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
          <Form.Item
            name="tfo"
            label={t('config.proxyTfo')}
            valuePropName="checked"
            tooltip={t('config.proxyTcpOnly')}
          >
            <Switch />
          </Form.Item>
          <Form.Item
            name="mptcp"
            label={t('config.proxyMptcp')}
            valuePropName="checked"
            tooltip={t('config.proxyTcpOnly')}
          >
            <Switch />
          </Form.Item>
          <Alert
            type="info"
            showIcon
            className="proxy-tcp-only-hint"
            message={t('config.proxyTcpOnly')}
          />
        </Form>
      </Modal>
    </div>
  );
};

export default ConfigPage;
