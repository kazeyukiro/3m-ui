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
  Segmented,
  Switch,
  Select,
  InputNumber,
  Table,
  Popconfirm,
  Alert,
  Layout,
  Menu,
  Tooltip,
  theme,
} from 'antd';
import { useNavigate } from 'react-router-dom';
import { useI18n, LOCALE_OPTIONS, type Locale } from '../i18n';
import PageHeader from '../components/PageHeader';
import useIsMobile from '../hooks/useIsMobile';
import { copyText } from '../utils/clipboard';
import { useThemeStore } from '../stores/themeStore';
import {
  IconLock,
  IconSettingsPanel,
  IconSettingsAccess,
  IconSettingsTelegram,
  IconSettingsSecurity,
  IconSettingsSubPage,
  IconSettingsSSL,
  IconSettingsProxy,
  IconSettingsTraffic,
  IconSettingsAbout,
  IconSettingsOps,
  IconCloudDown,
  IconCloudUp,
  IconTheme,
  IconGlobe,
  IconShield,
  IconApi,
  IconInfo,
  IconRestart,
  IconUpdate,
} from '../icons';;;
import {
  downloadBackup,
  restoreDatabase,
  openApiUrl,
  listLocalBackups,
  deleteLocalBackup,
  cleanupLocalBackups,
  restartPanel,
  checkUpdate,
  runUpdate,
  type LocalBackupItem,
  type UpdateInfo,
} from '../api/system';
import { fetchTelegramSettings, saveTelegramSettings, testTelegram, setTelegramCommands, TelegramSettings } from '../api/telegram';
import client from '../api/client';
import { setupTOTP, enableTOTP, disableTOTP, fetchMe, fetchGithubOAuthSettings, saveGithubOAuthSettings } from '../api/auth';
import QRCode from '../components/QRCode';

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
  | 'ops'
  | 'about';

