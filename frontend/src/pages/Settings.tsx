import React, { useEffect, useMemo, useState } from 'react';
import {
  Card,
  Button,
  Space,
  Typography,
  Tag,
  message,
  Modal,
  Upload,
  Form,
  Input,
  Switch,
  Select,
  InputNumber,
  Alert,
  Layout,
  Menu,
} from 'antd';
import { useNavigate } from 'react-router-dom';
import { useI18n } from '../i18n';
import { copyText } from '../utils/clipboard';
import { useThemeStore } from '../stores/themeStore';
import {
  LockOutlined,
  GlobalOutlined,
  BgColorsOutlined,
  InfoCircleOutlined,
  CloudDownloadOutlined,
  CloudUploadOutlined,
  ApiOutlined,
  SettingOutlined,
  SafetyCertificateOutlined,
  BellOutlined,
  LinkOutlined,
  ClusterOutlined,
  FileTextOutlined,
  DashboardOutlined,
} from '@ant-design/icons';
import { downloadBackup, restoreDatabase, openApiUrl } from '../api/system';
import { fetchTelegramSettings, saveTelegramSettings, testTelegram, setTelegramCommands, TelegramSettings } from '../api/telegram';
import client from '../api/client';

const { Text, Title } = Typography;
const { Sider, Content } = Layout;

type SectionKey =
  | 'panel'
  | 'access'
  | 'telegram'
  | 'security'
  | 'subscription'
  | 'ssl'
  | 'network'
  | 'traffic'
  | 'about';

