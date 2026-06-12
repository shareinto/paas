import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, test, vi } from 'vitest';
import { MemoryRouter } from 'react-router-dom';
import { DeliveryFlowTemplatePage } from './DeliveryFlowTemplatePage';

function renderPage() {
  return render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <DeliveryFlowTemplatePage />
        </MemoryRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );
}

afterEach(() => {
  cleanup();
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
  vi.resetModules();
});

test('租户交付流模板展示 Stage 卡片并支持本地添加', async () => {
  renderPage();

  expect(await screen.findByRole('heading', { name: '租户交付流模板' })).toBeInTheDocument();
  expect(screen.getByLabelText('交付流 DAG 画布')).toBeInTheDocument();
  const devCard = await screen.findByLabelText('dev Stage 模板');
  expect(within(devCard).getByText('开发')).toBeInTheDocument();
  expect(devCard.querySelector('.dag-stage-strip')).toBeTruthy();

  fireEvent.click(devCard);
  const inspector = screen.getByLabelText('Stage 属性面板');
  expect(await within(inspector).findByDisplayValue('dev')).toBeInTheDocument();
  expect(await within(inspector).findByText('已绑定集群：上海集群')).toBeInTheDocument();
  expect(within(inspector).getByRole('button', { name: '绑定集群' })).toBeInTheDocument();
  expect(within(inspector).getByRole('button', { name: '删除' })).toBeInTheDocument();
  expect(within(inspector).queryByLabelText('Stage 颜色')).not.toBeInTheDocument();

  await userEvent.click(screen.getByRole('button', { name: '添加 Stage' }));
  const newCard = await screen.findByLabelText('stage-5 Stage 模板');
  expect(within(newCard).getByText('新 Stage')).toBeInTheDocument();
  expect(await within(screen.getByLabelText('Stage 属性面板')).findByText('保存 DAG 后可绑定集群。')).toBeInTheDocument();
});

test('Stage 集群绑定使用弹窗和单选列表', async () => {
  renderPage();

  const devCard = await screen.findByLabelText('dev Stage 模板');
  fireEvent.click(devCard);
  await userEvent.click(within(screen.getByLabelText('Stage 属性面板')).getByRole('button', { name: '绑定集群' }));

  const dialog = await screen.findByRole('dialog', { name: '绑定集群' });
  expect(within(dialog).getByText('绑定到租户级 Stage，保存后作为该 Stage 的唯一目标集群。')).toBeInTheDocument();
  expect(within(dialog).getByText('同一集群可以绑定多个 Stage；一个 Stage 最多绑定一个集群，清空后该 Stage 暂不可部署。')).toBeInTheDocument();
  expect(within(dialog).getAllByLabelText('绑定集群').length).toBeGreaterThan(0);
  expect(screen.queryByText('Stage 绑定')).not.toBeInTheDocument();
});

