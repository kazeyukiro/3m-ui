import React, { useEffect, useState } from 'react';
import { Card, Button, Space, Typography, Tag, message, Upload, Form, Input, Switch, Select } from 'antd';
import { useNavigate } from 'react-router-dom';
import { useI18n } from '../i18n';
import { useThemeStore } from '../stores/themeStore';
import { LockOutlined, GlobalOutlined, BgColorsOutlined, InfoCircleOutlined, CloudDownloadOutlined, CloudUploadOutlined, ApiOutlined } from '@ant-design/icons';
import { downloadBackup, restoreDatabase, openApiUrl } from '../api/system';
import { fetchTelegramSettings, saveTelegramSettings, testTelegram, TelegramSettings } from '../api/telegram';
import client from '../api/client';

const { Text } = Typography;

const Settings: React.FC = () => {
  const { t, locale, setLocale } = useI18n();
  const { mode, setMode } = useThemeStore();
  const navigate = useNavigate();
  const [tgForm] = Form.useForm();
  const [tplForm] = Form.useForm();
  const [tplOut, setTplOut] = useState('');

  useEffect(() => {
    fetchTelegramSettings().then((s: TelegramSettings) => {
      tgForm.setFieldsValue({
        ...s,
        chat_ids: (s.chat_ids || []).join(','),
        bot_token: s.bot_token || '',
      });
    }).catch(() => {});
  }, []);

  return (
    <div>
      <h2>{t('settings.title')}</h2>
      <p style={{ color: 'rgba(0,0,0,0.45)' }}>{t('settings.subtitle')}</p>
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
      <Card title={<><ApiOutlined /> {t('settings.apiDocs')}</>} style={{ marginBottom: 16 }}>
        <Text type="secondary">{t('settings.apiDocsHint')}</Text>
        <div style={{ marginTop: 12 }}><Button type="link" href={openApiUrl} target="_blank" rel="noreferrer">{t('settings.openOpenAPI')}</Button></div>
      </Card>
      <Card title={t('settings.telegram')} style={{ marginBottom: 16 }}>
        <Form form={tgForm} layout="vertical" onFinish={async (values) => {
          try {
            await saveTelegramSettings({
              enabled: !!values.enabled,
              bot_token: values.bot_token,
              chat_ids: String(values.chat_ids || '').split(',').map((x: string) => x.trim()).filter(Boolean),
              notify_on_block: !!values.notify_on_block,
              notify_on_unblock: !!values.notify_on_unblock,
              notify_on_expiry: !!values.notify_on_expiry,
              notify_daily_digest: !!values.notify_daily_digest,
              keep_token: !values.bot_token || String(values.bot_token).includes('…'),
            });
            message.success(t('settings.telegramSaved'));
          } catch (e: any) { message.error(e.message || t('common.error')); }
        }}>
          <Form.Item name="enabled" label={t('common.enabled')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="bot_token" label={t('settings.botToken')}><Input.Password /></Form.Item>
          <Form.Item name="chat_ids" label={t('settings.chatIds')} tooltip={t('settings.chatIdsHint')}><Input placeholder="123456789, -100123..." /></Form.Item>
          <Form.Item name="notify_on_block" label={t('settings.notifyBlock')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="notify_on_unblock" label={t('settings.notifyUnblock')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="notify_on_expiry" label={t('settings.notifyExpiry')} valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="notify_daily_digest" label={t('settings.notifyDailyDigest')} valuePropName="checked"><Switch /></Form.Item>
          <Space>
            <Button type="primary" htmlType="submit">{t('common.save')}</Button>
            <Button onClick={async () => { try { await testTelegram(); message.success(t('settings.telegramTestOk')); } catch (e: any) { message.error(e.message || t('common.error')); } }}>{t('settings.telegramTest')}</Button>
          </Space>
        </Form>
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
      <Card title={<><InfoCircleOutlined /> {t('settings.about')}</>}>
        <Space direction="vertical"><Text strong>3M-UI</Text><Text type="secondary">{t('app.title')}</Text><Tag color="blue">v1.0.0</Tag></Space>
      </Card>
    </div>
  );
};

export default Settings;
