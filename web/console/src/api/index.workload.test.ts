import { afterEach, expect, test, vi } from 'vitest';

afterEach(() => {
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
  vi.resetModules();
  window.localStorage.clear();
});

test('真实 API 查询 Workload 不伪造 Release 和环境聚合状态', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url.endsWith('/api/applications/app_1/workloads')) {
      return jsonResponse({
        items: [
          { id: 'workload_api', application_id: 'app_1', name: 'order-api', display_name: '订单接口', workload_type: 'Deployment', status: 'enabled' },
          { id: 'workload_worker', application_id: 'app_1', name: 'order-worker', display_name: '订单任务', workload_type: 'StatefulSet', status: 'enabled' }
        ]
      });
    }
    return jsonResponse({ error: { code: 'not_found', message: '未处理请求' } }, 404);
  }));

  const api = await import('./index');
  await expect(api.listWorkloads('app_1')).resolves.toMatchObject([
    {
      id: 'workload_api',
      applicationId: 'app_1',
      name: 'order-api',
      displayName: '订单接口',
      workloadType: 'deployment',
      latestRelease: '',
      envStatuses: []
    },
    {
      id: 'workload_worker',
      workloadType: 'statefulset',
      latestRelease: '',
      envStatuses: []
    }
  ]);
});

test('真实 API 创建 Workload 使用服务端响应作为缓存数据来源', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  let postBody: Record<string, unknown> = {};
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/api/applications/app_1/workloads') && init?.method === 'POST') {
      postBody = JSON.parse(String(init.body));
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
    return jsonResponse({ error: { code: 'not_found', message: '未处理请求' } }, 404);
  }));

  const api = await import('./index');
  await expect(api.createWorkload('app_1', {
    name: 'order-worker',
    displayName: '订单任务',
    workloadType: 'statefulset',
    imageSourceMode: 'custom_image',
    customImage: 'registry.example.com/order/worker:20260611',
    replicas: 2
  })).resolves.toMatchObject({
    id: 'workload_worker',
    imageSourceMode: 'pipeline_artifact',
    imageSourceName: '主流水线'
  });
  expect(postBody).toMatchObject({
    name: 'order-worker',
    display_name: '订单任务',
    workload_type: 'statefulset',
    image_source_mode: 'custom_image',
    custom_image: 'registry.example.com/order/worker:20260611',
    replicas: 2
  });
  expect(postBody).not.toHaveProperty('imageSourceMode');
  expect(postBody).not.toHaveProperty('customImage');
});

test('真实 API 删除 Workload 提交软删除请求', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  let deleteBody: Record<string, unknown> = {};
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/api/applications/app_1/workloads/workload_api') && init?.method === 'DELETE') {
      deleteBody = JSON.parse(String(init.body));
      return jsonResponse({
        id: 'workload_api',
        application_id: 'app_1',
        name: 'order-api',
        display_name: '订单接口',
        workload_type: 'Deployment',
        status: 'deleted'
      });
    }
    return jsonResponse({ error: { code: 'not_found', message: '未处理请求' } }, 404);
  }));

  const api = await import('./index');
  await expect(api.deleteWorkload('app_1', 'workload_api')).resolves.toMatchObject({
    id: 'workload_api',
    status: 'deleted'
  });
  expect(fetch).toHaveBeenCalledWith('https://paas.example/api/applications/app_1/workloads/workload_api', expect.objectContaining({ method: 'DELETE' }));
  expect(deleteBody).toMatchObject({ actor: { type: 'user', id: 'usr_admin' } });
});

test('mock 删除 Workload 后列表不再返回该项', async () => {
  const api = await import('./index');

  await expect(api.listWorkloads('app_1')).resolves.toEqual(expect.arrayContaining([
    expect.objectContaining({ id: 'workload_api' })
  ]));
  await expect(api.deleteWorkload('app_1', 'workload_api')).resolves.toMatchObject({
    id: 'workload_api',
    status: 'deleted'
  });
  await expect(api.listWorkloads('app_1')).resolves.not.toEqual(expect.arrayContaining([
    expect.objectContaining({ id: 'workload_api' })
  ]));
});

test('真实 API 查询 Workload 环境配置映射配置文件和可写目录真实字段', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url.endsWith('/api/applications/app_1/workloads/workload_api/environment-configs')) {
      return jsonResponse({
        items: [
          {
            id: 'config_prod',
            workload_id: 'workload_api',
            environment_id: 'env_prod',
            env_name: 'prod',
            config_files: [{ mount_path: '/app/config/application.yaml', content: 'server:\n  port: 8080\n' }],
            writable_dirs: [{ mount_path: '/data', size_limit: '20Gi' }]
          }
        ]
      });
    }
    return jsonResponse({ error: { code: 'not_found', message: '未处理请求' } }, 404);
  }));

  const api = await import('./index');
  await expect(api.listWorkloadEnvironmentConfigs('app_1', 'workload_api')).resolves.toMatchObject([
    {
      id: 'config_prod',
      configFiles: [{ mountPath: '/app/config/application.yaml', content: 'server:\n  port: 8080\n' }],
      writableDirs: [{ mountPath: '/data', sizeLimit: '20Gi' }]
    }
  ]);
});

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}
