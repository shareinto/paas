import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { App } from '../app/App';
import { useSession } from '../app/store';

function renderFlow(path: string) {
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

test('本地用户登录后可以进入应用列表', async () => {
  window.localStorage.clear();
  useSession.setState({ token: '', userName: '平台用户' });
  renderFlow('/login');
  await userEvent.type(screen.getByPlaceholderText('请输入账号'), 'admin');
  await userEvent.type(screen.getByPlaceholderText('请输入密码'), 'password');
  await userEvent.click(screen.getByRole('button', { name: /^登\s*录$/ }));
  expect(await screen.findByText('订单服务', {}, { timeout: 10000 })).toBeInTheDocument();
});

test('OIDC mock 登录可以完成回调', async () => {
  window.localStorage.clear();
  useSession.setState({ token: '', userName: '平台用户' });
  renderFlow('/login');
  await userEvent.click(screen.getByRole('button', { name: '使用企业身份登录' }));
  await waitFor(() => expect(useSession.getState().token).toBe('mock-oidc-token'));
});

test('创建应用页面只维护应用基础信息', async () => {
  window.localStorage.setItem('paas_token', 'test');
  useSession.setState({ token: 'test', userName: '测试用户' });
  renderFlow('/apps/new');
  expect((await screen.findAllByText('创建应用')).length).toBeGreaterThan(0);
  expect(screen.getByText('基础信息')).toBeInTheDocument();
  expect(screen.getByText('后续配置')).toBeInTheDocument();
  expect(screen.queryByText('运行时环境')).not.toBeInTheDocument();
  expect(screen.queryByText('运行时预设')).not.toBeInTheDocument();
  expect(screen.queryByText('仓库目录')).not.toBeInTheDocument();
  expect(screen.queryByText('构建命令')).not.toBeInTheDocument();
  expect(screen.queryByText('产物拷贝命令')).not.toBeInTheDocument();
});

test('编辑应用页面不维护运行时环境', async () => {
  window.localStorage.setItem('paas_token', 'test');
  useSession.setState({ token: 'test', userName: '测试用户' });
  renderFlow('/apps/app_1/edit');
  expect(await screen.findByRole('heading', { name: '编辑应用' })).toBeInTheDocument();
  expect(screen.getByText('基础信息')).toBeInTheDocument();
  expect(screen.queryByText('运行时环境')).not.toBeInTheDocument();
  expect(screen.queryByText('运行时预设')).not.toBeInTheDocument();
});

test('应用详情中创建流水线可以选择多个运行时环境并添加代码源', async () => {
  window.localStorage.setItem('paas_token', 'test');
  useSession.setState({ token: 'test', userName: '测试用户' });
  renderFlow('/apps/app_1');
  await userEvent.click(await screen.findByRole('button', { name: /创建流水线/ }));
  const dialog = await screen.findByRole('dialog', { name: '创建构建流水线' });
  expect(await within(dialog).findByText('主代码源')).toBeInTheDocument();
  expect(within(dialog).getAllByText('运行时环境')).toHaveLength(1);
  expect(within(dialog).getByText('构建命令')).toBeInTheDocument();
  expect(within(dialog).getByText('产物拷贝命令')).toBeInTheDocument();

  await userEvent.click(within(dialog).getByRole('button', { name: /添加代码源/ }));

  await waitFor(() => expect(within(dialog).getByDisplayValue('source-2')).toBeInTheDocument());
  expect(within(dialog).getAllByText('构建命令')).toHaveLength(2);
  expect(within(dialog).getAllByText('产物拷贝命令')).toHaveLength(2);
  expect(within(dialog).getAllByText('运行时环境')).toHaveLength(1);

  await userEvent.click(within(dialog).getAllByRole('button', { name: /删\s*除/ })[1]);

  await waitFor(() => expect(within(dialog).queryByDisplayValue('source-2')).not.toBeInTheDocument());
  expect(within(dialog).getAllByText('构建命令')).toHaveLength(1);
  expect(within(dialog).getAllByText('产物拷贝命令')).toHaveLength(1);
  expect(within(dialog).getAllByText('运行时环境')).toHaveLength(1);
});

test('应用详情中创建流水线后列表展示新流水线', async () => {
  window.localStorage.setItem('paas_token', 'test');
  useSession.setState({ token: 'test', userName: '测试用户' });
  renderFlow('/apps/app_1');
  expect(await screen.findByText('主流水线')).toBeInTheDocument();

  await userEvent.click(await screen.findByRole('button', { name: /创建流水线/ }));
  const dialog = await screen.findByRole('dialog', { name: '创建构建流水线' });
  expect(await within(dialog).findByText('主代码源')).toBeInTheDocument();
  expect(within(dialog).getByLabelText('流水线标识')).toHaveValue('pipeline-2');
  expect(within(dialog).getAllByLabelText('显示名称')[0]).toHaveValue('流水线 2');
  await userEvent.click(within(dialog).getByRole('button', { name: /创\s*建/ }));

  const pipelinePanel = screen.getByText('构建流水线').closest('.ant-card') as HTMLElement;
  expect(await within(pipelinePanel).findByText('流水线 2')).toBeInTheDocument();
});

test('构建管理支持编辑构建环境和运行时环境', async () => {
  window.localStorage.setItem('paas_token', 'test');
  useSession.setState({ token: 'test', userName: '测试用户' });
  renderFlow('/jenkins-templates');
  expect(await screen.findByRole('heading', { name: '构建管理' })).toBeInTheDocument();
  expect(await screen.findByText('构建环境列表')).toBeInTheDocument();
  await waitFor(() => expect(screen.getAllByRole('button', { name: /编\s*辑/ }).length).toBeGreaterThan(0));

  await userEvent.click(screen.getAllByRole('button', { name: /编\s*辑/ })[0]);
  const buildDialog = await screen.findByRole('dialog', { name: '编辑构建环境' });
  expect(within(buildDialog).getByDisplayValue('java-springboot')).toBeDisabled();
  expect(within(buildDialog).queryByLabelText('显示名称')).not.toBeInTheDocument();
  expect(within(buildDialog).queryByLabelText('构建策略')).not.toBeInTheDocument();
  expect(within(buildDialog).queryByLabelText('Java 版本')).not.toBeInTheDocument();
  expect(within(buildDialog).queryByLabelText('Node 版本')).not.toBeInTheDocument();
  expect(within(buildDialog).queryByLabelText('Jenkins agent 标签')).not.toBeInTheDocument();
  expect(within(buildDialog).queryByLabelText('默认构建命令')).not.toBeInTheDocument();
  expect(within(buildDialog).queryByLabelText('默认产物路径')).not.toBeInTheDocument();
  const buildImage = within(buildDialog).getByDisplayValue('maven:3.9.9-eclipse-temurin-17');
  await userEvent.clear(buildImage);
  await userEvent.type(buildImage, 'maven:3.9.9-eclipse-temurin-21');
  await userEvent.click(within(buildDialog).getByRole('button', { name: /保\s*存/ }));
  expect(await screen.findByText(/maven:3\.9\.9-eclipse-temurin-21/)).toBeInTheDocument();

  await userEvent.click(screen.getByRole('tab', { name: '运行时环境' }));
  expect(await screen.findByText('运行时环境列表')).toBeInTheDocument();
  expect(await screen.findByText('java17')).toBeInTheDocument();
  const runtimeList = screen.getByText('运行时环境列表').closest('.ant-card') as HTMLElement;
  await waitFor(() => expect(within(runtimeList).getAllByRole('button', { name: /编\s*辑/ }).length).toBeGreaterThan(0));
  await userEvent.click(within(runtimeList).getAllByRole('button', { name: /编\s*辑/ })[0]);
  const runtimeDialog = (await screen.findByText('编辑运行时环境')).closest('.ant-modal') as HTMLElement;
  expect(within(runtimeDialog).getByDisplayValue('java17')).toBeDisabled();
  expect(within(runtimeDialog).queryByLabelText('显示名称')).not.toBeInTheDocument();
  expect(within(runtimeDialog).queryByLabelText('Dockerfile 路径')).not.toBeInTheDocument();
  expect(within(runtimeDialog).queryByLabelText('兼容构建策略')).not.toBeInTheDocument();
  expect(within(runtimeDialog).queryByLabelText('启动命令')).not.toBeInTheDocument();
  const runtimeImage = within(runtimeDialog).getByDisplayValue('registry.example/runtime/java17:1.0');
  await userEvent.clear(runtimeImage);
  await userEvent.type(runtimeImage, 'registry.example/runtime/java17:2.0');
  await userEvent.click(within(runtimeDialog).getByRole('button', { name: /保\s*存/ }));
  expect(await screen.findByText(/registry\.example\/runtime\/java17:2\.0/)).toBeInTheDocument();
}, 15000);

test('核心交付路径包含构建日志、版本、晋级审批、回滚和审计', async () => {
  window.localStorage.setItem('paas_token', 'test');
  useSession.setState({ token: 'test', userName: '测试用户' });
  renderFlow('/builds/build_128');
  expect(await screen.findByText('实时日志')).toBeInTheDocument();
  expect(await screen.findByText(/构建并推送镜像/)).toBeInTheDocument();

  cleanup();
  renderFlow('/freights');
  expect(await screen.findByText('变更包')).toBeInTheDocument();
  expect(await screen.findByText('v1.8.2')).toBeInTheDocument();

  cleanup();
  renderFlow('/promotions');
  expect(await screen.findByText('生产审批')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /回\s*滚/ })).toBeInTheDocument();

  cleanup();
  renderFlow('/audit');
  expect(await screen.findByRole('heading', { name: '审计日志' })).toBeInTheDocument();
  expect(await screen.findByText('审批通过生产发布')).toBeInTheDocument();
});

