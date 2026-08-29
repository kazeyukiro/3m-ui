import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Statistic, Button, Space, Tag, Progress, message } from 'antd';
import { PlayCircleOutlined, StopOutlined, RedoOutlined } from '@ant-design/icons';
import { fetchDashboard, startMihomo, stopMihomo, restartMihomo } from '../api/system';
import { useI18n } from '../i18n';
import { formatBytes } from '../utils/format';

const formatRate = (bps: number) => `${formatBytes(bps)}/s`;
const clampPct = (v: unknown) => {
  const n = Number(v);
  if (!Number.isFinite(n) || n < 0) return 0;
  if (n > 100) return 100;
  return Math.round(n * 10) / 10;
};

const Dashboard: React.FC = () => {
  const { t } = useI18n();
  const [data, setData] = useState<any>(null);
  const [busy, setBusy] = useState(false);

  const load = async () => {
    try { const d = await fetchDashboard(); setData(d); }
    catch (e: any) { message.error(e.message || t('dashboard.unavailable')); }
  };

  useEffect(() => { load(); const id = window.setInterval(load, 10000); return () => clearInterval(id); }, []);

  const act = async (a: 'start' | 'stop' | 'restart') => {
    setBusy(true);
    try {
      if (a === 'start') await startMihomo();
      else if (a === 'stop') await stopMihomo();
      else await restartMihomo();
      message.success(t(`dashboard.${a === 'start' ? 'started' : a === 'stop' ? 'stopped' : 'restarted'}`));
      load();
    } catch (e: any) { message.error(e.message || t('dashboard.operationFailed')); }
    finally { setBusy(false); }
  };

  const sys = data?.system || {};
  const users = data?.users || {};

  return (
    <div>
      <h2>{t('dashboard.title')}</h2>
      <p style={{ color: 'rgba(0,0,0,0.45)' }}>{t('dashboard.subtitle')}</p>
      <Row gutter={[16, 16]}>
        <Col xs={24} md={12} lg={8}>
          <Card title={t('dashboard.users') || 'Users'}>
            <Statistic title={t('dashboard.onlineUsers') || 'Online'} value={users.online ?? data?.onlineUsers ?? 0} />
            <div style={{ marginTop: 8, color: 'rgba(0,0,0,0.45)', fontSize: 13 }}>
              {(t('dashboard.totalUsers') || 'Total') + ': '}{users.total ?? 0}
              {' · '}
              {(t('dashboard.enabledUsers') || 'Enabled') + ': '}{users.enabled ?? 0}
            </div>
          </Card>
        </Col>
        <Col xs={24} md={12} lg={8}>
          <Card title={t('dashboard.status')}>
            <Space direction="vertical" style={{ width: '100%' }}>
              <Tag color={data?.mihomo?.running ? 'success' : 'error'}>{data?.mihomo?.running ? t('dashboard.running') : t('dashboard.stopped')}</Tag>
              <div>{t('dashboard.version')}: {data?.mihomo?.version || '-'}</div>
              <div>PID: {data?.mihomo?.pid || '-'} | {t('dashboard.uptime')}: {data?.mihomo?.uptime || '-'}</div>
              <Space>
                <Button icon={<PlayCircleOutlined />} onClick={() => act('start')} loading={busy}>{t('dashboard.start')}</Button>
                <Button icon={<StopOutlined />} danger onClick={() => act('stop')} loading={busy}>{t('dashboard.stop')}</Button>
                <Button icon={<RedoOutlined />} onClick={() => act('restart')} loading={busy}>{t('dashboard.restart')}</Button>
              </Space>
            </Space>
          </Card>
        </Col>
        <Col xs={24} md={12} lg={8}>
          <Card title={t('dashboard.listeners')}>
            <Row gutter={16}>
              <Col span={8}><Statistic title={t('dashboard.total')} value={data?.listeners?.total || 0} /></Col>
              <Col span={8}><Statistic title={t('dashboard.enabled')} value={data?.listeners?.enabled || 0} valueStyle={{ color: '#3f8600' }} /></Col>
              <Col span={8}><Statistic title={t('dashboard.disabled')} value={data?.listeners?.disabled || 0} valueStyle={{ color: '#cf1322' }} /></Col>
            </Row>
          </Card>
        </Col>
        <Col xs={24} md={12} lg={8}>
          <Card title={t('dashboard.traffic')}>
            <Row gutter={[8, 8]}>
              <Col span={12}><Statistic title={t('dashboard.uploadRate')} value={formatRate(data?.traffic?.uploadRate || 0)} /></Col>
              <Col span={12}><Statistic title={t('dashboard.downloadRate')} value={formatRate(data?.traffic?.downloadRate || 0)} /></Col>
              <Col span={12}><Statistic title={t('dashboard.onlineUsers')} value={data?.traffic?.onlineUsers || 0} /></Col>
              <Col span={12}><Statistic title={t('dashboard.activeConnections')} value={data?.traffic?.activeConnections || 0} /></Col>
            </Row>
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card title={`${t('dashboard.cpu')} ${clampPct(sys.cpu?.percent)}%`}><Progress percent={clampPct(sys.cpu?.percent)} size="small" status={clampPct(sys.cpu?.percent) > 90 ? 'exception' : 'normal'} /></Card>
        </Col>
        <Col xs={24} md={8}>
          <Card title={`${t('dashboard.memory')} ${clampPct(sys.memory?.percent)}%`}>
            <Progress percent={clampPct(sys.memory?.percent)} size="small" status={clampPct(sys.memory?.percent) > 90 ? 'exception' : 'normal'} />
            <div style={{ fontSize: 12, color: 'rgba(0,0,0,0.45)' }}>{formatBytes(sys.memory?.used || 0)} / {formatBytes(sys.memory?.total || 0)}</div>
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card title={`${t('dashboard.disk')} ${clampPct(sys.disk?.percent)}%`}>
            <Progress percent={clampPct(sys.disk?.percent)} size="small" status={clampPct(sys.disk?.percent) > 90 ? 'exception' : 'normal'} />
            <div style={{ fontSize: 12, color: 'rgba(0,0,0,0.45)' }}>{formatBytes(sys.disk?.used || 0)} / {formatBytes(sys.disk?.total || 0)}</div>
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
