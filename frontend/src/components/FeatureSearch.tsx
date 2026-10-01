import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { Input, Modal, Empty, theme, Typography } from 'antd';
import { useNavigate, useLocation } from 'react-router-dom';
import { useI18n } from '../i18n';
import {
  IconNavDashboard,
  IconNavListeners,
  IconNavUsers,
  IconNavShare,
  IconNavTraffic,
  IconNavCluster,
  IconNavRouting,
  IconNavCore,
  IconNavLogs,
  IconNavConfig,
  IconNavSettings,
  IconSearch,
} from '../icons';

const { Text } = Typography;

export type FeatureItem = {
  key: string;
  label: string;
  keywords: string[];
  icon: React.ReactNode;
  /** When set, shown as a secondary group label (e.g. Settings). */
  group?: string;
  /** Top-level nav entry; shown when the query is empty. */
  top?: boolean;
};

type Ctx = {
  open: boolean;
  setOpen: (v: boolean) => void;
  items: FeatureItem[];
};

const FeatureSearchContext = createContext<Ctx | null>(null);

export function useFeatureSearch() {
  const ctx = useContext(FeatureSearchContext);
  if (!ctx) throw new Error('useFeatureSearch must be used within FeatureSearchProvider');
  return ctx;
}

function useFeatureItems(): FeatureItem[] {
  const { t } = useI18n();
  return useMemo(() => {
    const settings = t('nav.settings');
    const routing = t('nav.routing');
    const config = t('nav.config');
    const core = t('nav.core');
    const users = t('nav.users');
    const listeners = t('nav.listeners');
    const share = t('nav.share');

    const top: FeatureItem[] = [
      {
        key: '/',
        top: true,
        label: t('nav.dashboard'),
        icon: <IconNavDashboard />,
        keywords: ['overview', 'home', 'status', '概览', '仪表盘', '首頁', 'ホーム'],
      },
      {
        key: '/listeners',
        top: true,
        label: listeners,
        icon: <IconNavListeners />,
        keywords: ['nodes', 'inbound', 'node', '节点', '節點', '监听', '監聽', 'インバウンド'],
      },
      {
        key: '/users',
        top: true,
        label: users,
        icon: <IconNavUsers />,
        keywords: ['client', 'account', '用户', '用戶', '客户端', 'クライアント'],
      },
      {
        key: '/share',
        top: true,
        label: share,
        icon: <IconNavShare />,
        keywords: ['subscription', 'sub', '订阅', '訂閱', '分享', '共有'],
      },
      {
        key: '/traffic',
        top: true,
        label: t('nav.traffic'),
        icon: <IconNavTraffic />,
        keywords: ['usage', 'stats', '流量', '统计', '統計', 'トラフィック'],
      },
      {
        key: '/cluster',
        top: true,
        label: t('nav.cluster'),
        icon: <IconNavCluster />,
        keywords: ['remote', 'node', '集群', '远程', '遠端', 'クラスター'],
      },
      {
        key: '/routing',
        top: true,
        label: routing,
        icon: <IconNavRouting />,
        keywords: ['rules', 'dns', 'warp', '规则', '規則', '路由', 'ルーティング'],
      },
      {
        key: '/core',
        top: true,
        label: core,
        icon: <IconNavCore />,
        keywords: ['mihomo', 'clash', '内核', '核心', 'コア'],
      },
      {
        key: '/logs',
        top: true,
        label: t('nav.logs'),
        icon: <IconNavLogs />,
        keywords: ['log', 'debug', '日志', '日誌', 'ログ'],
      },
      {
        key: '/config',
        top: true,
        label: config,
        icon: <IconNavConfig />,
        keywords: ['yaml', 'json', '配置', '設定', 'コンフィグ'],
      },
      {
        key: '/settings',
        top: true,
        label: settings,
        icon: <IconNavSettings />,
        keywords: ['panel', 'preference', '设置', '設定', 'パネル'],
      },
    ];

    const nested: FeatureItem[] = [
      // Settings · panel
      { key: '/settings?section=panel', group: settings, label: t('settings.navPanel') || 'Panel / appearance', icon: <IconNavSettings />, keywords: ['theme', 'locale', 'language', '外观', '主题', '語言', '语言', 'panel'] },
      { key: '/settings?section=panel', group: settings, label: t('settings.panelPort') || 'Panel port', icon: <IconNavSettings />, keywords: ['port', 'listen', '8080', '面板端口', '监听', 'panel port'] },
      { key: '/settings?section=panel', group: settings, label: t('settings.panelPublicURL') || 'Public URL', icon: <IconNavSettings />, keywords: ['public_url', '公网', '公網', '域名', 'public url'] },
      { key: '/settings?section=panel', group: settings, label: t('settings.theme') || 'Theme', icon: <IconNavSettings />, keywords: ['dark', 'light', 'system', '暗色', '亮色', '主题', '主題'] },
      { key: '/settings?section=panel', group: settings, label: t('settings.language') || 'Language', icon: <IconNavSettings />, keywords: ['i18n', 'locale', '中文', 'english', '语言', '語言'] },
      // Settings · access
      { key: '/settings?section=access', group: settings, label: t('settings.navAccess') || 'Access profile', icon: <IconNavSettings />, keywords: ['access', '访问档案', '訪問檔案'] },
      { key: '/settings?section=access', group: settings, label: t('settings.publicHost') || 'Public host', icon: <IconNavSettings />, keywords: ['host', 'cdn', '公网域名', 'public host'] },
      { key: '/settings?section=access', group: settings, label: t('settings.clientFingerprint') || 'Client fingerprint', icon: <IconNavSettings />, keywords: ['fingerprint', 'fp', 'chrome', 'firefox', '指纹', '指紋'] },
      { key: '/settings?section=access', group: settings, label: t('listeners.sni') || 'Access SNI', icon: <IconNavSettings />, keywords: ['sni', 'server name', 'tls sni'] },
      // Settings · telegram
      { key: '/settings?section=telegram', group: settings, label: t('settings.navTelegram') || 'Telegram', icon: <IconNavSettings />, keywords: ['bot', 'tg', 'telegram', '通知'] },
      { key: '/settings?section=telegram', group: settings, label: t('settings.botToken') || 'Bot token', icon: <IconNavSettings />, keywords: ['bot token', 'telegram token', '机器人', '機器人'] },
      { key: '/settings?section=telegram', group: settings, label: t('settings.chatIds') || 'Chat IDs', icon: <IconNavSettings />, keywords: ['chat id', '群组', '群組'] },
      { key: '/settings?section=telegram', group: settings, label: t('settings.notifyCPU') || 'CPU alert', icon: <IconNavSettings />, keywords: ['cpu warn', 'cpu alert', 'cpu通知', '负载'] },
      { key: '/settings?section=telegram', group: settings, label: t('settings.notifyTraffic') || 'Traffic alert', icon: <IconNavSettings />, keywords: ['traffic warn', '流量告警', '流量通知'] },
      { key: '/settings?section=telegram', group: settings, label: t('settings.notifyExpiry') || 'Expiry alert', icon: <IconNavSettings />, keywords: ['expire', '到期', '过期', '過期'] },
      { key: '/settings?section=telegram', group: settings, label: t('settings.telegramTest') || 'Test Telegram', icon: <IconNavSettings />, keywords: ['test bot', '测试通知', '測試通知'] },
      // Settings · security
      { key: '/settings?section=security', group: settings, label: t('settings.navSecurity') || 'Security & backup', icon: <IconNavSettings />, keywords: ['security', '安全', '备份', '備份'] },
      { key: '/change-password', group: settings, label: t('settings.changePassword') || 'Change password', icon: <IconNavSettings />, keywords: ['password', 'passwd', '改密', '修改密码', '修改密碼'] },
      { key: '/settings?section=security', group: settings, label: t('settings.totp') || '2FA / TOTP', icon: <IconNavSettings />, keywords: ['2fa', 'totp', 'mfa', 'otp', '双因素', '二步验证', '二步驗證'] },
      { key: '/settings?section=security', group: settings, label: t('settings.githubOAuth') || 'GitHub OAuth', icon: <IconNavSettings />, keywords: ['github', 'oauth', 'GitHub登录', 'GitHub登入'] },
      { key: '/settings?section=security', group: settings, label: t('settings.backup') || 'Backup & restore', icon: <IconNavSettings />, keywords: ['backup', 'restore', 'snapshot', '备份', '備份', '恢复', '還原', '数据库'] },
      { key: '/settings?section=security', group: settings, label: 'Web path prefix', icon: <IconNavSettings />, keywords: ['web_path', 'path prefix', '隐藏路径', '面板路径', 'secret path'] },
      // Settings · subscription / ssl / network / traffic / ops
      { key: '/settings?section=subscription', group: settings, label: t('settings.navSubscription') || 'Subscription page', icon: <IconNavSettings />, keywords: ['sub page', '订阅页', '訂閱頁', '模板'] },
      { key: '/settings?section=ssl', group: settings, label: t('settings.navSSL') || 'Certificate / SSL', icon: <IconNavSettings />, keywords: ['tls', 'cert', 'acme', '证书', '證書', 'https', 'ssl', 'letsencrypt'] },
      { key: '/settings?section=network', group: settings, label: t('settings.navNetwork') || 'Proxy & Geo', icon: <IconNavSettings />, keywords: ['network', '反代', 'geo', '网络', '網路'] },
      { key: '/settings?section=network', group: settings, label: 'WARP', icon: <IconNavSettings />, keywords: ['warp', 'wireguard', 'masque', 'cloudflare', 'cf warp'] },
      { key: '/settings?section=network', group: settings, label: 'GeoIP / GeoSite', icon: <IconNavSettings />, keywords: ['geoip', 'geosite', 'geo 数据', 'geo data', '规则集下载'] },
      { key: '/settings?section=traffic', group: settings, label: t('settings.navTraffic') || 'Traffic reset', icon: <IconNavSettings />, keywords: ['reset', 'cycle', 'monthly', '流量重置', '月流量'] },
      { key: '/settings?section=ops', group: settings, label: t('settings.navOps') || 'System ops', icon: <IconNavSettings />, keywords: ['ops', '系统操作', '维护'] },
      { key: '/settings?section=ops', group: settings, label: t('settings.panelUpdate') || 'Panel update', icon: <IconNavSettings />, keywords: ['panel update', 'upgrade panel', '升级面板', '升級面板', 'pre', 'stable'] },
      { key: '/settings?section=about', group: settings, label: t('settings.navAbout') || 'About', icon: <IconNavSettings />, keywords: ['version', 'license', '关于', '關於', 'about'] },
      // Routing
      { key: '/routing?scope=client', group: routing, label: t('routing.tabClient') || 'Client subscription rules', icon: <IconNavRouting />, keywords: ['proxy-groups', 'client rules', '订阅规则', '訂閱規則', '分流'] },
      { key: '/routing?scope=server', group: routing, label: t('routing.tabServer') || 'Server egress rules', icon: <IconNavRouting />, keywords: ['egress', 'server rules', '出口', '服务端规则', '服務端規則'] },
      { key: '/routing?scope=server', group: routing, label: t('routing.warpDomains') || 'WARP domains', icon: <IconNavRouting />, keywords: ['warp domain', 'warp 域名', '域名走warp'] },
      { key: '/routing?scope=server', group: routing, label: t('routing.warpGlobal') || 'Global WARP', icon: <IconNavRouting />, keywords: ['global warp', '全局 warp', 'match warp', '全部走warp'] },
      { key: '/routing?scope=client', group: routing, label: t('routing.ruleProviders') || 'Rule providers', icon: <IconNavRouting />, keywords: ['rule-provider', 'rule provider', '规则集', '規則集', 'mrs'] },
      { key: '/routing?scope=client', group: routing, label: t('routing.templates') || 'Rule templates', icon: <IconNavRouting />, keywords: ['template', 'tpl', '社区规则', 'cn direct', 'ads', '模板'] },
      { key: '/routing?scope=client', group: routing, label: t('routing.addRule') || 'Add rule', icon: <IconNavRouting />, keywords: ['add rule', '新建规则', '添加规则', 'domain', 'geosite'] },
      // Config
      { key: '/config?tab=visual', group: config, label: t('config.visual') || 'Visual proxies', icon: <IconNavConfig />, keywords: ['proxy', 'proxies', '可视化', '視覺化'] },
      { key: '/config?tab=yaml', group: config, label: t('config.yaml') || 'YAML editor', icon: <IconNavConfig />, keywords: ['yaml', 'raw', '编辑器', '編輯器', 'config.yaml'] },
      // Core
      { key: '/core', group: core, label: t('core.updates.title') || 'Core updates', icon: <IconNavCore />, keywords: ['update core', 'rollback', '升级内核', '回滚', '回滾', 'mihomo update'] },
      { key: '/core', group: core, label: t('dashboard.start') || 'Start core', icon: <IconNavCore />, keywords: ['start mihomo', '启动内核', '啟動核心'] },
      { key: '/core', group: core, label: t('dashboard.stop') || 'Stop core', icon: <IconNavCore />, keywords: ['stop mihomo', '停止内核', '停止核心'] },
      { key: '/core', group: core, label: t('dashboard.restart') || 'Restart core', icon: <IconNavCore />, keywords: ['restart mihomo', '重启内核', '重啟核心'] },
      // Listeners
      { key: '/listeners', group: listeners, label: t('listeners.create') || 'Create node', icon: <IconNavListeners />, keywords: ['add node', 'new inbound', '新建节点', '新建節點', 'create listener'] },
      { key: '/listeners', group: listeners, label: t('listeners.quickCreate') || 'Quick create node', icon: <IconNavListeners />, keywords: ['quick create', '快速创建', '一键节点', '一鍵節點'] },
      { key: '/listeners', group: listeners, label: t('listeners.applyCert') || 'Apply certificate', icon: <IconNavListeners />, keywords: ['apply cert', 'batch cert', '批量证书', '批量證書'] },
      // Users
      { key: '/users', group: users, label: t('users.create') || 'Create user', icon: <IconNavUsers />, keywords: ['add user', 'new client', '新建用户', '新建用戶'] },
      { key: '/users', group: users, label: t('users.quickCreate') || 'Quick create user', icon: <IconNavUsers />, keywords: ['quick user', '快速开户', '快速開戶'] },
      { key: '/users', group: users, label: t('users.resetTraffic') || 'Reset user traffic', icon: <IconNavUsers />, keywords: ['reset traffic', '清空流量', '重置流量'] },
      // Share
      { key: '/share', group: share, label: t('share.tabSub') || 'Subscription links', icon: <IconNavShare />, keywords: ['subscription url', '订阅链接', '訂閱連結', 'clash', 'sub link'] },
      { key: '/share', group: share, label: t('share.tabUri') || 'Node URIs', icon: <IconNavShare />, keywords: ['uri', 'vless://', 'vmess://', '分享链接', 'qr'] },
      { key: '/share', group: share, label: t('share.exportAll') || 'Export all subscriptions', icon: <IconNavShare />, keywords: ['export all', '导出全部', '匯出全部'] },
      // Traffic
      { key: '/traffic', group: t('nav.traffic'), label: t('traffic.byUser') || 'Traffic by user', icon: <IconNavTraffic />, keywords: ['by user', '用户流量', '用戶流量'] },
      { key: '/traffic', group: t('nav.traffic'), label: t('traffic.connections') || 'Live connections', icon: <IconNavTraffic />, keywords: ['connections', '连接', '連線', 'sockets'] },
      // Cluster
      { key: '/cluster', group: t('nav.cluster'), label: t('cluster.add') || 'Add remote panel', icon: <IconNavCluster />, keywords: ['add remote', '远程面板', '遠端面版'] },
      { key: '/cluster', group: t('nav.cluster'), label: t('cluster.health') || 'Health check', icon: <IconNavCluster />, keywords: ['health', '健康检查', '探活'] },
      { key: '/cluster', group: t('nav.cluster'), label: t('cluster.syncNodes') || 'Sync nodes', icon: <IconNavCluster />, keywords: ['sync nodes', '同步节点', '同步節點'] },
      { key: '/cluster', group: t('nav.cluster'), label: t('cluster.pushNode') || 'Push node', icon: <IconNavCluster />, keywords: ['push node', '推送节点', '推送節點'] },
      { key: '/cluster', group: t('nav.cluster'), label: t('cluster.restartCore') || 'Restart remote core', icon: <IconNavCluster />, keywords: ['remote restart', '远程重启', '遠端重啟'] },
      // Logs
      { key: '/logs', group: t('nav.logs'), label: t('logs.clear') || 'Clear logs', icon: <IconNavLogs />, keywords: ['clear log', '清空日志', '清空日誌'] },
      { key: '/logs', group: t('nav.logs'), label: t('logs.autoRefresh') || 'Auto-refresh logs', icon: <IconNavLogs />, keywords: ['auto refresh', '自动刷新', '自動重新整理'] },
      // Dashboard
      { key: '/', group: t('nav.dashboard'), label: t('dashboard.traffic') || 'Overview traffic', icon: <IconNavDashboard />, keywords: ['overview traffic', '总览流量', 'speed', '速率'] },
      { key: '/', group: t('nav.dashboard'), label: t('dashboard.activeConnections') || 'Connection stats', icon: <IconNavDashboard />, keywords: ['tcp', 'udp', 'open sockets', '连接统计', '連線統計'] },
    ];

return [...top, ...nested];
  }, [t]);
}

