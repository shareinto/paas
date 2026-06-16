import React from 'react';
import ReactDOM from 'react-dom/client';
import { ConfigProvider } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import { App } from './app/App';
import './styles.css';

const queryClient = new QueryClient();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider
      locale={zhCN}
      theme={{
        token: {
          colorPrimary: '#1f5fbf',
          colorSuccess: '#16834a',
          colorWarning: '#b76e00',
          colorError: '#c9352b',
          colorText: '#101828',
          colorTextSecondary: '#667085',
          colorBorder: '#d7dee8',
          colorBgLayout: '#f5f7fa',
          colorBgContainer: '#ffffff',
          borderRadius: 6,
          fontFamily: '-apple-system, BlinkMacSystemFont, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Noto Sans CJK SC", "Source Han Sans SC", "Segoe UI", Arial, sans-serif',
          fontSize: 13,
          controlHeight: 36,
          wireframe: false
        },
        components: {
          Button: { borderRadius: 6, controlHeight: 36 },
          Card: { headerHeight: 46, borderRadiusLG: 6 },
          Layout: { siderBg: '#111827', triggerBg: '#111827', headerBg: '#ffffff' },
          Menu: { darkItemBg: '#111827', darkSubMenuItemBg: '#111827', darkItemSelectedBg: '#1f5fbf' },
          Select: { controlHeight: 36 },
          Table: { cellPaddingBlock: 9, cellPaddingInline: 12, headerBg: '#f8fafc', headerColor: '#344054' },
          Form: { itemMarginBottom: 16 },
          Tabs: { cardBg: '#eef2f7' }
        }
      }}
    >
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </QueryClientProvider>
    </ConfigProvider>
  </React.StrictMode>
);
