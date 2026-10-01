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
      // Settings sections
      {
        key: '/settings?section=panel',
        group: settings,
        label: t('settings.navPanel') || 'Panel / appearance',
        icon: <IconNavSettings />,
        keywords: ['theme', 'locale', 'language', '外观', '主题', '語言', '语言', 'テーマ'],
      },
      {
        key: '/settings?section=access',
        group: settings,
        label: t('settings.navAccess') || 'Access profile',
        icon: <IconNavSettings />,
        keywords: ['sni', 'host', 'fingerprint', '访问', '档案', 'アクセス'],
      },
      {
        key: '/settings?section=telegram',
        group: settings,
        label: t('settings.navTelegram') || 'Telegram',
        icon: <IconNavSettings />,
        keywords: ['bot', 'tg', 'telegram', '通知'],
      },
      {
        key: '/settings?section=security',
        group: settings,
        label: t('settings.navSecurity') || 'Security & backup',
        icon: <IconNavSettings />,
        keywords: ['2fa', 'totp', 'backup', 'github', 'oauth', '安全', '备份', '備份', '密码', '密碼'],
      },
      {
        key: '/settings?section=subscription',
        group: settings,
        label: t('settings.navSubscription') || 'Subscription page',
        icon: <IconNavSettings />,
        keywords: ['sub page', '订阅页', '訂閱頁', '模板'],
      },
      {
        key: '/settings?section=ssl',
        group: settings,
        label: t('settings.navSSL') || 'Certificate / SSL',
        icon: <IconNavSettings />,
        keywords: ['tls', 'cert', 'acme', '证书', '證書', 'https', 'ssl'],
      },
      {
        key: '/settings?section=network',
        group: settings,
        label: t('settings.navNetwork') || 'Proxy & Geo',
        icon: <IconNavSettings />,
        keywords: ['warp', 'geoip', 'geosite', 'reverse', '反代', '网络', '網路'],
      },
      {
        key: '/settings?section=traffic',
        group: settings,
        label: t('settings.navTraffic') || 'Traffic reset',
        icon: <IconNavSettings />,
        keywords: ['reset', 'cycle', '流量重置', '月流量'],
      },
      {
        key: '/settings?section=ops',
        group: settings,
        label: t('settings.navOps') || 'System ops',
        icon: <IconNavSettings />,
        keywords: ['restart', 'update', 'panel update', '系统操作', '升级', '升級'],
      },
      {
        key: '/settings?section=about',
        group: settings,
        label: t('settings.navAbout') || 'About',
        icon: <IconNavSettings />,
        keywords: ['version', 'license', '关于', '關於'],
      },
      {
        key: '/change-password',
        group: settings,
        label: t('settings.changePassword') || t('nav.changePassword') || 'Change password',
        icon: <IconNavSettings />,
        keywords: ['password', 'passwd', '改密', '修改密码', '修改密碼'],
      },
      // Routing scopes
      {
        key: '/routing?scope=client',
        group: routing,
        label: t('routing.tabClient') || 'Client subscription rules',
        icon: <IconNavRouting />,
        keywords: ['proxy-groups', 'client rules', '订阅规则', '訂閱規則', '分流'],
      },
      {
        key: '/routing?scope=server',
        group: routing,
        label: t('routing.tabServer') || 'Server egress rules',
        icon: <IconNavRouting />,
        keywords: ['egress', 'server rules', 'warp domains', '出口', '服务端规则', '服務端規則'],
      },
      // Config tabs
      {
        key: '/config?tab=visual',
        group: config,
        label: t('config.visual') || 'Visual proxies',
        icon: <IconNavConfig />,
        keywords: ['proxy', 'proxies', '可视化', '視覺化'],
      },
      {
        key: '/config?tab=yaml',
        group: config,
        label: t('config.yaml') || 'YAML editor',
        icon: <IconNavConfig />,
        keywords: ['yaml', 'raw', '编辑器', '編輯器'],
      },
      // Core-related shortcuts (same page, different keywords)
      {
        key: '/core',
        group: core,
        label: t('core.updates.title') || 'Core updates',
        icon: <IconNavCore />,
        keywords: ['update core', 'rollback', '升级内核', '回滚', '回滾', 'mihomo update'],
      },
      {
        key: '/listeners',
        group: listeners,
        label: t('listeners.create') || 'Create node',
        icon: <IconNavListeners />,
        keywords: ['add node', 'new inbound', '新建节点', '新建節點', '添加节点'],
      },
      {
        key: '/users',
        group: users,
        label: t('users.create') || 'Create user',
        icon: <IconNavUsers />,
        keywords: ['add user', 'new client', '新建用户', '新建用戶', '添加用户'],
      },
      {
        key: '/share',
        group: share,
        label: t('share.title') || share,
        icon: <IconNavShare />,
        keywords: ['qr', 'uri', 'clash', 'v2ray', '订阅链接', '訂閱連結'],
      },
    ];

    return [...top, ...nested];
  }, [t]);
}

function matchItem(item: FeatureItem, q: string): boolean {
  if (!q) return true;
  const s = q.trim().toLowerCase();
  if (!s) return true;
  if (item.label.toLowerCase().includes(s)) return true;
  if (item.key.toLowerCase().includes(s)) return true;
  return item.keywords.some((k) => k.toLowerCase().includes(s));
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
        <div style={{ maxHeight: 360, overflowY: 'auto', padding: '6px 8px 10px' }}>
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
