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

  // Desktop only: restore collapsed preference is local; no drawer on mobile anymore
  useEffect(() => {
    if (isMobile) setCollapsed(false);
  }, [isMobile]);

  return (
    <Layout style={{ minHeight: '100vh' }}>
      {!isMobile && <Sidebar collapsed={collapsed} />}

      <Layout style={{ minWidth: 0, paddingBottom: isMobile ? 'calc(56px + env(safe-area-inset-bottom))' : 0 }}>
        <HeaderBar collapsed={collapsed} setCollapsed={setCollapsed} />
        <Content
          className="app-page-content"
          style={{
            margin: isMobile ? 8 : 24,
            padding: isMobile ? 12 : 24,
            background: colorBgContainer,
            borderRadius: borderRadiusLG,
            overflow: 'auto',
            minWidth: 0,
            maxWidth: isMobile ? undefined : 1400,
            width: isMobile ? undefined : 'calc(100% - 48px)',
            alignSelf: isMobile ? undefined : 'center',
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
