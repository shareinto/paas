import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import userEvent from '@testing-library/user-event';
import { afterEach } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { readFileSync } from 'node:fs';
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

afterEach(() => {
  cleanup();
  window.localStorage.clear();
  useSession.setState({ token: '', userName: '平台用户' });
});

test('显示中文控制台导航', async () => {
  renderApp('/apps');
  expect(await screen.findByText('CloudDeliver')).toBeInTheDocument();
  const sider = document.querySelector('.ant-layout-sider') as HTMLElement;
  expect(within(sider).getByText('项目')).toBeInTheDocument();
  expect(within(sider).getByText('应用')).toBeInTheDocument();
  expect(within(sider).queryByText('构建')).not.toBeInTheDocument();
  expect(within(sider).queryByText('部署')).not.toBeInTheDocument();
  expect(within(sider).queryByText('配置')).not.toBeInTheDocument();
  expect(within(sider).queryByText('版本')).not.toBeInTheDocument();
  expect(within(sider).getByText('构建管理')).toBeInTheDocument();
  expect(within(sider).getByText('审计日志')).toBeInTheDocument();
  expect(within(sider).queryByText('源码仓库')).not.toBeInTheDocument();
  expect(within(sider).queryByText('创建应用')).not.toBeInTheDocument();
  expect(within(sider).queryByText('发布晋级')).not.toBeInTheDocument();
  expect(within(sider).queryByText('部署模板')).not.toBeInTheDocument();
  expect(screen.queryByPlaceholderText('搜索应用、资源、文档')).not.toBeInTheDocument();
  expect(document.querySelector('.console-header')).not.toBeInTheDocument();
});

test('应用导航不再展示二级菜单且导航文字更醒目', async () => {
  renderApp('/apps/app_1');
  expect(await screen.findByText('CloudDeliver')).toBeInTheDocument();
  const sider = document.querySelector('.ant-layout-sider') as HTMLElement;
  const appItem = within(sider).getByText('应用').closest('.ant-menu-item') as HTMLElement;
  expect(within(appItem).getByLabelText('deployment-unit')).toBeInTheDocument();
  expect(within(sider).queryByText('构建')).not.toBeInTheDocument();
  expect(within(sider).queryByText('部署')).not.toBeInTheDocument();
  expect(within(sider).queryByText('配置')).not.toBeInTheDocument();

  const styles = readFileSync('src/styles.css', 'utf8');
  expect(styles).toMatch(/\.console-sider \.ant-menu-title-content \{[^}]*font-size: 15px;[^}]*font-weight: 650;/);
});

test('默认入口进入项目工作台列表', async () => {
  renderApp('/');

  expect(await screen.findByRole('heading', { name: '项目' })).toBeInTheDocument();
  expect(await screen.findByText('订单平台')).toBeInTheDocument();
});

test('打开应用详情时维护可关闭的应用 tab', async () => {
  renderApp('/apps');

  await userEvent.click(await screen.findByText('订单服务'));
  const tabs = await screen.findByTestId('application-detail-tabs');
  expect(tabs).toBeInTheDocument();
  expect(await within(tabs).findByRole('tab', { name: /订单服务/ })).toBeInTheDocument();

  await userEvent.click(within(document.querySelector('.console-sider') as HTMLElement).getByText('应用'));
  expect(await screen.findByRole('heading', { name: '应用' })).toBeInTheDocument();
  expect(within(tabs).getByRole('tab', { name: /订单服务/ })).toBeInTheDocument();

  await userEvent.click(await screen.findByText('订单前端'));
  expect(await within(tabs).findByRole('tab', { name: /订单前端/ })).toBeInTheDocument();
  expect(within(tabs).getByRole('tab', { name: /订单服务/ })).toBeInTheDocument();

  await userEvent.click(within(tabs).getByRole('tab', { name: /订单服务/ }));
  await waitFor(() => expect(within(tabs).getByRole('tab', { name: /订单服务/ })).toHaveAttribute('aria-selected', 'true'));
  expect((await screen.findAllByText('主流水线')).length).toBeGreaterThan(0);

  const activeTab = within(tabs).getByRole('tab', { name: /订单服务/ }).closest('.ant-tabs-tab') as HTMLElement;
  await userEvent.click(activeTab.querySelector('.ant-tabs-tab-remove') as HTMLElement);
  await waitFor(() => expect(within(tabs).queryByRole('tab', { name: /订单服务/ })).not.toBeInTheDocument());
  expect(within(tabs).getByRole('tab', { name: /订单前端/ })).toHaveAttribute('aria-selected', 'true');

  const lastTab = within(tabs).getByRole('tab', { name: /订单前端/ }).closest('.ant-tabs-tab') as HTMLElement;
  await userEvent.click(lastTab.querySelector('.ant-tabs-tab-remove') as HTMLElement);
  await waitFor(() => expect(screen.queryByTestId('application-detail-tabs')).not.toBeInTheDocument());
  expect(await screen.findByRole('heading', { name: '应用' })).toBeInTheDocument();
});
