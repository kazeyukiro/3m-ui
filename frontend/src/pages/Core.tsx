import { useState, useEffect } from 'react';
import { Alert, Button, Card, Descriptions, Space, Spin, message } from 'antd';
import { PlayCircleOutlined, StopOutlined, RedoOutlined } from '@ant-design/icons';
import client from '../api/client';
import { useI18n } from '../i18n';

type Status = { running: boolean; version: string; pid: number; uptime: string };

export default function Core() {
  const { t } = useI18n();
  const [status, setStatus] = useState<Status | null>(null);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  const load = async (showError = true) => {
    try {
      const { data } = await client.get('/mihomo/status');
      setStatus(data);
      return true;
    } catch (e: any) {
      if (showError) message.error(e.message || t('core.unavailable'));
      return false;
    } finally { setLoading(false); }
  };

  useEffect(() => { load(); const id = window.setInterval(load, 5000); return () => window.clearInterval(id); }, []);

  const action = async (path: string, successKey: 'started' | 'stopped' | 'restarted') => {
    setBusy(true);
    try {
      await client.post(`/mihomo/${path}`);
      message.success(t(`core.${successKey}`));
      if (!(await load(false))) message.warning(t('core.unavailable'));
    }
    catch (e: any) { message.error(e.message || t('core.operationFailed')); }
    finally { setBusy(false); }
  };

  return (
    <div>
      <h2>{t('core.title')}</h2>
      {loading ? <Spin /> : (
        <Card>
          <Space direction="vertical" style={{ width: '100%' }} size="large">
            <Alert message={status?.running ? t('core.running') : t('core.stopped')} type={status?.running ? 'success' : 'warning'} showIcon />
            <Descriptions bordered column={1}>
              <Descriptions.Item label={t('core.version')}>{status?.version || '-'}</Descriptions.Item>
              <Descriptions.Item label={t('core.pid')}>{status?.pid || '-'}</Descriptions.Item>
              <Descriptions.Item label={t('core.uptime')}>{status?.uptime || '-'}</Descriptions.Item>
            </Descriptions>
            <Space>
              <Button icon={<PlayCircleOutlined />} loading={busy} onClick={() => action('start', 'started')}>{t('core.start')}</Button>
              <Button icon={<StopOutlined />} danger loading={busy} onClick={() => action('stop', 'stopped')}>{t('core.stop')}</Button>
              <Button icon={<RedoOutlined />} loading={busy} onClick={() => action('restart', 'restarted')}>{t('core.restart')}</Button>
            </Space>
          </Space>
        </Card>
      )}
    </div>
  );
}
