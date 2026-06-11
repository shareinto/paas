import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, test } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ApplicationDetailPage } from './ApplicationDetailPage';

function renderPage() {
  return render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/apps/app_1']}>
          <Routes>
            <Route path="/apps/:id" element={<ApplicationDetailPage />} />
            <Route path="/apps/:id/promotions" element={<div>应用发布晋级页面</div>} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );
}

afterEach(() => {
  cleanup();
});

test('应用详情展示应用 Workload 入口和 Workload 列表', async () => {
  renderPage();

  expect(await screen.findByRole('tab', { name: '应用 Workload' })).toBeInTheDocument();
  expect(screen.getByRole('tab', { name: '发布晋级' })).toBeInTheDocument();

  const workloadPanel = await screen.findByTestId('workload-panel');
  expect(await within(workloadPanel).findByText('Deployment')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('StatefulSet')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('流水线产物')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('自定义镜像')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('v1.8.2')).toBeInTheDocument();
  expect(within(workloadPanel).getAllByText(/dev/).length).toBeGreaterThan(0);
  expect(within(workloadPanel).getAllByText(/prod/).length).toBeGreaterThan(0);
});

test('点击发布晋级入口进入当前应用发布晋级页', async () => {
  renderPage();

  await userEvent.click(await screen.findByRole('tab', { name: '发布晋级' }));

  expect(await screen.findByText('应用发布晋级页面')).toBeInTheDocument();
});

test('创建 Workload 抽屉支持类型切换和自定义镜像输入', async () => {
  renderPage();
  await userEvent.click(await screen.findByRole('button', { name: /创建 Workload/ }));

  const drawer = await screen.findByRole('dialog', { name: '创建 Workload' });
  expect(within(drawer).getByLabelText('Workload 类型')).toBeInTheDocument();

  await userEvent.click(within(drawer).getByText('StatefulSet'));
  expect(within(drawer).getByText('StatefulSet').closest('.ant-segmented-item')).toHaveClass('ant-segmented-item-selected');

  await userEvent.click(within(drawer).getByLabelText('镜像来源偏好'));
  await userEvent.click(await screen.findByTitle('发布时选择自定义镜像'));
  const imageInput = within(drawer).getByLabelText('自定义镜像地址');
  await userEvent.type(imageInput, 'registry.example.com/order/worker:20260611');
  expect(imageInput).toHaveValue('registry.example.com/order/worker:20260611');
  expect(within(drawer).getByText('当前 Workload 创建只保存 Workload 基础信息，自定义镜像请在创建 Freight 时选择；镜像 tag 可能被覆盖，建议使用 digest。')).toBeInTheDocument();
});

test('部署配置弹窗展示端口资源探针域名配置文件环境变量和可写目录字段', async () => {
  renderPage();
  const workloadPanel = await screen.findByTestId('workload-panel');
  const apiRow = (await within(workloadPanel).findByText('order-api')).closest('tr') as HTMLElement;

  await userEvent.click(within(apiRow).getByRole('button', { name: '部署配置' }));

  const modal = await screen.findByRole('dialog', { name: 'order-api 部署配置' });
  ['端口', '资源', '探针', '域名', '配置文件', '环境变量', '可写目录'].forEach((label) => {
    expect(within(modal).getAllByText(label).length).toBeGreaterThan(0);
  });
  expect(within(modal).getByText('配置内容摘要')).toBeInTheDocument();
  expect(await within(modal).findByText('spring.profiles.active: prod')).toBeInTheDocument();
  expect(within(modal).getByText('容量限制')).toBeInTheDocument();
  expect(within(modal).getByText('5Gi')).toBeInTheDocument();
  expect(document.querySelector('.ant-drawer')).not.toBeInTheDocument();
});

test('Workload 列表支持确认后删除 Workload', async () => {
  renderPage();
  const workloadPanel = await screen.findByTestId('workload-panel');
  const workerRow = (await within(workloadPanel).findByText('order-worker')).closest('tr') as HTMLElement;

  await userEvent.click(within(workerRow).getByRole('button', { name: '删除 Workload' }));
  await userEvent.click(await screen.findByRole('button', { name: '确认删除' }));

  await waitFor(() => {
    expect(within(workloadPanel).queryByText('order-worker')).not.toBeInTheDocument();
  });
  expect(within(workloadPanel).getByText('order-api')).toBeInTheDocument();
});

test('Workload 页面标题和按钮不使用英文用户文案', async () => {
  renderPage();
  const workloadPanel = await screen.findByTestId('workload-panel');
  await within(workloadPanel).findByText('Deployment');
  const visibleControls = within(workloadPanel)
    .getAllByRole('button')
    .map((item) => item.textContent?.trim() || '');

  expect(visibleControls).toEqual(expect.arrayContaining(['创建 Workload', '部署配置']));
  expect(visibleControls).toEqual(expect.arrayContaining(['删除']));
  expect(visibleControls).not.toEqual(expect.arrayContaining(['Create Workload', 'Deploy Config']));
  expect(within(workloadPanel).queryByText('Create Workload')).not.toBeInTheDocument();
  expect(within(workloadPanel).queryByText('Deploy Config')).not.toBeInTheDocument();
});