const Settings: React.FC = () => {
  const [section, setSection] = useState<SectionKey>('panel');
  const [panelServer, setPanelServer] = useState<{
    port?: number;
    listen?: string;
    public_url?: string;
    config_path?: string;
    hint?: string;
  }>({});
  const [panelForm] = Form.useForm();
  const { t, locale, setLocale } = useI18n();
  const { mode, setMode } = useThemeStore();
  const navigate = useNavigate();
  const [tgForm] = Form.useForm();
  const [accessForm] = Form.useForm();
  const [tplForm] = Form.useForm();
  const [tplOut, setTplOut] = useState('');
  const [acmeForm] = Form.useForm();
  const [acmeCmd, setAcmeCmd] = useState('');
  const [resetDay, setResetDay] = useState<number>(0);
  const [sslForm] = Form.useForm();
  const [subPageForm] = Form.useForm();
  const [sslStatus, setSslStatus] = useState<Record<string, unknown> | null>(null);


  useEffect(() => {
    client
      .get('/system/panel-server')
      .then((r) => {
        setPanelServer(r.data || {});
        panelForm.setFieldsValue({
          port: r.data?.port ?? 8080,
          listen: r.data?.listen || '',
          public_url: r.data?.public_url || '',
        });
      })
      .catch(() => {});

    client
      .get('/panel-settings')
      .then((r) => {
        const day = Number(r.data?.traffic_reset_day || 0);
        if (!Number.isNaN(day)) setResetDay(day);
        accessForm.setFieldsValue({
          public_host: r.data?.['access_profile.public_host'] || '',
          public_port: r.data?.['access_profile.public_port'] || '',
          sni: r.data?.['access_profile.sni'] || '',
          client_fingerprint: r.data?.['access_profile.client_fingerprint'] || 'chrome',
          alpn: r.data?.['access_profile.alpn'] || '',
        });
      })
      .catch((e: any) => {
        message.error(e.message || t('common.error'));
      });

    fetchTelegramSettings()
      .then((s: TelegramSettings) => {
        tgForm.setFieldsValue({
          ...s,
          chat_ids: (s.chat_ids || []).join(','),
          bot_token: s.bot_token || '',
          traffic_warn_pct: s.traffic_warn_pct ?? 80,
          expiry_warn_hours: s.expiry_warn_hours ?? 72,
          notify_on_traffic: s.notify_on_traffic ?? true,
          schedule: s.schedule || '@daily',
          language: s.language || 'zh',
          enabled_events: s.enabled_events
            ? s.enabled_events.split(',').map((x: string) => x.trim()).filter(Boolean)
            : ['login', 'cpu', 'crash'],
          expiry_warn_days: s.expiry_warn_days ?? 0,
          traffic_warn_gb: s.traffic_warn_gb ?? 0,
          attach_backup: s.attach_backup ?? false,
          proxy_url: s.proxy_url || '',
          api_server: s.api_server || '',
        });
      })
      .catch(() => {});

    client
      .get('/system/ssl')
      .then((r) => {
        sslForm.setFieldsValue(r.data || {});
      })
      .catch(() => {});
    client.get('/system/ssl/status').then((r) => setSslStatus(r.data)).catch(() => {});

    client
      .get('/system/subscription-page')
      .then((r) => {
        subPageForm.setFieldsValue(r.data || {});
      })
      .catch(() => {});
  }, []);

  const menuItems = useMemo(
    () => [
      {
        key: 'panel',
        icon: <DashboardOutlined />,
        label: t('settings.navPanel') || '面板 / 外观',
      },
      {
        key: 'access',
        icon: <LinkOutlined />,
        label: t('settings.navAccess') || '访问档案',
      },
      {
        key: 'telegram',
        icon: <BellOutlined />,
        label: t('settings.navTelegram') || 'Telegram',
      },
      {
        key: 'security',
        icon: <LockOutlined />,
        label: t('settings.navSecurity') || '安全与备份',
      },
      {
        key: 'subscription',
        icon: <FileTextOutlined />,
        label: t('settings.navSubscription') || '订阅页',
      },
      {
        key: 'ssl',
        icon: <SafetyCertificateOutlined />,
        label: t('settings.navSSL') || '证书 / SSL',
      },
      {
        key: 'network',
        icon: <ClusterOutlined />,
        label: t('settings.navNetwork') || '反代与 Geo',
      },
      {
        key: 'traffic',
        icon: <SettingOutlined />,
        label: t('settings.navTraffic') || '流量重置',
      },
      {
        key: 'about',
        icon: <InfoCircleOutlined />,
        label: t('settings.navAbout') || '关于',
      },
    ],
    [t, locale],
  );

  return (
    <div>
      <h2>{t('settings.title')}</h2>
      <p style={{ color: 'rgba(0,0,0,0.45)', marginBottom: 16 }}>{t('settings.subtitle')}</p>

      <Layout
        style={{
          background: 'transparent',
          minHeight: 480,
          gap: 16,
        }}
      >
        <Sider
          width={220}
          theme="light"
          style={{
            background: 'var(--ant-color-bg-container, #fff)',
            borderRadius: 8,
            padding: '8px 0',
            border: '1px solid var(--ant-color-border-secondary, #f0f0f0)',
          }}
          breakpoint="md"
          collapsedWidth={0}
        >
          <Menu
            mode="inline"
            selectedKeys={[section]}
            items={menuItems}
            onClick={({ key }) => setSection(key as SectionKey)}
            style={{ border: 'none' }}
          />
        </Sider>

        <Content style={{ minWidth: 0, flex: 1 }}>
          {section === 'panel' && (
            <Space direction="vertical" size={16} style={{ width: '100%' }}>
              <Card
                title={t('settings.panelServer') || 'Panel / NAT'}
                extra={
                  <span style={{ fontSize: 12, opacity: 0.65 }}>{panelServer.config_path || ''}</span>
                }
              >
                <Form
                  form={panelForm}
                  layout="vertical"
                  onFinish={async (values) => {
                    try {
                      const res = await client.put('/system/panel-server', {
                        port: Number(values.port),
                        listen: values.listen || '',
                        public_url: values.public_url || '',
                      });
                      const prevPort = panelServer.port;
                      const newPort = res.data?.port ?? Number(values.port);
                      setPanelServer((s) => ({ ...s, ...res.data }));
                      message.success(t('common.saved'));
                      if (prevPort && prevPort !== newPort) {
                        Modal.warning({
                          title: '端口已更改 — 需要重启面板',
                          content: res.data?.hint || `请执行: systemctl restart 3m-ui ，然后访问新端口 ${newPort}`,
                        });
                      }
                    } catch (e: any) {
                      message.error(e.message || t('common.error'));
                    }
                  }}
                >
                  <Form.Item
                    name="port"
                    label={t('settings.panelPort') || 'Panel port'}
                    rules={[{ required: true }]}
                    extra={
                      t('settings.panelPortHint') ||
                      'NAT: map this host port to the WAN. Restart 3m-ui after change.'
                    }
                  >
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </Form.Item>
                  <Form.Item
                    name="listen"
                    label={t('settings.panelListen') || 'Listen address'}
                    extra={
                      t('settings.panelListenHint') ||
                      'Empty = all interfaces (IPv4/IPv6). Use 127.0.0.1 for reverse-proxy only.'
                    }
                  >
                    <Input placeholder="0.0.0.0 / :: / 127.0.0.1" />
                  </Form.Item>
                  <Form.Item
                    name="public_url"
                    label={t('settings.panelPublicURL') || 'Public panel URL'}
                    extra={
                      t('settings.panelPublicURLHint') ||
                      'Used in subscription links behind NAT, e.g. https://panel.example.com:8443'
                    }
                  >
                    <Input placeholder="https://example.com:8443" />
                  </Form.Item>
                  <Button type="primary" htmlType="submit">
                    {t('common.save')}
                  </Button>
                </Form>
              </Card>

              <Card title={<><GlobalOutlined /> {t('settings.language')}</>}>
                <Select
                  value={locale}
                  style={{ width: 220 }}
                  onChange={(v) => setLocale(v)}
                  options={[
                    { value: 'zh-CN', label: '简体中文' },
                    { value: 'en', label: 'English' },
                  ]}
                />
              </Card>

              <Card title={<><BgColorsOutlined /> {t('settings.theme')}</>}>
                <Space wrap>
                  <Button type={mode === 'light' ? 'primary' : 'default'} onClick={() => setMode('light')}>
                    {t('settings.light')}
                  </Button>
                  <Button type={mode === 'dark' ? 'primary' : 'default'} onClick={() => setMode('dark')}>
                    {t('settings.dark')}
                  </Button>
                  <Button type={mode === 'system' ? 'primary' : 'default'} onClick={() => setMode('system')}>
                    {t('settings.system')}
                  </Button>
                </Space>
              </Card>
            </Space>
          )}

          {section === 'access' && (
            <Card title={t('settings.accessProfile') || 'Access profile'}>
              <Form
                form={accessForm}
                layout="vertical"
                onFinish={async (values) => {
                  try {
                    await client.put('/panel-settings', {
                      'access_profile.public_host': values.public_host || '',
                      'access_profile.public_port': values.public_port || '',
                      'access_profile.sni': values.sni || '',
                      'access_profile.client_fingerprint': values.client_fingerprint || '',
                      'access_profile.alpn': values.alpn || '',
                    });
                    message.success(t('common.saved'));
                  } catch (e: any) {
                    message.error(e.message || t('common.error'));
                  }
                }}
              >
                <Form.Item name="public_host" label={t('settings.publicHost')}>
                  <Input placeholder="example.com" />
                </Form.Item>
                <Form.Item name="public_port" label={t('settings.publicPort')}>
                  <Input placeholder="443" />
                </Form.Item>
                <Form.Item name="sni" label={t('listeners.sni')}>
                  <Input placeholder="www.example.com" />
                </Form.Item>
                <Form.Item name="client_fingerprint" label={t('settings.clientFingerprint')}>
                  <Select
                    options={[
                      { value: 'chrome', label: 'chrome' },
                      { value: 'firefox', label: 'firefox' },
                      { value: 'safari', label: 'safari' },
                      { value: 'ios', label: 'ios' },
                      { value: 'android', label: 'android' },
                      { value: 'edge', label: 'edge' },
                      { value: 'random', label: 'random' },
                    ]}
                  />
                </Form.Item>
                <Form.Item name="alpn" label={t('listeners.alpn')} tooltip={t('settings.alpnHint')}>
                  <Input placeholder="h2,http/1.1" />
                </Form.Item>
                <Button type="primary" htmlType="submit">
                  {t('common.save')}
                </Button>
              </Form>
            </Card>
          )}

          {section === 'telegram' && (
            <Card title={t('settings.telegram')}>
              <Form
                form={tgForm}
                layout="vertical"
                onFinish={async (values) => {
                  try {
                    const chat_ids = String(values.chat_ids || '')
                      .split(/[,;\s]+/)
                      .map((s: string) => s.trim())
                      .filter(Boolean);
                    const toInt = (v: unknown, fallback = 0) => {
                      if (typeof v === 'number' && Number.isFinite(v)) return Math.trunc(v);
                      if (v === '' || v == null) return fallback;
                      const n = parseInt(String(v), 10);
                      return Number.isFinite(n) ? n : fallback;
                    };
                    await saveTelegramSettings({
                      ...values,
                      chat_ids,
                      proxy_url: values.proxy_url || '',
                      keep_token: !values.bot_token,
                      enabled_events: Array.isArray(values.enabled_events)
                        ? values.enabled_events.join(',')
                        : values.enabled_events || '',
                      attach_backup: !!values.attach_backup,
                      cpu_warn_pct: toInt(values.cpu_warn_pct, 0),
                      traffic_warn_pct: toInt(values.traffic_warn_pct, 80),
                      expiry_warn_hours: toInt(values.expiry_warn_hours, 72),
                      expiry_warn_days: toInt(values.expiry_warn_days, 0),
                      traffic_warn_gb: toInt(values.traffic_warn_gb, 0),
                    });
                    message.success(t('settings.telegramSaved'));
                  } catch (e: any) {
                    message.error(e.message || t('common.error'));
                  }
                }}
              >
                <Form.Item name="enabled" label={t('common.enabled')} valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item name="bot_token" label={t('settings.botToken')}>
                  <Input.Password />
                </Form.Item>
                <Form.Item name="chat_ids" label={t('settings.chatIds')} tooltip={t('settings.chatIdsHint')}>
                  <Input placeholder="123456789, -100123..." />
                </Form.Item>
                <Form.Item name="notify_on_login" label={t('settings.notifyLogin') || 'Notify on panel login'} valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item name="notify_on_cpu" label={t('settings.notifyCPU') || 'Notify on high CPU'} valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item name="cpu_warn_pct" label={t('settings.cpuWarnPct') || 'CPU warn %'} initialValue={0}>
                  <InputNumber min={0} max={100} style={{ width: '100%' }} />
                </Form.Item>
                <Form.Item name="notify_on_block" label={t('settings.notifyBlock')} valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item name="notify_on_unblock" label={t('settings.notifyUnblock')} valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item name="notify_on_expiry" label={t('settings.notifyExpiry')} valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item name="notify_daily_digest" label={t('settings.notifyDailyDigest')} valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item name="notify_on_traffic" label={t('settings.notifyTraffic') || 'Traffic threshold warning'} valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item name="traffic_warn_pct" label={t('settings.trafficWarnPct') || 'Traffic warn %'}>
                  <InputNumber min={1} max={100} style={{ width: '100%' }} />
                </Form.Item>
                <Form.Item name="expiry_warn_hours" label={t('settings.expiryWarnHours') || 'Expiry warn (hours)'}>
                  <InputNumber min={1} max={720} style={{ width: '100%' }} />
                </Form.Item>
                <Form.Item name="enabled_events" label={t('settings.tgEvents') || 'Enabled events'}>
                  <Select
                    mode="multiple"
                    options={[
                      { value: 'login', label: 'login' },
                      { value: 'cpu', label: 'cpu' },
                      { value: 'crash', label: 'crash' },
                      { value: 'traffic', label: 'traffic' },
                      { value: 'expiry', label: 'expiry' },
                    ]}
                  />
                </Form.Item>
                <Form.Item name="language" label={t('settings.tgLanguage') || 'Bot language'}>
                  <Select
                    options={[
                      { value: 'zh', label: '中文' },
                      { value: 'en', label: 'English' },
                    ]}
                    allowClear
                  />
                </Form.Item>
                <Form.Item name="schedule" label={t('settings.tgSchedule') || 'Report schedule'} tooltip={t('settings.tgScheduleHint')}>
                  <Input placeholder="0 9 * * * / @daily" />
                </Form.Item>
                <Form.Item name="proxy_url" label={t('settings.tgProxy') || 'Proxy URL'}>
                  <Input placeholder="socks5://127.0.0.1:1080" />
                </Form.Item>
                <Form.Item name="api_server" label={t('settings.tgApiServer') || 'Telegram API server'}>
                  <Input placeholder="https://api.telegram.org" />
                </Form.Item>
                <Form.Item name="attach_backup" label={t('settings.tgAttachBackup') || 'Attach DB backup in report'} valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Space wrap>
                  <Button type="primary" htmlType="submit">
                    {t('common.save')}
                  </Button>
                  <Button
                    onClick={async () => {
                      try {
                        await testTelegram();
                        message.success(t('settings.telegramTestOk'));
                      } catch (e: any) {
                        message.error(e.message || t('common.error'));
                      }
                    }}
                  >
                    {t('settings.telegramTest')}
                  </Button>
                  <Button
                    onClick={async () => {
                      try {
                        await setTelegramCommands();
                        message.success(t('settings.tgCommandsSet') || 'Bot commands registered');
                      } catch (e: any) {
                        message.error(e.message || t('common.error'));
                      }
                    }}
                  >
                    {t('settings.tgSetCommands') || 'Set bot commands'}
                  </Button>
                </Space>
              </Form>
            </Card>
          )}

          {section === 'security' && (
            <Space direction="vertical" size={16} style={{ width: '100%' }}>
              <Card title={<><LockOutlined /> {t('settings.security')}</>}>
                <Button type="primary" onClick={() => navigate('/change-password')}>
                  {t('settings.changePassword')}
                </Button>
              </Card>
              <Card title={t('settings.backup') || 'Backup'}>
                <Space wrap>
                  <Button
                    icon={<CloudDownloadOutlined />}
                    onClick={async () => {
                      try {
                        await downloadBackup();
                        message.success(t('common.ok') || 'OK');
                      } catch (e: any) {
                        message.error(e.message || t('common.error'));
                      }
                    }}
                  >
                    {t('settings.downloadBackup') || 'Download backup'}
                  </Button>
                  <Upload
                    accept=".zip,.db"
                    showUploadList={false}
                    beforeUpload={async (file) => {
                      try {
                        await restoreDatabase(file);
                        message.success(t('settings.restoreDone') || 'Restored — restart panel');
                        Modal.warning({
                          title: t('settings.restoreRestart') || 'Restart required',
                          content:
                            t('settings.restoreRestartHint') ||
                            'Database restored. Restart 3m-ui service now or writes may be lost.',
                        });
                      } catch (e: any) {
                        message.error(e.message || t('common.error'));
                      }
                      return false;
                    }}
                  >
                    <Button icon={<CloudUploadOutlined />}>{t('settings.restoreBackup') || 'Restore'}</Button>
                  </Upload>
                </Space>
              </Card>
              <Card title={<><ApiOutlined /> {t('settings.apiDocs') || 'API'}</>}>
                <Button type="link" href={openApiUrl} target="_blank" rel="noreferrer">
                  {t('settings.openOpenAPI') || 'Open openapi.yaml'}
                </Button>
              </Card>
            </Space>
          )}

          {section === 'subscription' && (
            <Card title={t('settings.subPage') || 'Subscription page'}>
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message={t('settings.subscriptionFormatHint') || 'Subscription auto-format'}
                description={
                  t('settings.subscriptionFormatHintDesc') ||
                  'Clients are detected via User-Agent. Force format with ?target=clash|v2ray|singbox|html.'
                }
              />
              <Form
                form={subPageForm}
                layout="vertical"
                onFinish={async (values) => {
                  try {
                    await client.put('/system/subscription-page', {
                      theme_dir: values.theme_dir || '',
                      title: values.title || '',
                      support_url: values.support_url || '',
                      announce: values.announce || '',
                      web_page_url: values.web_page_url || '',
                      update_hours: values.update_hours ?? 12,
                      encrypt: !!values.encrypt,
                    });
                    message.success(t('common.saved'));
                  } catch (e: any) {
                    message.error(e.message || t('common.error'));
                  }
                }}
              >
                <Form.Item
                  name="theme_dir"
                  label={t('settings.subThemeDir') || 'Theme directory'}
                >
                  <Input placeholder="/var/lib/3m-ui/sub-theme" />
                </Form.Item>
                <Form.Item name="title" label={t('settings.subTitle') || 'Page title'}>
                  <Input />
                </Form.Item>
                <Form.Item name="support_url" label={t('settings.subSupportUrl') || 'Support URL'}>
                  <Input />
                </Form.Item>
                <Form.Item name="announce" label={t('settings.subAnnounce') || 'Announce'}>
                  <Input.TextArea rows={2} />
                </Form.Item>
                <Form.Item name="web_page_url" label={t('settings.subWebPage') || 'Web page URL'}>
                  <Input />
                </Form.Item>
                <Form.Item name="update_hours" label={t('settings.subUpdates') || 'Update interval (hours)'}>
                  <InputNumber min={1} max={168} style={{ width: '100%' }} />
                </Form.Item>
                <Form.Item
                  name="encrypt"
                  label={t('settings.subEncrypt') || 'Base64-encode URI list'}
                  valuePropName="checked"
                >
                  <Switch />
                </Form.Item>
                <Space wrap>
                  <Button type="primary" htmlType="submit">
                    {t('common.save')}
                  </Button>
                  <Button
                    onClick={async () => {
                      try {
                        const r = await client.get('/system/subscription-page/default-template', {
                          responseType: 'text',
                        });
                        const blob = new Blob([r.data], { type: 'text/plain' });
                        const a = document.createElement('a');
                        a.href = URL.createObjectURL(blob);
                        a.download = 'sub-template.html';
                        a.click();
                      } catch (e: any) {
                        message.error(e.message || t('common.error'));
                      }
                    }}
                  >
                    {t('settings.downloadDefaultTpl') || 'Download default template'}
                  </Button>
                </Space>
              </Form>
            </Card>
          )}

          {section === 'ssl' && (
            <Space direction="vertical" size={16} style={{ width: '100%' }}>
              <Card title={t('settings.panelSSL') || 'Panel SSL (ACME)'}>
                <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
                  {t('settings.panelSSLHint') ||
                    'Enable HTTPS via Let’s Encrypt or manual cert. Restart panel after save.'}
                </Text>
                {sslStatus && (
                  <Tag color={sslStatus.enabled ? 'green' : 'default'} style={{ marginBottom: 12 }}>
                    {sslStatus.enabled ? 'SSL on' : 'SSL off'}
                  </Tag>
                )}
                <Form
                  form={sslForm}
                  layout="vertical"
                  onFinish={async (values) => {
                    try {
                      await client.put('/system/ssl', {
                        enabled: !!values.enabled,
                        domain: values.domain || '',
                        email: values.email || '',
                        cache_dir: values.cache_dir || '/var/lib/3m-ui/acme',
                        cert_file: values.cert_file || '',
                        key_file: values.key_file || '',
                        listen_http: values.listen_http || ':80',
                        listen_tls: values.listen_tls || ':443',
                      });
                      message.success(t('settings.sslSaved') || 'SSL saved — restart panel');
                      const st = await client.get('/system/ssl/status');
                      setSslStatus(st.data || null);
                    } catch (e: any) {
                      message.error(e.message || t('common.error'));
                    }
                  }}
                >
                  <Form.Item name="enabled" label={t('common.enabled')} valuePropName="checked">
                    <Switch />
                  </Form.Item>
                  <Form.Item name="domain" label={t('settings.domain')}>
                    <Input placeholder="panel.example.com" />
                  </Form.Item>
                  <Form.Item name="email" label={t('settings.email')}>
                    <Input />
                  </Form.Item>
                  <Form.Item name="cache_dir" label={t('settings.acmeCacheDir') || 'ACME cache dir'}>
                    <Input placeholder="/var/lib/3m-ui/acme" />
                  </Form.Item>
                  <Form.Item name="listen_http" label="HTTP listen">
                    <Input placeholder=":80" />
                  </Form.Item>
                  <Form.Item name="listen_tls" label="TLS listen">
                    <Input placeholder=":443" />
                  </Form.Item>
                  <Form.Item name="cert_file" label={t('settings.manualCert') || 'Manual cert file'}>
                    <Input />
                  </Form.Item>
                  <Form.Item name="key_file" label={t('settings.manualKey') || 'Manual key file'}>
                    <Input />
                  </Form.Item>
                  <Button type="primary" htmlType="submit">
                    {t('common.save')}
                  </Button>
                </Form>
              </Card>

              <Card title={t('settings.certWizard') || 'Certificate wizard'}>
                <Form
                  form={acmeForm}
                  layout="vertical"
                  onFinish={async (values) => {
                    try {
                      const r = await client.post('/system/templates/acme', values);
                      setAcmeCmd(r.data?.command || r.data?.cmd || '');
                      message.success(t('settings.acmeGenerated'));
                    } catch (e: any) {
                      message.error(e.message || t('common.error'));
                    }
                  }}
                >
                  <Form.Item name="email" label={t('settings.email')} rules={[{ required: true }]}>
                    <Input />
                  </Form.Item>
                  <Form.Item name="domain" label={t('settings.domain')} rules={[{ required: true }]}>
                    <Input />
                  </Form.Item>
                  <Form.Item name="webroot" label={t('settings.webroot')}>
                    <Input placeholder="/var/www/html" />
                  </Form.Item>
                  <Button type="primary" htmlType="submit">
                    {t('settings.generateAcme')}
                  </Button>
                </Form>
                {acmeCmd && (
                  <div style={{ marginTop: 12 }}>
                    <Text type="secondary">{t('settings.acmeHint')}</Text>
                    <Input.TextArea style={{ marginTop: 8 }} rows={3} value={acmeCmd} readOnly />
                    <Button
                      style={{ marginTop: 8 }}
                      onClick={async () => {
                        const ok = await copyText(acmeCmd);
                        if (ok) message.success(t('common.copied'));
                        else message.error(t('common.copyFailed') || 'Copy failed');
                      }}
                    >
                      {t('common.copy')}
                    </Button>
                  </div>
                )}
              </Card>
            </Space>
          )}

          {section === 'network' && (
            <Space direction="vertical" size={16} style={{ width: '100%' }}>
              <Card title={t('settings.templates')}>
                <Form
                  form={tplForm}
                  layout="vertical"
                  initialValues={{ kind: 'nginx', upstream: '127.0.0.1:8080' }}
                  onFinish={async (values) => {
                    try {
                      const r = await client.post('/system/templates/reverse-proxy', values);
                      setTplOut(r.data.config || '');
                      message.success(t('settings.templateGenerated'));
                    } catch (e: any) {
                      message.error(e.message || t('common.error'));
                    }
                  }}
                >
                  <Form.Item name="kind" label={t('settings.proxyKind')}>
                    <Select
                      options={[
                        { value: 'nginx', label: 'Nginx' },
                        { value: 'caddy', label: 'Caddy' },
                      ]}
                    />
                  </Form.Item>
                  <Form.Item name="domain" label={t('settings.domain')} rules={[{ required: true }]}>
                    <Input placeholder="panel.example.com" />
                  </Form.Item>
                  <Form.Item name="upstream" label={t('settings.upstream')}>
                    <Input />
                  </Form.Item>
                  <Button type="primary" htmlType="submit">
                    {t('settings.generateTemplate')}
                  </Button>
                </Form>
                {tplOut && <Input.TextArea style={{ marginTop: 12 }} rows={12} value={tplOut} readOnly />}
              </Card>

              <Card title={t('settings.geofiles') || 'GeoIP / GeoSite'}>
                <Text type="secondary" style={{ display: 'block', marginBottom: 12 }}>
                  {t('settings.geofilesHint') ||
                    'Download latest MetaCubeX GeoIP/GeoSite into Mihomo data directory.'}
                </Text>
                <Button
                  type="primary"
                  onClick={async () => {
                    try {
                      await client.post('/system/geofiles/update');
                      message.success(t('settings.geofilesDone') || 'Geo files updated');
                    } catch (e: any) {
                      message.error(e.message || t('common.error'));
                    }
                  }}
                >
                  {t('settings.updateGeofiles') || 'Update geo files'}
                </Button>
              </Card>
            </Space>
          )}

          {section === 'traffic' && (
            <Card title={t('settings.trafficReset')}>
              <Space wrap>
                <span>{t('settings.trafficResetDay')}</span>
                <InputNumber min={0} max={31} value={resetDay} onChange={(v) => setResetDay(Number(v || 0))} />
                <Button
                  type="primary"
                  onClick={async () => {
                    try {
                      await client.put('/panel-settings', { traffic_reset_day: String(resetDay) });
                      message.success(t('common.saved'));
                    } catch (e: any) {
                      message.error(e.message || t('common.error'));
                    }
                  }}
                >
                  {t('common.save')}
                </Button>
              </Space>
            </Card>
          )}

          {section === 'about' && (
            <Card title={<><InfoCircleOutlined /> {t('settings.about')}</>}>
              <Space direction="vertical">
                <Title level={4} style={{ margin: 0 }}>
                  3M-UI
                </Title>
                <Text type="secondary">{t('app.title')}</Text>
                <Tag color="blue">v1.0.0</Tag>
              </Space>
            </Card>
          )}
        </Content>
      </Layout>
    </div>
  );
};

export default Settings;
