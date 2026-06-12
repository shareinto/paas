import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, test, vi } from 'vitest';
import { MemoryRouter, Route, Routes } from 'react-router-dom';

afterEach(() => {
  cleanup();
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
  vi.resetModules();
});

test('真实 API 列表无 items 时点击 Freight 会加载详情并展示 Workload 镜像', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
	  const fetchMock = vi.fn(async (url: string) => {
	    if (url.endsWith('/api/apps/app_1/freights?page=1&page_size=50')) {
	      return new Response(JSON.stringify({ items: [{ id: 'freight_1', name: '20260612.1', created_at: '2026-06-12T09:00:00Z' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
	    }
	    if (url.endsWith('/api/applications/app_1')) {
	      return new Response(JSON.stringify({ id: 'app_1', name: 'order-service', display_name: '订单服务', project_id: 'project_1', project: '订单平台' }), { status: 200 });
	    }
	    if (url.endsWith('/api/apps/app_1/stages')) {
	      return new Response(JSON.stringify({ items: [{
          tenant_id: 'tenant_1',
          project_id: 'project_1',
          application_id: 'app_1',
          delivery_stage_id: 'stage_dev',
          environment_id: 'env_dev',
          stage_key: 'dev',
          display_name: '开发',
          color: '#ED204E',
          order: 1,
          layout_column: 0,
          layout_row: 0,
          status: 'enabled',
          cluster_pool_size: 1,
          bound_cluster_id: 'cluster_shanghai',
          bound_cluster_name: '上海集群',
          upstream_stage_keys: [],
          downstream_stage_keys: []
        }] }), { status: 200 });
	    }
	    if (url.endsWith('/api/apps/app_1/freights/creation-context')) {
	      return new Response(JSON.stringify({
        enabled_workloads: [{ id: 'workload_api', name: 'api', display_name: '订单接口', status: 'enabled' }],
        latest_releases_by_workload: {},
        latest_artifacts_by_workload: {},
        stage_eligibility: { stage_dev: ['freight_1'] },
        stages: [{ id: 'stage_dev', name: 'dev', environment_id: 'env_dev' }]
      }), { status: 200 });
    }
    if (url.endsWith('/api/apps/app_1/delivery/stages/stage_dev/eligible-freights')) {
      return new Response(JSON.stringify([{ id: 'freight_1', name: '20260612.1', created_at: '2026-06-12T09:00:00Z' }]), { status: 200 });
    }
    if (url.endsWith('/api/freights/freight_1')) {
      return new Response(JSON.stringify({
        freight: { id: 'freight_1', name: '20260612.1', created_at: '2026-06-12T09:00:00Z' },
        items: [{
          id: 'item_1',
          workload_id: 'workload_api',
          workload_display_name: '订单接口',
          source_type: 'pipeline_artifact',
          release_id: 'release_1',
          build_artifact_id: 'artifact_1',
          image_ref: 'registry.local/order-api:20260612.1',
          image_repository: 'registry.local/order-api',
          image_tag: '20260612.1',
          digest: 'sha256:api'
        }]
      }), { status: 200 });
    }
    return new Response('', { status: 404 });
  });
  vi.stubGlobal('fetch', fetchMock);

  const { PromotionPage } = await import('./PromotionPage');
  render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/promotions']}>
          <PromotionPage />
        </MemoryRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );

	  const devCard = await screen.findByLabelText('dev Stage');
	  dragFreightToStage(await freightCardByName('20260612.1'), devCard);

	  const confirm = await within(devCard).findByLabelText('dev 发布确认');
	  expect(within(confirm).getByText('订单接口')).toBeInTheDocument();
	  expect(within(confirm).getByText('registry.local/order-api:20260612.1')).toBeInTheDocument();
	  expect(fetchMock).toHaveBeenCalledWith('https://paas.example/api/freights/freight_1', expect.any(Object));
	});

test('真实 API 使用当前路由应用 ID 和后端返回的真实 Stage ID', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
	  const fetchMock = vi.fn(async (url: string, options?: RequestInit) => {
	    if (url.endsWith('/api/apps/app_other/freights?page=1&page_size=50')) {
	      return new Response(JSON.stringify({ items: [{ id: 'freight_other', name: '20260613.1', created_at: '2026-06-13T09:00:00Z' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
	    }
	    if (url.endsWith('/api/applications/app_other')) {
	      return new Response(JSON.stringify({ id: 'app_other', name: 'other-order', display_name: '外部订单', project_id: 'project_other', project: '外部订单平台' }), { status: 200 });
	    }
	    if (url.endsWith('/api/apps/app_other/stages')) {
	      return new Response(JSON.stringify({ items: [{
          tenant_id: 'tenant_1',
          project_id: 'project_other',
          application_id: 'app_other',
          delivery_stage_id: 'delivery_stage_real_dev',
          environment_id: 'env_other_dev',
          stage_key: 'dev',
          display_name: '开发',
          color: '#ED204E',
          order: 1,
          layout_column: 0,
          layout_row: 0,
          status: 'enabled',
          cluster_pool_size: 1,
          bound_cluster_id: 'cluster_shanghai',
          bound_cluster_name: '上海集群',
          upstream_stage_keys: [],
          downstream_stage_keys: []
        }] }), { status: 200 });
	    }
	    if (url.endsWith('/api/apps/app_other/freights/creation-context')) {
	      return new Response(JSON.stringify({
        enabled_workloads: [{ id: 'workload_other_api', name: 'api', display_name: '外部订单接口', status: 'enabled' }],
        latest_releases_by_workload: {
          workload_other_api: {
            id: 'release_other',
            workload_id: 'workload_other_api',
            version: '20260613.1',
            image_uri: 'registry.local/other-api:20260613.1',
            image_repository: 'registry.local/other-api',
            image_tag: '20260613.1',
            image_digest: 'sha256:other',
            commit_sha: 'abc123',
            build_artifact_id: 'artifact_other',
            created_at: '2026-06-13T08:59:00Z'
          }
        },
        latest_artifacts_by_workload: {},
        stage_eligibility: { delivery_stage_real_dev: ['freight_other'] },
        stages: [{ id: 'delivery_stage_real_dev', name: 'dev', environment_id: 'env_other_dev', approval_required: false }]
      }), { status: 200 });
    }
    if (url.endsWith('/api/apps/app_other/delivery/stages/delivery_stage_real_dev/eligible-freights')) {
      return new Response(JSON.stringify([{ id: 'freight_other', name: '20260613.1', created_at: '2026-06-13T09:00:00Z' }]), { status: 200 });
    }
    if (url.endsWith('/api/freights/freight_other')) {
      return new Response(JSON.stringify({
        freight: { id: 'freight_other', name: '20260613.1', created_at: '2026-06-13T09:00:00Z' },
        items: [{
          id: 'item_other',
          workload_id: 'workload_other_api',
          workload_display_name: '外部订单接口',
          source_type: 'pipeline_artifact',
          image_ref: 'registry.local/other-api:20260613.1',
          digest: 'sha256:other'
        }]
      }), { status: 200 });
    }
    if (url.endsWith('/api/apps/app_other/freights') && options?.method === 'POST') {
      return new Response(JSON.stringify({ id: 'freight_created', name: '手工 Freight', created_at: '2026-06-13T10:00:00Z' }), { status: 201 });
    }
    return new Response('', { status: 404 });
  });
  vi.stubGlobal('fetch', fetchMock);

  const { PromotionPage } = await import('./PromotionPage');
  render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient()}>
        <MemoryRouter initialEntries={['/apps/app_other/promotions']}>
          <Routes>
            <Route path="/apps/:id/promotions" element={<PromotionPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </ConfigProvider>
  );

	  const devCard = await screen.findByLabelText('dev Stage');
	  dragFreightToStage(await freightCardByName('20260613.1'), devCard);
	  expect(await within(devCard).findByLabelText('dev 发布确认')).toBeInTheDocument();
	  await userEvent.click(within(devCard).getByRole('button', { name: /取\s*消/ }));

	  await userEvent.click(await screen.findByRole('button', { name: '创建 Freight' }));
  const drawer = await screen.findByRole('dialog', { name: '创建 Freight' });
  await userEvent.click(within(drawer).getByRole('button', { name: '从最新成功版本填充' }));
  await userEvent.click(within(drawer).getByRole('button', { name: '创建 Freight' }));

  expect(fetchMock).toHaveBeenCalledWith('https://paas.example/api/apps/app_other/freights?page=1&page_size=50', expect.any(Object));
  expect(fetchMock).toHaveBeenCalledWith('https://paas.example/api/apps/app_other/freights/creation-context', expect.any(Object));
  expect(fetchMock).toHaveBeenCalledWith('https://paas.example/api/apps/app_other/delivery/stages/delivery_stage_real_dev/eligible-freights', expect.any(Object));
  expect(fetchMock).toHaveBeenCalledWith('https://paas.example/api/apps/app_other/freights', expect.objectContaining({ method: 'POST' }));
});

async function freightCardByName(name: string) {
  const timeline = await screen.findByLabelText('Freight 时间轴');
  return within(timeline).getAllByTestId('freight-card').find((card) => within(card).queryByText(name)) as HTMLElement;
}

function dragFreightToStage(freightCard: HTMLElement, stageCard: HTMLElement) {
  const data = new Map<string, string>();
  const dataTransfer = {
    setData: (type: string, value: string) => data.set(type, value),
    getData: (type: string) => data.get(type) || ''
  } as DataTransfer;
  fireEvent.dragStart(freightCard, { dataTransfer });
  fireEvent.dragOver(stageCard, { dataTransfer });
  fireEvent.drop(stageCard, { dataTransfer });
}
