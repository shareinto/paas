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

test('应用详情默认展示应用 Workload 并保留镜像构建入口', async () => {
  renderPage();

  expect(await screen.findByRole('tab', { name: '应用 Workload' })).toHaveAttribute('aria-selected', 'true');
  expect(screen.getByRole('tab', { name: '镜像构建' })).toBeInTheDocument();
  expect(screen.getByRole('tab', { name: '发布晋级' })).toBeInTheDocument();
  ['总览', '环境', '版本', '构建', '配置', '日志', '监控', '设置'].forEach((name) => {
    expect(screen.queryByRole('tab', { name })).not.toBeInTheDocument();
  });
  const flow = screen.getByLabelText('交付流程');
  ['创建 Workload', '配置环境差异', '创建完整 Freight', '选择目标 Stage', '发布晋级', '回滚历史 Freight'].forEach((name) => {
    expect(within(flow).getByText(name)).toBeInTheDocument();
  });

  const workloadPanel = await screen.findByTestId('workload-panel');
  expect(await within(workloadPanel).findByText('Deployment')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('StatefulSet')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('流水线产物')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('自定义镜像')).toBeInTheDocument();
  expect(within(workloadPanel).getByRole('columnheader', { name: '端口' })).toBeInTheDocument();
  expect(within(workloadPanel).getByRole('columnheader', { name: '访问域名' })).toBeInTheDocument();
  expect(await within(workloadPanel).findByText('8080/TCP')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('order.example.com')).toBeInTheDocument();

  await userEvent.click(screen.getByRole('tab', { name: '镜像构建' }));
  expect(await screen.findByText('构建流水线')).toBeInTheDocument();
  const pipelinePanel = screen.getByText('构建流水线').closest('.ant-card') as HTMLElement;
  const pipelineRow = (await within(pipelinePanel).findByText('主流水线')).closest('tr') as HTMLElement;
  expect(within(pipelineRow).getByRole('button', { name: /编辑/ })).toBeInTheDocument();
});

test('点击发布晋级入口进入当前应用发布晋级页', async () => {
  renderPage();

  await userEvent.click(await screen.findByRole('tab', { name: '发布晋级' }));

  expect(await screen.findByText('应用发布晋级页面')).toBeInTheDocument();
});

test('创建 Workload 弹层展示步骤条运行参数和校验清单', async () => {
  renderPage();
  await userEvent.click(await screen.findByRole('button', { name: /创建 Workload/ }));

  const dialog = await screen.findByRole('dialog', { name: '创建 Workload' });
  ['基础信息', '镜像来源', '运行参数', '网络访问', '配置与目录', '预览校验'].forEach((name) => {
    expect(within(dialog).getByText(name)).toBeInTheDocument();
  });
  expect(within(dialog).getByLabelText('Workload 类型')).toBeInTheDocument();
  expect(within(dialog).getByLabelText('容器端口')).toBeInTheDocument();
  expect(within(dialog).getByLabelText('健康检查')).toBeInTheDocument();
  expect(within(dialog).getByText('校验清单')).toBeInTheDocument();

  await userEvent.click(within(dialog).getByText('StatefulSet'));
  expect(within(dialog).getByText('StatefulSet').closest('.ant-segmented-item')).toHaveClass('ant-segmented-item-selected');

  await userEvent.click(within(dialog).getByLabelText('镜像来源偏好'));
  await userEvent.click(await screen.findByTitle('发布时选择自定义镜像'));
  const imageInput = within(dialog).getByLabelText('自定义镜像地址');
  await userEvent.type(imageInput, 'registry.example.com/order/worker:20260611');
  expect(imageInput).toHaveValue('registry.example.com/order/worker:20260611');
  expect(within(dialog).getByText('当前 Workload 创建只保存 Workload 基础信息，自定义镜像请在创建 Freight 时选择；镜像 tag 可能被覆盖，建议使用 digest。')).toBeInTheDocument();
});

test('部署配置抽屉展示可编辑配置和值预览', async () => {
  renderPage();
  const workloadPanel = await screen.findByTestId('workload-panel');
  const apiRow = (await within(workloadPanel).findByText('order-api')).closest('tr') as HTMLElement;

  await userEvent.click(within(apiRow).getByRole('button', { name: '部署配置' }));

  const drawer = await screen.findByRole('dialog', { name: 'order-api 部署配置' });
  ['环境', 'Workload 类型', '资源规格', '探针', '网络访问', '配置文件', '环境变量', '可写目录', 'values 预览'].forEach((label) => {
    expect(within(drawer).getAllByText(label).length).toBeGreaterThan(0);
  });
  expect(within(drawer).getByText('保存配置')).toBeInTheDocument();
  expect(within(drawer).getByDisplayValue(/workload:/)).toBeInTheDocument();
  expect(document.querySelector('.ant-modal')).not.toBeInTheDocument();
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
