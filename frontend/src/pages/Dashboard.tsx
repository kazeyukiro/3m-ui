import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Statistic, Button, Space, Tag, Progress, Typography, Grid, message, theme } from 'antd';
import { PlayCircleOutlined, StopOutlined, RedoOutlined } from '@ant-design/icons';
import { fetchDashboard, startMihomo, stopMihomo, restartMihomo } from '../api/system';
import { isCanceledError } from '../api/client';
import { useI18n } from '../i18n';
import useIsMobile from '../hooks/useIsMobile';
import PageHeader from '../components/PageHeader';
import { formatBytes } from '../utils/format';
import { startVisiblePolling } from '../utils/visiblePolling';

const { Text } = Typography;

/** Dashboard refresh cadence — CPU, memory and traffic rates read as live. */
const DASHBOARD_POLL_MS = 1000;

const formatRate = (bps: number) => `${formatBytes(bps)}/s`;
const clampPct = (v: unknown) => {
  const n = Number(v);
  if (!Number.isFinite(n) || n < 0) return 0;
  if (n > 100) return 100;
  return Math.round(n * 10) / 10;
};

type ProcSample = {
  pid?: number;
  cpu_percent?: number;
  memory_used?: number;
  memory_percent?: number;
};

/**
 * Process usage, grouped by **process**: Panel | Core.
 *
 * A PID belongs to a process rather than to a metric, so it lives in the
 * group header. A flat four-cell wall can only park the two PIDs in the card
 * corner, where nothing says which one belongs to which process.
 *
 * Every metric leads with the **percentage** — the same number the colour is
 * derived from, so a red figure always has a visible reason. Absolute RSS
 * rides along as a neutral sub-line. A group with no sample renders "—"
 * instead of a misleading 0%; the card header already carries the
 * running/stopped state, so metrics keep their own label.
 */
