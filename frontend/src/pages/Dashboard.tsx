import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Statistic, Button, Space, Tag, Progress, Typography, Grid, message } from 'antd';
import { PlayCircleOutlined, StopOutlined, RedoOutlined } from '@ant-design/icons';
import { fetchDashboard, startMihomo, stopMihomo, restartMihomo } from '../api/system';
import { useI18n } from '../i18n';
import useIsMobile from '../hooks/useIsMobile';
import PageHeader from '../components/PageHeader';
import { formatBytes } from '../utils/format';

const { Text } = Typography;

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

const muted: React.CSSProperties = { fontSize: 12, color: 'rgba(0,0,0,0.45)', lineHeight: 1.4 };

/** Usage level → color. ≥80% high (red), ≥50% medium (orange), else inherit. */
const HIGH = '#cf1322';
const MED = '#d46b08';
const LOW = '#3f8600';
function usageColor(pct: number): string | undefined {
  if (pct >= 80) return HIGH;
  if (pct >= 50) return MED;
  return undefined;
}

/** Bar variant: always resolves, so a healthy bar reads green rather than neutral. */
function usageBarColor(pct: number): string {
  if (pct >= 80) return HIGH;
  if (pct >= 50) return MED;
  return LOW;
}

/**
 * Equal-width number wall (no bars).
 * 4 cells: Panel CPU | Panel Mem | Core CPU | Core Mem
 * Mobile: 2 cols (2×2 grid); Desktop: 4 cols (1×4 row).
 *
 * Every cell leads with the **percentage** — the same number the colour is
 * derived from, so a red figure always has a visible reason. Absolute RSS
 * rides along as a neutral sub-line. A cell with no sample renders "—"
 * instead of a misleading 0%; the card header already carries the
 * running/stopped state, so cells keep their own label.
 */
const ProcessUsageWall: React.FC<{
  panel: ProcSample | undefined;
  core: ProcSample | undefined;
  coreRunning: boolean;
  panelCpuLabel: string;
  panelMemLabel: string;
  coreCpuLabel: string;
  coreMemLabel: string;
}> = ({ panel, core, coreRunning, panelCpuLabel, panelMemLabel, coreCpuLabel, coreMemLabel }) => {
  // Match the Col breakpoints exactly so separators never land on a row edge.
  const screens = Grid.useBreakpoint();
  const twoCol = !screens.sm;

  const cells = [
    {
      key: 'panel-cpu',
      label: panelCpuLabel,
      pct: clampPct(panel?.cpu_percent),
      detail: '',
      live: Boolean(panel),
    },
    {
      key: 'panel-mem',
      label: panelMemLabel,
      pct: clampPct(panel?.memory_percent),
      detail: panel?.memory_used ? formatBytes(panel.memory_used) : '',
      live: Boolean(panel),
    },
    {
      key: 'core-cpu',
      label: coreCpuLabel,
      pct: clampPct(core?.cpu_percent),
      detail: '',
      live: coreRunning && Boolean(core),
    },
    {
      key: 'core-mem',
      label: coreMemLabel,
      pct: clampPct(core?.memory_percent),
      detail: coreRunning && core?.memory_used ? formatBytes(core.memory_used) : '',
      live: coreRunning && Boolean(core),
    },
  ];

  const cellStyle: React.CSSProperties = {
    textAlign: 'center',
    padding: '12px 4px',
    minWidth: 0,
  };
  const numStyle = (pct: number): React.CSSProperties => ({
    fontSize: 30,
    fontWeight: 700,
    lineHeight: 1.1,
    fontVariantNumeric: 'tabular-nums',
    letterSpacing: '-0.02em',
    color: usageColor(pct),
  });
  const detailStyle: React.CSSProperties = {
    fontSize: 11,
    color: 'rgba(0,0,0,0.45)',
    marginTop: 2,
    lineHeight: 1.3,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  };
  const labelStyle: React.CSSProperties = {
    fontSize: 12,
    color: 'rgba(0,0,0,0.45)',
    marginTop: 4,
    lineHeight: 1.4,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  };
  const emptyStyle: React.CSSProperties = {
    ...numStyle(0),
    color: 'rgba(0,0,0,0.35)',
  };
  const border = '1px solid rgba(0,0,0,0.06)';
  // In the 2×2 layout only the left cell of each row gets a separator.
  const withRightBorder = (i: number) => (twoCol ? i % 2 === 0 : i < cells.length - 1);

  return (
    <Row gutter={[0, 0]} style={{ borderTop: border, borderBottom: border }}>
      {cells.map((c, i) => (
        <Col xs={12} sm={6} key={c.key} style={withRightBorder(i) ? { borderRight: border } : undefined}>
          <div style={cellStyle}>
            {c.live ? (
              <>
                <div style={numStyle(c.pct)}>{c.pct}%</div>
                {c.detail ? <div style={detailStyle}>{c.detail}</div> : null}
              </>
            ) : (
              <div style={emptyStyle}>—</div>
            )}
            <div style={labelStyle}>{c.label}</div>
          </div>
        </Col>
      ))}
    </Row>
  );
};

