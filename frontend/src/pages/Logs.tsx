import React, { useEffect, useState } from 'react';
import { Card, List, Tag, Button, Space, Empty, Spin, message } from 'antd';
import { ReloadOutlined, ClearOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import client from '../api/client';
import { useI18n } from '../i18n';

interface LogEntry { timestamp: string; level: string; payload: string; }

const levelColor: Record<string, string> = { debug: 'default', info: 'blue', warn: 'orange', warning: 'orange', error: 'red', fatal: 'red' };

const Logs: React.FC = () => {
  const { t } = useI18n();
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);

  const load = async () => {
    setLoading(true);
    try { const { data } = await client.get<LogEntry[]>('/mihomo/logs'); setLogs(Array.isArray(data) ? data : []); }
    catch (e: any) { message.error(e.message || t('common.error')); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); if (!autoRefresh) return; const id = window.setInterval(load, 3000); return () => clearInterval(id); }, [autoRefresh]);

  return (
    <div>
      <h2>{t('logs.title')}</h2>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ReloadOutlined />} onClick={load}>{t('common.refresh')}</Button>
        <Button icon={<ClearOutlined />} onClick={() => setLogs([])}>{t('logs.clear')}</Button>
        <Button type={autoRefresh ? 'primary' : 'default'} onClick={() => setAutoRefresh(!autoRefresh)}>{t('logs.autoRefresh')}: {autoRefresh ? t('common.enabled') : t('common.disabled')}</Button>
      </Space>
      <Card>
        {loading && logs.length === 0 ? <Spin /> : logs.length === 0 ? <Empty description={t('logs.empty')} /> : (
          <List size="small" dataSource={logs} renderItem={(log, i) => (
            <List.Item key={i} style={{ fontFamily: 'monospace', fontSize: 13 }}>
              <span style={{ color: '#888', marginRight: 8 }}>[{dayjs(log.timestamp).format('YYYY-MM-DD HH:mm:ss')}]</span>
              <Tag color={levelColor[log.level?.toLowerCase()] || 'default'} style={{ marginRight: 8 }}>{log.level?.toUpperCase()}</Tag>
              <span>{log.payload}</span>
            </List.Item>
          )} />
        )}
      </Card>
    </div>
  );
};

export default Logs;
