import React, { useEffect, useState } from 'react';
import { Card, Row, Col, Statistic, Button, Space, Tag, Progress, Typography, message } from 'antd';
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
function usageColor(pct: number): string | undefined {
  if (pct >= 80) return HIGH;
  if (pct >= 50) return MED;
  return undefined;
}

/**
 * Double-column equal-width number wall (no bars).
 * 4 cells: Panel CPU | Panel Mem | Core CPU | Core Mem
 * Mobile: 2 cols (2×2 grid); Desktop: 4 cols (1×4 row).
 * Big number + small label; color encodes high/medium usage.
 */
const ProcessUsageWall: React.FC<{
  panel: ProcSample | undefined;
  core: ProcSample | undefined;
  coreRunning: boolean;
  panelCpuLabel: string;
  panelMemLabel: string;
  coreCpuLabel: string;
  coreMemLabel: string;
  stoppedLabel: string;
}> = ({ panel, core, coreRunning, panelCpuLabel, panelMemLabel, coreCpuLabel, coreMemLabel, stoppedLabel }) => {
  const panelCpu = clampPct(panel?.cpu_percent);
  const panelMemPct = clampPct(panel?.memory_percent);
  const panelMemUsed = formatBytes(panel?.memory_used || 0);
  const coreCpu = clampPct(core?.cpu_percent);
  const coreMemPct = clampPct(core?.memory_percent);
  const coreMemUsed = formatBytes(core?.memory_used || 0);

  const cellStyle: React.CSSProperties = {
    textAlign: 'center',
    padding: '10px 4px',
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
  const labelStyle: React.CSSProperties = {
    fontSize: 12,
    color: 'rgba(0,0,0,0.45)',
    marginTop: 4,
    lineHeight: 1.4,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  };
  const stoppedStyle: React.CSSProperties = {
    ...numStyle(0),
    fontSize: 20,
    color: 'rgba(0,0,0,0.35)',
  };
  const border = '1px solid rgba(0,0,0,0.06)';

  return (
    <Row gutter={[0, 0]} style={{ borderTop: border, borderBottom: border }}>
      <Col xs={12} sm={6} style={{ borderRight: border }}>
        <div style={cellStyle}>
          <div style={numStyle(panelCpu)}>{panelCpu}%</div>
          <div style={labelStyle}>{panelCpuLabel}</div>
        </div>
      </Col>
      <Col xs={12} sm={6} style={{ borderRight: border }}>
        <div style={cellStyle}>
          <div style={numStyle(panelMemPct)}>{panelMemUsed}</div>
          <div style={labelStyle}>{panelMemLabel}</div>
        </div>
      </Col>
      <Col xs={12} sm={6} style={{ borderRight: border }}>
        <div style={cellStyle}>
          {coreRunning ? (
            <>
              <div style={numStyle(coreCpu)}>{coreCpu}%</div>
              <div style={labelStyle}>{coreCpuLabel}</div>
            </>
          ) : (
            <>
              <div style={stoppedStyle}>—</div>
              <div style={labelStyle}>{stoppedLabel}</div>
            </>
          )}
        </div>
      </Col>
      <Col xs={12} sm={6}>
        <div style={cellStyle}>
          {coreRunning ? (
            <>
              <div style={numStyle(coreMemPct)}>{coreMemUsed}</div>
              <div style={labelStyle}>{coreMemLabel}</div>
            </>
          ) : (
            <>
              <div style={stoppedStyle}>—</div>
              <div style={labelStyle}>{stoppedLabel}</div>
            </>
          )}
        </div>
      </Col>
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

        {/* Host resources: 3-up on desktop, stacked on phone */}
        <Col xs={24} sm={8}>
          <Card size={cardSize} title={`${t('dashboard.cpu')} ${clampPct(sys.cpu?.percent)}%`}>
            <Progress percent={clampPct(sys.cpu?.percent)} size="small" status={clampPct(sys.cpu?.percent) > 90 ? 'exception' : 'normal'} />
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card size={cardSize} title={`${t('dashboard.memory')} ${clampPct(sys.memory?.percent)}%`}>
            <Progress percent={clampPct(sys.memory?.percent)} size="small" status={clampPct(sys.memory?.percent) > 90 ? 'exception' : 'normal'} />
            <div style={muted}>{formatBytes(sys.memory?.used || 0)} / {formatBytes(sys.memory?.total || 0)}</div>
          </Card>
        </Col>
        <Col xs={24} sm={8}>
          <Card size={cardSize} title={`${t('dashboard.disk')} ${clampPct(sys.disk?.percent)}%`}>
            <Progress percent={clampPct(sys.disk?.percent)} size="small" status={clampPct(sys.disk?.percent) > 90 ? 'exception' : 'normal'} />
            <div style={muted}>{formatBytes(sys.disk?.used || 0)} / {formatBytes(sys.disk?.total || 0)}</div>
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
              stoppedLabel={t('dashboard.stoppedStatus')}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