const Settings: React.FC = () => {
  const [section, setSection] = useState<SectionKey>('panel');
  const [warpMode, setWarpMode] = useState<'wireguard' | 'masque'>('wireguard');
  const [warpYamlOpen, setWarpYamlOpen] = useState(false);
  const [warpYaml, setWarpYaml] = useState('');
  const [warpYamlTitle, setWarpYamlTitle] = useState('');
  const [panelServer, setPanelServer] = useState<{
    port?: number;
    listen?: string;
    public_url?: string;
    config_path?: string;
    hint?: string;
  }>({});
  const [panelForm] = Form.useForm();
  const { t, locale, setLocale } = useI18n();
  const [localBackups, setLocalBackups] = useState<LocalBackupItem[]>([]);
  const [backupTotalBytes, setBackupTotalBytes] = useState(0);
  const [backupDir, setBackupDir] = useState('');
  const [backupsLoading, setBackupsLoading] = useState(false);
  const [cleanupKeep, setCleanupKeep] = useState(3);
  const [cleanupDays, setCleanupDays] = useState(7);

  const formatBytes = (n: number) => {
    if (!n || n < 0) return '0 B';
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
    return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
  };

  const loadLocalBackups = async () => {
    setBackupsLoading(true);
    try {
      const data = await listLocalBackups();
      setLocalBackups(data.items || []);
      setBackupTotalBytes(data.total_bytes || 0);
      setBackupDir(data.dir || '');
    } catch (e: any) {
      message.error(e?.message || t('common.error'));
    } finally {
      setBackupsLoading(false);
    }
  };

  useEffect(() => {
    void loadLocalBackups();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const isMobile = useIsMobile();
  const [totpEnabled, setTotpEnabled] = useState(false);
  const [githubForm] = Form.useForm();
  const [githubCallback, setGithubCallback] = useState('');
  const [githubSaving, setGithubSaving] = useState(false);
  const [totpSecret, setTotpSecret] = useState('');
  const [totpUrl, setTotpUrl] = useState('');
  const [totpCode, setTotpCode] = useState('');
  const [totpPassword, setTotpPassword] = useState('');
  const [totpLoading, setTotpLoading] = useState(false);
  const [totpSetupOpen, setTotpSetupOpen] = useState(false);
  const { mode, setMode } = useThemeStore();
  const isDark = useThemeStore((s) => s.isDark);
  const { token } = theme.useToken();
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
    fetchMe()
      .then((me) => {
        if (me && typeof me.totp_enabled === 'boolean') {
          setTotpEnabled(!!me.totp_enabled);
        }
      })
      .catch(() => {});
  }, []);

  useEffect(() => {
    if (section !== 'security') return;
    fetchMe()
      .then((me) => {
        if (me && typeof me.totp_enabled === 'boolean') {
          setTotpEnabled(!!me.totp_enabled);
        }
      })
      .catch(() => {});
  }, [section]);

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
          language: s.language || 'zh-CN',
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
        icon: <IconSettingsPanel />,
        label: t('settings.navPanel') || '面板 / 外观',
      },
      {
        key: 'access',
        icon: <IconSettingsAccess />,
        label: t('settings.navAccess') || '访问档案',
      },
      {
        key: 'telegram',
        icon: <IconSettingsTelegram />,
        label: t('settings.navTelegram') || 'Telegram',
      },
      {
        key: 'security',
        icon: <IconSettingsSecurity />,
        label: t('settings.navSecurity') || '安全与备份',
      },
      {
        key: 'subscription',
        icon: <IconSettingsSubPage />,
        label: t('settings.navSubscription') || '订阅页',
      },
      {
        key: 'ssl',
        icon: <IconSettingsSSL />,
        label: t('settings.navSSL') || '证书 / SSL',
      },
      {
        key: 'network',
        icon: <IconSettingsProxy />,
        label: t('settings.navNetwork') || '反代与 Geo',
      },
      {
        key: 'traffic',
        icon: <IconSettingsTraffic />,
        label: t('settings.navTraffic') || '流量重置',
      },
      {
        key: 'ops',
        icon: <IconSettingsOps />,
        label: t('settings.navOps') || '系统操作',
      },
      {
        key: 'about',
        icon: <IconSettingsAbout />,
        label: t('settings.navAbout') || '关于',
      },
    ],
    [t, locale],
  );

  useEffect(() => {
    if (section !== 'security') return;
    fetchGithubOAuthSettings()
      .then((s) => {
        githubForm.setFieldsValue({
          enabled: !!s.enabled,
          client_id: s.client_id || '',
          client_secret: s.client_secret || '',
          allowed_logins: (s.allowed_logins || []).join(', '),
        });
      })
      .catch(() => {});
  }, [section, githubForm]);

  return (
    <div>
      <PageHeader title={t('settings.title')} subtitle={t('settings.subtitle') || undefined} />

      {isMobile && (
        <Select
          className="settings-section-select"
          value={section}
          onChange={(v) => setSection(v as SectionKey)}
          options={menuItems.map((it: any) => ({
            value: it.key,
            label: (
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
                {it.icon}
                {it.label}
              </span>
            ),
          }))}
          style={{ width: '100%', marginBottom: 12 }}
          size="large"
        />
      )}

      <Layout
        className="settings-layout"
        style={{
          background: 'transparent',
          minHeight: isMobile ? 0 : 480,
          gap: isMobile ? 10 : 16,
        }}
      >
        {!isMobile && (
        <Sider
          className="settings-sider"
          width={220}
          theme={isDark ? 'dark' : 'light'}
          style={{
            background: 'var(--ant-color-bg-container, #fff)',
            borderRadius: 8,
            padding: '8px 0',
            border: '1px solid var(--ant-color-border-secondary, #f0f0f0)',
          }}
        >
          <Menu
            className="settings-menu"
            mode="inline"
            selectedKeys={[section]}
            items={menuItems}
            onClick={({ key }) => setSection(key as SectionKey)}
            style={{ border: 'none' }}
          />
        </Sider>
        )}

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

              <Card title={<><IconGlobe /> {t('settings.language')}</>}>
                <Select
                  value={locale}
                  style={{ width: 280 }}
                  showSearch
                  optionFilterProp="label"
                  onChange={(v) => setLocale(v as Locale)}
                  options={LOCALE_OPTIONS.map((o) => ({ value: o.key, label: o.label }))}
                />
              </Card>

              <Card title={<><IconTheme /> {t('settings.theme')}</>}>
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
                <Form.Item name="public_host" label={t('settings.publicHost')} tooltip={t('settings.publicHostHint') || 'Public hostname/IP clients connect to (e.g. cdn.example.com). Defaults to bind address if empty.'}>
                  <Input placeholder="example.com" />
                </Form.Item>
                <Form.Item name="public_port" label={t('settings.publicPort')} tooltip={t('settings.publicPortHint') || 'Public port clients connect to (e.g. 443 for CDN). Defaults to panel port if empty.'}>
                  <Input placeholder="443" />
                </Form.Item>
                <Form.Item name="sni" label={t('listeners.sni')} tooltip={t('settings.accessSniHint') || 'SNI sent by client during TLS handshake. Must match the certificate. Leave empty to use public_host.'}>
                  <Input placeholder="www.example.com" />
                </Form.Item>
                <Form.Item name="client_fingerprint" label={t('settings.clientFingerprint')} tooltip={t('settings.clientFingerprintHint') || 'uTLS fingerprint for client TLS hello. "chrome" is recommended for stealth.'}>
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
                <Form.Item name="enabled" label={t('common.enabled')} valuePropName="checked" tooltip={t('settings.tgEnabledHint')}>
                  <Switch />
                </Form.Item>
                <Form.Item name="bot_token" label={t('settings.botToken')} tooltip={t('settings.botTokenHint') || 'Get from @BotFather. Format: 123456789:ABCdefGHIjklMNOpqrsTUVwxyz'}>
                  <Input.Password />
                </Form.Item>
                <Form.Item name="chat_ids" label={t('settings.chatIds')} tooltip={t('settings.chatIdsHint')}>
                  <Input placeholder="123456789, -100123..." />
                </Form.Item>
                <Form.Item name="notify_on_login" label={t('settings.notifyLogin') || 'Notify on panel login'} valuePropName="checked" tooltip={t('settings.notifyLoginHint')}>
                  <Switch />
                </Form.Item>
                <Form.Item name="notify_on_cpu" label={t('settings.notifyCPU') || 'Notify on high CPU'} valuePropName="checked" tooltip={t('settings.notifyCPUHint')}>
                  <Switch />
                </Form.Item>
                <Form.Item name="cpu_warn_pct" label={t('settings.cpuWarnPct') || 'CPU warn %'} tooltip={t('settings.cpuWarnPctHint') || '0 = disabled. Alert when panel CPU usage exceeds this percentage.'} initialValue={0}>
                  <InputNumber min={0} max={100} style={{ width: '100%' }} />
                </Form.Item>
                <Form.Item name="notify_on_block" label={t('settings.notifyBlock')} valuePropName="checked" tooltip={t('settings.notifyBlockHint')}>
                  <Switch />
                </Form.Item>
                <Form.Item name="notify_on_unblock" label={t('settings.notifyUnblock')} valuePropName="checked" tooltip={t('settings.notifyUnblockHint')}>
                  <Switch />
                </Form.Item>
                <Form.Item name="notify_on_expiry" label={t('settings.notifyExpiry')} valuePropName="checked" tooltip={t('settings.notifyExpiryHint')}>
                  <Switch />
                </Form.Item>
                <Form.Item name="notify_daily_digest" label={t('settings.notifyDailyDigest')} valuePropName="checked" tooltip={t('settings.notifyDailyDigestHint')}>
                  <Switch />
                </Form.Item>
                <Form.Item name="notify_on_traffic" label={t('settings.notifyTraffic') || 'Traffic threshold warning'} valuePropName="checked" tooltip={t('settings.notifyTrafficHint')}>
                  <Switch />
                </Form.Item>
                <Form.Item name="traffic_warn_pct" label={t('settings.trafficWarnPct') || 'Traffic warn %'} tooltip={t('settings.trafficWarnPctHint') || '0 = disabled. Alert when user traffic exceeds this percentage of their quota.'}>
                  <InputNumber min={1} max={100} style={{ width: '100%' }} />
                </Form.Item>
                <Form.Item name="expiry_warn_hours" label={t('settings.expiryWarnHours') || 'Expiry warn (hours)'} tooltip={t('settings.expiryWarnHoursHint') || 'Hours before user expiry to send a warning notification.'}>
                  <InputNumber min={1} max={720} style={{ width: '100%' }} />
                </Form.Item>
                <Form.Item name="enabled_events" label={t('settings.tgEvents') || 'Enabled events'} tooltip={t('settings.tgEventsHint')}>
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
                <Form.Item name="language" label={t('settings.tgLanguage') || 'Bot language'} tooltip={t('settings.tgLanguageHint')}>
                  <Select
                    showSearch
                    optionFilterProp="label"
                    options={[
                      { value: 'en', label: 'English' },
                      { value: 'zh-CN', label: '简体中文' },
                      { value: 'zh-TW', label: '繁體中文' },
                      { value: 'ja', label: '日本語' },
                      { value: 'ko', label: '한국어' },
                      { value: 'es', label: 'Español' },
                      { value: 'fr', label: 'Français' },
                      { value: 'de', label: 'Deutsch' },
                      { value: 'ru', label: 'Русский' },
                      { value: 'pt-BR', label: 'Português (Brasil)' },
                      { value: 'vi', label: 'Tiếng Việt' },
                      { value: 'id', label: 'Bahasa Indonesia' },
                      { value: 'th', label: 'ไทย' },
                      { value: 'tr', label: 'Türkçe' },
                      { value: 'ar', label: 'العربية' },
                      { value: 'hi', label: 'हिन्दी' },
                      { value: 'pl', label: 'Polski' },
                      { value: 'uk', label: 'Українська' },
                    ]}
                    allowClear
                  />
                </Form.Item>
                <Form.Item name="schedule" label={t('settings.tgSchedule') || 'Report schedule'} tooltip={t('settings.tgScheduleHint')}>
                  <Input placeholder="0 9 * * * / @daily" />
                </Form.Item>
                <Form.Item name="proxy_url" label={t('settings.tgProxy') || 'Proxy URL'} tooltip={t('settings.tgProxyHint') || 'SOCKS5/HTTP proxy for Telegram API if GitHub/Telegram is blocked. e.g. socks5://127.0.0.1:1080'}>
                  <Input placeholder="socks5://127.0.0.1:1080" />
                </Form.Item>
                <Form.Item name="api_server" label={t('settings.tgApiServer') || 'Telegram API server'} tooltip={t('settings.tgApiServerHint') || 'Custom Telegram API server (for Bot API instances). Leave empty for default api.telegram.org'}>
                  <Input placeholder="https://api.telegram.org" />
                </Form.Item>
                <Form.Item name="attach_backup" label={t('settings.tgAttachBackup') || 'Attach DB backup in report'} valuePropName="checked" tooltip={t('settings.tgAttachBackupHint')}>
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
              <Card title={<><IconLock /> {t('settings.security')}</>}>
                <Space direction="vertical" style={{ width: '100%' }} size="middle">
                  <Button type="primary" onClick={() => navigate('/change-password')}>
                    {t('settings.changePassword')}
                  </Button>
                  <div>
                    <Space direction="vertical" size="small" style={{ width: '100%' }}>
                      <Space wrap align="center">
                        <IconShield />
                        <Typography.Text strong>{t('settings.totp', 'Two-factor (TOTP)')}</Typography.Text>
                        {totpEnabled ? (
                          <Tag color="success">{t('settings.totpOn', 'Enabled')}</Tag>
                        ) : (
                          <Tag>{t('common.disabled', 'Disabled')}</Tag>
                        )}
                      </Space>
                      <Typography.Text type="secondary" style={{ display: 'block' }}>
                        {t(
                          'settings.totpDesc',
                          'Protect admin login with an authenticator app (Google Authenticator, Microsoft Authenticator, etc.).',
                        )}
                      </Typography.Text>
                      {totpEnabled ? (
                        <Space direction="vertical" size="middle" style={{ width: '100%', maxWidth: 420 }}>
                          <Alert
                            type="warning"
                            showIcon
                            message={t('settings.totpDisableHint', 'Enter password and a current code to disable 2FA.')}
                          />
                          <Input.Password
                            placeholder={t('login.password')}
                            value={totpPassword}
                            onChange={(e) => setTotpPassword(e.target.value)}
                            size="large"
                            autoComplete="current-password"
                          />
                          <Input
                            placeholder={t('login.totpLabel', 'Verification code')}
                            value={totpCode}
                            onChange={(e) => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 8))}
                            inputMode="numeric"
                            maxLength={8}
                            size="large"
                            style={{ letterSpacing: 4 }}
                            autoComplete="one-time-code"
                          />
                          <Button
                            danger
                            block
                            loading={totpLoading}
                            onClick={async () => {
                              if (!totpPassword || !totpCode) {
                                message.warning(t('settings.totpNeedBoth', 'Password and code are required'));
                                return;
                              }
                              setTotpLoading(true);
                              try {
                                await disableTOTP(totpPassword, totpCode);
                                setTotpEnabled(false);
                                setTotpPassword('');
                                setTotpCode('');
                                message.success(t('common.success'));
                              } catch (e: any) {
                                message.error(e.message || t('common.error'));
                              } finally {
                                setTotpLoading(false);
                              }
                            }}
                          >
                            {t('settings.totpDisable', 'Disable 2FA')}
                          </Button>
                        </Space>
                      ) : (
                        <Button
                          type="primary"
                          icon={<IconShield />}
                          onClick={async () => {
                            setTotpLoading(true);
                            try {
                              const r = await setupTOTP();
                              setTotpSecret(r.secret);
                              setTotpUrl(r.otpauth_url);
                              setTotpCode('');
                              setTotpSetupOpen(true);
                            } catch (e: any) {
                              message.error(e.message || t('common.error'));
                            } finally {
                              setTotpLoading(false);
                            }
                          }}
                          loading={totpLoading}
                        >
                          {t('settings.totpSetupBtn', 'Setup 2FA')}
                        </Button>
                      )}
                    </Space>
                    <Modal
                      title={t('settings.totpSetupBtn', 'Setup 2FA')}
                      open={totpSetupOpen}
                      onCancel={() => {
                        setTotpSetupOpen(false);
                        setTotpSecret('');
                        setTotpUrl('');
                        setTotpCode('');
                      }}
                      footer={null}
                      destroyOnClose
                      width={440}
                    >
                      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                        <Alert
                          type="info"
                          showIcon
                          message={t('settings.totpSetup', 'Scan with authenticator')}
                          description={t(
                            'settings.totpSetupSteps',
                            '1) Scan the QR code  2) Or enter the secret manually  3) Confirm with a 6-digit code',
                          )}
                        />
                        {totpUrl ? (
                          <div style={{ display: 'flex', justifyContent: 'center', padding: 8 }}>
                            <QRCode value={totpUrl} size={200} />
                          </div>
                        ) : null}
                        {totpSecret ? (
                          <div>
                            <Typography.Text type="secondary">{t('settings.totpSecret', 'Secret key')}</Typography.Text>
                            <Typography.Paragraph copyable code style={{ marginBottom: 0, wordBreak: 'break-all' }}>
                              {totpSecret}
                            </Typography.Paragraph>
                          </div>
                        ) : null}
                        <Input
                          placeholder={t('login.totpLabel', 'Verification code')}
                          value={totpCode}
                          onChange={(e) => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 8))}
                          inputMode="numeric"
                          maxLength={8}
                          size="large"
                          style={{ letterSpacing: 6, textAlign: 'center', fontSize: 18 }}
                          autoComplete="one-time-code"
                        />
                        <Button
                          type="primary"
                          block
                          size="large"
                          loading={totpLoading}
                          onClick={async () => {
                            if (!/^\d{6,8}$/.test(totpCode)) {
                              message.warning(t('login.totpFormat', 'Enter 6–8 digits'));
                              return;
                            }
                            setTotpLoading(true);
                            try {
                              await enableTOTP(totpCode);
                              setTotpEnabled(true);
                              setTotpSecret('');
                              setTotpUrl('');
                              setTotpCode('');
                              setTotpSetupOpen(false);
                              message.success(t('settings.totpEnabledOk', 'Two-factor authentication enabled'));
                            } catch (e: any) {
                              message.error(e.message || t('common.error'));
                            } finally {
                              setTotpLoading(false);
                            }
                          }}
                        >
                          {t('settings.totpEnable', 'Enable')}
                        </Button>
                      </Space>
                    </Modal>
                  </div>
                  <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
                    {t('settings.webPathHint', 'Panel path prefix (web_path) is set in /etc/3m-ui/config.yaml then restart. Example: web_path: "/secret" → open http://IP:8080/secret/')}
                  </Typography.Paragraph>
                </Space>
              </Card>
              <Card title={t('settings.githubOAuth') || 'GitHub OAuth'} style={{ marginBottom: 16 }}>
                <Typography.Paragraph type="secondary" style={{ marginBottom: 12 }}>
                  {t('settings.githubOAuthHint') ||
                    'Optional panel login via GitHub. Create an OAuth App on GitHub (Settings → Developer settings). Callback URL must match public_url + /api/v1/auth/oauth/github/callback. Allowed logins: GitHub usernames that may bind to local admin (comma-separated).'}
                </Typography.Paragraph>
                <Form
                  form={githubForm}
                  layout="vertical"
                  onFinish={async (values) => {
                    setGithubSaving(true);
                    try {
                      const allowed = String(values.allowed_logins || '')
                        .split(/[,\s]+/)
                        .map((x: string) => x.trim())
                        .filter(Boolean);
                      const res = await saveGithubOAuthSettings({
                        enabled: !!values.enabled,
                        client_id: (values.client_id || '').trim(),
                        client_secret: (values.client_secret || '').trim(),
                        allowed_logins: allowed,
                      });
                      if (res.callback_url) setGithubCallback(res.callback_url);
                      message.success(t('settings.githubOAuthSaved') || 'GitHub OAuth saved');
                    } catch (e: any) {
                      message.error(e?.message || t('common.error'));
                    } finally {
                      setGithubSaving(false);
                    }
                  }}
                >
                  <Form.Item name="enabled" label={t('settings.githubOAuthEnable') || 'Enable'} valuePropName="checked" tooltip={t('settings.githubOAuthEnableHint')}>
                    <Switch />
                  </Form.Item>
                  <Form.Item name="client_id" label="Client ID" rules={[{ required: false }]} tooltip={t('settings.githubClientIdHint')}>
                    <Input placeholder="Ov23..." autoComplete="off" />
                  </Form.Item>
                  <Form.Item name="client_secret" label="Client Secret" tooltip={t('settings.githubClientSecretHint')}>
                    <Input.Password placeholder="github_oauth_..." autoComplete="new-password" />
                  </Form.Item>
                  <Form.Item
                    name="allowed_logins"
                    label={t('settings.githubAllowed') || 'Allowed GitHub usernames'}
                    tooltip={t('settings.githubAllowedHint') || 'Comma-separated. Required to bind a GitHub account that is not linked yet.'}
                  >
                    <Input placeholder="your-github-login" />
                  </Form.Item>
                  {githubCallback ? (
                    <Typography.Paragraph copyable type="secondary" style={{ marginBottom: 8 }}>
                      {t('settings.githubCallbackLabel')}: {githubCallback}
                    </Typography.Paragraph>
                  ) : null}
                  <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
                    {t('settings.githubCallbackHint')}
                  </Typography.Paragraph>
                  <Button type="primary" htmlType="submit" loading={githubSaving}>
                    {t('common.save') || 'Save'}
                  </Button>
                </Form>
              </Card>

              <Card title={t('settings.backup') || 'Backup'}>
                <Space wrap>
                  <Button
                    icon={<IconCloudDown />}
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
                        const res = await restoreDatabase(file);
                        // Backend now os.Exit(0)s 500ms after responding so
                        // systemd's Restart=always brings the panel back with
                        // the freshly-restored DB. We poll /api/v1/health and
                        // reload the page automatically when it's back.
                        message.success(t('settings.restoreDone') || 'Restored — panel is restarting');
                        const mihomoNote = res?.data?.mihomo_config
                          ? `\nMihomo config: ${res.data.mihomo_config}`
                          : '';
                        Modal.warning({
                          title: t('settings.restoreRestart') || 'Restarting panel…',
                          content: (
                            <div>
                              <p>{t('settings.restoreRestartHint') ||
                                'Database restored. The panel is restarting now (systemd Restart=always). This page will auto-reload when it is back.'}</p>
                              {mihomoNote && <p style={{ fontSize: 12, opacity: 0.7 }}>{mihomoNote}</p>}
                              <p style={{ fontSize: 12, opacity: 0.7 }}>
                                {t('settings.restoreAutoReload', 'Auto-reloading in 5s…') ||
                                  'Auto-reloading in 5s…'}
                              </p>
                            </div>
                          ),
                        });
                        // Poll /api/v1/health; reload when panel is back (or after 30s).
                        const startedAt = Date.now();
                        const poll = window.setInterval(() => {
                          fetch('/api/v1/health', { cache: 'no-store' })
                            .then((r) => (r.ok ? r.json() : Promise.reject(r.status)))
                            .then(() => {
                              window.clearInterval(poll);
                              window.location.reload();
                            })
                            .catch(() => {
                              if (Date.now() - startedAt > 30000) {
                                window.clearInterval(poll);
                                message.warning(t('settings.restoreTimeout', 'Panel did not come back in 30s; please refresh manually.'));
                              }
                            });
                        }, 2000);
                      } catch (e: any) {
                        message.error(e?.response?.data?.error || e.message || t('common.error'));
                      }
                      return false;
                    }}
                  >
                    <Button icon={<IconCloudUp />}>{t('settings.restoreBackup') || 'Restore'}</Button>
                  </Upload>
                </Space>
                <div className="settings-local-backups" style={{ marginTop: 16 }}>
                  <div
                    style={{
                      marginBottom: 8,
                      display: 'flex',
                      flexWrap: 'wrap',
                      gap: 8,
                      alignItems: 'center',
                      justifyContent: 'space-between',
                    }}
                  >
                    <span style={{ wordBreak: 'break-word' }}>
                      {t('settings.localBackups') || 'On-disk backups'}
                      {backupDir ? (
                        <Typography.Text type="secondary" style={{ marginLeft: 8, fontSize: 12 }}>
                          ({backupDir})
                        </Typography.Text>
                      ) : null}
                      <Typography.Text type="secondary" style={{ marginLeft: 8 }}>
                        {formatBytes(backupTotalBytes)} · {localBackups.length}
                      </Typography.Text>
                    </span>
                    <Button size="small" onClick={() => void loadLocalBackups()} loading={backupsLoading}>
                      {t('common.refresh') || 'Refresh'}
                    </Button>
                  </div>
                  <Space wrap style={{ marginBottom: 12, width: '100%' }} size={[8, 8]}>
                    <InputNumber
                      min={1}
                      max={100}
                      value={cleanupKeep}
                      onChange={(v) => setCleanupKeep(Number(v) || 1)}
                      addonBefore={t('settings.backupKeep') || 'Keep last'}
                      style={{ width: '100%', minWidth: 140, maxWidth: 220 }}
                    />
                    <InputNumber
                      min={1}
                      max={3650}
                      value={cleanupDays}
                      onChange={(v) => setCleanupDays(Number(v) || 1)}
                      addonBefore={t('settings.backupOlderDays') || 'Older than (days)'}
                      style={{ width: '100%', minWidth: 160, maxWidth: 260 }}
                    />
                    <Button
                      danger
                      block={false}
                      style={{ minHeight: 32 }}
                      onClick={async () => {
                        try {
                          const res = await cleanupLocalBackups({
                            keep: cleanupKeep,
                            older_than_days: cleanupDays,
                          });
                          message.success(
                            `${t('settings.backupCleaned') || 'Cleaned'}: ${res.deleted_count} · -${formatBytes(res.freed_bytes || 0)}`,
                          );
                          await loadLocalBackups();
                        } catch (e: any) {
                          message.error(e?.message || t('common.error'));
                        }
                      }}
                    >
                      {t('settings.cleanupBackups') || 'Cleanup'}
                    </Button>
                  </Space>
                  <Table
                    size="small"
                    loading={backupsLoading}
                    rowKey="name"
                    dataSource={localBackups}
                    pagination={{ pageSize: 5, simple: true, hideOnSinglePage: true }}
                    scroll={{ x: 480 }}
                    locale={{ emptyText: t('common.empty') || 'No data' }}
                    columns={[
                      {
                        title: t('common.name') || 'Name',
                        dataIndex: 'name',
                        ellipsis: true,
                      },
                      {
                        title: t('settings.backupSize') || 'Size',
                        dataIndex: 'size',
                        width: 96,
                        render: (v: number) => formatBytes(v || 0),
                      },
                      {
                        title: t('settings.backupTime') || 'Time',
                        dataIndex: 'mod_time',
                        width: 160,
                        render: (v: string) => (v ? new Date(v).toLocaleString() : '—'),
                      },
                      {
                        title: t('common.actions') || 'Actions',
                        key: 'act',
                        width: 88,
                        fixed: 'right',
                        render: (_: unknown, row: LocalBackupItem) => (
                          <Popconfirm
                            title={t('common.confirmDelete') || 'Delete?'}
                            onConfirm={async () => {
                              try {
                                await deleteLocalBackup(row.name);
                                message.success(t('common.ok') || 'OK');
                                await loadLocalBackups();
                              } catch (e: any) {
                                message.error(e?.message || t('common.error'));
                              }
                            }}
                          >
                            <Button size="small" danger type="link">
                              {t('common.delete') || 'Delete'}
                            </Button>
                          </Popconfirm>
                        ),
                      },
                    ]}
                  />
                </div>
              </Card>
              <Card title={<><IconApi /> {t('settings.apiDocs') || 'API'}</>}>
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
                <Form.Item name="title" label={t('settings.subTitle') || 'Page title'} tooltip={t('settings.subTitleHint') || 'Title shown on the user subscription info page.'}>
                  <Input />
                </Form.Item>
                <Form.Item name="support_url" label={t('settings.subSupportUrl') || 'Support URL'} tooltip={t('settings.subSupportUrlHint') || 'Optional link shown on the subscription page (e.g. Telegram support link).'}>
                  <Input />
                </Form.Item>
                <Form.Item name="announce" label={t('settings.subAnnounce') || 'Announce'} tooltip={t('settings.subAnnounceHint')}>
                  <Input.TextArea rows={2} />
                </Form.Item>
                <Form.Item name="web_page_url" label={t('settings.subWebPage') || 'Web page URL'} tooltip={t('settings.subWebPageHint')}>
                  <Input />
                </Form.Item>
                <Form.Item name="update_hours" label={t('settings.subUpdates') || 'Update interval (hours)'} tooltip={t('settings.subUpdatesHint')}>
                  <InputNumber min={1} max={168} style={{ width: '100%' }} />
                </Form.Item>
                <Form.Item
                  name="encrypt"
                  label={t('settings.subEncrypt') || 'Base64-encode URI list'}
                  valuePropName="checked"
                  tooltip={t('settings.subEncryptHint')}
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
                  <Form.Item name="enabled" label={t('common.enabled')} valuePropName="checked" tooltip={t('settings.sslEnabledHint')}>
                    <Switch />
                  </Form.Item>
                  <Form.Item name="domain" label={t('settings.domainOrIP') || 'Domain or public IP'} extra={t('settings.domainOrIPExtra') || 'Hostname or IPv4/IPv6. IP uses Let\'s Encrypt shortlived profile (~6 days).'}>
                    <Input placeholder="panel.example.com or 203.0.113.10" />
                  </Form.Item>
                  <Form.Item name="email" label={t('settings.email')} tooltip={t('settings.sslEmailHint')}>
                    <Input placeholder="you@example.com" />
                  </Form.Item>
                  <Form.Item name="cache_dir" label={t('settings.acmeCacheDir') || 'ACME cache dir'} tooltip={t('settings.acmeCacheDirHint')}>
                    <Input placeholder="/var/lib/3m-ui/acme" />
                  </Form.Item>
                  <Form.Item name="listen_http" label={t('settings.listenHttp') || 'HTTP listen'} tooltip={t('settings.listenHttpHint')}>
                    <Input placeholder=":80" />
                  </Form.Item>
                  <Form.Item name="listen_tls" label={t('settings.listenTls') || 'TLS listen'} tooltip={t('settings.listenTlsHint')}>
                    <Input placeholder=":443" />
                  </Form.Item>
                  <Form.Item name="cert_file" label={t('settings.manualCert') || 'Manual cert file'} tooltip={t('settings.manualCertHint')}>
                    <Input />
                  </Form.Item>
                  <Form.Item name="key_file" label={t('settings.manualKey') || 'Manual key file'} tooltip={t('settings.manualKeyHint')}>
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
                  <Form.Item name="email" label={t('settings.email')} rules={[{ required: true }]} tooltip={t('settings.certbotEmailHint')}>
                    <Input />
                  </Form.Item>
                  <Form.Item name="domain" label={t('settings.domain')} rules={[{ required: true }]} tooltip={t('settings.certbotDomainHint')}>
                    <Input />
                  </Form.Item>
                  <Form.Item name="webroot" label={t('settings.webroot')} tooltip={t('settings.webrootHint')}>
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
                  <Form.Item name="kind" label={t('settings.proxyKind')} tooltip={t('settings.proxyKindHint')}>
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
                  <Form.Item name="upstream" label={t('settings.upstream')} tooltip={t('settings.upstreamHint')}>
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

              <Card title={t('settings.warp', 'Cloudflare WARP')} style={{ marginTop: 16 }}>
                <Space direction="vertical" style={{ width: '100%' }} size="middle">
                  <Text type="secondary">{t('settings.warpHint', 'One-click register a WARP WireGuard config (YAML for Mihomo outbound).')}</Text>
                  <Segmented
                    value={warpMode}
                    onChange={(v) => setWarpMode(v as 'wireguard' | 'masque')}
                    options={[
                      { label: t('settings.warpWireguard', 'WireGuard'), value: 'wireguard' },
                      { label: t('settings.warpMasque', 'MASQUE'), value: 'masque' },
                    ]}
                  />
                  <Button
                    onClick={async () => {
                      try {
                        const res = await client.post(`/system/templates/warp/register?mode=${warpMode}`);
                        const yaml =
                          (typeof res.data?.yaml === 'string' && res.data.yaml) ||
                          (typeof res.data?.masque_yaml === 'string' && res.data.masque_yaml) ||
                          '';
                        if (!yaml.trim()) {
                          message.error(t('settings.warpEmpty', 'WARP registration returned empty YAML'));
                          return;
                        }
                        try {
                          await copyText(yaml);
                          message.success(t('settings.warpDone', 'WARP registered — YAML copied'));
                        } catch {
                          message.success(t('settings.warpDoneNoCopy', 'WARP registered (copy failed — select text in the dialog)'));
                        }
                        setWarpYamlTitle(
                          warpMode === 'masque' ? 'WARP MASQUE YAML' : 'WARP WireGuard YAML',
                        );
                        setWarpYaml(yaml);
                        setWarpYamlOpen(true);
                      } catch (e: any) {
                        message.error(e?.response?.data?.error || e.message || t('common.error'));
                      }
                    }}
                  >
                    {t('settings.warpRegister', 'Register WARP')} ({warpMode === 'masque' ? 'MASQUE' : 'WireGuard'})
                  </Button>
                </Space>
              </Card>

              <Modal
                open={warpYamlOpen}
                title={warpYamlTitle}
                width={Math.min(720, typeof window !== 'undefined' ? window.innerWidth - 32 : 720)}
                onCancel={() => setWarpYamlOpen(false)}
                onOk={() => setWarpYamlOpen(false)}
                okText={t('common.close', 'Close')}
                cancelButtonProps={{ style: { display: 'none' } }}
                destroyOnHidden
                styles={{
                  container: { background: token.colorBgElevated },
                  header: { background: token.colorBgElevated, color: token.colorText },
                  body: { background: token.colorBgElevated },
                  footer: { background: token.colorBgElevated },
                }}
              >
                <pre
                  style={{
                    margin: 0,
                    padding: 12,
                    maxHeight: 420,
                    overflow: 'auto',
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-word',
                    fontFamily:
                      'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                    fontSize: 12,
                    lineHeight: 1.45,
                    background: token.colorBgContainer,
                    color: token.colorText,
                    border: `1px solid ${token.colorBorderSecondary}`,
                    borderRadius: token.borderRadiusLG ?? token.borderRadius,
                  }}
                >
                  {warpYaml}
                </pre>
              </Modal>

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

          {section === 'ops' && (
            <SystemOpsCard />
          )}

          {section === 'about' && (
            <Card title={<><IconInfo /> {t('settings.about')}</>}>
              <AboutPanelVersion subtitle={t('app.title')} />
            </Card>
          )}
        </Content>
      </Layout>
    </div>
  );
};


