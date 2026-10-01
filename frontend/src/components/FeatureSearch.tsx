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
  return useMemo(
    () => [
      {
        key: '/',
        label: t('nav.dashboard'),
        icon: <IconNavDashboard />,
        keywords: ['overview', 'home', 'status', '概览', '仪表盘', '首頁', 'ホーム'],
      },
      {
        key: '/listeners',
        label: t('nav.listeners'),
        icon: <IconNavListeners />,
        keywords: ['nodes', 'inbound', 'node', '节点', '節點', '监听', '監聽', 'インバウンド'],
      },
      {
        key: '/users',
        label: t('nav.users'),
        icon: <IconNavUsers />,
        keywords: ['client', 'account', '用户', '用戶', '客户端', 'クライアント'],
      },
      {
        key: '/share',
        label: t('nav.share'),
        icon: <IconNavShare />,
        keywords: ['subscription', 'sub', '订阅', '訂閱', '分享', '共有'],
      },
      {
        key: '/traffic',
        label: t('nav.traffic'),
        icon: <IconNavTraffic />,
        keywords: ['usage', 'stats', '流量', '统计', '統計', 'トラフィック'],
      },
      {
        key: '/cluster',
        label: t('nav.cluster'),
        icon: <IconNavCluster />,
        keywords: ['remote', 'node', '集群', '远程', '遠端', 'クラスター'],
      },
      {
        key: '/routing',
        label: t('nav.routing'),
        icon: <IconNavRouting />,
        keywords: ['rules', 'dns', 'warp', '规则', '規則', '路由', 'ルーティング'],
      },
      {
        key: '/core',
        label: t('nav.core'),
        icon: <IconNavCore />,
        keywords: ['mihomo', 'clash', '内核', '核心', 'コア'],
      },
      {
        key: '/logs',
        label: t('nav.logs'),
        icon: <IconNavLogs />,
        keywords: ['log', 'debug', '日志', '日誌', 'ログ'],
      },
      {
        key: '/config',
        label: t('nav.config'),
        icon: <IconNavConfig />,
        keywords: ['yaml', 'json', '配置', '設定', 'コンフィグ'],
      },
      {
        key: '/settings',
        label: t('nav.settings'),
        icon: <IconNavSettings />,
        keywords: ['panel', 'preference', '设置', '設定', 'パネル'],
      },
    ],
    [t],
  );
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

  const filtered = useMemo(() => items.filter((it) => matchItem(it, query)), [items, query]);

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
      if (location.pathname !== key) navigate(key);
    },
    [navigate, location.pathname],
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
              const current = location.pathname === it.key || (it.key !== '/' && location.pathname.startsWith(it.key));
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
                  <span style={{ flex: 1, fontWeight: current ? 600 : 500 }}>{it.label}</span>
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
                  if (location.pathname !== it.key) navigate(it.key);
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