test('核心控制台页面可以通过 mock API 独立渲染', async () => {
  window.localStorage.setItem('paas_token', 'test');
  useSession.setState({ token: 'test', userName: '测试用户' });

  renderFlow('/projects');
  expect(await screen.findByRole('heading', { name: '项目' })).toBeInTheDocument();
  expect(await screen.findByText('订单平台')).toBeInTheDocument();

  cleanup();
  renderFlow('/apps/app_1');
  expect(await screen.findByRole('heading', { name: '订单服务' })).toBeInTheDocument();
  expect(await screen.findByText('应用标识')).toBeInTheDocument();
  expect(await screen.findByText('构建流水线')).toBeInTheDocument();

  cleanup();
  renderFlow('/builds');
  expect(await screen.findByRole('heading', { name: '构建' })).toBeInTheDocument();
  expect(await screen.findByText('build_128')).toBeInTheDocument();

  cleanup();
  renderFlow('/templates');
  expect(await screen.findByText('部署模板配置')).toBeInTheDocument();
  expect(await screen.findByText('校验通过')).toBeInTheDocument();
});

test('项目管理页面支持创建和删除项目', async () => {
  window.localStorage.setItem('paas_token', 'test');
  useSession.setState({ token: 'test', userName: '测试用户' });
  renderFlow('/projects');
  expect(await screen.findByRole('heading', { name: '项目' })).toBeInTheDocument();
  expect(await screen.findByText('订单平台')).toBeInTheDocument();

  await userEvent.click(screen.getByRole('button', { name: /创建项目/ }));
  const dialog = screen.getByRole('dialog', { name: '创建项目' });
  await userEvent.type(screen.getByPlaceholderText('order'), 'test-project');
  await userEvent.type(screen.getByPlaceholderText('订单平台'), '测试项目');
  await userEvent.type(screen.getByPlaceholderText('说明项目用途'), '页面创建删除验证');
  await userEvent.click(within(dialog).getByRole('button', { name: /创\s*建/ }));

  expect(await screen.findByText('测试项目')).toBeInTheDocument();
  const row = screen.getByText('测试项目').closest('tr');
  expect(row).not.toBeNull();
  await userEvent.click(within(row as HTMLTableRowElement).getByRole('button', { name: /删\s*除/ }));
  await userEvent.click(within(screen.getByRole('tooltip')).getByRole('button', { name: /删\s*除/ }));
  await waitFor(() => expect(screen.queryByText('测试项目')).not.toBeInTheDocument());
});