const Dashboard: React.FC = () => {
  const { t } = useI18n();
  const isMobile = useIsMobile();
  const [data, setData] = useState<any>(null);
  const [busy, setBusy] = useState(false);
  const cardSize = isMobile ? 'small' as const : 'default' as const;
  const gutter = isMobile ? ([8, 8] as [number, number]) : ([16, 16] as [number, number]);

  const load = async () => {
    try {
      const d = await fetchDashboard();
      setData(d);
    } catch (e: any) {
      message.error(e.message || t('dashboard.unavailable'));
    }
  };

  useEffect(() => {
    load();
    const id = window.setInterval(load, 10000);
    return () => clearInterval(id);
  }, []);

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
              <Col span={8}><Statistic title={t('dashboard.enabled')} value={data?.listeners?.enabled || 0} valueStyle={{ color: '#3f8600' }} /></Col>
              <Col span={8}><Statistic title={t('dashboard.disabled')} value={data?.listeners?.disabled || 0} valueStyle={{ color: '#cf1322' }} /></Col>
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
                      <span style={{ fontSize: 13, color: 'rgba(0,0,0,0.65)' }}>{m.label}</span>
                      <span style={{ fontSize: 13, fontWeight: 600, color, fontVariantNumeric: 'tabular-nums' }}>
                        {m.pct}%
                      </span>
                    </div>
                    <Progress
                      percent={m.pct}
                      showInfo={false}
                      size="small"
                      strokeColor={color}
                      trailColor="rgba(0,0,0,0.06)"
                      strokeLinecap="butt"
                    />
                    {m.detail ? <div style={{ ...muted, fontSize: 11, marginTop: 2 }}>{m.detail}</div> : null}
                  </div>
                );
              })}
            </Space>
          </Card>
        </Col>

        {/* Process usage: 4-cell number wall (Panel CPU/Mem + Core CPU/Mem) */}
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
            extra={
              <Space size={12}>
                {data?.panel?.pid ? <Text type="secondary" style={{ fontSize: 12 }}>PID {data.panel.pid}</Text> : null}
                {coreRunning && data?.core?.pid ? <Text type="secondary" style={{ fontSize: 12 }}>PID {data.core.pid}</Text> : null}
              </Space>
            }
          >
            <ProcessUsageWall
              panel={data?.panel}
              core={data?.core}
              coreRunning={coreRunning}
              panelCpuLabel={t('dashboard.panelCpu', 'Panel CPU')}
              panelMemLabel={t('dashboard.panelMem', 'Panel Mem')}
              coreCpuLabel={t('dashboard.coreCpu', 'Core CPU')}
              coreMemLabel={t('dashboard.coreMem', 'Core Mem')}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
