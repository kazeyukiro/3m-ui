import React, { useCallback, useEffect, useState } from 'react';
import { Button, Card, Col, Row, Typography, message, theme } from 'antd';
import { Link } from 'react-router-dom';
import {
  IconPlay,
  IconStop,
  IconRestart,
  IconNavLogs,
  IconNavConfig,
  IconChart,
  IconHistory,
  IconDownload,
  IconDisk,
  IconCloudUp,
  IconCloudDown,
  IconUser,
} from '../icons';
import {
  fetchDashboard,
  startMihomo,
  stopMihomo,
  restartMihomo,
  downloadBackup,
  isTransientNetworkError,
  type DashboardResponse,
  type ProcessUsageSample,
} from '../api/system';
import { isCanceledError } from '../api/client';
import { useI18n } from '../i18n';
import useIsMobile from '../hooks/useIsMobile';
import { formatBytes } from '../utils/format';
import { startVisiblePolling } from '../utils/visiblePolling';

const { Text } = Typography;

/** Dashboard refresh cadence — CPU, memory, traffic read live. */
const DASHBOARD_POLL_MS = 1000;

const clampPct = (v: unknown) => {
  const n = Number(v);
  if (!Number.isFinite(n) || n < 0) return 0;
  if (n > 100) return 100;
  return Math.round(n * 10) / 10;
};

const formatRate = (bps: number) => `${formatBytes(bps)}/s`;

/* ---------- Ring gauge — thin stroke, 3X-UI style ---------- */
const Gauge: React.FC<{ pct: number; label: string; sub?: string; size?: number }> = ({
  pct,
  label,
  sub,
  size = 76,
}) => {
  const { token } = theme.useToken();
  const r = 31;
  const stroke = 5;
  const C = 2 * Math.PI * r;
  const pctVal = clampPct(pct);
  const off = C * (1 - pctVal / 100);
  const track = token.colorFillSecondary;
  const color = token.colorPrimary;
  const pctText = pctVal.toFixed(pctVal % 1 === 0 ? 0 : 2) + '%';
  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 10, minWidth: 140 }}>
      <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} aria-hidden>
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke={track} strokeWidth={stroke} />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke={color}
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={C.toFixed(2)}
          strokeDashoffset={off.toFixed(2)}
          transform={`rotate(-90 ${size / 2} ${size / 2})`}
        />
        <text x={size / 2} y={size / 2 + 4.5} textAnchor="middle" fontSize={14} fontWeight={600} fill={token.colorText}>
          {pctText}
        </text>
      </svg>
      <div style={{ fontSize: 13, color: token.colorTextSecondary, textAlign: 'center' }}>
        <b style={{ color: token.colorText }}>{label}</b>
        {sub ? `: ${sub}` : ''}
      </div>
    </div>
  );
};

/* ---------- Panel card: title → divider → content ---------- */
const PanelCard: React.FC<{
  title: string;
  status?: React.ReactNode;
  isMobile: boolean;
  children: React.ReactNode;
}> = ({ title, status, isMobile, children }) => {
  const { token } = theme.useToken();
  return (
    <Card size="small" styles={{ body: { padding: 0 } }} style={{ borderRadius: 12, height: '100%' }}>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: isMobile ? '12px 14px' : '14px 16px',
        }}
      >
        <Text strong style={{ fontSize: 15 }}>
          {title}
        </Text>
        {status}
      </div>
      <div style={{ height: 1, background: token.colorBorderSecondary }} />
      <div style={{ padding: isMobile ? 12 : 14 }}>{children}</div>
    </Card>
  );
};

