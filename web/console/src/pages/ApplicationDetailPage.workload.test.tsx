import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, test } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ApplicationWorkspacePage } from './ApplicationDetailPage';

function renderPage(path = '/apps/app_1') {
  return render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={[path]}>
          <Routes>
            <Route path="/apps/:id" element={<ApplicationWorkspacePage />} />
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

test('应用详情展示统一交付工作台并移除二级页签', async () => {
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

  const leftColumn = await screen.findByTestId('delivery-workspace-left');
  const rightColumn = await screen.findByTestId('delivery-workspace-right');
  expect(leftColumn).toHaveClass('application-delivery-left');
  expect(rightColumn).toHaveClass('application-delivery-right');
  expect(within(rightColumn).getByLabelText('应用部署 DAG')).toBeInTheDocument();
  expect(within(rightColumn).queryByText('发布包')).not.toBeInTheDocument();

  const freightPanel = within(leftColumn).getByText('1 发布包').closest('.workspace-section-card') as HTMLElement;
  expect(freightPanel).toBeInTheDocument();
  expect(freightPanel.querySelector('.workspace-section-title')).toBeInTheDocument();
  expect(freightPanel.querySelector('.anticon-inbox')).toBeInTheDocument();
  const createFreightButton = await within(freightPanel).findByRole('button', { name: /创建 Freight/ });
  await waitFor(() => expect(createFreightButton).not.toBeDisabled());
  expect(createFreightButton).toHaveTextContent('');
  const freightCard = (await within(freightPanel).findByText('20260609.1')).closest('.freight-timeline-card') as HTMLElement;
  expect(freightCard).toHaveClass('workspace-item-card', 'workspace-item-card--freight');
  expect(within(freightCard).queryByText('拖拽')).not.toBeInTheDocument();
  expect(within(freightCard).queryByText('2026-06-09 14:20')).not.toBeInTheDocument();
  expect(within(freightCard).queryByText('前端入口')).not.toBeInTheDocument();
  expect(within(freightCard).getByRole('button', { name: '审批' })).toHaveTextContent('');

  const pipelinePanel = await within(leftColumn).findByTestId('pipeline-panel');
  expect(pipelinePanel).toHaveClass('workspace-section-card');
  expect(pipelinePanel.querySelector('.workspace-section-head')).toBeInTheDocument();
  expect(pipelinePanel.querySelector('.workspace-section-title')).toBeInTheDocument();
  expect(pipelinePanel.querySelector('.anticon-build')).toBeInTheDocument();
  expect(within(pipelinePanel).getByText('2 构建')).toBeInTheDocument();
  const createPipelineButton = within(pipelinePanel).getByRole('button', { name: /创建流水线/ });
  expect(createPipelineButton).toBeInTheDocument();
  expect(createPipelineButton).toHaveTextContent('');
  const workloadPanel = await within(leftColumn).findByTestId('workload-panel');
  expect(workloadPanel).toHaveClass('workspace-section-card');
  expect(workloadPanel.querySelector('.workspace-section-head')).toBeInTheDocument();
  expect(workloadPanel.querySelector('.workspace-section-title')).toBeInTheDocument();
  expect(workloadPanel.querySelector('.anticon-deployment-unit')).toBeInTheDocument();
  expect(within(workloadPanel).getByText('3 工作负载')).toBeInTheDocument();
  const createWorkloadButton = within(workloadPanel).getByRole('button', { name: /创建工作负载/ });
  expect(createWorkloadButton).toBeInTheDocument();
  expect(createWorkloadButton).toHaveTextContent('');
  expect(within(leftColumn).getAllByText(/^(1 发布包|2 构建|3 工作负载)$/).map((item) => item.textContent)).toEqual(['1 发布包', '2 构建', '3 工作负载']);
  expect(screen.queryByText('近期发布记录')).not.toBeInTheDocument();
  expect(screen.queryByText('生产审批')).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /回滚/ })).not.toBeInTheDocument();
  const pipelineCard = (await within(pipelinePanel).findByText('主流水线')).closest('.resource-card') as HTMLElement;
  expect(pipelineCard).toHaveClass('workspace-item-card', 'workspace-item-card--pipeline');
  expect(within(pipelineCard).queryByText('关联 Workload')).not.toBeInTheDocument();
  expect(within(pipelineCard).queryByText('代码源')).not.toBeInTheDocument();
  expect(within(pipelineCard).queryByText('运行时环境')).not.toBeInTheDocument();
  expect(within(pipelineCard).getByRole('button', { name: /触发构建/ })).toHaveTextContent('');
  expect(within(pipelineCard).getByRole('button', { name: /编辑/ })).toHaveTextContent('');
  expect(within(pipelineCard).getByRole('button', { name: /删除/ })).toHaveTextContent('');
  expect(within(pipelineCard).queryByRole('button', { name: /历史/ })).not.toBeInTheDocument();
  const workloadCard = (await within(workloadPanel).findByText('订单接口')).closest('.resource-card') as HTMLElement;
  expect(workloadCard).toHaveClass('workspace-item-card', 'workspace-item-card--workload');
  expect(within(workloadCard).queryByText('无状态')).not.toBeInTheDocument();
  expect(within(workloadCard).queryByText('8080/TCP')).not.toBeInTheDocument();
  expect(within(workloadCard).getByRole('button', { name: '编辑' })).toHaveTextContent('');
  expect(within(workloadCard).getByRole('button', { name: '删除' })).toHaveTextContent('');
});