function SystemOpsCard() {
  const { t } = useI18n();
  const isMobile = useIsMobile();
  const [restartLoading, setRestartLoading] = useState(false);
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
  const [updateLoading, setUpdateLoading] = useState(false);
  const [checking, setChecking] = useState(false);

  const doCheckUpdate = async () => {
    setChecking(true);
    try {
      const info = await checkUpdate();
      setUpdateInfo(info);
    } catch (e: any) {
      message.error(e?.message || t('common.error'));
    } finally {
      setChecking(false);
    }
  };

  useEffect(() => {
    doCheckUpdate();
  }, []);

  const doRestart = () => {
    Modal.confirm({
      title: t('settings.restartConfirmTitle') || 'Restart panel?',
      content: t('settings.restartConfirmHint') || 'The panel will exit and systemd will restart it. You will be briefly disconnected.',
      okText: t('settings.restartNow') || 'Restart now',
      okType: 'danger',
      cancelText: t('common.cancel') || 'Cancel',
      onOk: async () => {
        setRestartLoading(true);
        try {
          await restartPanel();
          message.success(t('settings.restarting') || 'Restarting…');
          // Poll health and reload
          const startedAt = Date.now();
          const poll = window.setInterval(() => {
            fetch('/api/v1/health', { cache: 'no-store' })
              .then((r) => (r.ok ? r.json() : Promise.reject(r.status)))
              .then(() => {
                window.clearInterval(poll);
                window.location.reload();
              })
              .catch(() => {
                if (Date.now() - startedAt > 30000) {
                  window.clearInterval(poll);
                  message.warning(t('settings.restoreTimeout') || 'Panel did not come back in 30s; please refresh manually.');
                }
              });
          }, 2000);
        } catch (e: any) {
          message.error(e?.message || t('common.error'));
          setRestartLoading(false);
        }
      },
    });
  };

  const doUpdate = () => {
    Modal.confirm({
      title: t('settings.updateConfirmTitle') || 'Update panel?',
      content: (
        <div>
          <p>{t('settings.updateConfirmHint') || 'This will download the latest release and restart the panel. The process takes 30-60 seconds.'}</p>
          {updateInfo?.latest_version && (
            <p style={{ fontSize: 12, opacity: 0.7 }}>
              {t('settings.currentVersion') || 'Current'}: {updateInfo.current_version} → {t('settings.latestVersion') || 'Latest'}: {updateInfo.latest_version}
            </p>
          )}
        </div>
      ),
      okText: t('settings.updateNow') || 'Update now',
      okType: 'primary',
      cancelText: t('common.cancel') || 'Cancel',
      onOk: async () => {
        setUpdateLoading(true);
        try {
          await runUpdate(updateInfo?.target_channel === 'pre' ? 'pre' : 'stable');
          message.success(t('settings.updating') || 'Update started. Panel will restart automatically.');
          // Poll health and reload after update (longer timeout — update takes ~60s)
          const startedAt = Date.now();
          const poll = window.setInterval(() => {
            fetch('/api/v1/health', { cache: 'no-store' })
              .then((r) => (r.ok ? r.json() : Promise.reject(r.status)))
              .then(() => {
                window.clearInterval(poll);
                window.location.reload();
              })
              .catch(() => {
                if (Date.now() - startedAt > 120000) {
                  window.clearInterval(poll);
                  message.warning(t('settings.updateTimeout') || 'Update is taking longer than 2 minutes. Check SSH: 3m-ui logs');
                }
              });
          }, 3000);
        } catch (e: any) {
          message.error(e?.message || t('common.error'));
          setUpdateLoading(false);
        }
      },
    });
  };

  const cardStyle: React.CSSProperties = {
    marginBottom: 16,
  };

  return (
    <Space direction="vertical" style={{ width: '100%' }} size={16}>
      {/* Restart panel */}
      <Card
        title={<><IconRestart /> {t('settings.restartPanel') || 'Restart panel'}</>}
        style={cardStyle}
      >
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Text type="secondary" style={{ display: 'block' }}>
            {t('settings.restartPanelHint') || 'Restart the 3m-ui panel process. systemd will automatically bring it back. Use this after config changes that require a restart, or if the panel is acting up.'}
          </Text>
          <Button
            type="default"
            danger
            icon={<IconRestart />}
            loading={restartLoading}
            onClick={doRestart}
            block={isMobile}
          >
            {t('settings.restartNow') || 'Restart now'}
          </Button>
        </Space>
      </Card>

      {/* Check / run update */}
      <Card
        title={<><IconUpdate /> {t('settings.panelUpdate') || 'Panel update'}</>}
        style={cardStyle}
      >
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Text type="secondary" style={{ display: 'block' }}>
            {t('settings.updateHint') || 'Check for the latest 3m-ui release and update with one click. The panel will download the latest binary, replace itself, and restart automatically.'}
          </Text>

          {updateInfo && (
            <Space direction="vertical" style={{ width: '100%' }} size="middle">
              <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'center' }}>
                <Space size={4}>
                  <Text type="secondary">{t('settings.currentVersion') || 'Current'}:</Text>
                  <Tag color={updateInfo.current_version === 'dev' ? 'orange' : 'blue'}>
                    {updateInfo.current_version}
                  </Tag>
                  {updateInfo.current_channel && (
                    <Tag color={updateInfo.current_channel === 'pre' ? 'purple' : 'cyan'} style={{ marginLeft: 4 }}>
                      {updateInfo.current_channel === 'pre' ? (t('settings.preRelease') || 'Pre-release') : (t('settings.stable') || 'Stable')}
                    </Tag>
                  )}
                </Space>
              </div>

              {/* Channel switcher */}
              {updateInfo.latest_stable || updateInfo.latest_pre ? (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, alignItems: 'center' }}>
                  <Text type="secondary" style={{ fontSize: 13 }}>
                    {t('settings.channelSwitch') || 'Channel'}:
                  </Text>
                  <Segmented
                    value={updateInfo.target_channel || 'pre'}
                    onChange={(v) => {
                      setUpdateInfo({ ...updateInfo, target_channel: v as string });
                      // Recompute latest_version + update_available for new target
                      const newLatest = v === 'stable' ? updateInfo.latest_stable : updateInfo.latest_pre;
                      const cur = updateInfo.current_version?.replace(/^v/, '') || '';
                      const lat = newLatest?.replace(/^v/, '') || '';
                      setUpdateInfo({
                        ...updateInfo,
                        target_channel: v as string,
                        latest_version: newLatest || '',
                        update_available: cur !== lat && cur !== 'dev' && lat !== '',
                      });
                    }}
                    options={[
                      {
                        label: (
                          <span>
                            {t('settings.stable') || 'Stable'}
                            {updateInfo.latest_stable ? (
                              <Text type="secondary" style={{ fontSize: 11, marginLeft: 4 }}>
                                {updateInfo.latest_stable}
                              </Text>
                            ) : null}
                          </span>
                        ),
                        value: 'stable',
                      },
                      {
                        label: (
                          <span>
                            {t('settings.preRelease') || 'Pre-release'}
                            {updateInfo.latest_pre ? (
                              <Text type="secondary" style={{ fontSize: 11, marginLeft: 4 }}>
                                {updateInfo.latest_pre}
                              </Text>
                            ) : null}
                          </span>
                        ),
                        value: 'pre',
                      },
                    ]}
                    size={isMobile ? 'middle' : 'small'}
                  />
                </div>
              ) : null}

              {/* Target version display */}
              <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap', alignItems: 'center' }}>
                <Space size={4}>
                  <Text type="secondary">{t('settings.latestVersion') || 'Latest'}:</Text>
                  {updateInfo.latest_version ? (
                    <Tag color={updateInfo.update_available ? 'green' : 'default'}>
                      {updateInfo.latest_version}
                    </Tag>
                  ) : (
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      {updateInfo.error || t('settings.unknown') || 'unknown'}
                    </Text>
                  )}
                </Space>
                {updateInfo.update_available ? (
                  <Tag color="success" style={{ marginLeft: 'auto' }}>
                    {t('settings.updateAvailable') || 'Update available'}
                  </Tag>
                ) : updateInfo.latest_version ? (
                  <Tag color="default" style={{ marginLeft: 'auto' }}>
                    {t('settings.upToDate') || 'Up to date'}
                  </Tag>
                ) : null}
              </div>
            </Space>
          )}

          {updateInfo?.release_notes && (
            <details style={{ fontSize: 12, opacity: 0.8 }}>
              <summary style={{ cursor: 'pointer', color: 'var(--ant-color-text-secondary)' }}>
                {t('settings.releaseNotes') || 'Release notes'}
              </summary>
              <pre style={{ whiteSpace: 'pre-wrap', maxHeight: 200, overflow: 'auto', marginTop: 8, padding: 8, borderRadius: 6, background: 'var(--ant-color-fill-quaternary, rgba(0,0,0,0.02))' }}>
                {updateInfo.release_notes.slice(0, 2000)}
              </pre>
            </details>
          )}

          <Space wrap>
            <Button
              icon={<IconUpdate />}
              loading={checking}
              onClick={doCheckUpdate}
            >
              {t('settings.checkUpdate') || 'Check for updates'}
            </Button>
            <Button
              type="primary"
              icon={<IconUpdate />}
              loading={updateLoading}
              disabled={!updateInfo?.update_available}
              onClick={doUpdate}
              title={updateInfo?.target_channel === 'pre'
                ? (t('settings.switchToPre') || 'Switch to pre-release channel')
                : (t('settings.switchToStable') || 'Switch to stable channel')}
              block={isMobile}
            >
              {t('settings.updateNow') || 'Update now'}
            </Button>
          </Space>

          {!updateInfo?.update_available && updateInfo?.latest_version && (
            <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>
              {t('settings.upToDateHint') || 'You are running the latest version. Check again later or update via SSH: 3m-ui update'}
            </Text>
          )}
          {updateInfo?.error && (
            <Text type="secondary" style={{ fontSize: 12, display: 'block' }}>
              {t('settings.updateCheckFailed') || 'Cannot check for updates (GitHub API unreachable). Update via SSH: 3m-ui update'}
            </Text>
          )}
        </Space>
      </Card>
    </Space>
  );
}

function AboutPanelVersion({ subtitle }: { subtitle: string }) {
  const [info, setInfo] = useState<{ version?: string; git_commit?: string; build_time?: string }>({});
  useEffect(() => {
    fetch('/api/v1/health')
      .then((r) => r.json())
      .then((d) => setInfo(d || {}))
      .catch(() => setInfo({}));
  }, []);
  const ver = info.version || 'dev';
  const label = ver === 'dev' ? 'dev' : ver.startsWith('v') ? ver : `v${ver}`;
  return (
    <Space direction="vertical">
      <Title level={4} style={{ margin: 0 }}>
        3M-UI
      </Title>
      <Text type="secondary">{subtitle}</Text>
      <Tag color="blue">{label}</Tag>
      {info.git_commit && info.git_commit !== 'unknown' ? (
        <Text type="secondary" style={{ fontSize: 12 }}>
          {info.git_commit.slice(0, 12)}
          {info.build_time && info.build_time !== 'unknown' ? ` · ${info.build_time}` : ''}
        </Text>
      ) : null}
    </Space>
  );
}

export default Settings;