/* ---------- Cell row with vertical separators ---------- */
const Cell: React.FC<{ first?: boolean; isMobile: boolean; children: React.ReactNode }> = ({
  first,
  isMobile,
  children,
}) => {
  const { token } = theme.useToken();
  return (
    <div
      style={{
        flex: '1 1 0',
        minWidth: 92,
        display: 'flex',
        alignItems: 'center',
        gap: 7,
        padding: isMobile ? '11px 12px' : '13px 14px',
        color: token.colorTextSecondary,
        fontSize: 13,
        fontWeight: 500,
        borderLeft: first ? 'none' : `1px solid ${token.colorBorderSecondary}`,
        whiteSpace: 'nowrap',
        overflow: 'hidden',
      }}
    >
      {children}
    </div>
  );
};

const CellsRow: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div style={{ display: 'flex', flexWrap: 'wrap' }}>{children}</div>
);

/* ---------- Metric: label above value ---------- */
const Metric: React.FC<{ label: string; children: React.ReactNode }> = ({ label, children }) => {
  const { token } = theme.useToken();
  return (
    <div style={{ flex: '1 1 0', minWidth: 0 }}>
      <div style={{ fontSize: 12, color: token.colorTextSecondary, marginBottom: 6 }}>{label}</div>
      <div style={{ display: 'flex', alignItems: 'center', gap: 7, fontSize: 14, fontWeight: 600, color: token.colorText }}>
        {children}
      </div>
    </div>
  );
};

const MetricRow: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div style={{ display: 'flex', gap: 18 }}>{children}</div>
);

const linkStyle: React.CSSProperties = { display: 'flex', alignItems: 'center', gap: 7, color: 'inherit' };
const linkBtnStyle: React.CSSProperties = { padding: 0, height: 'auto' };

