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
        token: { colorPrimary: '#1677ff', borderRadius: 6, fontSize: 13 },
        components: {
          Layout: { siderBg: '#071426', triggerBg: '#071426', headerBg: '#ffffff' },
          Table: { cellPaddingBlock: 9, cellPaddingInline: 12 },
          Form: { itemMarginBottom: 16 }
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
