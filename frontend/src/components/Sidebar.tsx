import React from 'react';
import { Layout, Menu } from 'antd';
import {
  DashboardOutlined, NodeIndexOutlined, UserOutlined, SettingOutlined,
  CodeOutlined, FileTextOutlined, ApiOutlined, LogoutOutlined, LineChartOutlined, CloudServerOutlined, BranchesOutlined, ShareAltOutlined,
} from '@ant-design/icons';
import { useNavigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '../stores/authStore';
import { useI18n } from '../i18n';

const { Sider } = Layout;

export function useSidebarMenuItems(onNavigate?: () => void) {
  const navigate = useNavigate();
  const location = useLocation();
  const logout = useAuthStore((s) => s.logout);
  const { t } = useI18n();

  const items = [
    { key: '/', icon: <DashboardOutlined />, label: t('nav.dashboard') },
    { key: '/listeners', icon: <NodeIndexOutlined />, label: t('nav.listeners') },
    { key: '/users', icon: <UserOutlined />, label: t('nav.users') },
    { key: '/share', icon: <ShareAltOutlined />, label: t('nav.share') },
    { key: '/traffic', icon: <LineChartOutlined />, label: t('nav.traffic') },
    { key: '/cluster', icon: <CloudServerOutlined />, label: t('nav.cluster') },
    { key: '/routing', icon: <BranchesOutlined />, label: t('nav.routing') },
    { key: '/core', icon: <ApiOutlined />, label: t('nav.core') },
    { key: '/logs', icon: <FileTextOutlined />, label: t('nav.logs') },
    { key: '/config', icon: <CodeOutlined />, label: t('nav.config') },
    { key: '/settings', icon: <SettingOutlined />, label: t('nav.settings') },
  ];

  const onMenuClick = ({ key }: { key: string }) => {
    navigate(key);
    onNavigate?.();
  };

  const onLogout = () => {
    logout();
    navigate('/login');
    onNavigate?.();
  };

  return { items, selectedKeys: [location.pathname], onMenuClick, onLogout, t };
}

export const SidebarMenu: React.FC<{ onNavigate?: () => void; style?: React.CSSProperties }> = ({
  onNavigate,
  style,
}) => {
  const { items, selectedKeys, onMenuClick, onLogout, t } = useSidebarMenuItems(onNavigate);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', ...style }}>
      <div style={{ height: 64, display: 'flex', alignItems: 'center', justifyContent: 'center', fontWeight: 700, fontSize: 18, flexShrink: 0 }}>3M-UI</div>
      <Menu mode="inline" selectedKeys={selectedKeys} items={items} onClick={onMenuClick} style={{ flex: 1, borderInlineEnd: 'none' }} />
      <Menu mode="inline" selectable={false} items={[{ key: 'logout', icon: <LogoutOutlined />, label: t('nav.logout'), onClick: onLogout }]} style={{ borderInlineEnd: 'none', borderTop: '1px solid rgba(5,5,5,0.06)' }} />
    </div>
  );
};

const Sidebar: React.FC<{ collapsed: boolean }> = ({ collapsed }) => (
  <Sider trigger={null} collapsible collapsed={collapsed} theme="light" breakpoint="md" collapsedWidth={80} width={220}
    style={{ overflow: 'auto', height: '100vh', position: 'sticky', insetInlineStart: 0, top: 0, bottom: 0 }}>
    <SidebarMenu />
  </Sider>
);

export default Sidebar;
