import React, { useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Card, Form, Input, Button, Typography, message } from 'antd';
import { UserOutlined, LockOutlined } from '@ant-design/icons';
import { login } from '../api/auth';
import { useAuthStore } from '../stores/authStore';
import { useI18n } from '../i18n';

const { Title } = Typography;

const Login: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const [loading, setLoading] = useState(false);
  const { t } = useI18n();
  const from = (location.state as { from?: { pathname?: string } } | null)?.from?.pathname || '/';

  const onFinish = async (values: { username: string; password: string }) => {
    setLoading(true);
    try {
      const result = await login(values);
      message.success(t('login.welcomeBack'));
      // First-login / forced rotation must complete before the panel shell loads APIs.
      if (result.must_change_password || useAuthStore.getState().mustChangePassword) {
        navigate('/change-password', { replace: true });
      } else {
        navigate(from === '/login' || from === '/change-password' ? '/' : from, { replace: true });
      }
    } catch (e: any) {
      message.error(e?.message || t('login.failed'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      className="login-page"
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: '#f0f2f5',
        padding: '16px 0',
      }}
    >
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
          <Form.Item name="username" rules={[{ required: true, message: t('login.username') }]}>
            <Input prefix={<UserOutlined />} placeholder={t('login.username')} autoComplete="username" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: t('login.password') }]}>
            <Input.Password
              prefix={<LockOutlined />}
              placeholder={t('login.password')}
              autoComplete="current-password"
            />
          </Form.Item>
          <Button type="primary" htmlType="submit" block loading={loading}>
            {t('login.button')}
          </Button>
        </Form>
      </Card>
    </div>
  );
};

export default Login;
