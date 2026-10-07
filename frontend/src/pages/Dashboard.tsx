import React, { useCallback, useEffect, useState } from 'react';
import {
  Button,
  Card,
  Col,
  Progress,
  Row,
  Space,
  Table,
  Typography,
  message,
  theme,
  Tag,
} from 'antd';
import { Link } from 'react-router-dom';
import { IconPlay, IconStop, IconRestart } from '../icons';
import {
  fetchDashboard,
  startMihomo,
  stopMihomo,
  restartMihomo,
  isTransientNetworkError,
  type DashboardResponse,
} from '../api/system';
import { isCanceledError } from '../api/client';
import { useI18n } from '../i18n';
import useIsMobile from '../hooks/useIsMobile';
import { formatBytes } from '../utils/format';
import { startVisiblePolling } from '../utils/visiblePolling';

const { Text, Title } = Typography;

const DASHBOARD_POLL_MS = 1000;
const HISTORY_LEN = 90;

const formatRate = (bps: number) => `${formatBytes(bps)}/s`;
const clampPct = (v: unknown) => {
  const n = Number(v);
  if (!Number.isFinite(n) || n < 0) return 0;
  if (n > 100) return 100;
  return Math.round(n * 10) / 10;
};

function pushHistory(buf: number[], value: number, max = HISTORY_LEN): number[] {
  const next = buf.length >= max ? buf.slice(buf.length - max + 1) : buf.slice();
  next.push(Number.isFinite(value) ? value : 0);
  return next;
}

/** m-ui style metric card: title + icon slot, big number, optional progress / hint */
const MetricCard: React.FC<{
  title: string;
  value: React.ReactNode;
  hint?: React.ReactNode;
  progress?: number;
  progressColor?: string;
  loading?: boolean;
}> = ({ title, value, hint, progress, progressColor, loading }) => {
  const { token } = theme.useToken();
  return (
    <Card
      size="small"
      loading={loading}
      styles={{ body: { padding: '16px 18px' } }}
      style={{ borderRadius: 10, height: '100%' }}
    >
      <div style={{ fontSize: 13, fontWeight: 500, color: token.colorTextSecondary, marginBottom: 8 }}>
        {title}
      </div>
      <div
        style={{
          fontSize: 26,
          fontWeight: 700,
          lineHeight: 1.2,
          fontVariantNumeric: 'tabular-nums',
          letterSpacing: '-0.02em',
        }}
      >
        {value}
      </div>
      {progress != null ? (
        <Progress
          percent={clampPct(progress)}
          showInfo={false}
          size="small"
          strokeColor={progressColor || token.colorPrimary}
          trailColor={token.colorFillSecondary}
          style={{ marginTop: 12, marginBottom: 0 }}
        />
      ) : null}
      {hint ? (
        <div style={{ marginTop: 8, fontSize: 12, color: token.colorTextSecondary }}>{hint}</div>
      ) : null}
    </Card>
  );
};

const RateChart: React.FC<{ up: number[]; down: number[]; height?: number }> = ({
  up,
  down,
  height = 160,
}) => {
  const { token } = theme.useToken();
  const w = 480;
  const h = height;
  const max = Math.max(1, ...up, ...down);
  const path = (data: number[]) => {
    if (data.length < 2) return '';
    return data
      .map((v, i) => {
        const x = (i / (data.length - 1)) * w;
        const y = h - (v / max) * (h - 12) - 6;
        return `${x.toFixed(1)},${y.toFixed(1)}`;
      })
      .join(' ');
  };
  return (
    <svg
      width="100%"
      height={h}
      viewBox={`0 0 ${w} ${h}`}
      preserveAspectRatio="none"
      style={{ display: 'block' }}
      aria-hidden
    >
      {[0.25, 0.5, 0.75].map((f) => (
        <line
          key={f}
          x1={0}
          x2={w}
          y1={h * f}
          y2={h * f}
          stroke={token.colorBorderSecondary}
          strokeDasharray="3 3"
        />
      ))}
      <polyline points={path(down)} fill="none" stroke={token.colorSuccess} strokeWidth={2} strokeLinejoin="round" />
      <polyline
        points={path(up)}
        fill="none"
        stroke={token.colorError}
        strokeWidth={2}
        strokeLinejoin="round"
        strokeDasharray="4 3"
      />
    </svg>
  );
};

