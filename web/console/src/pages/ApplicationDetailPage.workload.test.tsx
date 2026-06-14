import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, test } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ApplicationWorkspacePage } from './ApplicationDetailPage';

function renderPage(path = '/apps/app_1/build') {
  return render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={[path]}>
          <Routes>
            <Route path="/apps/:id/build" element={<ApplicationWorkspacePage section="build" />} />
            <Route path="/apps/:id/deploy" element={<ApplicationWorkspacePage section="deploy" />} />
            <Route path="/apps/:id/config" element={<ApplicationWorkspacePage section="config" />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );
}

afterEach(() => {
  cleanup();
});

test('应用构建页只展示流水线并移除摘要和页签', async () => {
  renderPage();

  expect((await screen.findAllByLabelText('选择项目')).length).toBeGreaterThan(0);
  expect(screen.getAllByLabelText('选择应用').length).toBeGreaterThan(0);
  expect(screen.queryByText('应用标识')).not.toBeInTheDocument();
  expect(screen.queryByText('所属项目')).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /编辑应用/ })).not.toBeInTheDocument();
  expect(screen.queryByText('流水线绑定 Workload、代码源和运行时环境。')).not.toBeInTheDocument();
  ['构建', '部署', '应用 Workload', '镜像构建', '发布晋级', '总览', '环境', '版本', '配置', '日志', '监控', '设置'].forEach((name) => {
    expect(screen.queryByRole('tab', { name })).not.toBeInTheDocument();
  });
  expect(screen.queryByLabelText('交付流程')).not.toBeInTheDocument();

  const pipelinePanel = await screen.findByTestId('pipeline-panel');
  expect(within(pipelinePanel).queryByRole('heading', { name: '流水线' })).not.toBeInTheDocument();
  expect(within(pipelinePanel).getByRole('button', { name: /创建流水线/ })).toBeInTheDocument();
  expect(screen.queryByTestId('workload-panel')).not.toBeInTheDocument();
  const pipelineCard = (await within(pipelinePanel).findByText('主流水线')).closest('.resource-card') as HTMLElement;
  expect(within(pipelineCard).getByText('关联 Workload')).toBeInTheDocument();
  expect(within(pipelineCard).getByText('代码源')).toBeInTheDocument();
  expect(within(pipelineCard).getByText('运行时环境')).toBeInTheDocument();
  expect(within(pipelineCard).getByRole('button', { name: /触发构建/ })).toBeInTheDocument();
  expect(within(pipelineCard).queryByRole('button', { name: /历史/ })).not.toBeInTheDocument();
});

test('部署页展示发布晋级内容', async () => {
  renderPage('/apps/app_1/deploy');

  expect(await screen.findByText('Freight 时间轴')).toBeInTheDocument();
  expect(await screen.findByLabelText('应用部署 DAG')).toBeInTheDocument();
  expect(screen.getByTestId('promotion-confirm-panel')).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /编辑应用/ })).not.toBeInTheDocument();
  expect(screen.queryByRole('heading', { name: '发布晋级' })).not.toBeInTheDocument();
  expect(screen.queryByText('拖拽 Freight 到目标 Stage，系统按交付流 DAG 校验上游依赖、审批和验证要求。')).not.toBeInTheDocument();
  expect(screen.queryByTestId('pipeline-panel')).not.toBeInTheDocument();
  expect(screen.queryByTestId('workload-panel')).not.toBeInTheDocument();
});

test('配置页只展示工作负载管理', async () => {
  renderPage('/apps/app_1/config');

  const workloadPanel = await screen.findByTestId('workload-panel');
  expect(screen.queryByRole('button', { name: /编辑应用/ })).not.toBeInTheDocument();
  expect(within(workloadPanel).queryByRole('heading', { name: '工作负载管理' })).not.toBeInTheDocument();
  expect(within(workloadPanel).queryByText('按最小可独立部署单元管理 Workload。')).not.toBeInTheDocument();
  expect(within(workloadPanel).getByRole('button', { name: /创建工作负载/ })).toBeInTheDocument();
  expect(await within(workloadPanel).findByText('无状态')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('有状态')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('流水线产物')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('自定义镜像')).toBeInTheDocument();
  expect(await within(workloadPanel).findByText('8080/TCP')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('order.example.com')).toBeInTheDocument();
  expect(screen.queryByTestId('pipeline-panel')).not.toBeInTheDocument();
});

test('流水线构建弹窗按倒序号展示构建并可查看日志', async () => {
  renderPage();

  const pipelinePanel = await screen.findByTestId('pipeline-panel');
  const pipelineCard = (await within(pipelinePanel).findByText('主流水线')).closest('.resource-card') as HTMLElement;
  await userEvent.click(within(pipelineCard).getByRole('button', { name: /触发构建/ }));

  const dialog = await screen.findByRole('dialog', { name: /构建历史/ });
  expect(await within(dialog).findByRole('button', { name: /触发构建/ })).toBeInTheDocument();
  expect(await within(dialog).findByText('构建 2')).toBeInTheDocument();
  expect(within(dialog).getByText('构建时间')).toBeInTheDocument();
  expect(within(dialog).queryByText(/build_/)).not.toBeInTheDocument();
  await userEvent.click(within(dialog).getByText('构建 2'));
  expect(await within(dialog).findByText(/\[INFO\] 检出平台托管源码仓库/)).toBeInTheDocument();
});