const Dashboard: React.FC = () => {
  const { t } = useI18n();
  const isMobile = useIsMobile();
  const { token } = theme.useToken();
  const [data, setData] = useState<DashboardResponse | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(
    async (signal?: AbortSignal) => {
      try {
        const d = await fetchDashboard(signal);
        if (signal?.aborted) return;
        setData(d);
      } catch (e: unknown) {
        if (signal?.aborted || isCanceledError(e)) return;
        const msg = e instanceof Error ? e.message : t('dashboard.unavailable');
        message.error(msg || t('dashboard.unavailable'));
      }
    },
    [t],
  );

  useEffect(() => startVisiblePolling((signal) => load(signal), DASHBOARD_POLL_MS), [load]);

  const act = async (a: 'start' | 'stop' | 'restart') => {
    setBusy(true);
    try {
      if (a === 'start') await startMihomo();
      else if (a === 'stop') await stopMihomo();
      else await restartMihomo();
      message.success(t(`dashboard.${a === 'start' ? 'started' : a === 'stop' ? 'stopped' : 'restarted'}`));
      await load();
    } catch (e: unknown) {
      if (a === 'restart' && isTransientNetworkError(e)) {
        await new Promise((r) => setTimeout(r, 1500));
        try {
          await load();
          message.success(t('dashboard.restarted'));
          return;
        } catch {
          /* fall through */
        }
      }
      const msg = e instanceof Error ? e.message : t('dashboard.operationFailed');
      message.error(msg || t('dashboard.operationFailed'));
    } finally {
      setBusy(false);
    }
  };

  const backup = async () => {
    try {
      await downloadBackup();
      message.success(t('dashboard.backupDone'));
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : t('dashboard.operationFailed');
      message.error(msg || t('dashboard.operationFailed'));
    }
  };

  const sys = data?.system;
  const users = data?.users;
  const traffic = data?.traffic;
  const core: ProcessUsageSample | undefined = data?.core;
  const panel: ProcessUsageSample | undefined = data?.panel;
  const running = !!data?.mihomo?.running;

  const cpuPct = clampPct(sys?.cpu?.percent);
  const memPct = clampPct(sys?.memory?.percent);
  const diskPct = clampPct(sys?.disk?.percent);
  const coreCpuPct = clampPct(core?.cpu_percent);
  const upRate = traffic?.uploadRate || 0;
  const downRate = traffic?.downloadRate || 0;
  const online = users?.online ?? traffic?.onlineUsers ?? 0;
  const tcp = traffic?.tcpConnections ?? 0;
  const udp = traffic?.udpConnections ?? 0;
  const listeners = data?.listeners;

  const gutter: [number, number] = isMobile ? [8, 8] : [16, 16];

  const statusDot = (color: string) => (
    <span
      style={{
        width: 8,
        height: 8,
        borderRadius: '50%',
        background: color,
        boxShadow: `0 0 0 3px ${color}22`,
        display: 'inline-block',
      }}
    />
  );

  return (
    <div className="page-root" style={{ maxWidth: 1300 }}>
      {/* Top: ring gauges */}
      <Card
        size="small"
        styles={{ body: { padding: isMobile ? 16 : 22 } }}
        style={{ marginBottom: isMobile ? 8 : 16, borderRadius: 12 }}
      >
        <Row gutter={[16, 16]} justify="space-around" align="middle" wrap>
          <Col flex="1 1 140px">
            <Gauge pct={cpuPct} label={t('dashboard.cpu')} />
          </Col>
          <Col flex="1 1 140px">
            <Gauge
              pct={memPct}
              label={t('dashboard.memory')}
              sub={sys ? `${formatBytes(sys.memory.used)} / ${formatBytes(sys.memory.total)}` : undefined}
            />
          </Col>
          <Col flex="1 1 140px">
            <Gauge
              pct={diskPct}
              label={t('dashboard.disk')}
              sub={sys ? `${formatBytes(sys.disk.used)} / ${formatBytes(sys.disk.total)}` : undefined}
            />
          </Col>
          <Col flex="1 1 140px">
            <Gauge pct={coreCpuPct} label={t('dashboard.coreCpu')} />
          </Col>
        </Row>
      </Card>

      {/* Card grid */}
      <Row gutter={gutter}>
        {/* Mihomo */}
        <Col xs={24} md={12}>
          <PanelCard
            title={t('dashboard.coreName')}
            isMobile={isMobile}
            status={
              <span style={{ display: 'flex', alignItems: 'center', gap: 7, fontSize: 13 }}>
                {statusDot(running ? token.colorSuccess : token.colorError)}
                {running ? t('dashboard.running') : t('dashboard.stoppedStatus')}
              </span>
            }
          >
            <CellsRow>
              <Cell first isMobile={isMobile}>
                <Link to="/logs" style={linkStyle}>
                  <IconNavLogs size={15} />
                  {t('nav.logs')}
                </Link>
              </Cell>
              {running ? (
                <Cell isMobile={isMobile}>
                  <Button
                    type="link"
                    size="small"
                    style={linkBtnStyle}
                    danger
                    loading={busy}
                    icon={<IconStop size={15} />}
                    onClick={() => act('stop')}
                  >
                    {t('dashboard.stop')}
                  </Button>
                </Cell>
              ) : (
                <Cell isMobile={isMobile}>
                  <Button
                    type="link"
                    size="small"
                    style={linkBtnStyle}
                    loading={busy}
                    icon={<IconPlay size={15} />}
                    onClick={() => act('start')}
                  >
                    {t('dashboard.start')}
                  </Button>
                </Cell>
              )}
              <Cell isMobile={isMobile}>
                <Button
                  type="link"
                  size="small"
                  style={linkBtnStyle}
                  loading={busy}
                  disabled={!running}
                  icon={<IconRestart size={15} />}
                  onClick={() => act('restart')}
                >
                  {t('dashboard.restart')}
                </Button>
              </Cell>
              <Cell isMobile={isMobile}>
                <span style={{ display: 'flex', alignItems: 'center', gap: 7 }}>
                  <IconDisk size={15} />
                  {data?.mihomo?.version || '—'}
                </span>
              </Cell>
            </CellsRow>
            <div style={{ height: 1, background: token.colorBorderSecondary, margin: '2px 0' }} />
            <MetricRow>
              <Metric label={t('dashboard.panel')}>
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start', gap: 2 }}>
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                    {panel?.memory_used != null ? formatBytes(panel.memory_used) : '—'}
                    {panel?.cpu_percent != null && (
                      <span style={{ fontWeight: 400, fontSize: 12, color: token.colorTextSecondary }}>
                        {t('dashboard.cpu')} {clampPct(panel.cpu_percent)}%
                      </span>
                    )}
                  </span>
                  <span style={{ fontSize: 11, color: token.colorTextSecondary, fontWeight: 400 }}>
                    {panel?.pid ? `PID ${panel.pid}` : t('dashboard.panelUsage')}
                  </span>
                </div>
              </Metric>
              <Metric label={t('dashboard.core')}>
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start', gap: 2 }}>
                  <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6 }}>
                    {running && core?.memory_used != null ? formatBytes(core.memory_used) : '—'}
                    {running && core?.cpu_percent != null && (
                      <span style={{ fontWeight: 400, fontSize: 12, color: token.colorTextSecondary }}>
                        {t('dashboard.cpu')} {clampPct(core.cpu_percent)}%
                      </span>
                    )}
                  </span>
                  <span style={{ fontSize: 11, color: token.colorTextSecondary, fontWeight: 400 }}>
                    {running && core?.pid ? `PID ${core.pid}` : t('dashboard.coreUsage')}
                  </span>
                </div>
              </Metric>
            </MetricRow>
          </PanelCard>
        </Col>

        {/* Manage */}
        <Col xs={24} md={12}>
          <PanelCard title={t('dashboard.manage')} isMobile={isMobile}>
            <CellsRow>
              <Cell first isMobile={isMobile}>
                <Link to="/logs" style={linkStyle}>
                  <IconNavLogs size={15} />
                  {t('nav.logs')}
                </Link>
              </Cell>
              <Cell isMobile={isMobile}>
                <Link to="/config" style={linkStyle}>
                  <IconNavConfig size={15} />
                  {t('nav.config')}
                </Link>
              </Cell>
              <Cell isMobile={isMobile}>
                <Button
                  type="link"
                  size="small"
                  style={linkBtnStyle}
                  icon={<IconDownload size={15} />}
                  onClick={backup}
                >
                  {t('dashboard.backup')}
                </Button>
              </Cell>
            </CellsRow>
          </PanelCard>
        </Col>

        {/* Charts */}
        <Col xs={24} md={12}>
          <PanelCard title={t('dashboard.charts')} isMobile={isMobile}>
            <CellsRow>
              <Cell first isMobile={isMobile}>
                <Link to="/traffic" style={linkStyle}>
                  <IconHistory size={15} />
                  {t('dashboard.systemHistory')}
                </Link>
              </Cell>
              <Cell isMobile={isMobile}>
                <Link to="/traffic" style={linkStyle}>
                  <IconChart size={15} />
                  {t('dashboard.mihomoMetrics')}
                </Link>
              </Cell>
            </CellsRow>
          </PanelCard>
        </Col>

        {/* Uptime */}
        <Col xs={24} md={12}>
          <PanelCard title={t('dashboard.uptime')} isMobile={isMobile}>
            <MetricRow>
              <Metric label={t('dashboard.coreName')}>
                <IconHistory size={15} style={{ color: token.colorTextSecondary }} />
                {data?.mihomo?.uptime || '—'}
              </Metric>
              <Metric label={t('dashboard.coreCpu')}>
                <IconChart size={15} style={{ color: token.colorTextSecondary }} />
                {coreCpuPct}%
              </Metric>
            </MetricRow>
          </PanelCard>
        </Col>

        {/* Usage */}
        <Col xs={24} md={12}>
          <PanelCard title={t('dashboard.usage')} isMobile={isMobile}>
            <MetricRow>
              <Metric label={t('dashboard.memory')}>
                <IconDisk size={15} style={{ color: token.colorTextSecondary }} />
                {sys ? `${formatBytes(sys.memory.used)} / ${formatBytes(sys.memory.total)}` : '—'}
              </Metric>
              <Metric label={t('dashboard.disk')}>
                <IconDisk size={15} style={{ color: token.colorTextSecondary }} />
                {sys ? `${formatBytes(sys.disk.used)} / ${formatBytes(sys.disk.total)}` : '—'}
              </Metric>
            </MetricRow>
          </PanelCard>
        </Col>

        {/* Overall Speed */}
        <Col xs={24} md={12}>
          <PanelCard title={t('dashboard.overallSpeed')} isMobile={isMobile}>
            <MetricRow>
              <Metric label={t('dashboard.upload')}>
                <IconCloudUp size={15} style={{ color: token.colorTextSecondary }} />
                {formatRate(upRate)}
              </Metric>
              <Metric label={t('dashboard.download')}>
                <IconCloudDown size={15} style={{ color: token.colorTextSecondary }} />
                {formatRate(downRate)}
              </Metric>
            </MetricRow>
          </PanelCard>
        </Col>

        {/* Total Data */}
        <Col xs={24} md={12}>
          <PanelCard title={t('dashboard.totalData')} isMobile={isMobile}>
            <MetricRow>
              <Metric label={t('dashboard.sent')}>
                <IconCloudUp size={15} style={{ color: token.colorTextSecondary }} />
                {formatBytes(traffic?.totalUpload || 0)}
              </Metric>
              <Metric label={t('dashboard.received')}>
                <IconCloudDown size={15} style={{ color: token.colorTextSecondary }} />
                {formatBytes(traffic?.totalDownload || 0)}
              </Metric>
            </MetricRow>
          </PanelCard>
        </Col>

        {/* Connection Stats */}
        <Col xs={24} md={12}>
          <PanelCard title={t('dashboard.connectionStats')} isMobile={isMobile}>
            <MetricRow>
              <Metric label={t('dashboard.tcp')}>
                <IconChart size={15} style={{ color: token.colorTextSecondary }} />
                {tcp}
              </Metric>
              <Metric label={t('dashboard.udp')}>
                <IconChart size={15} style={{ color: token.colorTextSecondary }} />
                {udp}
              </Metric>
            </MetricRow>
          </PanelCard>
        </Col>

        {/* Nodes */}
        <Col xs={24} md={12}>
          <PanelCard
            title={t('dashboard.listeners')}
            isMobile={isMobile}
            status={
              <span style={{ display: 'flex', alignItems: 'center', gap: 7, fontSize: 13 }}>
                {statusDot(token.colorSuccess)}
                {listeners?.enabled ?? 0} / {listeners?.total ?? 0} {t('dashboard.enabled')}
              </span>
            }
          >
            <MetricRow>
              <Metric label={t('dashboard.enabled')}>
                {statusDot(token.colorSuccess)}
                <span style={{ marginLeft: 2 }}>{listeners?.enabled ?? 0}</span>
              </Metric>
              <Metric label={t('dashboard.disabled')}>
                {statusDot(token.colorError)}
                <span style={{ marginLeft: 2 }}>{listeners?.disabled ?? 0}</span>
              </Metric>
            </MetricRow>
          </PanelCard>
        </Col>

        {/* Users */}
        <Col xs={24} md={12}>
          <PanelCard
            title={t('dashboard.users')}
            isMobile={isMobile}
            status={
              <span style={{ display: 'flex', alignItems: 'center', gap: 7, fontSize: 13, color: token.colorWarning }}>
                {statusDot(token.colorWarning)}
                {online} {t('dashboard.onlineUsers')}
              </span>
            }
          >
            <MetricRow>
              <Metric label={t('dashboard.onlineUsers')}>
                <IconUser size={15} style={{ color: token.colorTextSecondary }} />
                {online}
              </Metric>
              <Metric label={t('dashboard.totalUsers')}>
                <IconUser size={15} style={{ color: token.colorTextSecondary }} />
                {users?.total ?? 0}
              </Metric>
            </MetricRow>
          </PanelCard>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
