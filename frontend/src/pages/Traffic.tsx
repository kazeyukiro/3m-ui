import React, { useEffect, useState } from 'react';
import { Card, Table, Tag, Space, Statistic, Row, Col, message, Button } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { fetchTrafficStatus, fetchTrafficUsers, fetchConnections, UserTraffic, ConnectionView, TrafficStatus } from '../api/traffic';
import { useI18n } from '../i18n';
import { formatBytes } from '../utils/format';

const TrafficPage: React.FC = () => {
  const { t } = useI18n();
  const [status, setStatus] = useState<TrafficStatus | null>(null);
  const [users, setUsers] = useState<UserTraffic[]>([]);
  const [connections, setConnections] = useState<ConnectionView[]>([]);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    try {
      const [s, u, c] = await Promise.all([fetchTrafficStatus(), fetchTrafficUsers(), fetchConnections()]);
      setStatus(s);
      setUsers(u || []);
      setConnections(c || []);
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
    const id = window.setInterval(load, 8000);
    return () => window.clearInterval(id);
  }, []);

  const userColumns = [
    { title: t('users.username'), dataIndex: 'username', key: 'username' },
    {
      title: t('traffic.upload'),
      dataIndex: 'upload_bytes',
      key: 'up',
      render: (v: number) => formatBytes(v),
    },
    {
      title: t('traffic.download'),
      dataIndex: 'download_bytes',
      key: 'down',
      render: (v: number) => formatBytes(v),
    },
    {
      title: t('users.traffic'),
      key: 'quota',
      render: (_: any, r: UserTraffic) =>
        r.traffic_limit > 0
          ? `${formatBytes(r.traffic_used)} / ${formatBytes(r.traffic_limit)}`
          : `${formatBytes(r.traffic_used)} / ∞`,
    },
    {
      title: t('common.status'),
      key: 'st',
      render: (_: any, r: UserTraffic) => (
        <Space>
          {r.online ? <Tag color="success">{t('users.online')}</Tag> : <Tag>{t('users.offline')}</Tag>}
          {r.blocked && <Tag color="error">{t('users.blocked')}</Tag>}
        </Space>
      ),
    },
  ];

  const connColumns = [
    {
      title: t('users.username'),
      key: 'user',
      render: (_: any, r: ConnectionView) => r.username || '-',
    },
    {
      title: t('traffic.listener'),
      key: 'listener',
      render: (_: any, r: ConnectionView) => r.listener_name || r.listener_id || '-',
    },
    { title: t('traffic.network'), dataIndex: 'network', key: 'network' },
    {
      title: t('traffic.upload'),
      dataIndex: 'upload',
      key: 'up',
      render: (v: number) => formatBytes(v),
    },
    {
      title: t('traffic.download'),
      dataIndex: 'download',
      key: 'down',
      render: (v: number) => formatBytes(v),
    },
    { title: t('traffic.rule'), dataIndex: 'rule', key: 'rule', ellipsis: true },
  ];

  return (
    <div>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2 style={{ margin: 0 }}>{t('traffic.title')}</h2>
        <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
          {t('common.refresh')}
        </Button>
      </Space>
      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={12} md={6}>
          <Card>
            <Statistic title={t('traffic.uploadRate')} value={formatBytes(status?.upload_rate || 0) + '/s'} />
          </Card>
        </Col>
        <Col xs={12} md={6}>
          <Card>
            <Statistic title={t('traffic.downloadRate')} value={formatBytes(status?.download_rate || 0) + '/s'} />
          </Card>
        </Col>
        <Col xs={12} md={6}>
          <Card>
            <Statistic title={t('traffic.connections')} value={status?.connections || connections.length} />
          </Card>
        </Col>
        <Col xs={12} md={6}>
          <Card>
            <Statistic title={t('traffic.onlineUsers')} value={users.filter((u) => u.online).length} />
          </Card>
        </Col>
      </Row>
      <Card title={t('traffic.byUser')} style={{ marginBottom: 16 }}>
        <Table rowKey="user_id" loading={loading} dataSource={users} columns={userColumns} scroll={{ x: 720 }} size="middle" />
      </Card>
      <Card title={t('traffic.connections')}>
        <Table
          rowKey={(r, i) => r.id || String(i)}
          loading={loading}
          dataSource={connections}
          columns={connColumns}
          scroll={{ x: 800 }}
          size="middle"
          pagination={{ pageSize: 20 }}
        />
      </Card>
    </div>
  );
};

export default TrafficPage;
