import React, { useState, useEffect } from 'react';
import { Layout, Drawer, theme } from 'antd';
import Sidebar, { SidebarMenu } from './Sidebar';
import HeaderBar from './Header';
import useIsMobile from '../hooks/useIsMobile';

const { Content } = Layout;

const AppLayout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [collapsed, setCollapsed] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const isMobile = useIsMobile();
  const {
    token: { colorBgContainer, borderRadiusLG },
  } = theme.useToken();

  // Close drawer when switching to desktop
  useEffect(() => {
    if (!isMobile) setDrawerOpen(false);
  }, [isMobile]);

  return (
    <Layout style={{ minHeight: '100vh' }}>
      {!isMobile && <Sidebar collapsed={collapsed} />}

      <Layout style={{ minWidth: 0 }}>
        <HeaderBar
          collapsed={collapsed}
          setCollapsed={setCollapsed}
          onOpenMobileNav={() => setDrawerOpen(true)}
        />
        <Content
          className="app-page-content"
          style={{
            margin: isMobile ? 8 : 24,
            padding: isMobile ? 12 : 24,
            background: colorBgContainer,
            borderRadius: borderRadiusLG,
            overflow: 'auto',
            minWidth: 0,
          }}
        >
          {children}
        </Content>
      </Layout>

      <Drawer
        className="app-mobile-drawer"
        placement="left"
        open={isMobile && drawerOpen}
        onClose={() => setDrawerOpen(false)}
        width={280}
        styles={{ body: { padding: 0 } }}
        title={null}
        closable={false}
      >
        <div className="mobile-nav-brand">3M-UI</div>
        <SidebarMenu onNavigate={() => setDrawerOpen(false)} />
      </Drawer>
    </Layout>
  );
};

export default AppLayout;
