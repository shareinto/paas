import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, test, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';

function renderApp(path: string, App: () => JSX.Element) {
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
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
  vi.resetModules();
  window.localStorage.clear();
});

test('真实 API 创建流水线后即使列表接口暂未返回也立即显示新流水线', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  window.localStorage.setItem('paas_token', 'test');
  const { useSession } = await import('../app/store');
  const { App } = await import('../app/App');
  useSession.setState({ token: 'test', userName: '测试用户' });

  let pipelineListCalls = 0;
  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    const method = init?.method || 'GET';
    if (method === 'GET' && url.endsWith('/api/applications/app_1')) {
      return jsonResponse({ id: 'app_1', name: 'order-api', display_name: '订单服务', project_id: 'project_1', status: 'active' });
    }
    if (method === 'GET' && url.endsWith('/api/applications/app_1/workloads')) {
      return jsonResponse({ items: [{ id: 'workload_api', application_id: 'app_1', name: 'order-api', display_name: '订单接口', workload_type: 'deployment', status: 'enabled' }] });
    }
    if (method === 'GET' && url.endsWith('/api/projects?page=1&page_size=50')) {
      return jsonResponse({ items: [{ id: 'project_1', display_name: '订单平台' }], total: 1, page: 1, page_size: 50 });
    }
    if (method === 'GET' && url.endsWith('/api/projects/project_1/source-repositories?page=1&page_size=100')) {
      return jsonResponse({ items: [{ id: 'repo_1', name: 'order-api', display_name: '订单服务仓库', default_branch: 'main', status: 'ready' }], total: 1, page: 1, page_size: 100 });
    }
    if (method === 'GET' && url.endsWith('/api/build-environments?page=1&page_size=100')) {
      return jsonResponse({ items: [{ id: 'build_env_java', name: 'java-springboot', status: 'enabled', is_default: true }], total: 1, page: 1, page_size: 100 });
    }
    if (method === 'GET' && url.endsWith('/api/runtime-environments?page=1&page_size=100')) {
      return jsonResponse({ items: [{ id: 'runtime_env_java17', name: 'java17', runtime_base_image: 'registry.example/runtime/java17:1.0', artifact_deploy_path: '/app/', status: 'enabled', is_default: true }], total: 1, page: 1, page_size: 100 });
    }
    if (method === 'GET' && url.endsWith('/api/apps/app_1/build-pipelines?page=1&page_size=50')) {
      pipelineListCalls += 1;
      return jsonResponse({ items: [], total: 0, page: 1, page_size: 50 });
    }
    if (method === 'POST' && url.endsWith('/api/apps/app_1/build-pipelines')) {
      const body = JSON.parse(String(init?.body || '{}'));
      expect(body.workload_id).toBe('workload_api');
      return jsonResponse({ id: 'build_pipeline_2', application_id: 'app_1', workload_id: 'workload_api', name: 'main', display_name: '主流水线', status: 'active', updated_at: '2026-06-09T10:00:00Z' }, 201);
    }
    return jsonResponse({ error: { code: 'not_found', message: `未处理请求 ${method} ${url}` } }, 404);
  });
  vi.stubGlobal('fetch', fetchMock);

  renderApp('/apps/app_1', App);

  await userEvent.click(await screen.findByRole('button', { name: /创建流水线/ }));
  const dialog = await screen.findByRole('dialog', { name: '创建构建流水线' });
  expect(await within(dialog).findByText('主代码源')).toBeInTheDocument();
  expect(within(dialog).getByText('订单接口 (order-api)')).toBeInTheDocument();

  await userEvent.click(within(dialog).getByRole('button', { name: /创\s*建/ }));

  await userEvent.click(await screen.findByRole('tab', { name: '构建' }));
  const pipelinePanel = await screen.findByTestId('pipeline-panel');
  expect(await within(pipelinePanel).findByText('主流水线')).toBeInTheDocument();
  expect(within(pipelinePanel).getByText('主代码源')).toBeInTheDocument();
  expect(pipelineListCalls).toBeGreaterThanOrEqual(2);
}, 10000);

