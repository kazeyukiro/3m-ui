import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, Form, Input, Button, Typography, message } from 'antd';
import { changePassword } from '../api/auth';
import { useI18n } from '../i18n';

const { Title } = Typography;

const ChangePassword: React.FC = () => {
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const { t } = useI18n();

  const onFinish = async (values: { current_password: string; new_password: string; confirm: string }) => {
    if (values.new_password !== values.confirm) { message.error(t('password.mismatch')); return; }
    setLoading(true);
    try { await changePassword(values.current_password, values.new_password); message.success(t('password.success')); navigate('/'); }
    catch (e: any) { message.error(e.message || t('password.failed')); }
    finally { setLoading(false); }
  };

  return (
    <div style={{ maxWidth: 480, margin: '0 auto' }}>
      <Card>
        <Title level={4}>{t('password.title')}</Title>
        <Form layout="vertical" onFinish={onFinish}>
          <Form.Item label={t('password.current')} name="current_password" rules={[{ required: true }]}><Input.Password /></Form.Item>
          <Form.Item label={t('password.new')} name="new_password" rules={[{ required: true, min: 8 }]}><Input.Password /></Form.Item>
          <Form.Item label={t('password.confirm')} name="confirm" rules={[{ required: true }]}><Input.Password /></Form.Item>
          <Button type="primary" htmlType="submit" loading={loading}>{t('password.button')}</Button>
        </Form>
      </Card>
    </div>
  );
};

export default ChangePassword;
