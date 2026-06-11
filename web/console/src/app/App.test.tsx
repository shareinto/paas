import { render, screen, within } from '@testing-library/react';
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
  const sider = document.querySelector('.ant-layout-sider') as HTMLElement;
  expect(within(sider).getByText('项目')).toBeInTheDocument();
  expect(within(sider).getByText('应用')).toBeInTheDocument();
  expect(within(sider).queryByText('版本')).not.toBeInTheDocument();
  expect(within(sider).getByText('构建管理')).toBeInTheDocument();
  expect(within(sider).getByText('审计日志')).toBeInTheDocument();
  expect(within(sider).queryByText('源码仓库')).not.toBeInTheDocument();
  expect(within(sider).queryByText('创建应用')).not.toBeInTheDocument();
  expect(within(sider).queryByText('构建')).not.toBeInTheDocument();
  expect(within(sider).queryByText('发布晋级')).not.toBeInTheDocument();
  expect(within(sider).queryByText('部署模板')).not.toBeInTheDocument();
  expect(screen.getByPlaceholderText('搜索应用、资源、文档')).toBeInTheDocument();
});

test('默认入口进入项目工作台列表', async () => {
  renderApp('/');

  expect(await screen.findByRole('heading', { name: '项目' })).toBeInTheDocument();
  expect(await screen.findByText('订单平台')).toBeInTheDocument();
});