test('真实 API 使用当前租户加载模板集群并保存绑定', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  const fetchMock = vi.fn(async (url: string, options?: RequestInit) => {
    if (url.endsWith('/api/tenants?page=1&page_size=100')) {
      return new Response(JSON.stringify({ items: [{ id: 'tenant_real', name: 'pilot', display_name: '试点租户', updated_at: '2026-06-12T09:00:00Z' }], total: 1, page: 1, page_size: 100 }), { status: 200 });
    }
    if (url.endsWith('/api/tenants/tenant_real/delivery-flow-template')) {
      return new Response(JSON.stringify({
        id: 'template_real',
        tenant_id: 'tenant_real',
        name: '试点交付流',
        stages: [{ id: 'stage_dev', tenant_id: 'tenant_real', template_id: 'template_real', stage_key: 'dev', display_name: '开发', color: '#4f46e5', stage_order: 1, layout_column: 0, layout_row: 0, status: 'enabled' }],
        edges: []
      }), { status: 200 });
    }
    if (url.endsWith('/api/clusters?tenant_id=tenant_real&page=1&page_size=100&actor_id=usr_admin')) {
      return new Response(JSON.stringify({ items: [{ id: 'cluster_real', name: '试点集群', region: 'llt', status: 'ready' }], total: 1, page: 1, page_size: 100 }), { status: 200 });
    }
    if (url.endsWith('/api/tenants/tenant_real/delivery-flow-template/stages/dev/cluster-bindings') && options?.method !== 'PUT') {
      return new Response(JSON.stringify({ items: [{ id: 'binding_real', tenant_id: 'tenant_real', stage_key: 'dev', cluster_id: 'cluster_real', cluster_name: '试点集群', status: 'active' }] }), { status: 200 });
    }
    if (url.endsWith('/api/tenants/tenant_real/delivery-flow-template/stages/dev/cluster-bindings') && options?.method === 'PUT') {
      return new Response(JSON.stringify({ items: [{ id: 'binding_real', tenant_id: 'tenant_real', stage_key: 'dev', cluster_id: 'cluster_real', cluster_name: '试点集群', status: 'active' }] }), { status: 200 });
    }
    return new Response('', { status: 404 });
  });
  vi.stubGlobal('fetch', fetchMock);
  const { DeliveryFlowTemplatePage: RealAPIPage } = await import('./DeliveryFlowTemplatePage');

  render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <RealAPIPage />
        </MemoryRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );

  const devCard = await screen.findByLabelText('dev Stage 模板');
  fireEvent.click(devCard);
  const inspector = screen.getByLabelText('Stage 属性面板');
  expect(await within(inspector).findByText('已绑定集群：试点集群')).toBeInTheDocument();
  await userEvent.click(within(inspector).getByRole('button', { name: '绑定集群' }));
  const dialog = await screen.findByRole('dialog', { name: '绑定集群' });
  expect(within(dialog).getAllByLabelText('绑定集群').length).toBeGreaterThan(0);
  await userEvent.click(within(dialog).getByRole('button', { name: 'OK' }));

  expect(fetchMock).toHaveBeenCalledWith('https://paas.example/api/tenants/tenant_real/delivery-flow-template', expect.any(Object));
  expect(fetchMock).toHaveBeenCalledWith('https://paas.example/api/clusters?tenant_id=tenant_real&page=1&page_size=100&actor_id=usr_admin', expect.any(Object));
  expect(fetchMock).toHaveBeenCalledWith(
    'https://paas.example/api/tenants/tenant_real/delivery-flow-template/stages/dev/cluster-bindings',
    expect.objectContaining({
      method: 'PUT',
      body: JSON.stringify({ actor: { type: 'user', id: 'usr_admin' }, clusters: [{ cluster_id: 'cluster_real', cluster_name: '试点集群' }] })
    })
  );
});

