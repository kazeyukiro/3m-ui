import React, { useState, useEffect } from 'react';
import { Layout, theme } from 'antd';
import Sidebar from './Sidebar';
import HeaderBar from './Header';
import MobileBottomNav from './MobileBottomNav';
import useIsMobile from '../hooks/useIsMobile';

const { Content } = Layout;

const AppLayout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [collapsed, setCollapsed] = useState(false);
  const isMobile = useIsMobile();
  const {
    token: { colorBgContainer, borderRadiusLG },
  } = theme.useToken();

  useEffect(() => {
    if (isMobile) setCollapsed(false);
  }, [isMobile]);

  return (
    <Layout className={isMobile ? 'app-shell app-shell-mobile' : 'app-shell'} style={{ minHeight: '100dvh' }}>
      {!isMobile && <Sidebar collapsed={collapsed} />}

      <Layout
        className="app-main"
        style={{
          minWidth: 0,
          // Reserve bottom nav only; do not stretch the white content card full-height
          paddingBottom: isMobile ? 'calc(52px + env(safe-area-inset-bottom, 0px))' : 0,
        }}
      >
        <HeaderBar collapsed={collapsed} setCollapsed={setCollapsed} />
        <Content
          className="app-page-content"
          style={{
            margin: isMobile ? '6px 6px 8px' : 24,
            padding: isMobile ? 10 : 24,
            background: colorBgContainer,
            borderRadius: borderRadiusLG,
            overflow: 'auto',
            minWidth: 0,
            maxWidth: isMobile ? undefined : 1400,
            width: isMobile ? undefined : 'calc(100% - 48px)',
            alignSelf: isMobile ? 'stretch' : 'center',
            // Critical: prevent Ant Layout flex from stretching content into a tall empty card
            flex: isMobile ? '0 0 auto' : undefined,
            height: isMobile ? 'auto' : undefined,
            minHeight: isMobile ? 0 : undefined,
          }}
        >
          {children}
        </Content>
      </Layout>

      {isMobile && <MobileBottomNav />}
    </Layout>
  );
};

export default AppLayout;