function matchItem(item: FeatureItem, q: string): boolean {
  const s = q.trim().toLowerCase();
  if (!s) return true;
  const hay = [item.label, item.key, item.group || '', ...(item.keywords || [])].join(' ').toLowerCase();
  return s.split(/\s+/).every((tok) => Boolean(tok) && hay.includes(tok));
}

export const FeatureSearchProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [open, setOpen] = useState(false);
  const items = useFeatureItems();
  const navigate = useNavigate();
  const location = useLocation();
  const { t } = useI18n();
  const { token } = theme.useToken();
  const [query, setQuery] = useState('');
  const [activeIdx, setActiveIdx] = useState(0);
  const inputRef = useRef<any>(null);

  const filtered = useMemo(() => {
    const q = query.trim();
    if (!q) return items.filter((it) => it.top);
    // Prefer unique keys; keep first occurrence order
    const seen = new Set<string>();
    const out: FeatureItem[] = [];
    for (const it of items) {
      if (!matchItem(it, q)) continue;
      const id = `${it.key}::${it.label}`;
      if (seen.has(id)) continue;
      seen.add(id);
      out.push(it);
    }
    return out;
  }, [items, query]);

  useEffect(() => {
    if (open) {
      setQuery('');
      setActiveIdx(0);
      const id = window.setTimeout(() => inputRef.current?.focus?.(), 50);
      return () => window.clearTimeout(id);
    }
  }, [open]);

  useEffect(() => {
    setActiveIdx(0);
  }, [query]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const isMod = e.metaKey || e.ctrlKey;
      if (isMod && (e.key === 'k' || e.key === 'K')) {
        e.preventDefault();
        setOpen((v) => !v);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

  const go = useCallback(
    (key: string) => {
      setOpen(false);
      const [path, qs] = key.split('?');
      const target = qs ? `${path}?${qs}` : path;
      const current = location.pathname + (location.search || '');
      if (current !== target && current !== path) navigate(target);
      else if (location.pathname !== path || (qs && location.search !== `?${qs}`)) navigate(target);
    },
    [navigate, location.pathname, location.search],
  );

  const onInputKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIdx((i) => Math.min(i + 1, Math.max(filtered.length - 1, 0)));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIdx((i) => Math.max(i - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const hit = filtered[activeIdx];
      if (hit) go(hit.key);
    }
  };

  const modLabel =
    typeof navigator !== 'undefined' && /Mac|iPhone|iPad/i.test(navigator.platform || navigator.userAgent)
      ? '⌘K'
      : 'Ctrl K';

  return (
    <FeatureSearchContext.Provider value={{ open, setOpen, items }}>
      {children}
      <Modal
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        closable={false}
        width={480}
        centered
        destroyOnClose
        styles={{
          body: { padding: 0 },
          container: { padding: 0, overflow: 'hidden', borderRadius: 12 },
        }}
      >
        <div style={{ padding: '12px 12px 8px', borderBottom: `1px solid ${token.colorBorderSecondary}` }}>
          <Input
            ref={inputRef}
            allowClear
            size="large"
            prefix={<IconSearch style={{ opacity: 0.55 }} />}
            placeholder={t('nav.searchPlaceholder')}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={onInputKeyDown}
            variant="borderless"
            style={{ fontSize: 15 }}
          />
        </div>
        <div style={{ maxHeight: 420, overflowY: 'auto', padding: '6px 8px 10px' }}>
          {filtered.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('nav.searchEmpty')} style={{ margin: '24px 0' }} />
          ) : (
            filtered.map((it, idx) => {
              const active = idx === activeIdx;
              const [p, qs] = it.key.split('?');
              const current = location.pathname === p && (qs ? location.search === `?${qs}` : it.top ? true : !location.search);
              return (
                <button
                  key={it.key}
                  type="button"
                  onMouseEnter={() => setActiveIdx(idx)}
                  onClick={() => go(it.key)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 10,
                    width: '100%',
                    textAlign: 'start',
                    border: 'none',
                    cursor: 'pointer',
                    borderRadius: 8,
                    padding: '10px 12px',
                    background: active ? token.colorFillSecondary : 'transparent',
                    color: token.colorText,
                  }}
                >
                  <span style={{ display: 'inline-flex', opacity: 0.75, fontSize: 16 }}>{it.icon}</span>
                  <span style={{ flex: 1, minWidth: 0 }}>
                    <span style={{ fontWeight: current ? 600 : 500 }}>{it.label}</span>
                    {it.group ? (
                      <span style={{ display: 'block', fontSize: 11, color: token.colorTextSecondary, marginTop: 2 }}>
                        {it.group}
                      </span>
                    ) : null}
                  </span>
                  {current ? (
                    <Text type="secondary" style={{ fontSize: 11 }}>
                      {t('nav.searchCurrent')}
                    </Text>
                  ) : null}
                </button>
              );
            })
          )}
        </div>
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            padding: '8px 14px',
            borderTop: `1px solid ${token.colorBorderSecondary}`,
            fontSize: 11,
            color: token.colorTextSecondary,
          }}
        >
          <span>↑↓ {t('nav.searchNavigate')} · Enter {t('nav.searchSelect')}</span>
          <span>{modLabel}</span>
        </div>
      </Modal>
    </FeatureSearchContext.Provider>
  );
};