test('租户管理页面支持创建、搜索和编辑租户', async () => {
  window.localStorage.setItem('paas_token', 'test');
  useSession.setState({ token: 'test', userName: '测试用户' });
  renderFlow('/tenants');
  expect(await screen.findByRole('heading', { name: '租户管理' })).toBeInTheDocument();
  expect(await screen.findByText('研发中心')).toBeInTheDocument();

  await userEvent.click(screen.getByRole('button', { name: /创建租户/ }));
  const createDialog = screen.getByRole('dialog', { name: '创建租户' });
  await userEvent.type(within(createDialog).getByPlaceholderText('rnd'), 'ops');
  await userEvent.type(within(createDialog).getByPlaceholderText('研发中心'), '运维中心');
  await userEvent.type(within(createDialog).getByPlaceholderText('说明租户用途'), '平台运维租户');
  await userEvent.click(within(createDialog).getByRole('button', { name: /创\s*建/ }));

  expect(await screen.findByText('运维中心')).toBeInTheDocument();
  await userEvent.type(screen.getByPlaceholderText('搜索租户名称或标识'), 'ops');
  expect(screen.getByText('运维中心')).toBeInTheDocument();

  const row = screen.getByText('运维中心').closest('tr');
  expect(row).not.toBeNull();
  await userEvent.click(within(row as HTMLTableRowElement).getByRole('button', { name: /编\s*辑/ }));
  const editDialog = await screen.findByRole('dialog', { name: '编辑租户' });
  expect(within(editDialog).getByDisplayValue('ops')).toBeDisabled();
  const displayName = within(editDialog).getByDisplayValue('运维中心');
  await userEvent.clear(displayName);
  await userEvent.type(displayName, '平台运维');
  const description = within(editDialog).getByDisplayValue('平台运维租户');
  await userEvent.clear(description);
  await userEvent.type(description, '统一运维租户');
  await userEvent.click(within(editDialog).getByRole('button', { name: /保\s*存/ }));

  expect(await screen.findByText('平台运维')).toBeInTheDocument();
  expect(await screen.findByText('统一运维租户')).toBeInTheDocument();
});