test('保存 DAG 调用图模板接口', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  const fetchMock = vi.fn(async (url: string, options?: RequestInit) => {
    if (url.endsWith('/api/tenants?page=1&page_size=100')) {
      return new Response(JSON.stringify({ items: [{ id: 'tenant_real', name: 'pilot', display_name: '试点租户', updated_at: '2026-06-12T09:00:00Z' }], total: 1, page: 1, page_size: 100 }), { status: 200 });
    }
    if (url.endsWith('/api/tenants/tenant_real/delivery-flow-template')) {
      return new Response(JSON.stringify({
        id: 'template_real',
        tenant_id: 'tenant_real',
        name: '试点交付流',
        stages: [
          { id: 'stage_dev', tenant_id: 'tenant_real', template_id: 'template_real', stage_key: 'dev', display_name: '开发', color: '#4f46e5', stage_order: 1, layout_column: 0, layout_row: 0, status: 'enabled' },
          { id: 'stage_test', tenant_id: 'tenant_real', template_id: 'template_real', stage_key: 'test', display_name: '测试', color: '#0891b2', stage_order: 2, layout_column: 1, layout_row: 0, status: 'enabled' }
        ],
        edges: [{ id: 'edge_1', tenant_id: 'tenant_real', template_id: 'template_real', from_stage_key: 'dev', to_stage_key: 'test' }]
      }), { status: 200 });
    }
    if (url.endsWith('/api/clusters?tenant_id=tenant_real&page=1&page_size=100&actor_id=usr_admin')) {
      return new Response(JSON.stringify({ items: [] }), { status: 200 });
    }
    if (url.includes('/cluster-bindings')) {
      return new Response(JSON.stringify({ items: [] }), { status: 200 });
    }
    if (url.endsWith('/api/tenants/tenant_real/delivery-flow-template/graph') && options?.method === 'PUT') {
      return new Response(JSON.stringify({}), { status: 200 });
    }
    return new Response('', { status: 404 });
  });
  vi.stubGlobal('fetch', fetchMock);
  const { DeliveryFlowTemplatePage: RealAPIPage } = await import('./DeliveryFlowTemplatePage');

  render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <RealAPIPage />
        </MemoryRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );

  await screen.findByLabelText('dev Stage 模板');
  await userEvent.click(screen.getByRole('button', { name: '保存 DAG' }));

  const graphCall = fetchMock.mock.calls.find(([url, options]) => url === 'https://paas.example/api/tenants/tenant_real/delivery-flow-template/graph' && options?.method === 'PUT');
  expect(graphCall).toBeTruthy();
  const body = JSON.parse(String(graphCall?.[1]?.body));
  expect(body.stages).toHaveLength(2);
  expect(body.stages.map((stage: any) => stage.stage_key)).toEqual(['dev', 'test']);
  expect(body.stages[1].layout_column).toBe(1);
  expect(body.edges).toEqual([{ from_stage_key: 'dev', to_stage_key: 'test' }]);
  expect(body.deleted_stage_keys).toEqual([]);
});

test('删除 Stage 后保存 DAG 显式发送删除列表', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  const fetchMock = vi.fn(async (url: string, options?: RequestInit) => {
    if (url.endsWith('/api/tenants?page=1&page_size=100')) {
      return new Response(JSON.stringify({ items: [{ id: 'tenant_real', name: 'pilot', display_name: '试点租户', updated_at: '2026-06-12T09:00:00Z' }], total: 1, page: 1, page_size: 100 }), { status: 200 });
    }
    if (url.endsWith('/api/tenants/tenant_real/delivery-flow-template')) {
      return new Response(JSON.stringify({
        id: 'template_real',
        tenant_id: 'tenant_real',
        name: '试点交付流',
        stages: [
          { id: 'stage_dev', tenant_id: 'tenant_real', template_id: 'template_real', stage_key: 'dev', display_name: '开发', color: '#4f46e5', stage_order: 1, layout_column: 0, layout_row: 0, status: 'enabled' },
          { id: 'stage_test', tenant_id: 'tenant_real', template_id: 'template_real', stage_key: 'test', display_name: '测试', color: '#0891b2', stage_order: 2, layout_column: 1, layout_row: 0, status: 'enabled' }
        ],
        edges: [{ id: 'edge_1', tenant_id: 'tenant_real', template_id: 'template_real', from_stage_key: 'dev', to_stage_key: 'test' }]
      }), { status: 200 });
    }
    if (url.endsWith('/api/clusters?tenant_id=tenant_real&page=1&page_size=100&actor_id=usr_admin')) {
      return new Response(JSON.stringify({ items: [] }), { status: 200 });
    }
    if (url.includes('/cluster-bindings')) {
      return new Response(JSON.stringify({ items: [] }), { status: 200 });
    }
    if (url.endsWith('/api/tenants/tenant_real/delivery-flow-template/graph') && options?.method === 'PUT') {
      return new Response(JSON.stringify({}), { status: 200 });
    }
    return new Response('', { status: 404 });
  });
  vi.stubGlobal('fetch', fetchMock);
  const { DeliveryFlowTemplatePage: RealAPIPage } = await import('./DeliveryFlowTemplatePage');

  render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <RealAPIPage />
        </MemoryRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );

  const testCard = await screen.findByLabelText('test Stage 模板');
  fireEvent.click(testCard);
  await userEvent.click(within(screen.getByLabelText('Stage 属性面板')).getByRole('button', { name: '删除' }));
  const confirmText = await screen.findByText('确认删除该 Stage？保存 DAG 后生效。');
  const popover = confirmText.closest('.ant-popover') as HTMLElement;
  await userEvent.click(within(popover).getByRole('button', { name: /删\s*除/ }));
  await waitFor(() => expect(screen.queryByLabelText('test Stage 模板')).not.toBeInTheDocument());
  await userEvent.click(screen.getByRole('button', { name: '保存 DAG' }));

  const graphCall = fetchMock.mock.calls.find(([url, options]) => url === 'https://paas.example/api/tenants/tenant_real/delivery-flow-template/graph' && options?.method === 'PUT');
  expect(graphCall).toBeTruthy();
  const body = JSON.parse(String(graphCall?.[1]?.body));
  expect(body.stages.map((stage: any) => stage.stage_key)).toEqual(['dev']);
  expect(body.deleted_stage_keys).toEqual(['test']);
});

