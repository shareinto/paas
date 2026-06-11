import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, within } from '@testing-library/react';
import { afterEach, expect, test } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { FreightsPage } from './FreightsPage';

function renderPage() {
  return render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/freights']}>
          <FreightsPage />
        </MemoryRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );
}

afterEach(() => {
  cleanup();
});

test('版本页展示完整应用 Freight 摘要而不是单镜像列表', async () => {
  renderPage();

  expect(await screen.findByRole('heading', { name: '版本' })).toBeInTheDocument();
  expect(await screen.findByText('20260611.1')).toBeInTheDocument();
  expect(screen.getByRole('columnheader', { name: '覆盖 Workload' })).toBeInTheDocument();
  expect(screen.getByRole('columnheader', { name: '镜像摘要' })).toBeInTheDocument();
  expect(screen.queryByRole('columnheader', { name: '镜像 digest' })).not.toBeInTheDocument();

  const row = screen.getByText('20260611.1').closest('tr') as HTMLElement;
  expect(within(row).getByText('3 个 Workload')).toBeInTheDocument();
  expect(within(row).getByText(/前端入口/)).toBeInTheDocument();
  expect(within(row).getByText(/订单接口/)).toBeInTheDocument();
  expect(within(row).getByText(/异步任务/)).toBeInTheDocument();
});