const DashboardPage: React.FC = () => {
  const { t } = useI18n();
  const { token } = theme.useToken();
  const isMobile = useIsMobile();
  const [data, setData] = useState<DashboardResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);
  const [hist, setHist] = useState({ up: [] as number[], down: [] as number[] });

  const load = useCallback(async (signal?: AbortSignal) => {
    try {
      const res = await fetchDashboard(signal);
      setData(res);
      setUpdatedAt(new Date());
      setHist((prev) => ({
        up: pushHistory(prev.up, Number(res.traffic?.uploadRate) || 0),
        down: pushHistory(prev.down, Number(res.traffic?.downloadRate) || 0),
      }));
    } catch (e) {
      if (isCanceledError(e)) return;
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!autoRefresh) {
      load();
      return;
    }
    return startVisiblePolling(() => load(), DASHBOARD_POLL_MS);
  }, [load, autoRefresh]);

  const runAction = async (fn: () => Promise<unknown>, okMsg: string) => {
    setActionLoading(true);
    try {
      await fn();
      message.success(okMsg);
      await load();
    } catch (e: any) {
      if (isTransientNetworkError(e)) {
        message.success(okMsg);
        setTimeout(() => load(), 1500);
      } else {
        message.error(e?.message || t('common.error'));
      }
    } finally {
      setActionLoading(false);
    }
  };

  const mihomo = data?.mihomo;
  const running = !!mihomo?.running;
  const sys = data?.system;
  const traffic = data?.traffic;
  const users = data?.users;
  const panel = data?.panel;
  const core = data?.core;

  const cpuPct = clampPct(sys?.cpu?.percent);
  const memPct = clampPct(sys?.memory?.percent);
  const diskPct = clampPct(sys?.disk?.percent);
  const online = users?.online ?? traffic?.onlineUsers ?? 0;
  const upRate = Number(traffic?.uploadRate) || 0;
  const downRate = Number(traffic?.downloadRate) || 0;
  const tcp = Number(traffic?.tcpConnections) || 0;
  const udp = Number(traffic?.udpConnections) || 0;
  const active = Number(traffic?.activeConnections) || tcp + udp;

  const timeStr = updatedAt
    ? updatedAt.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    : '—';

  const gutter: [number, number] = isMobile ? [10, 10] : [16, 16];
  const showLoading = loading && !data;

  return (
    <div style={{ maxWidth: 1200, margin: '0 auto' }}>
      {/* Header — m-ui: title + subtitle + controls */}
      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'flex-start',
          justifyContent: 'space-between',
          gap: 12,
          marginBottom: isMobile ? 14 : 20,
        }}
      >
        <div>
          <Title level={3} style={{ margin: 0, fontWeight: 700, letterSpacing: '-0.02em' }}>
            {t('dashboard.title') || t('nav.dashboard') || '仪表盘'}
          </Title>
          <Text type="secondary" style={{ fontSize: 13 }}>
            {t('dashboard.subtitle') || '运行时、系统监控与流量概览'}
          </Text>
        </div>
        <Space wrap size={8}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {t('dashboard.lastUpdated') || '最后更新'}: {timeStr}
          </Text>
          <Text type="secondary" style={{ fontSize: 12 }}>
            {autoRefresh
              ? `${t('dashboard.autoRefresh') || '自动刷新'}: ${DASHBOARD_POLL_MS / 1000}s`
              : t('dashboard.autoRefreshPaused') || '自动刷新已暂停'}
          </Text>
          <Button size="small" onClick={() => setAutoRefresh((v) => !v)}>
            {autoRefresh ? t('dashboard.pause') || '暂停' : t('dashboard.autoRefresh') || '自动刷新'}
          </Button>
          <Tag
            color={running ? 'success' : 'default'}
            style={{ margin: 0, lineHeight: '24px', borderRadius: 6 }}
          >
            {running ? t('dashboard.running') || '运行中' : t('dashboard.stopped') || '已停止'}
          </Tag>
          <Button size="small" onClick={() => load()} loading={loading && !!data}>
            {t('common.refresh') || '刷新'}
          </Button>
        </Space>
      </div>

      {/* Row 1 — system metrics (m-ui 4 cards) */}
      <Row gutter={gutter} style={{ marginBottom: gutter[1] }}>
        <Col xs={12} lg={6}>
          <MetricCard
            loading={showLoading}
            title={t('dashboard.systemCpu') || '系统 CPU'}
            value={`${cpuPct.toFixed(1)}%`}
            progress={cpuPct}
          />
        </Col>
        <Col xs={12} lg={6}>
          <MetricCard
            loading={showLoading}
            title={t('dashboard.systemMemory') || '系统内存'}
            value={`${memPct.toFixed(1)}%`}
            progress={memPct}
            progressColor="#f59e0b"
            hint={
              sys
                ? `${formatBytes(sys.memory.used)} / ${formatBytes(sys.memory.total)}`
                : undefined
            }
          />
        </Col>
        <Col xs={12} lg={6}>
          <MetricCard
            loading={showLoading}
            title={t('dashboard.systemDisk') || '系统硬盘'}
            value={`${diskPct.toFixed(1)}%`}
            progress={diskPct}
            progressColor="#10b981"
            hint={
              sys ? `${formatBytes(sys.disk.used)} / ${formatBytes(sys.disk.total)}` : undefined
            }
          />
        </Col>
        <Col xs={12} lg={6}>
          <MetricCard
            loading={showLoading}
            title={t('dashboard.onlineUsers') || '在线用户'}
            value={<span style={{ color: token.colorWarning }}>{online}</span>}
            hint={t('dashboard.recentActive') || '最近窗口活跃'}
          />
        </Col>
      </Row>

      {/* Row 2 — traffic / connections */}
      <Row gutter={gutter} style={{ marginBottom: gutter[1] }}>
        <Col xs={12} lg={6}>
          <MetricCard
            loading={showLoading}
            title={t('dashboard.uploadRate') || '上行速率'}
            value={formatRate(upRate)}
            hint={`${t('dashboard.totalUpload') || '累计上传'} ${formatBytes(traffic?.totalUpload || 0)}`}
          />
        </Col>
        <Col xs={12} lg={6}>
          <MetricCard
            loading={showLoading}
            title={t('dashboard.downloadRate') || '下行速率'}
            value={formatRate(downRate)}
            hint={`${t('dashboard.totalDownload') || '累计下载'} ${formatBytes(traffic?.totalDownload || 0)}`}
          />
        </Col>
        <Col xs={12} lg={6}>
          <MetricCard
            loading={showLoading}
            title={t('dashboard.activeConnections') || '活跃连接'}
            value={active}
            hint={`TCP ${tcp} · UDP ${udp}`}
          />
        </Col>
        <Col xs={12} lg={6}>
          <MetricCard
            loading={showLoading}
            title={t('dashboard.listeners') || '节点'}
            value={`${data?.listeners?.enabled ?? 0}/${data?.listeners?.total ?? 0}`}
            hint={t('dashboard.enabled') || '已启用 / 全部'}
          />
        </Col>
      </Row>

      {/* Row 3 — chart + runtime */}
      <Row gutter={gutter} style={{ marginBottom: gutter[1] }}>
        <Col xs={24} lg={14}>
          <Card
            size="small"
            title={t('dashboard.traffic') || '实时流量'}
            extra={
              <Text type="secondary" style={{ fontSize: 12 }}>
                ↓ {formatRate(downRate)} · ↑ {formatRate(upRate)}
              </Text>
            }
            styles={{ body: { padding: 16 } }}
            style={{ borderRadius: 10, height: '100%' }}
          >
            <div style={{ display: 'flex', gap: 16, marginBottom: 8, fontSize: 12, color: token.colorTextSecondary }}>
              <span>
                <span style={{ color: token.colorSuccess }}>━</span> {t('dashboard.download') || '下载'}
              </span>
              <span>
                <span style={{ color: token.colorError }}>┄</span> {t('dashboard.upload') || '上传'}
              </span>
            </div>
            <RateChart up={hist.up} down={hist.down} height={isMobile ? 140 : 180} />
          </Card>
        </Col>
        <Col xs={24} lg={10}>
          <Card
            size="small"
            title={
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
                Mihomo
                <Tag color={running ? 'success' : 'default'} style={{ margin: 0 }}>
                  {running ? t('dashboard.running') : t('dashboard.stopped')}
                </Tag>
              </span>
            }
            styles={{ body: { padding: 16 } }}
            style={{ borderRadius: 10, height: '100%' }}
          >
            <div style={{ display: 'grid', gap: 10, fontSize: 13 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <Text type="secondary">{t('dashboard.version') || '版本'}</Text>
                <Text strong>{mihomo?.version || '—'}</Text>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <Text type="secondary">{t('dashboard.uptime') || '运行时间'}</Text>
                <Text strong>{mihomo?.uptime || '—'}</Text>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <Text type="secondary">{t('dashboard.panel') || '面板'}</Text>
                <Text strong>
                  {panel?.memory_used != null ? formatBytes(panel.memory_used) : '—'}
                  {panel?.cpu_percent != null ? ` · ${clampPct(panel.cpu_percent)}%` : ''}
                  {panel?.pid ? ` · PID ${panel.pid}` : ''}
                </Text>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <Text type="secondary">{t('dashboard.core') || '核心'}</Text>
                <Text strong>
                  {running && core?.memory_used != null ? formatBytes(core.memory_used) : '—'}
                  {running && core?.cpu_percent != null ? ` · ${clampPct(core.cpu_percent)}%` : ''}
                  {running && core?.pid ? ` · PID ${core.pid}` : ''}
                </Text>
              </div>
            </div>
            <div style={{ marginTop: 16, display: 'flex', flexWrap: 'wrap', gap: 8 }}>
              {!running ? (
                <Button
                  type="primary"
                  icon={<IconPlay />}
                  loading={actionLoading}
                  onClick={() => runAction(startMihomo, t('dashboard.started'))}
                >
                  {t('dashboard.start')}
                </Button>
              ) : (
                <>
                  <Button
                    icon={<IconRestart />}
                    loading={actionLoading}
                    onClick={() => runAction(restartMihomo, t('dashboard.restarted'))}
                  >
                    {t('dashboard.restart')}
                  </Button>
                  <Button
                    danger
                    icon={<IconStop />}
                    loading={actionLoading}
                    onClick={() => runAction(stopMihomo, t('dashboard.stopped'))}
                  >
                    {t('dashboard.stop')}
                  </Button>
                </>
              )}
              <Link to="/logs">
                <Button>{t('nav.logs')}</Button>
              </Link>
              <Link to="/traffic">
                <Button>{t('nav.traffic')}</Button>
              </Link>
            </div>
          </Card>
        </Col>
      </Row>

      {/* Row 4 — quick stats table style (m-ui traffic users simplified) */}
      <Row gutter={gutter}>
        <Col xs={24} md={12}>
          <Card
            size="small"
            title={t('dashboard.users') || '用户'}
            styles={{ body: { padding: 0 } }}
            style={{ borderRadius: 10 }}
          >
            <Table
              size="small"
              pagination={false}
              showHeader
              dataSource={[
                {
                  key: 'online',
                  name: t('dashboard.onlineUsers') || '在线',
                  value: String(online),
                },
                {
                  key: 'enabled',
                  name: t('dashboard.enabled') || '已启用',
                  value: String(users?.enabled ?? '—'),
                },
                {
                  key: 'total',
                  name: t('dashboard.totalUsers') || '总数',
                  value: String(users?.total ?? '—'),
                },
              ]}
              columns={[
                { title: t('common.name') || '项目', dataIndex: 'name', key: 'name' },
                {
                  title: t('common.value') || '数值',
                  dataIndex: 'value',
                  key: 'value',
                  align: 'right' as const,
                },
              ]}
            />
          </Card>
        </Col>
        <Col xs={24} md={12}>
          <Card
            size="small"
            title={t('dashboard.listeners') || '节点'}
            styles={{ body: { padding: 0 } }}
            style={{ borderRadius: 10 }}
          >
            <Table
              size="small"
              pagination={false}
              dataSource={[
                {
                  key: 'en',
                  name: t('dashboard.enabled') || '已启用',
                  value: String(data?.listeners?.enabled ?? 0),
                },
                {
                  key: 'dis',
                  name: t('dashboard.disabled') || '已禁用',
                  value: String(data?.listeners?.disabled ?? 0),
                },
                {
                  key: 'tot',
                  name: t('common.total') || '全部',
                  value: String(data?.listeners?.total ?? 0),
                },
              ]}
              columns={[
                { title: t('common.name') || '项目', dataIndex: 'name', key: 'name' },
                {
                  title: t('common.value') || '数值',
                  dataIndex: 'value',
                  key: 'value',
                  align: 'right' as const,
                },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default DashboardPage;
