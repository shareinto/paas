import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { readFileSync } from 'node:fs';
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

test('应用详情交付工作台桌面布局使用固定操作列和弹性画布', () => {
  const styles = readFileSync('src/styles.css', 'utf8');

  expect(styles).toMatch(/\.application-delivery-workspace \{[^}]*grid-template-columns: minmax\(320px, 440px\) minmax\(0, 1fr\);/);
  expect(styles).toMatch(/@media \(max-width: 1180px\) \{[\s\S]*\.promotion-workspace, \.delivery-dag-editor, \.application-delivery-workspace \{ grid-template-columns: 1fr; \}/);
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
  expect(await within(dialog).findByText(/\[INFO\] 检出已登记源码仓库/)).toBeInTheDocument();
});

test('工作负载创建弹层使用单页横向表单并最终创建', async () => {
  renderPage('/apps/app_1');
  await userEvent.click(await screen.findByRole('button', { name: /创建工作负载/ }));

  const dialog = await screen.findByRole('dialog', { name: '创建工作负载' });
  expect(within(dialog).queryByText('校验清单')).not.toBeInTheDocument();
  ['基本信息', '构建来源', '运行配置', '访问配置', '环境变量', 'Nginx Sidecar', '高级配置'].forEach((name) => {
    expect(within(dialog).getByText(name)).toBeInTheDocument();
  });
  expect(within(dialog).queryByRole('button', { name: '下一步' })).not.toBeInTheDocument();
  expect(within(dialog).queryByRole('button', { name: '上一步' })).not.toBeInTheDocument();
  expect(within(dialog).queryByText('预览校验')).not.toBeInTheDocument();
  const form = within(dialog).getByTestId('workload-large-form');
  expect(form).toHaveClass('ant-form-horizontal');
  expect(form).not.toHaveClass('ant-form-vertical');
  expect(within(dialog).getByText('工作负载标识')).toHaveTextContent('工作负载标识 *');
  await userEvent.type(within(dialog).getByLabelText('工作负载标识'), 'order-search');
  await userEvent.type(within(dialog).getByLabelText('显示名称'), '订单搜索');
  await userEvent.click(within(dialog).getByText('有状态'));
  expect(within(dialog).getByText('有状态').closest('.ant-segmented-item')).toHaveClass('ant-segmented-item-selected');
  await userEvent.click(within(dialog).getByText('自定义镜像'));
  expect(within(dialog).getByText('自定义镜像').closest('.ant-segmented-item')).toHaveClass('ant-segmented-item-selected');
  expect(within(dialog).getByText('自定义镜像会在创建 Freight 时选择具体镜像，不要求关联流水线。')).toBeInTheDocument();
  expect(within(dialog).getByLabelText('副本数')).toBeInTheDocument();
  expect(within(dialog).getByLabelText('访问域名')).toBeInTheDocument();
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
  expect(within(dialog).getByLabelText('Values 覆盖 JSON')).toBeInTheDocument();
  await userEvent.click(within(dialog).getByRole('button', { name: '创建' }));

  expect(await screen.findByText('订单搜索')).toBeInTheDocument();
});

test('工作负载创建时可配置 Nginx Sidecar 并保存到默认配置', async () => {
  const api = await import('../api');
  renderPage('/apps/app_1');
  await userEvent.click(await screen.findByRole('button', { name: /创建工作负载/ }));

  const dialog = await screen.findByRole('dialog', { name: '创建工作负载' });
  await userEvent.type(within(dialog).getByLabelText('工作负载标识'), 'order-nginx');
  await userEvent.type(within(dialog).getByLabelText('显示名称'), '订单网关');
  await userEvent.click(within(dialog).getByText('自定义镜像'));
  await userEvent.click(within(dialog).getByRole('switch', { name: '启用 Nginx Sidecar' }));

  expect(await within(dialog).findByLabelText('Nginx 镜像')).toHaveValue('nginx:1.25-alpine');
  expect(within(dialog).getByLabelText('监听端口')).toHaveValue('80');
  await userEvent.clear(within(dialog).getByLabelText('监听端口'));
  await userEvent.type(within(dialog).getByLabelText('监听端口'), '8081');
  await userEvent.clear(within(dialog).getByLabelText('nginx.conf'));
  fireEvent.change(within(dialog).getByLabelText('nginx.conf'), { target: { value: 'events {}\nhttp { include /etc/nginx/conf.d/*.conf; }' } });
  expect(within(dialog).getByLabelText('conf.d 文件名 1')).toHaveValue('default.conf');
  await userEvent.clear(within(dialog).getByLabelText('conf.d 配置内容 1'));
  fireEvent.change(within(dialog).getByLabelText('conf.d 配置内容 1'), { target: { value: 'server { listen 8081; }' } });
  await userEvent.click(within(dialog).getByRole('button', { name: '添加 conf.d 配置' }));
  await userEvent.clear(within(dialog).getByLabelText('conf.d 文件名 2'));
  await userEvent.type(within(dialog).getByLabelText('conf.d 文件名 2'), 'api.conf');
  await userEvent.clear(within(dialog).getByLabelText('conf.d 配置内容 2'));
  fireEvent.change(within(dialog).getByLabelText('conf.d 配置内容 2'), { target: { value: 'server { listen 8081; location /api { proxy_pass http://127.0.0.1:8080; } }' } });
  await userEvent.click(within(dialog).getByRole('button', { name: '创建' }));

  const workload = (await api.listWorkloads('app_1')).find((item) => item.name === 'order-nginx');
  expect(workload).toBeTruthy();
  await expect(api.getWorkloadDefaultConfig('app_1', workload!.id)).resolves.toMatchObject({
    valuesOverride: {
      nginxSidecar: {
        enabled: true,
        image: 'nginx:1.25-alpine',
        port: 8081,
        routeServiceToSidecar: true,
        nginxConf: 'events {}\nhttp { include /etc/nginx/conf.d/*.conf; }',
        confD: [
          { fileName: 'default.conf', content: 'server { listen 8081; }' },
          { fileName: 'api.conf', content: 'server { listen 8081; location /api { proxy_pass http://127.0.0.1:8080; } }' }
        ]
      }
    }
  });
});

test('工作负载变化同步到 Freight 抽屉和 Stage 配置弹窗', async () => {
  renderPage('/apps/app_1');
  await userEvent.click(await screen.findByRole('button', { name: /创建工作负载/ }));

  const createDialog = await screen.findByRole('dialog', { name: '创建工作负载' });
  await userEvent.type(within(createDialog).getByLabelText('工作负载标识'), 'order-cache');
  await userEvent.type(within(createDialog).getByLabelText('显示名称'), '订单联动');
  await userEvent.click(within(createDialog).getByText('自定义镜像'));
  await userEvent.click(within(createDialog).getByRole('button', { name: '创建' }));

  const workloadPanel = await screen.findByTestId('workload-panel');
  const createdCard = (await within(workloadPanel).findByText('订单联动')).closest('.resource-card') as HTMLElement;
  const freightPanel = screen.getByText('1 发布包').closest('.workspace-section-card') as HTMLElement;

  const refreshedCreateFreightButton = within(freightPanel).getByRole('button', { name: '创建 Freight' });
  await waitFor(() => expect(refreshedCreateFreightButton).not.toBeDisabled());
  await userEvent.click(refreshedCreateFreightButton);
  const freightDrawer = await screen.findByRole('dialog', { name: '创建 Freight' });
  expect(within(freightDrawer).getByText('订单联动')).toBeInTheDocument();
  await userEvent.click(within(freightDrawer).getByRole('button', { name: '取消' }));
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

  await userEvent.click((await screen.findAllByRole('button', { name: '编辑配置' }))[0]);
  const configDialog = await screen.findByRole('dialog', { name: '编辑 Stage 配置' });
  expect(within(configDialog).getByRole('option', { name: '订单联动' })).toBeInTheDocument();
  await userEvent.click(within(configDialog).getByRole('button', { name: '取消' }));
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

  await userEvent.click(within(createdCard).getByRole('button', { name: '删除' }));
  await userEvent.click(await screen.findByRole('button', { name: '确认删除' }));
  await waitFor(() => expect(within(workloadPanel).queryByText('订单联动')).not.toBeInTheDocument());

  const freightPanelAfterDelete = screen.getByText('1 发布包').closest('.workspace-section-card') as HTMLElement;
  const createFreightAfterDeleteButton = within(freightPanelAfterDelete).getByRole('button', { name: '创建 Freight' });
  await waitFor(() => expect(createFreightAfterDeleteButton).not.toBeDisabled());
  await userEvent.click(createFreightAfterDeleteButton);
  const refreshedFreightDrawer = await screen.findByRole('dialog');
  expect(within(refreshedFreightDrawer).queryByText('订单联动')).not.toBeInTheDocument();
  await userEvent.click(within(refreshedFreightDrawer).getByRole('button', { name: '取消' }));
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

  await userEvent.click((await screen.findAllByRole('button', { name: '编辑配置' }))[0]);
  const refreshedConfigDialog = await screen.findByRole('dialog');
  expect(within(refreshedConfigDialog).queryByRole('option', { name: '订单联动' })).not.toBeInTheDocument();
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

test('已被工作负载关联的流水线不允许删除', async () => {
  const api = await import('../api');
  await api.createWorkload('app_1', {
    name: 'delete-lock',
    displayName: '删除保护',
    workloadType: 'deployment',
    imageSourceMode: 'pipeline_artifact',
    pipelineId: 'pipeline_1'
  });
  renderPage('/apps/app_1');

  const pipelinePanel = await screen.findByTestId('pipeline-panel');
  const pipelineCard = (await within(pipelinePanel).findByText('主流水线')).closest('.resource-card') as HTMLElement;
  const deleteButton = within(pipelineCard).getByRole('button', { name: /删除/ });

  await waitFor(() => expect(deleteButton).toBeDisabled());
  await userEvent.hover(deleteButton);
  expect(await screen.findByText('已有工作负载关联，不能删除')).toBeInTheDocument();
});