test('源码仓库管理页面支持创建、筛选和删除仓库', async () => {
  window.localStorage.setItem('paas_token', 'test');
  useSession.setState({ token: 'test', userName: '测试用户' });
  renderFlow('/source-repositories');
  expect(await screen.findByRole('heading', { name: '源码仓库' })).toBeInTheDocument();
  expect((await screen.findAllByText('订单平台')).length).toBeGreaterThan(0);

  await userEvent.click(screen.getByRole('button', { name: /创建仓库/ }));
  const dialog = screen.getByRole('dialog', { name: '创建平台托管源码仓库' });
  await userEvent.type(screen.getByPlaceholderText('order-api'), 'test-repo');
  await userEvent.type(screen.getByPlaceholderText('订单服务仓库'), '测试源码仓库');
  await userEvent.click(within(dialog).getByRole('button', { name: /创\s*建/ }));

  expect(await screen.findByText('测试源码仓库')).toBeInTheDocument();
  await userEvent.type(screen.getByPlaceholderText('搜索仓库名称'), 'test-repo');
  expect(screen.getByText('测试源码仓库')).toBeInTheDocument();

  const row = screen.getByText('测试源码仓库').closest('tr');
  expect(row).not.toBeNull();
  await userEvent.click(within(row as HTMLTableRowElement).getByRole('button', { name: /删\s*除/ }));
  await userEvent.click(within(screen.getByRole('tooltip')).getByRole('button', { name: /删\s*除/ }));
  await waitFor(() => expect(screen.queryByText('测试源码仓库')).not.toBeInTheDocument());
});