test('真实 API 构建历史按开始时间倒序编号', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  window.localStorage.setItem('paas_token', 'test');
  const { useSession } = await import('../app/store');
  const { App } = await import('../app/App');
  useSession.setState({ token: 'test', userName: '测试用户' });

  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    const method = init?.method || 'GET';
    if (method === 'GET' && url.endsWith('/api/applications/app_1')) {
      return jsonResponse({ id: 'app_1', name: 'order-api', display_name: '订单服务', project_id: 'project_1', status: 'active' });
    }
    if (method === 'GET' && url.endsWith('/api/applications/app_1/workloads')) {
      return jsonResponse({ items: [] });
    }
    if (method === 'GET' && url.endsWith('/api/apps/app_1/build-pipelines?page=1&page_size=50')) {
      return jsonResponse({
        items: [{ id: 'pipeline_1', application_id: 'app_1', name: 'main', display_name: '主流水线', status: 'active', updated_at: '2026-06-09T10:00:00Z' }],
        total: 1,
        page: 1,
        page_size: 50
      });
    }
    if (method === 'GET' && url.endsWith('/api/build-pipelines/pipeline_1/sources')) {
      return jsonResponse({ items: [] });
    }
    if (method === 'GET' && url.endsWith('/api/apps/app_1/builds?page=1&page_size=50')) {
      return jsonResponse({
        items: [
          { id: 'build_old', pipeline_id: 'pipeline_1', status: 'failed', git_ref: 'main', startedAt: '2026-05-29 16:40' },
          { id: 'build_new', pipeline_id: 'pipeline_1', status: 'succeeded', git_ref: 'main', startedAt: '2026-05-30 10:01' }
        ],
        total: 2,
        page: 1,
        page_size: 50
      });
    }
    return jsonResponse({ error: { code: 'not_found', message: `未处理请求 ${method} ${url}` } }, 404);
  });
  vi.stubGlobal('fetch', fetchMock);

  renderApp('/apps/app_1', App);

  const pipelinePanel = await screen.findByTestId('pipeline-panel');
  const pipelineCard = (await within(pipelinePanel).findByText('主流水线')).closest('.resource-card') as HTMLElement;
  await userEvent.click(within(pipelineCard).getByRole('button', { name: /触发构建/ }));

  const dialog = await screen.findByRole('dialog', { name: /构建历史/ });
  const firstBuild = (await within(dialog).findByText('构建 2')).closest('button') as HTMLElement;
  const secondBuild = within(dialog).getByText('构建 1').closest('button') as HTMLElement;
  expect(within(firstBuild).getByText('2026-05-30 10:01')).toBeInTheDocument();
  expect(within(secondBuild).getByText('2026-05-29 16:40')).toBeInTheDocument();
}, 10000);

test('真实 API 在构建弹窗触发构建后选中新构建并展示日志', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  window.localStorage.setItem('paas_token', 'test');
  const { useSession } = await import('../app/store');
  const { App } = await import('../app/App');
  useSession.setState({ token: 'test', userName: '测试用户' });

  let builds = [
    { id: 'build_old', pipeline_id: 'pipeline_1', status: 'succeeded', git_ref: 'main', started_at: '2026-05-30T10:01:00Z' }
  ];
  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    const method = init?.method || 'GET';
    if (method === 'GET' && url.endsWith('/api/applications/app_1')) {
      return jsonResponse({ id: 'app_1', name: 'order-api', display_name: '订单服务', project_id: 'project_1', status: 'active' });
    }
    if (method === 'GET' && url.endsWith('/api/applications/app_1/workloads')) return jsonResponse({ items: [] });
    if (method === 'GET' && url.endsWith('/api/apps/app_1/build-pipelines?page=1&page_size=50')) {
      return jsonResponse({ items: [{ id: 'pipeline_1', application_id: 'app_1', name: 'main', display_name: '主流水线', status: 'active' }], total: 1, page: 1, page_size: 50 });
    }
    if (method === 'GET' && url.endsWith('/api/build-pipelines/pipeline_1/sources')) {
      return jsonResponse({ items: [{ id: 'source_1', pipeline_id: 'pipeline_1', key: 'main', display_name: '主代码源', build_spec: { default_ref: 'main' } }] });
    }
    if (method === 'GET' && url.endsWith('/api/apps/app_1/builds?page=1&page_size=50')) {
      return jsonResponse({ items: builds, total: builds.length, page: 1, page_size: 50 });
    }
    if (method === 'POST' && url.endsWith('/api/build-pipelines/pipeline_1/builds')) {
      const body = JSON.parse(String(init?.body || '{}'));
      expect(body.git_ref).toBe('main');
      const run = { id: 'build_new', pipeline_id: 'pipeline_1', status: 'queued', git_ref: 'main', started_at: '2026-05-31T10:01:00Z' };
      builds = [run, ...builds];
      return jsonResponse(run, 201);
    }
    if (method === 'GET' && url.endsWith('/api/builds/build_new/logs/stream')) {
      return new Response('event: status\ndata: queued\n\nevent: log\ndata: 新构建日志\n\n', { status: 200 });
    }
    return jsonResponse({ error: { code: 'not_found', message: `未处理请求 ${method} ${url}` } }, 404);
  });
  vi.stubGlobal('fetch', fetchMock);

  renderApp('/apps/app_1', App);

  const pipelinePanel = await screen.findByTestId('pipeline-panel');
  const pipelineCard = (await within(pipelinePanel).findByText('主流水线')).closest('.resource-card') as HTMLElement;
  await userEvent.click(within(pipelineCard).getByRole('button', { name: /触发构建/ }));
  const dialog = await screen.findByRole('dialog', { name: /构建历史/ });
  await userEvent.click(await within(dialog).findByRole('button', { name: /^触发构建$/ }));

  expect(await within(dialog).findByText('构建 2')).toBeInTheDocument();
  expect(await within(dialog).findByText('新构建日志')).toBeInTheDocument();
}, 10000);

