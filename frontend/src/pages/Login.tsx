import React, { useState } from 'react';
import { Card, Form, Input, Button, Typography, message, Space, Dropdown } from 'antd';
import { UserOutlined, LockOutlined, GlobalOutlined, BgColorsOutlined } from '@ant-design/icons';
import { useNavigate, useLocation } from 'react-router-dom';
import { login } from '../api/auth';
import { useAuthStore } from '../stores/authStore';
import { useI18n, LOCALE_OPTIONS, type Locale } from '../i18n';
import { useThemeStore, type ThemeMode } from '../stores/themeStore';

const { Title } = Typography;

const Login: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const [loading, setLoading] = useState(false);
  const [totpNeeded, setTotpNeeded] = useState(false);
  const [pendingCreds, setPendingCreds] = useState<{ username: string; password: string } | null>(null);
  const { t, locale, setLocale } = useI18n();
  const { mode, setMode, isDark } = useThemeStore();
  const from = (location.state as { from?: { pathname?: string } } | null)?.from?.pathname || '/';

  const finishLogin = (result: { must_change_password?: boolean }) => {
    message.success(t('login.welcomeBack'));
    if (result.must_change_password || useAuthStore.getState().mustChangePassword) {
      navigate('/change-password', { replace: true });
    } else {
      navigate(from === '/login' || from === '/change-password' ? '/' : from, { replace: true });
    }
  };

  const onFinish = async (values: { username: string; password: string; totp_code?: string }) => {
    setLoading(true);
    try {
      const payload = pendingCreds && totpNeeded
        ? { ...pendingCreds, totp_code: values.totp_code }
        : { username: values.username, password: values.password, totp_code: values.totp_code };
      const result = await login(payload);
      if (result.totp_required && !result.token) {
        setPendingCreds({ username: payload.username, password: payload.password });
        setTotpNeeded(true);
        message.info(t('login.totpRequired', 'Enter authenticator code'));
        return;
      }
      setTotpNeeded(false);
      setPendingCreds(null);
      finishLogin(result);
    } catch (e: any) {
      message.error(e?.message || t('login.failed'));
    } finally {
      setLoading(false);
    }
  };

  const langItems = LOCALE_OPTIONS.map((o) => ({ key: o.key, label: o.label }));
  const themeItems = [
    { key: 'light', label: t('settings.light') || 'Light' },
    { key: 'dark', label: t('settings.dark') || 'Dark' },
    { key: 'system', label: t('settings.system') || 'System' },
  ];

  return (
    <div
      className="login-page"
      style={{
        minHeight: '100vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        background: isDark ? '#141414' : '#f0f2f5',
        padding: '16px 0',
        position: 'relative',
      }}
    >
      <div style={{ position: 'absolute', top: 16, right: 16 }}>
        <Space>
          <Dropdown
            menu={{
              items: themeItems,
              selectedKeys: [mode],
              onClick: (e) => setMode(e.key as ThemeMode),
            }}
          >
            <Button type="text" icon={<BgColorsOutlined />}>
              {t('settings.theme') || 'Theme'}
            </Button>
          </Dropdown>
          <Dropdown
            menu={{
              items: langItems,
              selectedKeys: [locale],
              onClick: (e) => setLocale(e.key as Locale),
            }}
          >
            <Button type="text" icon={<GlobalOutlined />}>
              {LOCALE_OPTIONS.find((o) => o.key === locale)?.label || locale}
            </Button>
          </Dropdown>
        </Space>
      </div>
      <Card style={{ width: '100%', maxWidth: 420, margin: '0 16px' }}>
        <div style={{ textAlign: 'center', marginBottom: 24 }}>
          <img
            src="/logo.png"
            alt="3m-ui"
            width={96}
            height={96}
            style={{ display: 'block', margin: '0 auto 12px', objectFit: 'contain' }}
          />
          <Title level={3} style={{ marginBottom: 4 }}>
            {t('login.title')}
          </Title>
          <Typography.Text type="secondary">{t('login.subtitle')}</Typography.Text>
        </div>
        <Form onFinish={onFinish} initialValues={{ username: 'admin' }}>
          <Form.Item name="username" rules={[{ required: !totpNeeded, message: t('login.username') }]} hidden={totpNeeded}>
            <Input prefix={<UserOutlined />} placeholder={t('login.username')} autoComplete="username" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: !totpNeeded, message: t('login.password') }]} hidden={totpNeeded}>
            <Input.Password
              prefix={<LockOutlined />}
              placeholder={t('login.password')}
              autoComplete="current-password"
            />
          </Form.Item>
          {totpNeeded && (
            <Form.Item name="totp_code" rules={[{ required: true, message: t('login.totpRequired', 'Authenticator code') }]}>
              <Input prefix={<LockOutlined />} placeholder="TOTP" inputMode="numeric" autoComplete="one-time-code" maxLength={8} />
            </Form.Item>
          )}
          <Button type="primary" htmlType="submit" block loading={loading}>
            {totpNeeded ? t('login.totpVerify', 'Verify') : t('login.button')}
          </Button>
        </Form>
      </Card>
    </div>
  );
};

export default Login;
