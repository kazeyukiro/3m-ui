import React, { useCallback, useEffect, useState } from 'react';
import { Button, Card, Col, Row, Space, Tag, Typography, message, theme } from 'antd';
import { Link } from 'react-router-dom';
import { IconPlay, IconStop, IconRestart } from '../icons';
import {
  fetchDashboard,
  startMihomo,
  stopMihomo,
  restartMihomo,
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

/** Dashboard refresh cadence — CPU, memory and traffic rates read as live. */
const DASHBOARD_POLL_MS = 2000;
/** Ring buffer length for sparklines / speed chart (~2 min at 0.5 Hz). */
const HISTORY_LEN = 60;

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

/** Minimal SVG sparkline — no chart library dependency. */
const Sparkline: React.FC<{
  data: number[];
  color: string;
  height?: number;
  fillOpacity?: number;
}> = ({ data, color, height = 36, fillOpacity = 0.12 }) => {
  const w = 120;
  const h = height;
  if (data.length < 2) {
    return <svg width="100%" height={h} viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" />;
  }
  const min = Math.min(...data);
  const max = Math.max(...data);
  const span = max - min || 1;
  const pts = data.map((v, i) => {
    const x = (i / (data.length - 1)) * w;
    const y = h - ((v - min) / span) * (h - 4) - 2;
    return `${x.toFixed(2)},${y.toFixed(2)}`;
  });
  const line = pts.join(' ');
  const area = `0,${h} ${line} ${w},${h}`;
  return (
    <svg width="100%" height={h} viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" style={{ display: 'block' }}>
      <polygon points={area} fill={color} opacity={fillOpacity} />
      <polyline points={line} fill="none" stroke={color} strokeWidth={1.5} strokeLinejoin="round" strokeLinecap="round" />
    </svg>
  );
};

/** Dual-series area chart for upload / download rates. */
const SpeedChart: React.FC<{
  up: number[];
  down: number[];
  upColor: string;
  downColor: string;
  height?: number;
}> = ({ up, down, upColor, downColor, height = 160 }) => {
  const w = 400;
  const h = height;
  const n = Math.max(up.length, down.length, 2);
  const all = [...up, ...down];
  const max = Math.max(...all, 1);
  const toPts = (series: number[]) => {
    const pad = series.length < n ? Array(n - series.length).fill(0).concat(series) : series;
    return pad
      .map((v, i) => {
        const x = (i / (n - 1)) * w;
        const y = h - (v / max) * (h - 8) - 4;
        return `${x.toFixed(2)},${y.toFixed(2)}`;
      })
      .join(' ');
  };
  const upLine = toPts(up);
  const downLine = toPts(down);
  const upArea = `0,${h} ${upLine} ${w},${h}`;
  const downArea = `0,${h} ${downLine} ${w},${h}`;
  return (
    <svg width="100%" height={h} viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" style={{ display: 'block' }}>
      {[0.25, 0.5, 0.75].map((p) => (
        <line
          key={p}
          x1={0}
          x2={w}
          y1={h * p}
          y2={h * p}
          stroke="currentColor"
          strokeOpacity={0.08}
          strokeDasharray="4 4"
        />
      ))}
      <polygon points={downArea} fill={downColor} opacity={0.1} />
      <polyline points={downLine} fill="none" stroke={downColor} strokeWidth={2} strokeLinejoin="round" />
      <polygon points={upArea} fill={upColor} opacity={0.1} />
      <polyline points={upLine} fill="none" stroke={upColor} strokeWidth={2} strokeLinejoin="round" />
    </svg>
  );
};

type MetricCardProps = {
  title: string;
  value: string;
  unit?: string;
  detail?: string;
  peak?: string;
  series: number[];
  color: string;
  isMobile: boolean;
};

const MetricCard: React.FC<MetricCardProps> = ({ title, value, unit, detail, peak, series, color, isMobile }) => {
  const { token } = theme.useToken();
  return (
    <Card
      size="small"
      styles={{
        body: { padding: isMobile ? 12 : 16 },
      }}
      style={{ height: '100%', borderRadius: 12 }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 4 }}>
        <Text type="secondary" style={{ fontSize: 12, fontWeight: 600, letterSpacing: '0.02em' }}>
          {title}
        </Text>
        {peak ? (
          <Text type="secondary" style={{ fontSize: 11 }}>
            {peak}
          </Text>
        ) : null}
      </div>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 4, marginBottom: 4 }}>
        <span
          style={{
            fontSize: isMobile ? 28 : 34,
            fontWeight: 700,
            lineHeight: 1.1,
            fontVariantNumeric: 'tabular-nums',
            letterSpacing: '-0.02em',
            color: token.colorText,
          }}
        >
          {value}
        </span>
        {unit ? (
          <Text type="secondary" style={{ fontSize: 14, fontWeight: 500 }}>
            {unit}
          </Text>
        ) : null}
      </div>
      {detail ? (
        <div style={{ fontSize: 11, color: token.colorTextSecondary, marginBottom: 6, lineHeight: 1.3 }}>{detail}</div>
      ) : (
        <div style={{ height: 14, marginBottom: 6 }} />
      )}
      <Sparkline data={series} color={color} height={isMobile ? 28 : 36} />
    </Card>
  );
};

