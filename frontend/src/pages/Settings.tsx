import React from 'react';
import { Card, Button, Space, Typography, Tag, message, Upload } from 'antd';
import { useNavigate } from 'react-router-dom';
import { useI18n } from '../i18n';
import { useThemeStore, ThemeMode } from '../stores/themeStore';
import { LockOutlined, GlobalOutlined, BgColorsOutlined, InfoCircleOutlined, CloudDownloadOutlined, CloudUploadOutlined, ApiOutlined } from '@ant-design/icons';
import { downloadBackup, restoreDatabase, openApiUrl } from '../api/system';

const { Title, Text } = Typography;

const Settings: React.FC = () => {
  const { t, locale, setLocale } = useI18n();
  const { mode, setMode } = useThemeStore();
  const navigate = useNavigate();

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
        <div style={{ marginTop: 12 }}>
          <Button type="primary" onClick={() => navigate('/change-password')}>{t('settings.changePassword')}</Button>
        </div>
      </Card>

      <Card title={<><CloudDownloadOutlined /> {t('settings.backup')}</>} style={{ marginBottom: 16 }}>
        <Text type="secondary">{t('settings.backupHint')}</Text>
        <div style={{ marginTop: 12 }}>
          <Space wrap>
            <Button icon={<CloudDownloadOutlined />} onClick={async () => {
              try { await downloadBackup(); message.success(t('settings.backupDone')); }
              catch (e: any) { message.error(e.message || t('common.error')); }
            }}>{t('settings.downloadBackup')}</Button>
            <Upload
              accept=".db,application/octet-stream"
              showUploadList={false}
              beforeUpload={async (file) => {
                try {
                  await restoreDatabase(file as File);
                  message.success(t('settings.restoreDone'));
                } catch (e: any) {
                  message.error(e.message || t('common.error'));
                }
                return false;
              }}
            >
              <Button icon={<CloudUploadOutlined />}>{t('settings.restoreDb')}</Button>
            </Upload>
          </Space>
        </div>
      </Card>
      <Card title={<><ApiOutlined /> {t('settings.apiDocs')}</>} style={{ marginBottom: 16 }}>
        <Text type="secondary">{t('settings.apiDocsHint')}</Text>
        <div style={{ marginTop: 12 }}>
          <Button type="link" href={openApiUrl} target="_blank" rel="noreferrer">{t('settings.openOpenAPI')}</Button>
        </div>
      </Card>
      <Card title={<><InfoCircleOutlined /> {t('settings.about')}</>}>
        <Space direction="vertical">
          <Text strong>3M-UI</Text>
          <Text type="secondary">{t('app.title')}</Text>
          <Tag color="blue">v1.0.0</Tag>
        </Space>
      </Card>
    </div>
  );
};

export default Settings;
