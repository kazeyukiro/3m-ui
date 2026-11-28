import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, Form, Input, Button, Typography, message, Alert } from 'antd';
import { changePassword } from '../api/auth';
import { useAuthStore } from '../stores/authStore';
import { useI18n } from '../i18n';

const { Title } = Typography;

const ChangePassword: React.FC = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const { t } = useI18n();
  const token = useAuthStore((s) => s.token);
  const mustChange = useAuthStore((s) => s.mustChangePassword);

  useEffect(() => {
    if (!token) {
      navigate('/login', { replace: true });
    }
  }, [token, navigate]);

  const onFinish = async (values: { current_password: string; new_password: string; confirm: string }) => {
    if (values.new_password !== values.confirm) {
      message.error(t('password.mismatch'));
      return;
    }
    setLoading(true);
    try {
      await changePassword(values.current_password, values.new_password);
      message.success(t('password.success'));
      navigate('/', { replace: true });
    } catch (e: any) {
      const msg = e?.message || t('password.failed');
      message.error(msg);
      if (String(msg).toLowerCase().includes('log in')) {
        navigate('/login', { replace: true });
      }
    } finally {
      setLoading(false);
    }
  };

  if (!token) return null;

  return (
    <div style={{ maxWidth: 480, margin: '48px auto', padding: '0 16px' }}>
      <Card>
        <Title level={4}>{t('password.title')}</Title>
        {mustChange ? (
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message={t('password.requiredHint') || 'You must set a new password before using the panel.'}
          />
        ) : null}
        <Form layout="vertical" onFinish={onFinish}>
          <Form.Item label={t('password.current')} name="current_password" rules={[{ required: true }]}>
            <Input.Password autoComplete="current-password" />
          </Form.Item>
          <Form.Item label={t('password.new')} name="new_password" rules={[{ required: true, min: 8 }]}>
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Form.Item label={t('password.confirm')} name="confirm" rules={[{ required: true, min: 8 }]}>
            <Input.Password autoComplete="new-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" loading={loading} block>
            {t('password.button')}
          </Button>
        </Form>
      </Card>
    </div>
  );
};

export default ChangePassword;