/** Compact search field / icon for the desktop sidebar (3X-UI style). */
export const SidebarFeatureSearch: React.FC<{ collapsed?: boolean }> = ({ collapsed }) => {
  const { setOpen, items } = useFeatureSearch();
  const { t } = useI18n();
  const navigate = useNavigate();
  const location = useLocation();
  const { token } = theme.useToken();
  const [q, setQ] = useState('');
  const filtered = useMemo(() => items.filter((it) => matchItem(it, q)), [items, q]);

  if (collapsed) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', padding: '0 0 8px' }}>
        <button
          type="button"
          title={t('nav.search')}
          onClick={() => setOpen(true)}
          style={{
            border: 'none',
            background: token.colorFillTertiary,
            borderRadius: 8,
            width: 36,
            height: 36,
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
            cursor: 'pointer',
            color: token.colorTextSecondary,
          }}
        >
          <IconSearch size={16} />
        </button>
      </div>
    );
  }

  return (
    <div style={{ padding: '0 12px 10px' }}>
      <Input
        allowClear
        size="middle"
        prefix={<IconSearch style={{ opacity: 0.5 }} />}
        placeholder={t('nav.searchPlaceholder')}
        value={q}
        onChange={(e) => setQ(e.target.value)}
        onFocus={() => {
          /* keep inline filter when typing; open palette on empty click optional */
        }}
        onClick={() => {
          if (!q) setOpen(true);
        }}
        suffix={
          <Text type="secondary" style={{ fontSize: 11, fontFamily: 'ui-monospace, monospace' }}>
            {typeof navigator !== 'undefined' && /Mac|iPhone|iPad/i.test(navigator.platform || navigator.userAgent)
              ? '⌘K'
              : 'Ctrl K'}
          </Text>
        }
        styles={{ input: { fontSize: 13 } }}
      />
      {q.trim() ? (
        <div
          style={{
            marginTop: 6,
            maxHeight: 220,
            overflowY: 'auto',
            borderRadius: 8,
            border: `1px solid ${token.colorBorderSecondary}`,
            background: token.colorBgElevated,
          }}
        >
          {filtered.length === 0 ? (
            <div style={{ padding: 12, fontSize: 12, color: token.colorTextSecondary }}>{t('nav.searchEmpty')}</div>
          ) : (
            filtered.map((it) => (
              <button
                key={it.key}
                type="button"
                onClick={() => {
                  setQ('');
                  navigate(it.key);
                }}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                  width: '100%',
                  border: 'none',
                  background: 'transparent',
                  padding: '8px 10px',
                  cursor: 'pointer',
                  textAlign: 'start',
                  color: token.colorText,
                }}
              >
                <span style={{ opacity: 0.7 }}>{it.icon}</span>
                <span style={{ fontSize: 13 }}>{it.label}</span>
              </button>
            ))
          )}
        </div>
      ) : null}
    </div>
  );
};

/** Mobile header search button. */
export const HeaderFeatureSearchButton: React.FC = () => {
  const { setOpen } = useFeatureSearch();
  const { t } = useI18n();
  return (
    <button
      type="button"
      aria-label={t('nav.search')}
      onClick={() => setOpen(true)}
      style={{
        border: 'none',
        background: 'transparent',
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        width: 36,
        height: 36,
        borderRadius: 8,
        cursor: 'pointer',
        padding: 0,
        color: 'inherit',
      }}
    >
      <IconSearch size={18} />
    </button>
  );
};