test('真实 API 有未完成构建时禁用触发并可取消当前构建', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  window.localStorage.setItem('paas_token', 'test');
  const { useSession } = await import('../app/store');
  const { App } = await import('../app/App');
  useSession.setState({ token: 'test', userName: '测试用户' });

  let build = { id: 'build_running', pipeline_id: 'pipeline_1', status: 'running', git_ref: 'main', started_at: '2026-05-31T10:01:00Z' };
  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    const method = init?.method || 'GET';
    if (method === 'GET' && url.endsWith('/api/applications/app_1')) {
      return jsonResponse({ id: 'app_1', name: 'order-api', display_name: '订单服务', project_id: 'project_1', status: 'active' });
    }
    if (method === 'GET' && url.endsWith('/api/applications/app_1/workloads')) return jsonResponse({ items: [] });
    if (method === 'GET' && url.endsWith('/api/apps/app_1/build-pipelines?page=1&page_size=50')) {
      return jsonResponse({ items: [{ id: 'pipeline_1', application_id: 'app_1', name: 'main', display_name: '主流水线', status: 'active' }], total: 1, page: 1, page_size: 50 });
    }
    if (method === 'GET' && url.endsWith('/api/build-pipelines/pipeline_1/sources')) return jsonResponse({ items: [] });
    if (method === 'GET' && url.endsWith('/api/apps/app_1/builds?page=1&page_size=50')) {
      return jsonResponse({ items: [build], total: 1, page: 1, page_size: 50 });
    }
    if (method === 'GET' && url.endsWith('/api/builds/build_running/logs/stream')) {
      return new Response('event: status\ndata: running\n\nevent: log\ndata: running log\n\n', { status: 200 });
    }
    if (method === 'POST' && url.endsWith('/api/builds/build_running/cancel')) {
      const body = JSON.parse(String(init?.body || '{}'));
      expect(body.actor.id).toBe('usr_admin');
      build = { ...build, status: 'aborted' };
      return jsonResponse(build);
    }
    return jsonResponse({ error: { code: 'not_found', message: `未处理请求 ${method} ${url}` } }, 404);
  });
  vi.stubGlobal('fetch', fetchMock);

  renderApp('/apps/app_1', App);

  const pipelinePanel = await screen.findByTestId('pipeline-panel');
  const pipelineCard = (await within(pipelinePanel).findByText('主流水线')).closest('.resource-card') as HTMLElement;
  await userEvent.click(within(pipelineCard).getByRole('button', { name: /触发构建/ }));
  const dialog = await screen.findByRole('dialog', { name: /构建历史/ });
  expect(await within(dialog).findByRole('button', { name: /^触发构建$/ })).toBeDisabled();
  await userEvent.click(await within(dialog).findByText('构建 1'));
  await userEvent.click(await within(dialog).findByRole('button', { name: /取消构建/ }));

  expect((await within(dialog).findAllByText('已取消')).length).toBeGreaterThan(0);
  expect(fetchMock).toHaveBeenCalledWith('https://paas.example/api/builds/build_running/cancel', expect.objectContaining({ method: 'POST' }));
}, 10000);

