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

test('应用详情只展示构建和部署两个页签并默认进入构建', async () => {
  renderPage();

  expect(await screen.findByRole('tab', { name: '构建' })).toHaveAttribute('aria-selected', 'true');
  expect(screen.getByRole('tab', { name: '部署' })).toBeInTheDocument();
  ['应用 Workload', '镜像构建', '发布晋级', '总览', '环境', '版本', '配置', '日志', '监控', '设置'].forEach((name) => {
    expect(screen.queryByRole('tab', { name })).not.toBeInTheDocument();
  });
  const flow = screen.getByLabelText('交付流程');
  ['创建 Workload', '配置环境差异', '创建完整 Freight', '选择目标 Stage', '发布晋级', '回滚历史 Freight'].forEach((name) => {
    expect(within(flow).getByText(name)).toBeInTheDocument();
  });

  const buildTab = await screen.findByTestId('build-tab');
  expect(within(buildTab).getByRole('heading', { name: '流水线' })).toBeInTheDocument();
  expect(within(buildTab).getByRole('heading', { name: '工作负载管理' })).toBeInTheDocument();
  expect(within(buildTab).getByRole('button', { name: /创建流水线/ })).toBeInTheDocument();
  expect(within(buildTab).getByRole('button', { name: /创建 Workload/ })).toBeInTheDocument();
  expect(within(buildTab).queryByRole('columnheader', { name: '端口' })).not.toBeInTheDocument();

  const workloadPanel = await screen.findByTestId('workload-panel');
  expect(await within(workloadPanel).findByText('Deployment')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('StatefulSet')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('流水线产物')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('自定义镜像')).toBeInTheDocument();
  expect(await within(workloadPanel).findByText('8080/TCP')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('order.example.com')).toBeInTheDocument();

  const pipelinePanel = await screen.findByTestId('pipeline-panel');
  const pipelineCard = (await within(pipelinePanel).findByText('主流水线')).closest('.resource-card') as HTMLElement;
  expect(within(pipelineCard).getByText('绑定 Workload')).toBeInTheDocument();
  expect(within(pipelineCard).getByText('代码源')).toBeInTheDocument();
  expect(within(pipelineCard).getByText('运行时环境')).toBeInTheDocument();
  expect(within(pipelineCard).getByRole('button', { name: /历史/ })).toBeInTheDocument();
});

test('部署页签嵌入发布晋级内容且保留旧路由兼容', async () => {
  renderPage();

  await userEvent.click(await screen.findByRole('tab', { name: '部署' }));

  expect(await screen.findByText('Freight 时间轴')).toBeInTheDocument();
  expect(screen.getByTestId('promotion-confirm-panel')).toBeInTheDocument();
});

test('流水线历史弹窗按序号展示构建并可查看日志', async () => {
  renderPage();

  const pipelinePanel = await screen.findByTestId('pipeline-panel');
  const pipelineCard = (await within(pipelinePanel).findByText('主流水线')).closest('.resource-card') as HTMLElement;
  await userEvent.click(within(pipelineCard).getByRole('button', { name: /历史/ }));

  const dialog = await screen.findByRole('dialog', { name: /构建历史/ });
  expect(await within(dialog).findByText('构建 1')).toBeInTheDocument();
  expect(within(dialog).getByText('构建时间')).toBeInTheDocument();
  expect(within(dialog).queryByText(/build_/)).not.toBeInTheDocument();
  await userEvent.click(within(dialog).getByText('构建 1'));
  expect(await within(dialog).findByText(/\[INFO\] 检出平台托管源码仓库/)).toBeInTheDocument();
});

test('Workload 创建弹层支持分步并最终创建', async () => {
  renderPage();
  await userEvent.click(await screen.findByRole('button', { name: /创建 Workload/ }));

  const dialog = await screen.findByRole('dialog', { name: '创建 Workload' });
  ['基础信息', '镜像来源', '运行参数', '网络访问', '配置与目录', '预览校验'].forEach((name) => {
    expect(within(dialog).getByText(name)).toBeInTheDocument();
  });
  await userEvent.type(within(dialog).getByLabelText('Workload 标识'), 'order-search');
  await userEvent.type(within(dialog).getByLabelText('显示名称'), '订单搜索');
  await userEvent.click(within(dialog).getByText('StatefulSet'));
  expect(within(dialog).getByText('StatefulSet').closest('.ant-segmented-item')).toHaveClass('ant-segmented-item-selected');
  await userEvent.click(within(dialog).getByRole('button', { name: '下一步' }));

  await userEvent.click(within(dialog).getByLabelText('镜像来源偏好'));
  await userEvent.click(await screen.findByTitle('发布时选择自定义镜像'));
  expect(within(dialog).getByText('自定义镜像只保存来源偏好，实际镜像版本在创建 Freight 时选择。')).toBeInTheDocument();
  await userEvent.click(within(dialog).getByRole('button', { name: '下一步' }));
  await userEvent.click(within(dialog).getByRole('button', { name: '下一步' }));
  await userEvent.click(within(dialog).getByRole('button', { name: '下一步' }));
  await userEvent.click(within(dialog).getByRole('button', { name: '下一步' }));
  await userEvent.click(within(dialog).getByRole('button', { name: '创建' }));

  expect(await screen.findByText('order-search')).toBeInTheDocument();
});

test('Workload 编辑使用弹窗保存后更新卡片', async () => {
  renderPage();
  const workloadPanel = await screen.findByTestId('workload-panel');
  const apiCard = (await within(workloadPanel).findByText('order-api')).closest('.resource-card') as HTMLElement;

  await userEvent.click(within(apiCard).getByRole('button', { name: '编辑' }));

  const dialog = await screen.findByRole('dialog', { name: '编辑 Workload' });
  expect(document.querySelector('.ant-drawer')).not.toBeInTheDocument();
  await userEvent.clear(within(dialog).getByLabelText('显示名称'));
  await userEvent.type(within(dialog).getByLabelText('显示名称'), '订单接口 v2');
  for (let i = 0; i < 5; i += 1) {
    await userEvent.click(within(dialog).getByRole('button', { name: '下一步' }));
  }
  await userEvent.click(within(dialog).getByRole('button', { name: '保存' }));

  expect(await within(workloadPanel).findByText('订单接口 v2')).toBeInTheDocument();
});

test('Workload 列表支持确认后删除 Workload', async () => {
  renderPage();
  const workloadPanel = await screen.findByTestId('workload-panel');
  const workerCard = (await within(workloadPanel).findByText('order-worker')).closest('.resource-card') as HTMLElement;

  await userEvent.click(within(workerCard).getByRole('button', { name: '删除' }));
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

  expect(visibleControls).toEqual(expect.arrayContaining(['创建 Workload', '编辑']));
  expect(visibleControls).toEqual(expect.arrayContaining(['删除']));
  expect(visibleControls).not.toEqual(expect.arrayContaining(['Create Workload', 'Deploy Config']));
  expect(within(workloadPanel).queryByText('Create Workload')).not.toBeInTheDocument();
  expect(within(workloadPanel).queryByText('Deploy Config')).not.toBeInTheDocument();
});