test('旧部署路由也展示统一交付工作台', async () => {
  renderPage('/apps/app_1/deploy');

  expect(await screen.findByText('1 发布包')).toBeInTheDocument();
  expect(await screen.findByLabelText('应用部署 DAG')).toBeInTheDocument();
  expect(screen.getByTestId('promotion-confirm-panel')).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /编辑应用/ })).not.toBeInTheDocument();
  expect(screen.queryByRole('heading', { name: '发布晋级' })).not.toBeInTheDocument();
  expect(screen.queryByText('拖拽 Freight 到目标 Stage，系统按交付流 DAG 校验上游依赖、审批和验证要求。')).not.toBeInTheDocument();
  expect(await screen.findByTestId('pipeline-panel')).toBeInTheDocument();
  expect(await screen.findByTestId('workload-panel')).toBeInTheDocument();
});

test('旧配置路由也展示统一交付工作台', async () => {
  renderPage('/apps/app_1/config');

  const workloadPanel = await screen.findByTestId('workload-panel');
  expect(screen.queryByRole('button', { name: /编辑应用/ })).not.toBeInTheDocument();
  expect(within(workloadPanel).getByText('3 工作负载')).toBeInTheDocument();
  expect(within(workloadPanel).getByRole('button', { name: /创建工作负载/ })).toBeInTheDocument();
  expect(await within(workloadPanel).findByText('订单接口')).toBeInTheDocument();
  expect(within(workloadPanel).queryByText('无状态')).not.toBeInTheDocument();
  expect(within(workloadPanel).queryByText('有状态')).not.toBeInTheDocument();
  expect(within(workloadPanel).queryByText('流水线产物')).not.toBeInTheDocument();
  expect(within(workloadPanel).queryByText('自定义镜像')).not.toBeInTheDocument();
  expect(within(workloadPanel).queryByText('8080/TCP')).not.toBeInTheDocument();
  expect(within(workloadPanel).queryByText('order.example.com')).not.toBeInTheDocument();
  expect(await screen.findByTestId('pipeline-panel')).toBeInTheDocument();
  expect(await screen.findByText('1 发布包')).toBeInTheDocument();
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
  renderPage('/apps/app_1');
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

  expect(await screen.findByText('订单搜索')).toBeInTheDocument();
});

test('工作负载编辑使用中文弹窗保存后更新卡片', async () => {
  renderPage('/apps/app_1');
  const workloadPanel = await screen.findByTestId('workload-panel');
  const apiCard = (await within(workloadPanel).findByText('订单接口')).closest('.resource-card') as HTMLElement;

  await userEvent.click(within(apiCard).getByRole('button', { name: '编辑' }));

  const dialog = await screen.findByRole('dialog', { name: '编辑工作负载' });
  expect(document.querySelector('.ant-drawer')).not.toBeInTheDocument();
  await userEvent.clear(within(dialog).getByLabelText('显示名称'));
  await userEvent.type(within(dialog).getByLabelText('显示名称'), '订单接口 v2');
  await userEvent.click(within(dialog).getByRole('button', { name: '保存' }));

  expect(await within(workloadPanel).findByText('订单接口 v2')).toBeInTheDocument();
});

test('工作负载列表支持确认后删除', async () => {
  renderPage('/apps/app_1');
  const workloadPanel = await screen.findByTestId('workload-panel');
  const workerCard = (await within(workloadPanel).findByText('订单任务')).closest('.resource-card') as HTMLElement;

  await userEvent.click(within(workerCard).getByRole('button', { name: '删除' }));
  await userEvent.click(await screen.findByRole('button', { name: '确认删除' }));

  await waitFor(() => {
    expect(within(workloadPanel).queryByText('订单任务')).not.toBeInTheDocument();
  });
  expect(within(workloadPanel).getByText(/^订单接口/)).toBeInTheDocument();
});

test('工作负载页面标题和按钮不使用英文用户文案', async () => {
  renderPage('/apps/app_1');
  const workloadPanel = await screen.findByTestId('workload-panel');
  await within(workloadPanel).findByText(/^订单接口/);
  const visibleControls = within(workloadPanel)
    .getAllByRole('button')
    .map((item) => item.textContent?.trim() || '');

  expect(within(workloadPanel).getByRole('button', { name: /创建工作负载/ })).toHaveTextContent('');
  expect(visibleControls).toEqual(expect.arrayContaining(['']));
  expect(within(workloadPanel).getAllByRole('button', { name: '编辑' })[0]).toHaveTextContent('');
  expect(within(workloadPanel).getAllByRole('button', { name: '删除' })[0]).toHaveTextContent('');
  expect(visibleControls).not.toEqual(expect.arrayContaining(['创建 Workload', 'Create Workload', 'Deploy Config']));
  expect(within(workloadPanel).queryByText('Workload')).not.toBeInTheDocument();
  expect(within(workloadPanel).queryByText('Create Workload')).not.toBeInTheDocument();
  expect(within(workloadPanel).queryByText('Deploy Config')).not.toBeInTheDocument();
});
