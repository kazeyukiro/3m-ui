import React, { useEffect, useState, useCallback } from 'react';
import { Card, List, Tag, Button, Space, Empty, Spin, message, theme, Tabs } from 'antd';
import { IconRefreshList, IconClear } from '../icons';
import dayjs from 'dayjs';
import client from '../api/client';
import { useI18n } from '../i18n';
import useIsMobile from '../hooks/useIsMobile';
import PageHeader from '../components/PageHeader';

interface LogEntry { timestamp: string; level: string; payload: string; }

const levelColor: Record<string, string> = { debug: 'default', info: 'blue', warn: 'orange', warning: 'orange', error: 'red', fatal: 'red' };

type LogSource = 'mihomo' | 'panel';

const Logs: React.FC = () => {
  const { t } = useI18n();
  const isMobile = useIsMobile();
  const { token } = theme.useToken();
  const [source, setSource] = useState<LogSource>('mihomo');
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const path = source === 'panel' ? '/system/panel-logs' : '/mihomo/logs';
      const { data } = await client.get<LogEntry[]>(path);
      setLogs(Array.isArray(data) ? data : []);
    } catch (e: any) {
      message.error(e.message || t('common.error'));
    } finally {
      setLoading(false);
    }
  }, [source, t]);

  useEffect(() => {
    load();
    if (!autoRefresh) return;
    const id = window.setInterval(load, 3000);
    return () => clearInterval(id);
  }, [autoRefresh, load]);

  const list = (
    <Card size={isMobile ? 'small' : 'default'}>
      {loading && logs.length === 0 ? (
        <Spin />
      ) : logs.length === 0 ? (
        <Empty description={t('logs.empty')} />
      ) : (
        <List
          size="small"
          dataSource={logs}
          renderItem={(log, i) => (
            <List.Item key={i} style={{ fontFamily: 'monospace', fontSize: 13 }}>
              <span style={{ color: token.colorTextTertiary, marginRight: 8 }}>
                [{dayjs(log.timestamp).format('YYYY-MM-DD HH:mm:ss')}]
              </span>
              <Tag color={levelColor[log.level?.toLowerCase()] || 'default'} style={{ marginRight: 8 }}>
                {log.level?.toUpperCase()}
              </Tag>
              <span>{log.payload}</span>
            </List.Item>
          )}
        />
      )}
    </Card>
  );

  return (
    <div>
      <PageHeader title={t('logs.title')} subtitle={t('logs.subtitle')} />
      <Space style={{ marginBottom: 16 }} wrap>
        <Button icon={<IconRefreshList />} onClick={load}>{t('common.refresh')}</Button>
        <Button icon={<IconClear />} onClick={() => setLogs([])}>{t('logs.clear')}</Button>
        <Button
          type={autoRefresh ? 'primary' : 'default'}
          onClick={() => setAutoRefresh(!autoRefresh)}
        >
          {t('logs.autoRefresh')}: {autoRefresh ? t('common.enabled') : t('common.disabled')}
        </Button>
      </Space>
      <Tabs
        activeKey={source}
        onChange={(k) => { setSource(k as LogSource); setLogs([]); }}
        items={[
          { key: 'mihomo', label: t('logs.tabCore') || 'Mihomo', children: list },
          { key: 'panel', label: t('logs.tabPanel') || 'Panel', children: list },
        ]}
      />
    </div>
  );
};

export default Logs;
