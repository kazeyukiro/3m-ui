import React from 'react';
import { Layout, Button, Space, Tag, Dropdown, Typography } from 'antd';
import {
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  UserOutlined,
  GlobalOutlined,
  BgColorsOutlined,
} from '@ant-design/icons';
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
        padding: isMobile ? '0 12px' : '0 24px',
        background: 'transparent',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 8,
        height: isMobile ? 52 : 64,
        lineHeight: isMobile ? '52px' : '64px',
      }}
    >
      {isMobile ? (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
          <img src="/logo.png" alt="" width={28} height={28} style={{ objectFit: 'contain', flexShrink: 0 }} />
          <Text strong style={{ fontSize: 16 }}>
            3M-UI
          </Text>
        </div>
      ) : (
        <Button
          type="text"
          icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
          onClick={() => setCollapsed(!collapsed)}
        />
      )}

      <Space size={isMobile ? 4 : 'middle'} wrap={false}>
        <Dropdown
          menu={{
            items: themeItems,
            selectedKeys: [mode],
            onClick: (e) => setMode(e.key as ThemeMode),
          }}
        >
          <Button type="text" icon={<BgColorsOutlined />}>
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
          <Button type="text" icon={<GlobalOutlined />}>
            {!isMobile && (LOCALE_OPTIONS.find((o) => o.key === locale)?.label || locale)}
          </Button>
        </Dropdown>
        <Tag icon={<UserOutlined />} title={displayName} style={{ marginInlineEnd: 0 }}>
          {isMobile ? shortName : displayName}
        </Tag>
      </Space>
    </Header>
  );
};

export default HeaderBar;
