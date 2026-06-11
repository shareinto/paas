import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, within } from '@testing-library/react';
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
      return jsonResponse({ id: 'build_pipeline_2', application_id: 'app_1', name: 'main', display_name: '主流水线', status: 'active', updated_at: '2026-06-09T10:00:00Z' }, 201);
    }
    return jsonResponse({ error: { code: 'not_found', message: `未处理请求 ${method} ${url}` } }, 404);
  });
  vi.stubGlobal('fetch', fetchMock);

  renderApp('/apps/app_1', App);

  await userEvent.click(await screen.findByRole('button', { name: /创建流水线/ }));
  const dialog = await screen.findByRole('dialog', { name: '创建构建流水线' });
  expect(await within(dialog).findByText('主代码源')).toBeInTheDocument();

  await userEvent.click(within(dialog).getByRole('button', { name: /创\s*建/ }));

  await userEvent.click(await screen.findByRole('tab', { name: '构建' }));
  const pipelinePanel = screen.getByText('构建流水线').closest('.ant-card') as HTMLElement;
  expect(await within(pipelinePanel).findByText('主流水线')).toBeInTheDocument();
  expect(within(pipelinePanel).getByText('主代码源')).toBeInTheDocument();
  expect(pipelineListCalls).toBeGreaterThanOrEqual(2);
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

  await userEvent.click(await screen.findByRole('button', { name: /创建 Workload/ }));
  const drawer = await screen.findByRole('dialog', { name: '创建 Workload' });
  await userEvent.type(within(drawer).getByLabelText('Workload 标识'), 'order-worker');
  await userEvent.type(within(drawer).getByLabelText('显示名称'), '订单任务');
  await userEvent.click(within(drawer).getByText('StatefulSet'));
  await userEvent.click(within(drawer).getByLabelText('镜像来源偏好'));
  await userEvent.click(await screen.findByTitle('发布时选择自定义镜像'));
  await userEvent.type(within(drawer).getByLabelText('自定义镜像地址'), 'registry.example.com/order/worker:20260611');
  await userEvent.click(within(drawer).getByRole('button', { name: /创\s*建/ }));

  const row = (await screen.findByText('order-worker')).closest('tr') as HTMLElement;
  expect(within(row).getByText('流水线产物')).toBeInTheDocument();
  expect(within(row).getByText('暂无 Release')).toBeInTheDocument();
  expect(within(row).getByText('暂无环境状态')).toBeInTheDocument();
  expect(within(row).queryByText('自定义镜像')).not.toBeInTheDocument();
  expect(within(row).queryByText('registry.example.com/order/worker:20260611')).not.toBeInTheDocument();
}, 10000);

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}