const ProcessUsageWall: React.FC<{
  panel: ProcSample | undefined;
  core: ProcSample | undefined;
  coreRunning: boolean;
  panelLabel: string;
  coreLabel: string;
  cpuLabel: string;
  memLabel: string;
  pidLabel: string;
}> = ({ panel, core, coreRunning, panelLabel, coreLabel, cpuLabel, memLabel, pidLabel }) => {
  const { token } = theme.useToken();
  const screens = Grid.useBreakpoint();
  const stacked = !screens.sm; // xs: groups stacked; sm+: side by side

  /** Usage level → color. ≥80% high (red), ≥50% medium (orange), else inherit. */
  const usageColor = (pct: number): string | undefined => {
    if (pct >= 80) return token.colorError;
    if (pct >= 50) return token.colorWarning;
    return undefined;
  };

  const groups = [
    {
      key: 'panel',
      name: panelLabel,
      pid: panel?.pid,
      live: Boolean(panel),
      metrics: [
        { key: 'cpu', label: cpuLabel, pct: clampPct(panel?.cpu_percent), detail: '' },
        {
          key: 'mem',
          label: memLabel,
          pct: clampPct(panel?.memory_percent),
          detail: panel?.memory_used ? formatBytes(panel.memory_used) : '',
        },
      ],
    },
    {
      key: 'core',
      name: coreLabel,
      pid: core?.pid,
      live: coreRunning && Boolean(core),
      metrics: [
        { key: 'cpu', label: cpuLabel, pct: clampPct(core?.cpu_percent), detail: '' },
        {
          key: 'mem',
          label: memLabel,
          pct: clampPct(core?.memory_percent),
          detail: coreRunning && core?.memory_used ? formatBytes(core.memory_used) : '',
        },
      ],
    },
  ];

  const numStyle = (pct: number): React.CSSProperties => ({
    fontSize: 28,
    fontWeight: 700,
    lineHeight: 1.1,
    fontVariantNumeric: 'tabular-nums',
    letterSpacing: '-0.02em',
    color: usageColor(pct),
  });
  const detailStyle: React.CSSProperties = {
    fontSize: 11,
    color: token.colorTextSecondary,
    marginTop: 2,
    lineHeight: 1.3,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  };
  const labelStyle: React.CSSProperties = {
    fontSize: 12,
    color: token.colorTextSecondary,
    marginTop: 4,
    lineHeight: 1.4,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  };
  const emptyStyle: React.CSSProperties = {
    ...numStyle(0),
    color: token.colorTextDisabled,
  };
  const cellStyle: React.CSSProperties = { textAlign: 'center', padding: '10px 4px', minWidth: 0 };
  const border = `1px solid ${token.colorBorderSecondary}`;

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: stacked ? 'column' : 'row',
        border,
        borderRadius: 6,
        overflow: 'hidden',
      }}
    >
      {groups.map((g, i) => (
        <div
          key={g.key}
          style={{
            flex: 1,
            minWidth: 0,
            ...(stacked
              ? i > 0
                ? { borderTop: border }
                : null
              : i > 0
                ? { borderLeft: border }
                : null),
          }}
        >
          <div
            style={{
              display: 'flex',
              alignItems: 'baseline',
              justifyContent: 'space-between',
              gap: 8,
              padding: '6px 10px',
              borderBottom: border,
              background: token.colorFillQuaternary,
            }}
          >
            <span style={{ fontSize: 12, fontWeight: 600 }}>{g.name}</span>
            {g.live && g.pid ? (
              <span style={{ fontSize: 11, color: token.colorTextSecondary }}>
                {pidLabel} {g.pid}
              </span>
            ) : null}
          </div>
          <div style={{ display: 'flex' }}>
            {g.metrics.map((m, mi) => (
              <div
                key={m.key}
                style={{ ...cellStyle, flex: 1, ...(mi === 0 ? { borderRight: border } : null) }}
              >
                {g.live ? (
                  <>
                    <div style={numStyle(m.pct)}>{m.pct}%</div>
                    {/* Always reserve the sub-line so CPU and Mem baselines align. */}
                    <div style={detailStyle}>{m.detail || '\u00A0'}</div>
                  </>
                ) : (
                  <>
                    <div style={emptyStyle}>—</div>
                    <div style={detailStyle}>{'\u00A0'}</div>
                  </>
                )}
                <div style={labelStyle}>{m.label}</div>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
};

const Dashboard: React.FC = () => {
  const { t } = useI18n();
  const isMobile = useIsMobile();
  const { token } = theme.useToken();
  const [data, setData] = useState<any>(null);
  const [busy, setBusy] = useState(false);
  const cardSize = isMobile ? 'small' as const : 'default' as const;
  const gutter = isMobile ? ([8, 8] as [number, number]) : ([16, 16] as [number, number]);

  // Theme-adaptive muted text style (was a hardcoded rgba(0,0,0,0.45)).
  const muted: React.CSSProperties = { fontSize: 12, color: token.colorTextSecondary, lineHeight: 1.4 };

  /** Bar variant: always resolves, so a healthy bar reads green rather than neutral. */
  const usageBarColor = (pct: number): string => {
    if (pct >= 80) return token.colorError;
    if (pct >= 50) return token.colorWarning;
    return token.colorSuccess;
  };

  const load = async (signal?: AbortSignal) => {
    try {
      const d = await fetchDashboard(signal);
      if (signal?.aborted) return;
      setData(d);
    } catch (e: any) {
      if (signal?.aborted || isCanceledError(e)) return;
      message.error(e.message || t('dashboard.unavailable'));
    }
  };

  // Poll every second, but never stack requests: a slow response delays the
  // next one instead of piling up, and a hidden tab stops polling entirely.
  useEffect(() => startVisiblePolling((signal) => load(signal), DASHBOARD_POLL_MS), []);

  const act = async (a: 'start' | 'stop' | 'restart') => {
    setBusy(true);
    try {
      if (a === 'start') await startMihomo();
      else if (a === 'stop') await stopMihomo();
      else await restartMihomo();
      message.success(t(`dashboard.${a === 'start' ? 'started' : a === 'stop' ? 'stopped' : 'restarted'}`));
      load();
    } catch (e: any) {
      message.error(e.message || t('dashboard.operationFailed'));
    } finally {
      setBusy(false);
    }
  };

  const sys = data?.system || {};
  const users = data?.users || {};
  const coreRunning = !!data?.mihomo?.running;

  return (
    <div className="page-root" style={{ display: 'block' }}>
      <PageHeader title={t('dashboard.title')} subtitle={t('dashboard.subtitle')} />
      <Row gutter={gutter}>
        <Col xs={24} md={12} lg={8}>
          <Card size={cardSize} title={t('dashboard.users') || 'Users'}>
            <Statistic title={t('dashboard.onlineUsers') || 'Online'} value={users.online ?? data?.onlineUsers ?? 0} />
            <div style={{ marginTop: 8, ...muted }}>
              {(t('dashboard.totalUsers') || 'Total') + ': '}{users.total ?? 0}
              {' · '}
              {(t('dashboard.enabledUsers') || 'Enabled') + ': '}{users.enabled ?? 0}
            </div>
          </Card>
        </Col>

        <Col xs={24} md={12} lg={8}>
          <Card
            size={cardSize}
            title={
              <Space size={8} wrap>
                <span>{t('dashboard.status')}</span>
                <Tag color={coreRunning ? 'success' : 'default'}>
                  {coreRunning ? t('dashboard.running') : t('dashboard.stoppedStatus')}
                </Tag>
              </Space>
            }
          >
            <Space direction="vertical" size={isMobile ? 8 : 12} style={{ width: '100%' }}>
              <div style={{ fontSize: isMobile ? 13 : 14 }}>
                <Text type="secondary">{t('dashboard.version')}: </Text>
                {data?.mihomo?.version || '—'}
                {data?.mihomo?.pid ? (
                  <>
                    <Text type="secondary"> · PID </Text>
                    {data.mihomo.pid}
                  </>
                ) : null}
                {data?.mihomo?.uptime ? (
                  <>
                    <Text type="secondary"> · {t('dashboard.uptime')}: </Text>
                    {data.mihomo.uptime}
                  </>
                ) : null}
              </div>
              <Space wrap size={8}>
                <Button type="primary" icon={<PlayCircleOutlined />} onClick={() => act('start')} loading={busy} disabled={coreRunning}>
                  {t('dashboard.start')}
                </Button>
                <Button icon={<StopOutlined />} danger onClick={() => act('stop')} loading={busy} disabled={!coreRunning}>
                  {t('dashboard.stop')}
                </Button>
                <Button icon={<RedoOutlined />} onClick={() => act('restart')} loading={busy}>
                  {t('dashboard.restart')}
                </Button>
              </Space>
            </Space>
          </Card>
        </Col>

        <Col xs={24} md={12} lg={8}>
          <Card size={cardSize} title={t('dashboard.listeners')}>
            <Row gutter={isMobile ? [8, 8] : 16}>
              <Col span={8}><Statistic title={t('dashboard.total')} value={data?.listeners?.total || 0} /></Col>
              <Col span={8}><Statistic title={t('dashboard.enabled')} value={data?.listeners?.enabled || 0} valueStyle={{ color: token.colorSuccess }} /></Col>
              <Col span={8}><Statistic title={t('dashboard.disabled')} value={data?.listeners?.disabled || 0} valueStyle={{ color: token.colorError }} /></Col>
            </Row>
          </Card>
        </Col>

        <Col xs={24} md={12} lg={8}>
          <Card size={cardSize} title={t('dashboard.traffic')}>
            <Row gutter={[8, 8]}>
              <Col span={12}><Statistic title={t('dashboard.uploadRate')} value={formatRate(data?.traffic?.uploadRate || 0)} /></Col>
              <Col span={12}><Statistic title={t('dashboard.downloadRate')} value={formatRate(data?.traffic?.downloadRate || 0)} /></Col>
              <Col span={12}><Statistic title={t('dashboard.onlineUsers')} value={data?.traffic?.onlineUsers || 0} /></Col>
              <Col span={12}><Statistic title={t('dashboard.activeConnections')} value={data?.traffic?.activeConnections || 0} /></Col>
            </Row>
          </Card>
        </Col>

        {/* Host resources: one compact card (was three) with slim color-coded bars */}
        <Col xs={24} md={12} lg={8}>
          <Card size={cardSize} title={t('dashboard.system')}>
            <Space direction="vertical" size={isMobile ? 10 : 12} style={{ width: '100%' }}>
              {[
                { key: 'cpu', label: t('dashboard.cpu'), pct: clampPct(sys.cpu?.percent), detail: '' },
                {
                  key: 'memory',
                  label: t('dashboard.memory'),
                  pct: clampPct(sys.memory?.percent),
                  detail: `${formatBytes(sys.memory?.used || 0)} / ${formatBytes(sys.memory?.total || 0)}`,
                },
                {
                  key: 'disk',
                  label: t('dashboard.disk'),
                  pct: clampPct(sys.disk?.percent),
                  detail: `${formatBytes(sys.disk?.used || 0)} / ${formatBytes(sys.disk?.total || 0)}`,
                },
              ].map((m) => {
                const color = usageBarColor(m.pct);
                return (
                  <div key={m.key}>
                    <div style={{ display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 8 }}>
                      <span style={{ fontSize: 13, color: token.colorText }}>{m.label}</span>
                      <span style={{ fontSize: 13, fontWeight: 600, color, fontVariantNumeric: 'tabular-nums' }}>
                        {m.pct}%
                      </span>
                    </div>
                    <Progress
                      percent={m.pct}
                      showInfo={false}
                      size="small"
                      strokeColor={color}
                      trailColor={token.colorBorderSecondary}
                      strokeLinecap="butt"
                    />
                    {m.detail ? <div style={{ ...muted, fontSize: 11, marginTop: 2 }}>{m.detail}</div> : null}
                  </div>
                );
              })}
            </Space>
          </Card>
        </Col>

        {/* Process usage: Panel / Core groups, each carrying its own PID */}
        <Col xs={24}>
          <Card
            size={cardSize}
            title={
              <Space size={8} wrap>
                <span>{t('dashboard.processUsage', 'Process usage')}</span>
                <Tag color={coreRunning ? 'processing' : 'default'} style={{ margin: 0 }}>
                  {coreRunning ? t('dashboard.running') : t('dashboard.stoppedStatus')}
                </Tag>
              </Space>
            }
          >
            <ProcessUsageWall
              panel={data?.panel}
              core={data?.core}
              coreRunning={coreRunning}
              panelLabel={t('dashboard.panel', 'Panel')}
              coreLabel={t('dashboard.core', 'Core')}
              cpuLabel={t('dashboard.cpu')}
              memLabel={t('dashboard.memory')}
              pidLabel={t('dashboard.pid')}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
