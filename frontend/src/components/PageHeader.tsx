import React from 'react';
import { Typography, Space } from 'antd';
import useIsMobile from '../hooks/useIsMobile';

const { Title, Paragraph } = Typography;

/** Consistent page title + optional subtitle/actions for mobile & desktop. */
const PageHeader: React.FC<{
  title: React.ReactNode;
  subtitle?: React.ReactNode;
  extra?: React.ReactNode;
}> = ({ title, subtitle, extra }) => {
  const isMobile = useIsMobile();
  return (
    <div
      className="page-header"
      style={{
        display: 'flex',
        flexDirection: isMobile ? 'column' : 'row',
        alignItems: isMobile ? 'stretch' : 'flex-start',
        justifyContent: 'space-between',
        gap: isMobile ? 8 : 16,
        marginBottom: isMobile ? 12 : 16,
      }}
    >
      <div style={{ minWidth: 0, flex: 1 }}>
        <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}>
          {title}
        </Title>
        {subtitle ? (
          <Paragraph type="secondary" style={{ margin: '4px 0 0', fontSize: isMobile ? 13 : 14 }}>
            {subtitle}
          </Paragraph>
        ) : null}
      </div>
      {extra ? (
        <Space wrap style={{ width: isMobile ? '100%' : undefined }}>
          {extra}
        </Space>
      ) : null}
    </div>
  );
};

export default PageHeader;
