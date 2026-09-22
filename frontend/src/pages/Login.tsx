import React, { useEffect, useRef, useState } from 'react';
import { Card, Form, Input, Button, Typography, message, Space, Dropdown, Steps, Alert, theme } from 'antd';
import {
  UserOutlined,
  LockOutlined,
  GlobalOutlined,
  BgColorsOutlined,
  SafetyOutlined,
  ArrowLeftOutlined,
} from '../icons';
import { useNavigate, useLocation } from 'react-router-dom';
import { login } from '../api/auth';
import { useAuthStore } from '../stores/authStore';
import { useI18n, LOCALE_OPTIONS, type Locale } from '../i18n';
import { useThemeStore, type ThemeMode } from '../stores/themeStore';

const { Title, Text } = Typography;

const Login: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const [loading, setLoading] = useState(false);
  const [totpNeeded, setTotpNeeded] = useState(false);
  const [pendingCreds, setPendingCreds] = useState<{ username: string; password: string } | null>(null);
  const [form] = Form.useForm();
  const totpInputRef = useRef<any>(null);
  const { t, locale, setLocale } = useI18n();
  const { mode, setMode } = useThemeStore();
  const { token } = theme.useToken();
  const from = (location.state as { from?: { pathname?: string } } | null)?.from?.pathname || '/';

  useEffect(() => {
    if (totpNeeded) {
      // Focus OTP field after the second step renders
      const id = window.setTimeout(() => totpInputRef.current?.focus?.(), 80);
      return () => window.clearTimeout(id);
    }
  }, [totpNeeded]);

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
      const payload =
        pendingCreds && totpNeeded
          ? { ...pendingCreds, totp_code: (values.totp_code || '').trim() }
          : {
              username: values.username,
              password: values.password,
              totp_code: values.totp_code ? values.totp_code.trim() : undefined,
            };
      const result = await login(payload);
      if (result.totp_required && !result.token) {
        setPendingCreds({ username: payload.username, password: payload.password });
        setTotpNeeded(true);
        form.setFieldsValue({ totp_code: undefined });
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

  const backToPassword = () => {
    setTotpNeeded(false);
    setPendingCreds(null);
    form.setFieldsValue({ totp_code: undefined });
  };

  const langItems = LOCALE_OPTIONS.map((o) => ({ key: o.key, label: o.label }));
  const themeItems = [
    { key: 'light' as ThemeMode, label: t('settings.light') || 'Light' },
    { key: 'dark' as ThemeMode, label: t('settings.dark') || 'Dark' },
  ];

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        background: token.colorBgLayout,
        padding: '24px 0',
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
        <div style={{ textAlign: 'center', marginBottom: 20 }}>
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
          <Text type="secondary">{t('login.subtitle')}</Text>
        </div>

        {totpNeeded && (
          <div style={{ marginBottom: 16 }}>
            <Steps
              size="small"
              current={1}
              items={[
                { title: t('login.password') },
                { title: t('login.totpStep', 'Authenticator') },
              ]}
              style={{ marginBottom: 12 }}
            />
            <Alert
              type="info"
              showIcon
              icon={<SafetyOutlined />}
              message={t('login.totpRequired', 'Enter authenticator code')}
              description={t(
                'login.totpHint',
                'Open your authenticator app and enter the 6-digit code for this panel.',
              )}
              style={{ marginBottom: 8 }}
            />
          </div>
        )}

        <Form form={form} onFinish={onFinish} initialValues={{ username: 'admin' }} layout="vertical" requiredMark={false}>
          <Form.Item
            name="username"
            label={totpNeeded ? undefined : t('login.username')}
            rules={[{ required: !totpNeeded, message: t('login.username') }]}
            hidden={totpNeeded}
          >
            <Input prefix={<UserOutlined />} placeholder={t('login.username')} autoComplete="username" size="large" />
          </Form.Item>
          <Form.Item
            name="password"
            label={totpNeeded ? undefined : t('login.password')}
            rules={[{ required: !totpNeeded, message: t('login.password') }]}
            hidden={totpNeeded}
          >
            <Input.Password
              prefix={<LockOutlined />}
              placeholder={t('login.password')}
              autoComplete="current-password"
              size="large"
            />
          </Form.Item>
          {totpNeeded && (
            <Form.Item
              name="totp_code"
              label={t('login.totpLabel', 'Verification code')}
              rules={[
                { required: true, message: t('login.totpRequired', 'Authenticator code') },
                { pattern: /^\d{6,8}$/, message: t('login.totpFormat', 'Enter 6–8 digits') },
              ]}
            >
              <Input
                ref={totpInputRef}
                prefix={<SafetyOutlined />}
                placeholder="123456"
                inputMode="numeric"
                autoComplete="one-time-code"
                maxLength={8}
                size="large"
                style={{ letterSpacing: 6, fontSize: 20, textAlign: 'center' }}
                onChange={(e) => {
                  const v = e.target.value.replace(/\D/g, '').slice(0, 8);
                  form.setFieldsValue({ totp_code: v });
                }}
              />
            </Form.Item>
          )}
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Button type="primary" htmlType="submit" block loading={loading} size="large">
              {totpNeeded ? t('login.totpVerify', 'Verify') : t('login.button')}
            </Button>
            {totpNeeded && (
              <Button type="link" block icon={<ArrowLeftOutlined />} onClick={backToPassword} disabled={loading}>
                {t('login.totpBack', 'Back to password')}
              </Button>
            )}
          </Space>
        </Form>
      </Card>
    </div>
  );
};

export default Login;
