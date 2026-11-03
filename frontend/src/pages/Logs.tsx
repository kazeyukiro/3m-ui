import React, { useEffect, useState, useCallback, useMemo } from 'react';
import { Card, List, Tag, Button, Space, Empty, Spin, message, theme, Tabs, Input, Select } from 'antd';
import { IconRefreshList, IconClear, IconDownload, IconSearch } from '../icons';
import dayjs from 'dayjs';
import client from '../api/client';
import { useI18n } from '../i18n';
import useIsMobile from '../hooks/useIsMobile';
import PageHeader from '../components/PageHeader';
import { filterLogs, type LogEntry, type LogLevelFilter } from '../utils/logFilter';

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
  const [search, setSearch] = useState('');
  const [level, setLevel] = useState<LogLevelFilter>('all');

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

  const filtered = useMemo(() => filterLogs(logs, search, level), [logs, search, level]);

  const exportCsv = useCallback(() => {
    if (filtered.length === 0) {
      message.warning(t('logs.exportEmpty', 'Nothing to export'));
      return;
    }
    const header = ['timestamp', 'level', 'payload'].join(',');
    const rows = filtered.map((l) =>
      [l.timestamp, l.level, `"${(l.payload || '').replace(/"/g, '""')}"`].join(','),
    );
    const csv = [header, ...rows].join('\n');
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `3m-ui-${source}-logs-${dayjs().format('YYYYMMDD-HHmmss')}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    message.success(
      t('logs.exportSuccess', 'Exported {n} log lines').replace('{n}', String(filtered.length)),
    );
  }, [filtered, source, t]);

  const toolbar = (
    <Space style={{ marginBottom: 16 }} wrap>
      <Input
        allowClear
        prefix={<IconSearch />}
        placeholder={t('logs.search', 'Search')}
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        style={{ width: isMobile ? '100%' : 260 }}
      />
      <Select<LogLevelFilter>
        value={level}
        onChange={(v) => setLevel(v)}
        style={{ width: isMobile ? '100%' : 140 }}
        options={[
          { value: 'all', label: t('logs.all', 'All') },
          { value: 'info', label: t('logs.levelInfo', 'INFO') },
          { value: 'warning', label: t('logs.levelWarning', 'WARN') },
          { value: 'error', label: t('logs.levelError', 'ERROR') },
        ]}
      />
      <Button icon={<IconRefreshList />} onClick={load}>{t('common.refresh')}</Button>
      <Button
        icon={<IconClear />}
        onClick={() => {
          setLogs([]);
          setSearch('');
          setLevel('all');
        }}
      >
        {t('logs.clear')}
      </Button>
      <Button icon={<IconDownload />} onClick={exportCsv}>{t('logs.export', 'Export')}</Button>
      <Button
        type={autoRefresh ? 'primary' : 'default'}
        onClick={() => setAutoRefresh(!autoRefresh)}
      >
        {t('logs.autoRefresh')}: {autoRefresh ? t('common.enabled') : t('common.disabled')}
      </Button>
    </Space>
  );

  const list = (
    <Card size={isMobile ? 'small' : 'default'}>
      {loading && logs.length === 0 ? (
        <Spin />
      ) : filtered.length === 0 ? (
        <Empty
          description={
            logs.length === 0
              ? t('logs.empty')
              : t('logs.noMatch', 'No matching logs')
          }
        />
      ) : (
        <List
          size="small"
          dataSource={filtered}
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
      {toolbar}
      <Tabs
        activeKey={source}
        onChange={(k) => { setSource(k as LogSource); setLogs([]); setSearch(''); setLevel('all'); }}
        items={[
          { key: 'mihomo', label: t('logs.tabCore') || 'Mihomo', children: list },
          { key: 'panel', label: t('logs.tabPanel') || 'Panel', children: list },
        ]}
      />
    </div>
  );
};

export default Logs;