test('工作负载创建弹层使用滚动大页并最终创建', async () => {
  renderPage('/apps/app_1/config');
  await userEvent.click(await screen.findByRole('button', { name: /创建工作负载/ }));

  const dialog = await screen.findByRole('dialog', { name: '创建工作负载' });
  expect(within(dialog).queryByText('校验清单')).not.toBeInTheDocument();
  expect(within(dialog).queryByRole('button', { name: '下一步' })).not.toBeInTheDocument();
  expect(within(dialog).queryByRole('button', { name: '上一步' })).not.toBeInTheDocument();
  expect(within(dialog).queryByText('预览校验')).not.toBeInTheDocument();
  expect(within(dialog).getByTestId('workload-large-form')).toBeInTheDocument();
  await userEvent.type(within(dialog).getByLabelText('工作负载标识'), 'order-search');
  await userEvent.type(within(dialog).getByLabelText('显示名称'), '订单搜索');
  await userEvent.click(within(dialog).getByText('有状态'));
  expect(within(dialog).getByText('有状态').closest('.ant-segmented-item')).toHaveClass('ant-segmented-item-selected');
  await userEvent.click(within(dialog).getByText('自定义镜像'));
  expect(within(dialog).getByText('自定义镜像').closest('.ant-segmented-item')).toHaveClass('ant-segmented-item-selected');
  await userEvent.click(within(dialog).getByRole('button', { name: '添加环境变量' }));
  await userEvent.type(within(dialog).getByLabelText('环境变量键 1'), 'group');
  await userEvent.type(within(dialog).getByLabelText('环境变量值 1'), 'iot');
  await userEvent.click(within(dialog).getByRole('button', { name: '添加配置文件' }));
  await userEvent.type(within(dialog).getByLabelText('配置文件路径 1'), '/etc/app/app.yaml');
  await userEvent.type(within(dialog).getByLabelText('配置文件内容 1'), 'server.port: 8080');
  await userEvent.click(within(dialog).getByRole('button', { name: '添加可写目录' }));
  await userEvent.type(within(dialog).getByLabelText('可写目录 1'), '/data');
  await userEvent.type(within(dialog).getByLabelText('目录属主 1'), 'app:app');
  await userEvent.type(within(dialog).getByLabelText('目录权限 1'), '0775');
  await userEvent.click(within(dialog).getByRole('button', { name: '创建' }));

  expect(await screen.findByText('order-search')).toBeInTheDocument();
});

test('工作负载编辑使用中文弹窗保存后更新卡片', async () => {
  renderPage('/apps/app_1/config');
  const workloadPanel = await screen.findByTestId('workload-panel');
  const apiCard = (await within(workloadPanel).findByText('order-api')).closest('.resource-card') as HTMLElement;

  await userEvent.click(within(apiCard).getByRole('button', { name: '编辑' }));

  const dialog = await screen.findByRole('dialog', { name: '编辑工作负载' });
  expect(document.querySelector('.ant-drawer')).not.toBeInTheDocument();
  await userEvent.clear(within(dialog).getByLabelText('显示名称'));
  await userEvent.type(within(dialog).getByLabelText('显示名称'), '订单接口 v2');
  await userEvent.click(within(dialog).getByRole('button', { name: '保存' }));

  expect(await within(workloadPanel).findByText('订单接口 v2')).toBeInTheDocument();
});

test('工作负载列表支持确认后删除', async () => {
  renderPage('/apps/app_1/config');
  const workloadPanel = await screen.findByTestId('workload-panel');
  const workerCard = (await within(workloadPanel).findByText('order-worker')).closest('.resource-card') as HTMLElement;

  await userEvent.click(within(workerCard).getByRole('button', { name: '删除' }));
  await userEvent.click(await screen.findByRole('button', { name: '确认删除' }));

  await waitFor(() => {
    expect(within(workloadPanel).queryByText('order-worker')).not.toBeInTheDocument();
  });
  expect(within(workloadPanel).getByText('order-api')).toBeInTheDocument();
});

test('工作负载页面标题和按钮不使用英文用户文案', async () => {
  renderPage('/apps/app_1/config');
  const workloadPanel = await screen.findByTestId('workload-panel');
  await within(workloadPanel).findByText('无状态');
  const visibleControls = within(workloadPanel)
    .getAllByRole('button')
    .map((item) => item.textContent?.trim() || '');

  expect(visibleControls).toEqual(expect.arrayContaining(['创建工作负载', '编辑']));
  expect(visibleControls).toEqual(expect.arrayContaining(['删除']));
  expect(visibleControls).not.toEqual(expect.arrayContaining(['创建 Workload', 'Create Workload', 'Deploy Config']));
  expect(within(workloadPanel).queryByText('Workload')).not.toBeInTheDocument();
  expect(within(workloadPanel).queryByText('Create Workload')).not.toBeInTheDocument();
  expect(within(workloadPanel).queryByText('Deploy Config')).not.toBeInTheDocument();
});
