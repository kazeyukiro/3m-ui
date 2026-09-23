import React from 'react';
import { Layout, Button, Space, Tag, Dropdown, Typography } from 'antd';
import {
  IconMenuClose,
  IconMenuOpen,
  IconUser,
  IconGlobe,
  IconTheme,
} from '../icons';
import { useAuthStore } from '../stores/authStore';
import { useThemeStore, ThemeMode } from '../stores/themeStore';
import { useI18n, LOCALE_OPTIONS, type Locale } from '../i18n';
import useIsMobile from '../hooks/useIsMobile';

const { Header } = Layout;
const { Text } = Typography;

type Props = {
  collapsed: boolean;
  setCollapsed: (v: boolean) => void;
};

const HeaderBar: React.FC<Props> = ({ collapsed, setCollapsed }) => {
  const { t, locale, setLocale } = useI18n();
  const { mode, setMode } = useThemeStore();
  const username = useAuthStore((s) => s.username);
  const isMobile = useIsMobile();

  const langItems = LOCALE_OPTIONS.map((o) => ({ key: o.key, label: o.label }));

  const themeItems = [
    { key: 'light', label: t('settings.light') },
    { key: 'dark', label: t('settings.dark') },
    { key: 'system', label: t('settings.system') },
  ];

  const displayName = username || 'Admin';
  const shortName = displayName.length > 8 ? `${displayName.slice(0, 8)}…` : displayName;

  return (
    <Header
      className="app-header-bar"
      style={{
        padding: isMobile ? '0 10px' : '0 20px',
        background: 'transparent',
        display: 'flex',
        flexDirection: 'row',
        alignItems: 'center',
        justifyContent: 'flex-start',
        gap: 8,
        height: isMobile ? 48 : 56,
        lineHeight: 'normal',
        width: '100%',
        boxSizing: 'border-box',
      }}
    >
      {/* Left: brand / collapse */}
      <div
        className="app-header-left"
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          flex: '0 0 auto',
          minWidth: 0,
        }}
      >
        {isMobile ? (
          <>
            <img src="/logo.png" alt="" width={28} height={28} style={{ objectFit: 'contain', flexShrink: 0 }} />
            <Text strong style={{ fontSize: 15, lineHeight: 1.2 }}>
              3M-UI
            </Text>
          </>
        ) : (
          <Button
            type="text"
            icon={collapsed ? <IconMenuOpen /> : <IconMenuClose />}
            onClick={() => setCollapsed(!collapsed)}
            style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
          />
        )}
      </div>

      {/* Right: theme / lang / user — always flush end */}
      <div
        className="app-header-right"
        style={{
          marginLeft: 'auto',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'flex-end',
          gap: isMobile ? 2 : 6,
          flex: '0 1 auto',
          minWidth: 0,
          flexWrap: 'nowrap',
        }}
      >
        <Dropdown
          menu={{
            items: themeItems,
            selectedKeys: [mode],
            onClick: (e) => setMode(e.key as ThemeMode),
          }}
        >
          <Button
            type="text"
            className="app-header-action"
            icon={<IconTheme />}
            style={{ display: 'inline-flex', alignItems: 'center', gap: 6, height: 36, paddingInline: isMobile ? 8 : 10 }}
          >
            {!isMobile && t('settings.theme')}
          </Button>
        </Dropdown>
        <Dropdown
          menu={{
            items: langItems,
            selectedKeys: [locale],
            onClick: (e) => setLocale(e.key as Locale),
          }}
        >
          <Button
            type="text"
            className="app-header-action"
            icon={<IconGlobe />}
            style={{ display: 'inline-flex', alignItems: 'center', gap: 6, height: 36, paddingInline: isMobile ? 8 : 10 }}
          >
            {!isMobile && (LOCALE_OPTIONS.find((o) => o.key === locale)?.label || locale)}
          </Button>
        </Dropdown>
        <Tag
          icon={<IconUser size={14} />}
          style={{
            margin: 0,
            display: 'inline-flex',
            alignItems: 'center',
            gap: 4,
            height: 28,
            lineHeight: '28px',
            maxWidth: isMobile ? 88 : 140,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {isMobile ? shortName : displayName}
        </Tag>
      </div>
    </Header>
  );
};

export default HeaderBar;
