import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, within } from '@testing-library/react';
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

test('租户交付流模板展示 Stage 卡片并使用弹窗编辑', async () => {
  renderPage();

  expect(await screen.findByRole('heading', { name: '租户交付流模板' })).toBeInTheDocument();
  const devCard = await screen.findByLabelText('dev Stage 模板');
  expect(within(devCard).getByText('开发')).toBeInTheDocument();
  expect(devCard.querySelector('.stage-color-strip')).toBeTruthy();
  expect(await within(devCard).findByText('已绑定集群')).toBeInTheDocument();
  expect(await within(devCard).findByText('上海集群')).toBeInTheDocument();
  expect(within(devCard).getByRole('button', { name: '绑定集群' })).toBeInTheDocument();
  expect(within(devCard).getByRole('button', { name: '编辑' })).toBeInTheDocument();
  expect(within(devCard).getByRole('button', { name: '删除' })).toBeInTheDocument();

  await userEvent.click(screen.getByRole('button', { name: '添加 Stage' }));
  const stageDialog = await screen.findByRole('dialog', { name: '添加 Stage' });
  expect(within(stageDialog).getByLabelText('Stage key')).toBeInTheDocument();
  expect(within(stageDialog).getByLabelText('Stage 颜色')).toBeInTheDocument();
  expect(within(stageDialog).getByText('Stage key 创建后不可修改')).toBeInTheDocument();
});

test('Stage 集群绑定使用弹窗和多选列表', async () => {
  renderPage();

  const devCard = await screen.findByLabelText('dev Stage 模板');
  await userEvent.click(within(devCard).getByRole('button', { name: '绑定集群' }));

  const dialog = await screen.findByRole('dialog', { name: '绑定集群' });
  expect(within(dialog).getByText('绑定到租户级 Stage，保存后进入该 Stage 的可选集群池。')).toBeInTheDocument();
  expect(within(dialog).getByText('同一集群可绑定多个 Stage，绑定变更仅影响后续发布。')).toBeInTheDocument();
  expect(within(dialog).getByLabelText('可选集群')).toBeInTheDocument();
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
        stages: [{ id: 'stage_dev', tenant_id: 'tenant_real', template_id: 'template_real', stage_key: 'dev', display_name: '开发', color: '#1677ff', stage_order: 1, status: 'enabled' }]
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
  expect(await within(devCard).findByText('试点集群')).toBeInTheDocument();
  await userEvent.click(within(devCard).getByRole('button', { name: '绑定集群' }));
  const dialog = await screen.findByRole('dialog', { name: '绑定集群' });
  expect(within(dialog).getByLabelText('可选集群')).toBeInTheDocument();
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