type HistoryState = {
  cpu: number[];
  mem: number[];
  disk: number[];
  up: number[];
  down: number[];
  conns: number[];
};

const emptyHistory = (): HistoryState => ({
  cpu: [],
  mem: [],
  disk: [],
  up: [],
  down: [],
  conns: [],
});

const Dashboard: React.FC = () => {
  const { t } = useI18n();
  const isMobile = useIsMobile();
  const { token } = theme.useToken();
  const [data, setData] = useState<DashboardResponse | null>(null);
  const [busy, setBusy] = useState(false);
  const [hist, setHist] = useState<HistoryState>(emptyHistory);

  const load = useCallback(async (signal?: AbortSignal) => {
    try {
      const d = await fetchDashboard(signal);
      if (signal?.aborted) return;
      setData(d);
      const sys = d.system;
      const tr = d.traffic;
      setHist((prev) => ({
        cpu: pushHistory(prev.cpu, clampPct(sys?.cpu?.percent)),
        mem: pushHistory(prev.mem, clampPct(sys?.memory?.percent)),
        disk: pushHistory(prev.disk, clampPct(sys?.disk?.percent)),
        up: pushHistory(prev.up, Number(tr?.uploadRate) || 0),
        down: pushHistory(prev.down, Number(tr?.downloadRate) || 0),
        conns: pushHistory(prev.conns, Number(tr?.activeConnections) || 0),
      }));
    } catch (e: unknown) {
      if (signal?.aborted || isCanceledError(e)) return;
      const msg = e instanceof Error ? e.message : t('dashboard.unavailable');
      message.error(msg || t('dashboard.unavailable'));
    }
  }, [t]);

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

  const sys = data?.system;
  const users = data?.users;
  const traffic = data?.traffic;
  const panel: ProcessUsageSample | undefined = data?.panel;
  const core: ProcessUsageSample | undefined = data?.core;
  const coreRunning = !!data?.mihomo?.running;

  const cpuPct = clampPct(sys?.cpu?.percent);
  const memPct = clampPct(sys?.memory?.percent);
  const diskPct = clampPct(sys?.disk?.percent);
  const online = users?.online ?? traffic?.onlineUsers ?? 0;
  const conns = traffic?.activeConnections ?? 0;
  const upRate = traffic?.uploadRate || 0;
  const downRate = traffic?.downloadRate || 0;
  const peakUp = hist.up.length ? Math.max(0, ...hist.up) : 0;
  const peakDown = hist.down.length ? Math.max(0, ...hist.down) : 0;

  const accent = token.colorPrimary;
  const success = token.colorSuccess;
  const warning = token.colorWarning;

  const gutter: [number, number] = isMobile ? [8, 8] : [12, 12];

  return (
    <div className="page-root" style={{ display: 'block', maxWidth: 1400 }}>
      {/* Top control bar — 3X-UI style */}
      <Card
        size="small"
        styles={{ body: { padding: isMobile ? '10px 12px' : '12px 16px' } }}
        style={{ marginBottom: isMobile ? 8 : 12, borderRadius: 12 }}
      >
        <div
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            alignItems: 'center',
            gap: isMobile ? 8 : 12,
            justifyContent: 'space-between',
          }}
        >
          <Space size={8} wrap>
            <span
              style={{
                width: 8,
                height: 8,
                borderRadius: '50%',
                background: coreRunning ? success : token.colorTextQuaternary,
                display: 'inline-block',
                boxShadow: coreRunning ? `0 0 0 3px ${success}33` : undefined,
              }}
            />
            <Text strong style={{ fontSize: isMobile ? 13 : 14 }}>
              {t('dashboard.coreName')} · {coreRunning ? t('dashboard.running') : t('dashboard.stoppedStatus')}
            </Text>
            {data?.mihomo?.version ? (
              <Tag style={{ margin: 0, borderRadius: 999 }}>{data.mihomo.version}</Tag>
            ) : null}
            {data?.mihomo?.uptime ? (
              <Text type="secondary" style={{ fontSize: 12 }}>
                {t('dashboard.uptime')}: {data.mihomo.uptime}
              </Text>
            ) : null}
          </Space>
          <Space size={8} wrap>
            {!coreRunning ? (
              <Button type="primary" icon={<IconPlay />} onClick={() => act('start')} loading={busy} size={isMobile ? 'middle' : 'middle'}>
                {t('dashboard.start')}
              </Button>
            ) : null}
            <Button icon={<IconRestart />} onClick={() => act('restart')} loading={busy}>
              {t('dashboard.restart')}
            </Button>
            <Button icon={<IconStop />} danger onClick={() => act('stop')} loading={busy} disabled={!coreRunning}>
              {t('dashboard.stop')}
            </Button>
            {!isMobile ? (
              <>
                <Link to="/logs"><Button type="link" size="small">{t('nav.logs')}</Button></Link>
                <Link to="/config"><Button type="link" size="small">{t('nav.config')}</Button></Link>
              </>
            ) : null}
          </Space>
        </div>
      </Card>

      {/* Resource metric cards */}
      <Row gutter={gutter} style={{ marginBottom: isMobile ? 8 : 12 }}>
        <Col xs={12} sm={12} md={6}>
          <MetricCard
            title={t('dashboard.cpu')}
            value={String(cpuPct)}
            unit="%"
            detail={undefined}
            peak={hist.cpu.length ? `${t('dashboard.peak')} ${Math.max(...hist.cpu)}%` : undefined}
            series={hist.cpu}
            color={cpuPct >= 80 ? token.colorError : accent}
            isMobile={isMobile}
          />
        </Col>
        <Col xs={12} sm={12} md={6}>
          <MetricCard
            title={t('dashboard.memory')}
            value={String(memPct)}
            unit="%"
            detail={`${formatBytes(sys?.memory?.used || 0)} / ${formatBytes(sys?.memory?.total || 0)}`}
            peak={hist.mem.length ? `${t('dashboard.avg')} ${Math.round((hist.mem.reduce((a, b) => a + b, 0) / hist.mem.length) * 10) / 10}%` : undefined}
            series={hist.mem}
            color={memPct >= 80 ? token.colorError : accent}
            isMobile={isMobile}
          />
        </Col>
        <Col xs={12} sm={12} md={6}>
          <MetricCard
            title={t('dashboard.disk')}
            value={String(diskPct)}
            unit="%"
            detail={`${formatBytes(sys?.disk?.used || 0)} / ${formatBytes(sys?.disk?.total || 0)}`}
            peak={sys?.disk?.total ? `${t('dashboard.free')} ${formatBytes(Math.max(0, (sys?.disk?.total || 0) - (sys?.disk?.used || 0)))}` : undefined}
            series={hist.disk}
            color={diskPct >= 90 ? token.colorError : accent}
            isMobile={isMobile}
          />
        </Col>
        <Col xs={12} sm={12} md={6}>
          <MetricCard
            title={t('dashboard.users')}
            value={String(online)}
            unit=""
            detail={`${t('dashboard.totalUsers')}: ${users?.total ?? 0} · ${t('dashboard.enabledUsers')}: ${users?.enabled ?? 0}`}
            peak={`${t('dashboard.listeners')}: ${data?.listeners?.enabled ?? 0}/${data?.listeners?.total ?? 0}`}
            series={hist.conns.length ? hist.conns : [0, online]}
            color={success}
            isMobile={isMobile}
          />
        </Col>
      </Row>

      {/* Speed chart + connection stats */}
      <Row gutter={gutter} style={{ marginBottom: isMobile ? 8 : 12 }}>
        <Col xs={24} lg={16}>
          <Card
            size="small"
            styles={{ body: { padding: isMobile ? 12 : 16 } }}
            style={{ height: '100%', borderRadius: 12 }}
          >
            <div style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'space-between', gap: 8, marginBottom: 8 }}>
              <div>
                <Text type="secondary" style={{ fontSize: 12, fontWeight: 600, letterSpacing: '0.02em' }}>
                  {t('dashboard.traffic')}
                </Text>
                <div style={{ fontSize: 11, color: token.colorTextSecondary, marginTop: 2 }}>
                  {t('dashboard.uploadRate')} · {t('dashboard.peak')} {formatRate(peakUp)} · {t('dashboard.downloadRate')} · {t('dashboard.peak')}{' '}
                  {formatRate(peakDown)}
                </div>
              </div>
              <Space size={16} wrap>
                <span style={{ fontSize: isMobile ? 12 : 13 }}>
                  <Text type="secondary">↑ {t('dashboard.upload')} </Text>
                  <Text strong style={{ color: accent, fontVariantNumeric: 'tabular-nums' }}>
                    {formatRate(upRate)}
                  </Text>
                </span>
                <span style={{ fontSize: isMobile ? 12 : 13 }}>
                  <Text type="secondary">↓ {t('dashboard.download')} </Text>
                  <Text strong style={{ color: warning, fontVariantNumeric: 'tabular-nums' }}>
                    {formatRate(downRate)}
                  </Text>
                </span>
              </Space>
            </div>
            <SpeedChart
              up={hist.up}
              down={hist.down}
              upColor={accent}
              downColor={warning}
              height={isMobile ? 120 : 168}
            />
            <Row gutter={8} style={{ marginTop: 12 }}>
              <Col span={8}>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {t('dashboard.totalUpload')}
                </Text>
                <div style={{ fontWeight: 600, fontVariantNumeric: 'tabular-nums' }}>{formatBytes(traffic?.totalUpload || 0)}</div>
              </Col>
              <Col span={8}>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {t('dashboard.totalDownload')}
                </Text>
                <div style={{ fontWeight: 600, fontVariantNumeric: 'tabular-nums' }}>{formatBytes(traffic?.totalDownload || 0)}</div>
              </Col>
              <Col span={8}>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {t('dashboard.onlineUsers')}
                </Text>
                <div style={{ fontWeight: 600, fontVariantNumeric: 'tabular-nums' }}>{online}</div>
              </Col>
            </Row>
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card
            size="small"
            styles={{ body: { padding: isMobile ? 12 : 16 } }}
            style={{ height: '100%', borderRadius: 12 }}
          >
            <Text type="secondary" style={{ fontSize: 12, fontWeight: 600, letterSpacing: '0.02em' }}>
              {t('dashboard.activeConnections')}
            </Text>
            <div
              style={{
                fontSize: isMobile ? 36 : 44,
                fontWeight: 700,
                lineHeight: 1.15,
                fontVariantNumeric: 'tabular-nums',
                letterSpacing: '-0.02em',
                margin: '8px 0 4px',
              }}
            >
              {conns}
            </div>
            <Text type="secondary" style={{ fontSize: 12 }}>
              {t('dashboard.openSockets')}
            </Text>
            <div style={{ marginTop: 12 }}>
              <Sparkline data={hist.conns} color={accent} height={isMobile ? 48 : 64} fillOpacity={0.15} />
            </div>
            <div
              style={{
                marginTop: 16,
                display: 'grid',
                gridTemplateColumns: '1fr 1fr 1fr',
                gap: 8,
                textAlign: 'center',
              }}
            >
              <div>
                <div style={{ fontSize: 11, color: token.colorTextSecondary }}>{t('dashboard.total')}</div>
                <div style={{ fontWeight: 600 }}>{data?.listeners?.total ?? 0}</div>
              </div>
              <div>
                <div style={{ fontSize: 11, color: token.colorTextSecondary }}>{t('dashboard.enabled')}</div>
                <div style={{ fontWeight: 600, color: success }}>{data?.listeners?.enabled ?? 0}</div>
              </div>
              <div>
                <div style={{ fontSize: 11, color: token.colorTextSecondary }}>{t('dashboard.disabled')}</div>
                <div style={{ fontWeight: 600, color: token.colorError }}>{data?.listeners?.disabled ?? 0}</div>
              </div>
            </div>
            <Text type="secondary" style={{ fontSize: 11, display: 'block', marginTop: 8, textAlign: 'center' }}>
              {t('dashboard.listeners')}
            </Text>
          </Card>
        </Col>
      </Row>

      {/* Bottom strip: panel / core process */}
      <Card size="small" styles={{ body: { padding: isMobile ? 10 : 14 } }} style={{ borderRadius: 12 }}>
        <Row gutter={[12, 12]} align="middle">
          <Col xs={12} sm={6} md={4}>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t('dashboard.uptime')}
            </Text>
            <div style={{ fontWeight: 600, fontSize: isMobile ? 13 : 14 }}>{data?.mihomo?.uptime || '—'}</div>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t('dashboard.coreName')}
            </Text>
          </Col>
          <Col xs={12} sm={6} md={5}>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t('dashboard.panel')}
            </Text>
            <div style={{ fontWeight: 600, fontVariantNumeric: 'tabular-nums', fontSize: isMobile ? 13 : 14 }}>
              {panel?.memory_used != null ? formatBytes(panel.memory_used) : '—'}
              {panel?.cpu_percent != null ? (
                <Text type="secondary" style={{ fontWeight: 400, marginLeft: 6, fontSize: 12 }}>
                  {t('dashboard.cpu')} {clampPct(panel.cpu_percent)}%
                </Text>
              ) : null}
            </div>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {panel?.pid ? `PID ${panel.pid}` : t('dashboard.panelUsage')}
            </Text>
          </Col>
          <Col xs={12} sm={6} md={5}>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t('dashboard.core')}
            </Text>
            <div style={{ fontWeight: 600, fontVariantNumeric: 'tabular-nums', fontSize: isMobile ? 13 : 14 }}>
              {coreRunning && core?.memory_used != null ? formatBytes(core.memory_used) : '—'}
              {coreRunning && core?.cpu_percent != null ? (
                <Text type="secondary" style={{ fontWeight: 400, marginLeft: 6, fontSize: 12 }}>
                  {t('dashboard.cpu')} {clampPct(core.cpu_percent)}%
                </Text>
              ) : null}
            </div>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {coreRunning && core?.pid ? `PID ${core.pid}` : t('dashboard.coreUsage')}
            </Text>
          </Col>
          <Col xs={12} sm={6} md={5}>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t('dashboard.onlineUsers')}
            </Text>
            <div style={{ fontWeight: 600, fontVariantNumeric: 'tabular-nums', fontSize: isMobile ? 13 : 14 }}>
              {online}
              <Text type="secondary" style={{ fontWeight: 400, marginLeft: 6, fontSize: 12 }}>
                / {users?.total ?? 0}
              </Text>
            </div>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t('dashboard.enabledUsers')}: {users?.enabled ?? 0}
            </Text>
          </Col>
          <Col xs={24} sm={24} md={5}>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {t('dashboard.version')}
            </Text>
            <div style={{ fontWeight: 600, fontSize: isMobile ? 13 : 14, wordBreak: 'break-all' }}>
              {data?.mihomo?.version || '—'}
            </div>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {coreRunning ? t('dashboard.running') : t('dashboard.stoppedStatus')}
            </Text>
          </Col>
        </Row>
      </Card>
    </div>
  );
};

export default Dashboard;
