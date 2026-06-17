import { ConfigProvider } from 'antd';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, test, vi } from 'vitest';

async function renderPage() {
  const { ClustersPage } = await import('./ClustersPage');
  return render(
    <ConfigProvider>
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })}>
        <ClustersPage />
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

test('集群管理页默认选择首个租户并展示标签', async () => {
  await renderPage();

  expect(await screen.findByRole('heading', { name: '集群管理' })).toBeInTheDocument();
  expect(await screen.findByText('当前租户：研发中心')).toBeInTheDocument();
  expect(await screen.findByText('上海集群')).toBeInTheDocument();
  expect(screen.getAllByText('cloud=aliyun').length).toBeGreaterThan(0);
  expect(screen.getByText('zone=hangzhou')).toBeInTheDocument();
});

test('编辑标签会调用更新接口并刷新列表', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  let listCount = 0;
  let patchBody: any;
  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/api/tenants?page=1&page_size=100')) {
      return new Response(JSON.stringify({ items: [{ id: 'tenant_real', name: 'pilot', display_name: '试点租户', updated_at: '2026-06-12T09:00:00Z' }] }), { status: 200 });
    }
    if (url.endsWith('/api/clusters?tenant_id=tenant_real&page=1&page_size=100&actor_id=usr_admin')) {
      listCount += 1;
      const labels = listCount > 1 ? { cloud: 'tencent' } : { cloud: 'aliyun' };
      return new Response(JSON.stringify({ items: [{ id: 'cluster_real', tenant_id: 'tenant_real', name: '试点集群', region: 'cn-shanghai', status: 'ready', labels, last_heartbeat_at: '2026-06-12T09:05:00Z', updated_at: '2026-06-12T09:00:00Z' }] }), { status: 200 });
    }
    if (url.endsWith('/api/clusters/cluster_real') && init?.method === 'PATCH') {
      patchBody = JSON.parse(String(init.body));
      return new Response(JSON.stringify({ id: 'cluster_real', tenant_id: 'tenant_real', name: patchBody.name, region: patchBody.region, status: 'ready', labels: patchBody.labels, updated_at: '2026-06-12T09:10:00Z' }), { status: 200 });
    }
    return new Response('', { status: 404 });
  });
  vi.stubGlobal('fetch', fetchMock);

  await renderPage();

  expect(await screen.findByText('cloud=aliyun')).toBeInTheDocument();
  const row = screen.getByText('试点集群').closest('tr') as HTMLElement;
  await userEvent.click(within(row).getByRole('button', { name: /编辑/ }));
  const dialog = await screen.findByRole('dialog', { name: '编辑集群' });
  await userEvent.clear(within(dialog).getByLabelText('标签值'));
  await userEvent.type(within(dialog).getByLabelText('标签值'), ' tencent ');
  await userEvent.click(within(dialog).getByRole('button', { name: /添加标签/ }));
  const valueInputs = within(dialog).getAllByLabelText('标签值');
  await userEvent.type(valueInputs[1], 'ignored');
  await userEvent.click(within(dialog).getByRole('button', { name: /^保\s*存$/ }));

  await waitFor(() => expect(patchBody).toMatchObject({
    actor: { type: 'user', id: 'usr_admin' },
    name: '试点集群',
    region: 'cn-shanghai',
    labels: { cloud: 'tencent' }
  }));
  expect(patchBody.labels.ignored).toBeUndefined();
  expect(await screen.findByText('集群已更新')).toBeInTheDocument();
  expect(await screen.findByText('cloud=tencent')).toBeInTheDocument();
  expect(listCount).toBeGreaterThan(1);
});