test('单 Stage 模板添加卡片后保存会发送全部本地 Stage', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  const fetchMock = vi.fn(async (url: string, options?: RequestInit) => {
    if (url.endsWith('/api/tenants?page=1&page_size=100')) {
      return new Response(JSON.stringify({ items: [{ id: 'tenant_real', name: 'pilot', display_name: '试点租户', updated_at: '2026-06-12T09:00:00Z' }], total: 1, page: 1, page_size: 100 }), { status: 200 });
    }
    if (url.endsWith('/api/tenants/tenant_real/delivery-flow-template')) {
      return new Response(JSON.stringify({
        id: 'template_real',
        tenant_id: 'tenant_real',
        name: '试点交付流',
        stages: [{ id: 'stage_dev', tenant_id: 'tenant_real', template_id: 'template_real', stage_key: 'dev', display_name: '开发', color: '#4f46e5', stage_order: 1, layout_column: 0, layout_row: 0, status: 'enabled' }],
        edges: []
      }), { status: 200 });
    }
    if (url.endsWith('/api/clusters?tenant_id=tenant_real&page=1&page_size=100&actor_id=usr_admin')) {
      return new Response(JSON.stringify({ items: [] }), { status: 200 });
    }
    if (url.includes('/cluster-bindings')) {
      return new Response(JSON.stringify({ items: [] }), { status: 200 });
    }
    if (url.endsWith('/api/tenants/tenant_real/delivery-flow-template/graph') && options?.method === 'PUT') {
      return new Response(JSON.stringify({}), { status: 200 });
    }
    return new Response('', { status: 404 });
  });
  vi.stubGlobal('fetch', fetchMock);
  const { DeliveryFlowTemplatePage: RealAPIPage } = await import('./DeliveryFlowTemplatePage');

  render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter>
          <RealAPIPage />
        </MemoryRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );

  await screen.findByLabelText('dev Stage 模板');
  await userEvent.click(screen.getByRole('button', { name: '添加 Stage' }));
  await screen.findByLabelText('stage-2 Stage 模板');
  await userEvent.click(screen.getByRole('button', { name: '保存 DAG' }));

  const graphCall = fetchMock.mock.calls.find(([url, options]) => url === 'https://paas.example/api/tenants/tenant_real/delivery-flow-template/graph' && options?.method === 'PUT');
  expect(graphCall).toBeTruthy();
  const body = JSON.parse(String(graphCall?.[1]?.body));
  expect(body.stages.map((stage: any) => stage.stage_key)).toEqual(['dev', 'stage-2']);
  expect(body.deleted_stage_keys).toEqual([]);
});
