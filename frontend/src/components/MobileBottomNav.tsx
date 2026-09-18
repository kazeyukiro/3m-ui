import React, { useMemo, useState } from 'react';
import { Drawer, Grid } from 'antd';
import {
  DashboardOutlined,
  DeploymentUnitOutlined,
  TeamOutlined,
  FundProjectionScreenOutlined,
  AppstoreOutlined,
  ToolOutlined,
  ShareAltOutlined,
  CloudServerOutlined,
  ForkOutlined,
  RocketOutlined,
  ProfileOutlined,
  ControlOutlined,
  LogoutOutlined,
} from '@ant-design/icons';
import { useLocation, useNavigate } from 'react-router-dom';
import { useI18n } from '../i18n';
import { useAuthStore } from '../stores/authStore';

type NavItem = {
  key: string;
  icon: React.ReactNode;
  label: string;
  primary?: boolean;
};

/**
 * Mobile bottom tab bar. Primary destinations stay on the bar;
 * the rest open from "More" so all routes remain reachable.
 */
const MobileBottomNav: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { t } = useI18n();
  const logout = useAuthStore((s) => s.logout);
  const [moreOpen, setMoreOpen] = useState(false);
  const screens = Grid.useBreakpoint();

  // Only render on narrow layouts (hook may lag one frame; AppLayout also gates)
  const allItems: NavItem[] = useMemo(
    () => [
      { key: '/', icon: <DashboardOutlined />, label: t('nav.dashboard'), primary: true },
      { key: '/listeners', icon: <DeploymentUnitOutlined />, label: t('nav.listeners'), primary: true },
      { key: '/users', icon: <TeamOutlined />, label: t('nav.users'), primary: true },
      { key: '/traffic', icon: <FundProjectionScreenOutlined />, label: t('nav.traffic'), primary: true },
      { key: '/share', icon: <ShareAltOutlined />, label: t('nav.share') },
      { key: '/cluster', icon: <CloudServerOutlined />, label: t('nav.cluster') },
      { key: '/routing', icon: <ForkOutlined />, label: t('nav.routing') },
      { key: '/core', icon: <RocketOutlined />, label: t('nav.core') },
      { key: '/logs', icon: <ProfileOutlined />, label: t('nav.logs') },
      { key: '/config', icon: <ControlOutlined />, label: t('nav.config') },
      { key: '/settings', icon: <ToolOutlined />, label: t('nav.settings') },
    ],
    [t],
  );

  const primary = allItems.filter((i) => i.primary);
  const moreItems = allItems.filter((i) => !i.primary);

  const path = location.pathname;
  const isActive = (key: string) => (key === '/' ? path === '/' : path === key || path.startsWith(`${key}/`));
  const moreActive = moreItems.some((i) => isActive(i.key));

  const go = (key: string) => {
    navigate(key);
    setMoreOpen(false);
  };

  const onLogout = () => {
    logout();
    setMoreOpen(false);
    navigate('/login');
  };

  // Hide if somehow rendered on desktop
  if (screens.md) return null;

  return (
    <>
      <nav className="app-bottom-nav" aria-label="Main">
        {primary.map((item) => {
          const active = isActive(item.key);
          return (
            <button
              key={item.key}
              type="button"
              className={`app-bottom-nav-item${active ? ' is-active' : ''}`}
              onClick={() => go(item.key)}
              aria-current={active ? 'page' : undefined}
            >
              <span className="app-bottom-nav-icon">{item.icon}</span>
              <span className="app-bottom-nav-label">{item.label}</span>
            </button>
          );
        })}
        <button
          type="button"
          className={`app-bottom-nav-item${moreActive || moreOpen ? ' is-active' : ''}`}
          onClick={() => setMoreOpen(true)}
          aria-haspopup="dialog"
          aria-expanded={moreOpen}
        >
          <span className="app-bottom-nav-icon">
            <AppstoreOutlined />
          </span>
          <span className="app-bottom-nav-label">{t('nav.more', 'More')}</span>
        </button>
      </nav>

      <Drawer
        className="app-bottom-more-drawer"
        placement="bottom"
        open={moreOpen}
        onClose={() => setMoreOpen(false)}
        height="auto"
        title={t('nav.more', 'More')}
        styles={{
          body: { padding: '8px 12px 16px' },
          wrapper: { maxHeight: '75dvh' },
        }}
      >
        <div className="app-bottom-more-grid">
          {moreItems.map((item) => {
            const active = isActive(item.key);
            return (
              <button
                key={item.key}
                type="button"
                className={`app-bottom-more-item${active ? ' is-active' : ''}`}
                onClick={() => go(item.key)}
              >
                <span className="app-bottom-more-icon">{item.icon}</span>
                <span className="app-bottom-more-label">{item.label}</span>
              </button>
            );
          })}
          <button type="button" className="app-bottom-more-item is-danger" onClick={onLogout}>
            <span className="app-bottom-more-icon">
              <LogoutOutlined />
            </span>
            <span className="app-bottom-more-label">{t('nav.logout')}</span>
          </button>
        </div>
      </Drawer>
    </>
  );
};

export default MobileBottomNav;
