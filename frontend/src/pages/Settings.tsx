import React, { useEffect, useState } from 'react';
import { Card, Button, Space, Typography, Tag, message, Upload, Form, Input, Switch, Select, InputNumber } from 'antd';
import { useNavigate } from 'react-router-dom';
import { useI18n } from '../i18n';
import { copyText } from '../utils/clipboard';
import { useThemeStore } from '../stores/themeStore';
import { LockOutlined, GlobalOutlined, BgColorsOutlined, InfoCircleOutlined, CloudDownloadOutlined, CloudUploadOutlined, ApiOutlined } from '@ant-design/icons';
import { downloadBackup, restoreDatabase, openApiUrl } from '../api/system';
import { fetchTelegramSettings, saveTelegramSettings, testTelegram, TelegramSettings } from '../api/telegram';
import client from '../api/client';

const { Text } = Typography;

const Settings: React.FC = () => {
  const [panelServer, setPanelServer] = useState<{ port?: number; listen?: string; public_url?: string; config_path?: string; hint?: string }>({});
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
    client.get('/system/panel-server').then((r) => {
      setPanelServer(r.data || {});
      panelForm.setFieldsValue({
        port: r.data?.port ?? 8080,
        listen: r.data?.listen || '',
        public_url: r.data?.public_url || '',
      });
    }).catch(() => {});

    client.get('/panel-settings').then((r) => {
      const day = Number(r.data?.traffic_reset_day || 0);
      if (!Number.isNaN(day)) setResetDay(day);
      accessForm.setFieldsValue({
        public_host: r.data?.['access_profile.public_host'] || '',
        public_port: r.data?.['access_profile.public_port'] || '',
        sni: r.data?.['access_profile.sni'] || '',
        client_fingerprint: r.data?.['access_profile.client_fingerprint'] || 'chrome',
        alpn: r.data?.['access_profile.alpn'] || '',
      });
    }).catch((e: any) => { message.error(e.message || t('common.error')); });
    fetchTelegramSettings().then((s: TelegramSettings) => {
      tgForm.setFieldsValue({
        ...s,
        chat_ids: (s.chat_ids || []).join(','),
        bot_token: s.bot_token || '',
        traffic_warn_pct: s.traffic_warn_pct ?? 80,
        expiry_warn_hours: s.expiry_warn_hours ?? 72,
        notify_on_traffic: s.notify_on_traffic ?? true,
      });
    }).catch((e: any) => { message.error(e.message || t('common.error')); });
    client.get('/system/ssl').then((r) => {
      sslForm.setFieldsValue(r.data || {});
    }).catch(() => {});
    client.get('/system/ssl/status').then((r) => setSslStatus(r.data)).catch(() => {});
    client.get('/system/subscription-page').then((r) => {
      subPageForm.setFieldsValue(r.data || {});
    }).catch(() => {});
  }, []);

  return (
    <div>
      <h2>{t('settings.title')}</h2>
      <p style={{ color: 'rgba(0,0,0,0.45)' }}>{t('settings.subtitle')}</p>
      <Card title={t('settings.panelServer') || 'Panel / NAT'} style={{ marginBottom: 16 }} extra={<span style={{ fontSize: 12, opacity: 0.65 }}>{panelServer.config_path || ''}</span>}>
        <Form form={panelForm} layout="vertical" onFinish={async (values) => {
          try {
            const res = await client.put('/system/panel-server', {
              port: Number(values.port),
              listen: values.listen || '',
              public_url: values.public_url || '',
            });
            message.success(res.data?.message || t('common.saved') || 'Saved — restart service to apply port');
            setPanelServer((s) => ({ ...s, ...res.data }));
          } catch (e: any) {
            message.error(e?.response?.data?.error || e.message);
          }
        }}>
          <Form.Item name="port" label={t('settings.panelPort') || 'Panel port'} rules={[{ required: true }]} extra={t('settings.panelPortHint') || 'NAT: map this host port to the WAN. Restart 3m-ui after change.'}>
            <InputNumber min={1} max={65535} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="listen" label={t('settings.panelListen') || 'Listen address'} extra={t('settings.panelListenHint') || 'Empty = all interfaces (IPv4/IPv6). Use 127.0.0.1 for reverse-proxy only.'}>
            <Input placeholder="0.0.0.0 or :: or 127.0.0.1" />
          </Form.Item>
          <Form.Item name="public_url" label={t('settings.panelPublicURL') || 'Public panel URL'} extra={t('settings.panelPublicURLHint') || 'Used in subscription links behind NAT, e.g. https://panel.example.com:8443'}>
            <Input placeholder="https://your-domain:port" />
          </Form.Item>
          <Button type="primary" htmlType="submit">{t('common.save') || 'Save'}</Button>
          <p style={{ marginTop: 8, fontSize: 12, opacity: 0.65 }}>{panelServer.hint}</p>
        </Form>
      </Card>

      <Card title={<><GlobalOutlined /> {t('settings.language')}</>} style={{ marginBottom: 16 }}>
        <Space>
          <Button type={locale === 'zh' ? 'primary' : 'default'} onClick={() => setLocale('zh')}>中文</Button>
          <Button type={locale === 'en' ? 'primary' : 'default'} onClick={() => setLocale('en')}>English</Button>
        </Space>
      </Card>
      <Card title={<><BgColorsOutlined /> {t('settings.theme')}</>} style={{ marginBottom: 16 }}>
        <Space>
          <Button type={mode === 'light' ? 'primary' : 'default'} onClick={() => setMode('light')}>{t('settings.light')}</Button>
          <Button type={mode === 'dark' ? 'primary' : 'default'} onClick={() => setMode('dark')}>{t('settings.dark')}</Button>
          <Button type={mode === 'system' ? 'primary' : 'default'} onClick={() => setMode('system')}>{t('settings.system')}</Button>
        </Space>
      </Card>
      <Card title={<><LockOutlined /> {t('settings.passwordTitle')}</>} style={{ marginBottom: 16 }}>
        <Text type="secondary">{t('settings.passwordDescription')}</Text>
        <div style={{ marginTop: 12 }}><Button type="primary" onClick={() => navigate('/change-password')}>{t('settings.changePassword')}</Button></div>
      </Card>
      <Card title={<><CloudDownloadOutlined /> {t('settings.backup')}</>} style={{ marginBottom: 16 }}>
        <Text type="secondary">{t('settings.backupHint')}</Text>
        <div style={{ marginTop: 12 }}>
          <Space wrap>
            <Button icon={<CloudDownloadOutlined />} onClick={async () => { try { await downloadBackup(); message.success(t('settings.backupDone')); } catch (e: any) { message.error(e.message || t('common.error')); } }}>{t('settings.downloadBackup')}</Button>
            <Upload showUploadList={false} beforeUpload={async (file) => { try { await restoreDatabase(file as File); message.success(t('settings.restoreDone')); } catch (e: any) { message.error(e.message || t('common.error')); } return false; }}>
              <Button icon={<CloudUploadOutlined />}>{t('settings.restoreDb')}</Button>
            </Upload>
          </Space>
        </div>
      </Card>
      <Card title={t('settings.geofiles') || 'GeoIP / GeoSite'} style={{ marginBottom: 16 }}>
        <Text type="secondary">{t('settings.geofilesHint') || 'Download latest MetaCubeX GeoIP/GeoSite databases into the Mihomo data directory .'}</Text>
        <div style={{ marginTop: 12 }}>
          <Button type="primary" onClick={async () => {
            try {
              const r = await client.post('/system/geofiles/update');
              message.success(t('settings.geofilesDone') || 'Geo files updated');
              // r.data kept for future use; no console leak
              void r;
            } catch (e: any) {
              message.error(e.message || t('common.error'));
            }
          }}>{t('settings.updateGeofiles') || 'Update geo files'}</Button>
        </div>
      </Card>
      <Card title={<><ApiOutlined /> {t('settings.apiDocs')}</>} style={{ marginBottom: 16 }}>
        <Text type="secondary">{t('settings.apiDocsHint')}</Text>
        <div style={{ marginTop: 12 }}><Button type="link" href={openApiUrl} target="_blank" rel="noreferrer">{t('settings.openOpenAPI')}</Button></div>
      </Card>
      
      <Card title={t('settings.accessProfile')} style={{ marginBottom: 16 }}>
        <Text type="secondary">{t('settings.accessProfileHint')}</Text>
        <Form form={accessForm} layout="vertical" style={{ marginTop: 12 }} onFinish={async (values) => {
          try {
            await client.put('/panel-settings', {
              'access_profile.public_host': values.public_host || '',
              'access_profile.public_port': values.public_port || '',
              'access_profile.sni': values.sni || '',
              'access_profile.client_fingerprint': values.client_fingerprint || 'chrome',
              'access_profile.alpn': values.alpn || '',
            });
            message.success(t('common.saved'));
          } catch (e: any) { message.error(e.message || t('common.error')); }
        }}>
          <Form.Item name="public_host" label={t('settings.publicHost')}><Input placeholder="example.com" /></Form.Item>
          <Form.Item name="public_port" label={t('settings.publicPort')}><Input placeholder="443" /></Form.Item>
          <Form.Item name="sni" label={t('listeners.sni')}><Input placeholder="www.example.com" /></Form.Item>
          <Form.Item name="client_fingerprint" label={t('settings.clientFingerprint')}>
            <Select options={['chrome','firefox','safari','ios','android','edge','random'].map(v => ({ value: v, label: v }))} />
          </Form.Item>
          <Form.Item name="alpn" label={t('listeners.alpn')} tooltip={t('settings.alpnHint')}><Input placeholder="h2,http/1.1" /></Form.Item>
          <Button type="primary" htmlType="submit">{t('common.save')}</Button>
        </Form>
      </Card>

      <Card title={t('settings.telegram')} style={{ marginBottom: 16 }}>
        <Form form={tgForm} layout="vertical" onFinish={async (values) => {
          try {
            await saveTelegramSettings({
              enabled: !!values.enabled,
              bot_token: values.bot_token,
              chat_ids: String(values.chat_ids || '').split(',').map((x: string) => x.trim()).filter(Boolean),
              notify_on_login: !!values.notify_on_login,
              notify_on_cpu: !!values.notify_on_cpu,
              cpu_warn_pct: Number(values.cpu_warn_pct || 0),
              notify_on_block: !!values.notify_on_block,
              notify_on_unblock: !!values.notify_on_unblock,
              notify_on_expiry: !!values.notify_on_expiry,
              notify_on_traffic: !!values.notify_on_traffic,
              notify_daily_digest: !!values.notify_daily_digest,
              traffic_warn_pct: Number(values.traffic_warn_pct || 80),
              expiry_warn_hours: Number(values.expiry_warn_hours || 72),
              keep_token: !values.bot_token,
            });
            message.success(t('settings.telegramSaved'));
          } catch (e: any) { message.error(e.message || t('common.error')); }
        }}>
          <Form.Item name="enabled" label={t('common.enabled')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="bot_token" label={t('settings.botToken')}><Input.Password /></Form.Item>
          <Form.Item name="chat_ids" label={t('settings.chatIds')} tooltip={t('settings.chatIdsHint')}><Input placeholder="123456789, -100123..." /></Form.Item>
          <Form.Item name="notify_on_login" label={t('settings.notifyLogin') || 'Notify on panel login'} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="notify_on_cpu" label={t('settings.notifyCPU') || 'Notify on high CPU'} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="cpu_warn_pct" label={t('settings.cpuWarnPct') || 'CPU warn %'} initialValue={0}><Input type="number" min={0} max={100} /></Form.Item>
          <Form.Item name="notify_on_block" label={t('settings.notifyBlock')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="notify_on_unblock" label={t('settings.notifyUnblock')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="notify_on_expiry" label={t('settings.notifyExpiry')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="notify_daily_digest" label={t('settings.notifyDailyDigest')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="notify_on_traffic" label={t('settings.notifyTraffic') || 'Traffic threshold warning'} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="traffic_warn_pct" label={t('settings.trafficWarnPct') || 'Traffic warn %'}><InputNumber min={1} max={100} style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="expiry_warn_hours" label={t('settings.expiryWarnHours') || 'Expiry warn (hours)'}><InputNumber min={1} max={720} style={{ width: '100%' }} /></Form.Item>
          <Space>
            <Button type="primary" htmlType="submit">{t('common.save')}</Button>
            <Button onClick={async () => { try { await testTelegram(); message.success(t('settings.telegramTestOk')); } catch (e: any) { message.error(e.message || t('common.error')); } }}>{t('settings.telegramTest')}</Button>
          </Space>
        </Form>
      </Card>

      <Card title={t('settings.panelSSL') || 'Panel SSL (ACME)'} style={{ marginBottom: 16 }}>
        <Text type="secondary">{t('settings.panelSSLHint') || 'Enable HTTPS for the panel via Let\'s Encrypt (autocert) or a manual certificate. Restart the panel after saving.'}</Text>
        {sslStatus && (
          <div style={{ marginTop: 8 }}>
            <Tag color={sslStatus.enabled ? 'green' : 'default'}>{String(sslStatus.mode || 'disabled')}</Tag>
            {sslStatus.domain ? <Tag>{String(sslStatus.domain)}</Tag> : null}
            {sslStatus.has_cache ? <Tag color="blue">cert cached</Tag> : null}
          </div>
        )}
        <Form form={sslForm} layout="vertical" style={{ marginTop: 12 }} onFinish={async (values) => {
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
            message.success(t('settings.sslSaved') || 'SSL settings saved — restart panel to apply');
            const st = await client.get('/system/ssl/status');
            setSslStatus(st.data);
          } catch (e: any) { message.error(e.message || t('common.error')); }
        }}>
          <Form.Item name="enabled" label={t('common.enabled')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="domain" label={t('settings.domain')}><Input placeholder="panel.example.com" /></Form.Item>
          <Form.Item name="email" label={t('settings.email')}><Input placeholder="admin@example.com" /></Form.Item>
          <Form.Item name="cache_dir" label={t('settings.acmeCacheDir') || 'ACME cache dir'}><Input placeholder="/var/lib/3m-ui/acme" /></Form.Item>
          <Form.Item name="listen_http" label="HTTP listen"><Input placeholder=":80" /></Form.Item>
          <Form.Item name="listen_tls" label="TLS listen"><Input placeholder=":443" /></Form.Item>
          <Form.Item name="cert_file" label={t('settings.manualCert') || 'Manual cert file (optional)'}><Input placeholder="/path/fullchain.pem" /></Form.Item>
          <Form.Item name="key_file" label={t('settings.manualKey') || 'Manual key file (optional)'}><Input placeholder="/path/privkey.pem" /></Form.Item>
          <Button type="primary" htmlType="submit">{t('common.save')}</Button>
        </Form>
      </Card>

      <Card title={t('settings.subPage') || 'Subscription page template'} style={{ marginBottom: 16 }}>
        <Text type="secondary">{t('settings.subPageHint') || 'Custom HTML template directory (index.html or sub.html). Leave empty for the built-in page. Browsers hitting the subscription URL with Accept: text/html will see this page.'}</Text>
        <Form form={subPageForm} layout="vertical" style={{ marginTop: 12 }} onFinish={async (values) => {
          try {
            await client.put('/system/subscription-page', {
              theme_dir: values.theme_dir || '',
              title: values.title || '',
              support_url: values.support_url || '',
              announce: values.announce || '',
              web_page_url: values.web_page_url || '',
              update_hours: Number(values.update_hours || 12),
              encrypt: values.encrypt !== false,
            });
            message.success(t('common.saved'));
          } catch (e: any) { message.error(e.message || t('common.error')); }
        }}>
          <Form.Item name="theme_dir" label={t('settings.subThemeDir') || 'Theme directory'}><Input placeholder="/var/lib/3m-ui/sub-theme" /></Form.Item>
          <Form.Item name="title" label={t('settings.subTitle') || 'Page title'}><Input placeholder="My VPN Subscription" /></Form.Item>
          <Form.Item name="support_url" label={t('settings.subSupportUrl') || 'Support URL'}><Input placeholder="https://t.me/support" /></Form.Item>
          <Form.Item name="announce" label={t('settings.subAnnounce') || 'Announce'}><Input placeholder="optional announcement" /></Form.Item>
          <Form.Item name="web_page_url" label={t('settings.subWebPage') || 'Profile web page URL'}><Input placeholder="https://..." /></Form.Item>
          <Form.Item name="update_hours" label={t('settings.subUpdates') || 'Update interval (hours)'} initialValue={12}><Input type="number" min={1} /></Form.Item>
          <Form.Item name="encrypt" label={t('settings.subEncrypt') || 'Base64 encrypt URI list'} valuePropName="checked" initialValue={true}><Switch /></Form.Item>
          <Space>
            <Button type="primary" htmlType="submit">{t('common.save')}</Button>
            <Button onClick={async () => {
              try {
                const r = await client.get('/system/subscription-page/default-template', { responseType: 'text' });
                const blob = new Blob([r.data], { type: 'text/html' });
                const a = document.createElement('a');
                a.href = URL.createObjectURL(blob);
                a.download = 'sub-template.html';
                a.click();
              } catch (e: any) { message.error(e.message || t('common.error')); }
            }}>{t('settings.downloadDefaultTpl') || 'Download default template'}</Button>
          </Space>
        </Form>
      </Card>

      <Card title={t('settings.certWizard')} style={{ marginBottom: 16 }}>
        <Form form={acmeForm} layout="vertical" initialValues={{ webroot: '/var/www/acme' }} onFinish={async (values) => {
          try {
            const r = await client.post('/system/templates/acme', values);
            setAcmeCmd(r.data.command || '');
            message.success(t('settings.acmeGenerated'));
          } catch (e: any) { message.error(e.message || t('common.error')); }
        }}>
          <Form.Item name="domain" label={t('settings.domain')} rules={[{ required: true }]}><Input placeholder="panel.example.com" /></Form.Item>
          <Form.Item name="email" label={t('settings.email')}><Input placeholder="admin@example.com" /></Form.Item>
          <Form.Item name="webroot" label={t('settings.webroot')}><Input /></Form.Item>
          <Button type="primary" htmlType="submit">{t('settings.generateAcme')}</Button>
        </Form>
        {acmeCmd && (
          <div style={{ marginTop: 12 }}>
            <Text type="secondary">{t('settings.acmeHint')}</Text>
            <Input.TextArea style={{ marginTop: 8 }} rows={3} value={acmeCmd} readOnly />
            <Button style={{ marginTop: 8 }} onClick={async () => { const ok = await copyText(acmeCmd); if (ok) message.success(t('common.copied')); else message.error(t('common.copyFailed') || 'Copy failed'); }}>{t('common.copy')}</Button>
          </div>
        )}
      </Card>
      <Card title={t('settings.templates')} style={{ marginBottom: 16 }}>
        <Form form={tplForm} layout="vertical" initialValues={{ kind: 'nginx', upstream: '127.0.0.1:8080' }} onFinish={async (values) => {
          try { const r = await client.post('/system/templates/reverse-proxy', values); setTplOut(r.data.config || ''); message.success(t('settings.templateGenerated')); }
          catch (e: any) { message.error(e.message || t('common.error')); }
        }}>
          <Form.Item name="kind" label={t('settings.proxyKind')}><Select options={[{ value: 'nginx', label: 'Nginx' }, { value: 'caddy', label: 'Caddy' }]} /></Form.Item>
          <Form.Item name="domain" label={t('settings.domain')} rules={[{ required: true }]}><Input placeholder="panel.example.com" /></Form.Item>
          <Form.Item name="upstream" label={t('settings.upstream')}><Input /></Form.Item>
          <Button type="primary" htmlType="submit">{t('settings.generateTemplate')}</Button>
        </Form>
        {tplOut && <Input.TextArea style={{ marginTop: 12 }} rows={12} value={tplOut} readOnly />}
      </Card>
      <Card title={t('settings.trafficReset')} style={{ marginBottom: 16 }}>
        <Space wrap>
          <span>{t('settings.trafficResetDay')}</span>
          <InputNumber min={0} max={31} value={resetDay} onChange={(v) => setResetDay(Number(v || 0))} />
          <Button type="primary" onClick={async () => {
            try {
              await client.put('/panel-settings', { traffic_reset_day: String(resetDay) });
              message.success(t('common.saved'));
            } catch (e: any) { message.error(e.message || t('common.error')); }
          }}>{t('common.save')}</Button>
        </Space>
      </Card>
      <Card title={<><InfoCircleOutlined /> {t('settings.about')}</>}>
        <Space direction="vertical"><Text strong>3M-UI</Text><Text type="secondary">{t('app.title')}</Text><Tag color="blue">v1.0.0</Tag></Space>
      </Card>
    </div>
  );
};

export default Settings;