test('真实 API 构建弹窗随日志流状态自动更新构建状态', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  window.localStorage.setItem('paas_token', 'test');
  const { useSession } = await import('../app/store');
  const { App } = await import('../app/App');
  useSession.setState({ token: 'test', userName: '测试用户' });

  const build = { id: 'build_running', pipeline_id: 'pipeline_1', status: 'running', git_ref: 'main', started_at: '2026-05-31T10:01:00Z' };
  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    const method = init?.method || 'GET';
    if (method === 'GET' && url.endsWith('/api/applications/app_1')) {
      return jsonResponse({ id: 'app_1', name: 'order-api', display_name: '订单服务', project_id: 'project_1', status: 'active' });
    }
    if (method === 'GET' && url.endsWith('/api/applications/app_1/workloads')) return jsonResponse({ items: [] });
    if (method === 'GET' && url.endsWith('/api/apps/app_1/build-pipelines?page=1&page_size=50')) {
      return jsonResponse({ items: [{ id: 'pipeline_1', application_id: 'app_1', name: 'main', display_name: '主流水线', status: 'active' }], total: 1, page: 1, page_size: 50 });
    }
    if (method === 'GET' && url.endsWith('/api/build-pipelines/pipeline_1/sources')) return jsonResponse({ items: [] });
    if (method === 'GET' && url.endsWith('/api/apps/app_1/builds?page=1&page_size=50')) {
      return jsonResponse({ items: [build], total: 1, page: 1, page_size: 50 });
    }
    if (method === 'GET' && url.endsWith('/api/builds/build_running/logs/stream')) {
      return new Response('event: status\ndata: running\n\nevent: log\ndata: running log\n\nevent: status\ndata: succeeded\n\n', { status: 200 });
    }
    return jsonResponse({ error: { code: 'not_found', message: `未处理请求 ${method} ${url}` } }, 404);
  });
  vi.stubGlobal('fetch', fetchMock);

  renderApp('/apps/app_1', App);

  const pipelinePanel = await screen.findByTestId('pipeline-panel');
  const pipelineCard = (await within(pipelinePanel).findByText('主流水线')).closest('.resource-card') as HTMLElement;
  await userEvent.click(within(pipelineCard).getByRole('button', { name: /触发构建/ }));
  const dialog = await screen.findByRole('dialog', { name: /构建历史/ });
  const triggerButton = await within(dialog).findByRole('button', { name: /^触发构建$/ });

  expect(await within(dialog).findByText('running log')).toBeInTheDocument();
  await waitFor(() => expect(triggerButton).toBeEnabled());
  expect(within(dialog).queryByRole('button', { name: /取消构建/ })).not.toBeInTheDocument();
  expect(within(dialog).getAllByText('成功').length).toBeGreaterThan(0);
}, 10000);

test('真实 API 创建 Workload 后使用服务端镜像来源展示', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  window.localStorage.setItem('paas_token', 'test');
  const { useSession } = await import('../app/store');
  const { App } = await import('../app/App');
  useSession.setState({ token: 'test', userName: '测试用户' });

  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    const method = init?.method || 'GET';
    if (method === 'GET' && url.endsWith('/api/applications/app_1')) {
      return jsonResponse({ id: 'app_1', name: 'order-api', display_name: '订单服务', project_id: 'project_1', status: 'active' });
    }
    if (method === 'GET' && url.endsWith('/api/applications/app_1/workloads')) {
      return jsonResponse({ items: [] });
    }
    if (method === 'POST' && url.endsWith('/api/applications/app_1/workloads')) {
      return jsonResponse({
        id: 'workload_worker',
        application_id: 'app_1',
        name: 'order-worker',
        display_name: '订单任务',
        workload_type: 'statefulset',
        image_source_mode: 'pipeline_artifact',
        image_source_name: '主流水线',
        status: 'enabled'
      }, 201);
    }
    return jsonResponse({ error: { code: 'not_found', message: `未处理请求 ${method} ${url}` } }, 404);
  });
  vi.stubGlobal('fetch', fetchMock);

  renderApp('/apps/app_1', App);

  expect(await screen.findByRole('tab', { name: '构建' })).toHaveAttribute('aria-selected', 'true');
  await userEvent.click(await screen.findByRole('button', { name: /创建 Workload/ }));
  const dialog = await screen.findByRole('dialog', { name: '创建 Workload' });
  await userEvent.type(within(dialog).getByLabelText('Workload 标识'), 'order-worker');
  await userEvent.type(within(dialog).getByLabelText('显示名称'), '订单任务');
  await userEvent.click(within(dialog).getByText('StatefulSet'));
  await userEvent.click(within(dialog).getByRole('button', { name: '下一步' }));
  await userEvent.click(within(dialog).getByLabelText('镜像来源偏好'));
  await userEvent.click(await screen.findByTitle('发布时选择自定义镜像'));
  for (let i = 0; i < 4; i += 1) {
    await userEvent.click(within(dialog).getByRole('button', { name: '下一步' }));
  }
  await userEvent.click(within(dialog).getByRole('button', { name: '创建' }));

  const card = (await screen.findByText('order-worker')).closest('.resource-card') as HTMLElement;
  expect(within(card).getByText('流水线产物')).toBeInTheDocument();
  expect(within(card).getByText('暂无端口')).toBeInTheDocument();
  expect(within(card).getByText('集群内访问')).toBeInTheDocument();
  expect(within(card).queryByText('自定义镜像')).not.toBeInTheDocument();
  expect(within(card).queryByText('registry.example.com/order/worker:20260611')).not.toBeInTheDocument();
}, 10000);

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}
