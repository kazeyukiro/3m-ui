import React from 'react';
import { Typography, Space } from 'antd';
import useIsMobile from '../hooks/useIsMobile';

const { Title, Text } = Typography;

export interface PageHeaderProps {
  title: React.ReactNode;
  subtitle?: React.ReactNode;
  extra?: React.ReactNode;
  style?: React.CSSProperties;
}

/**
 * Shared page title. On mobile keeps a tight block under the app bar
 * (no flex-grow) so the first cards sit directly below.
 */
const PageHeader: React.FC<PageHeaderProps> = ({ title, subtitle, extra, style }) => {
  const isMobile = useIsMobile();

  return (
    <div
      className="page-header"
      style={{
        marginBottom: isMobile ? 10 : 20,
        flex: 'none',
        flexGrow: 0,
        flexShrink: 0,
        ...style,
      }}
    >
      <div
        className="page-header-title-row"
        style={{
          display: 'flex',
          alignItems: isMobile ? 'stretch' : 'flex-start',
          justifyContent: 'space-between',
          gap: isMobile ? 8 : 12,
          flexWrap: 'wrap',
        }}
      >
        <div style={{ minWidth: 0, flex: isMobile ? '1 1 auto' : '1 1 200px', maxWidth: '100%' }}>
          <Title level={isMobile ? 4 : 3} style={{ margin: 0, lineHeight: 1.3 }}>
            {title}
          </Title>
          {subtitle ? (
            <Text
              type="secondary"
              className="page-header-subtitle"
              style={{ display: 'block', marginTop: 2, fontSize: isMobile ? 12 : undefined }}
            >
              {subtitle}
            </Text>
          ) : null}
        </div>
        {extra ? (
          <div className="page-header-actions" style={{ flex: isMobile ? '1 1 100%' : '0 0 auto' }}>
            <Space wrap size="small" style={{ width: isMobile ? '100%' : undefined }}>
              {extra}
            </Space>
          </div>
        ) : null}
      </div>
    </div>
  );
};

export default PageHeader;
