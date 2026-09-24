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
    token: { colorBgContainer, borderRadiusLG, colorBgLayout },
  } = theme.useToken();

  useEffect(() => {
    if (isMobile) setCollapsed(false);
  }, [isMobile]);

  return (
    <Layout
      className={isMobile ? 'app-shell app-shell-mobile' : 'app-shell'}
      style={{ minHeight: '100dvh', background: isMobile ? colorBgLayout : undefined }}
    >
      {!isMobile && <Sidebar collapsed={collapsed} />}

      <Layout
        className="app-main"
        style={{
          minWidth: 0,
          background: isMobile ? colorBgLayout : undefined,
          paddingBottom: isMobile ? 'calc(56px + env(safe-area-inset-bottom, 0px))' : 0,
        }}
      >
        <HeaderBar collapsed={collapsed} setCollapsed={setCollapsed} />
        <Content
          className={isMobile ? 'app-page-content app-page-content-mobile' : 'app-page-content'}
          style={
            isMobile
              ? {
                  // No full-height white panel — page bg shows through; cards sit tight under the title
                  margin: 0,
                  padding: '6px 8px 10px',
                  background: 'transparent',
                  borderRadius: 0,
                  boxShadow: 'none',
                  overflow: 'visible',
                  minWidth: 0,
                  flex: '0 0 auto',
                  height: 'auto',
                  minHeight: 0,
                  alignSelf: 'stretch',
                }
              : {
                  margin: 16,
                  padding: 20,
                  background: colorBgContainer,
                  borderRadius: borderRadiusLG,
                  overflow: 'auto',
                  minWidth: 0,
                  maxWidth: 1400,
                  width: 'calc(100% - 48px)',
                  alignSelf: 'center',
                }
          }
        >
          {children}
        </Content>
      </Layout>

      {isMobile && <MobileBottomNav />}
    </Layout>
  );
};

export default AppLayout;
