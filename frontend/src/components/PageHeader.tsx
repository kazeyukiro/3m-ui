import React from 'react';
import { Typography, Space } from 'antd';
import useIsMobile from '../hooks/useIsMobile';

const { Title } = Typography;

/** Consistent page title + optional subtitle/actions for mobile & desktop. */
const PageHeader: React.FC<{
  title: React.ReactNode;
  subtitle?: React.ReactNode;
  extra?: React.ReactNode;
}> = ({ title, subtitle, extra }) => {
  const isMobile = useIsMobile();
  const sub =
    subtitle === undefined || subtitle === null || subtitle === false || subtitle === ''
      ? null
      : subtitle;

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
        {sub != null ? (
          <p
            className="page-header-subtitle"
            style={{
              margin: '6px 0 0',
              fontSize: isMobile ? 13 : 14,
              lineHeight: 1.45,
              opacity: 0.7,
              color: 'var(--ant-color-text-secondary, inherit)',
            }}
          >
            {sub}
          </p>
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
