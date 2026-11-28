import React from 'react';
import AppLayout from '../components/AppLayout';

const SidebarLayout: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  return <AppLayout>{children}</AppLayout>;
};

export default SidebarLayout;
