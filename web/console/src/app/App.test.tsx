import { render, screen } from '@testing-library/react';
import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { App } from './App';
import { useSession } from './store';

function renderApp(path: string) {
  window.localStorage.setItem('paas_token', 'test');
  useSession.setState({ token: 'test', userName: '测试用户' });
  return render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={[path]}>
          <App />
        </MemoryRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );
}

test('显示中文控制台导航', async () => {
  renderApp('/apps');
  expect(await screen.findByText('平台控制台')).toBeInTheDocument();
  expect(screen.getAllByText('创建应用').length).toBeGreaterThan(0);
  expect(screen.getByText('构建')).toBeInTheDocument();
  expect(screen.getByPlaceholderText('搜索应用、资源、文档')).toBeInTheDocument();
});
