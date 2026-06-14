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
        pipeline_id: 'pipeline_1',
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
    pipelineId: 'pipeline_1',
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
    pipeline_id: 'pipeline_1',
    custom_image: 'registry.example.com/order/worker:20260611',
    replicas: 2
  });
  expect(postBody).not.toHaveProperty('imageSourceMode');
  expect(postBody).not.toHaveProperty('customImage');
});

test('真实 API 更新 Workload 调用 PUT 并映射 image_source_mode', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  let putBody: Record<string, unknown> = {};
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/api/applications/app_1/workloads/workload_api') && init?.method === 'PUT') {
      putBody = JSON.parse(String(init.body));
      return jsonResponse({
        id: 'workload_api',
        application_id: 'app_1',
        name: 'order-api',
        display_name: '订单接口 v2',
        workload_type: 'Deployment',
        image_source_mode: 'mixed',
        pipeline_id: 'pipeline_2',
        status: 'enabled'
      });
    }
    return jsonResponse({ error: { code: 'not_found', message: '未处理请求' } }, 404);
  }));

  const api = await import('./index');
  await expect(api.updateWorkload('app_1', 'workload_api', {
    name: 'order-api',
    displayName: '订单接口 v2',
    workloadType: 'deployment',
    imageSourceMode: 'mixed',
    pipelineId: 'pipeline_2'
  })).resolves.toMatchObject({
    id: 'workload_api',
    displayName: '订单接口 v2',
    imageSourceMode: 'mixed'
  });
  expect(putBody).toMatchObject({
    name: 'order-api',
    display_name: '订单接口 v2',
    workload_type: 'deployment',
    image_source_mode: 'mixed',
    pipeline_id: 'pipeline_2'
  });
});

test('真实 API 保存和查询工作负载默认配置映射扩展字段', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  let putBody: Record<string, any> = {};
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/api/applications/app_1/workloads/workload_api/default-config') && init?.method === 'PUT') {
      putBody = JSON.parse(String(init.body));
      return jsonResponse({
        id: 'workload_default_config_api',
        workload_id: 'workload_api',
        environment_id: '',
        replicas: 2,
        env_vars: [{ name: 'group', value: 'iot' }],
        config_files: [{ mount_path: '/etc/app/app.yaml', content: 'server.port: 8080', base64_encoded: true }],
        writable_dirs: [{ mount_path: '/data', owner_group: 'app:app', mode: '0775' }]
      });
    }
    if (url.endsWith('/api/applications/app_1/workloads/workload_api/default-config') && init?.method !== 'PUT') {
      return jsonResponse({
        id: 'workload_default_config_api',
        workload_id: 'workload_api',
        environment_id: '',
        replicas: 2,
        env_vars: [{ name: 'group', value: 'iot' }],
        config_files: [{ mount_path: '/etc/app/app.yaml', content: 'server.port: 8080', base64_encoded: true }],
        writable_dirs: [{ mount_path: '/data', owner_group: 'app:app', mode: '0775' }]
      });
    }
    return jsonResponse({ error: { code: 'not_found', message: '未处理请求' } }, 404);
  }));

  const api = await import('./index');
  const input = {
    replicas: 2,
    envVars: [{ name: 'group', value: 'iot' }],
    configFiles: [{ mountPath: '/etc/app/app.yaml', content: 'server.port: 8080', base64Encoded: true }],
    writableDirs: [{ mountPath: '/data', ownerGroup: 'app:app', mode: '0775' }]
  };
  await expect(api.saveWorkloadDefaultConfig('app_1', 'workload_api', input)).resolves.toMatchObject(input);
  await expect(api.getWorkloadDefaultConfig('app_1', 'workload_api')).resolves.toMatchObject(input);
  expect(putBody).toMatchObject({
    replicas: 2,
    env_vars: [{ name: 'group', value: 'iot' }],
    config_files: [{ mount_path: '/etc/app/app.yaml', content: 'server.port: 8080', base64_encoded: true }],
    writable_dirs: [{ mount_path: '/data', owner_group: 'app:app', mode: '0775' }]
  });
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

test('真实 API 查询应用环境并保存 Workload 环境配置', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  let saveBody: Record<string, unknown> = {};
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/api/applications/app_1/environments')) {
      return jsonResponse({ items: [{ id: 'env_dev', application_id: 'app_1', name: 'dev', display_name: '开发环境' }] });
    }
    if (url.endsWith('/api/applications/app_1/workloads/workload_api/environment-configs/env_dev') && init?.method === 'PUT') {
      saveBody = JSON.parse(String(init.body));
      return jsonResponse({
        id: 'config_dev',
        workload_id: 'workload_api',
        environment_id: 'env_dev',
        replicas: 2,
        service_ports: [{ name: 'http', port: 80, target_port: 8080, protocol: 'TCP' }],
        ingress_hosts: [{ host: 'dev-order.example.com', path: '/' }],
        env_vars: [{ name: 'SPRING_PROFILES_ACTIVE', value: 'dev' }],
        config_files: [{ mount_path: '/app/config/application.yaml', content: 'server.port: 8080' }],
        writable_dirs: [{ mount_path: '/data', size_limit: '5Gi' }]
      });
    }
    return jsonResponse({ error: { code: 'not_found', message: '未处理请求' } }, 404);
  }));

  const api = await import('./index');
  await expect(api.listApplicationEnvironments('app_1')).resolves.toEqual([
    expect.objectContaining({ id: 'env_dev', name: 'dev', displayName: '开发环境' })
  ]);
  await expect(api.saveWorkloadEnvironmentConfig('app_1', 'workload_api', 'env_dev', {
    replicas: 2,
    servicePorts: [{ name: 'http', port: 80, targetPort: 8080, protocol: 'TCP' }],
    ingressHosts: [{ host: 'dev-order.example.com', path: '/', servicePort: 'http', tls: false }],
    envVars: [{ name: 'SPRING_PROFILES_ACTIVE', value: 'dev' }],
    configFiles: [{ mountPath: '/app/config/application.yaml', content: 'server.port: 8080' }],
    writableDirs: [{ mountPath: '/data', sizeLimit: '5Gi' }]
  })).resolves.toMatchObject({ id: 'config_dev', replicas: 2 });
  expect(saveBody).toMatchObject({
    replicas: 2,
    service_ports: [{ name: 'http', port: 80, target_port: 8080, protocol: 'TCP' }],
    ingress_hosts: [{ host: 'dev-order.example.com', path: '/', service_port: 'http', tls: false }],
    env_vars: [{ name: 'SPRING_PROFILES_ACTIVE', value: 'dev' }],
    config_files: [{ mount_path: '/app/config/application.yaml', content: 'server.port: 8080' }],
    writable_dirs: [{ mount_path: '/data', size_limit: '5Gi' }]
  });
});

test('真实 API 查询应用构建记录映射 pipeline_id', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    if (url.endsWith('/api/apps/app_1/builds?page=1&page_size=50')) {
      return jsonResponse({ items: [{ id: 'build_1', application_id: 'app_1', pipeline_id: 'pipeline_1', status: 'succeeded', git_ref: 'main', commit_sha: 'abc123', started_at: '2026-06-11T09:00:00Z', duration_seconds: 120 }] });
    }
    return jsonResponse({ error: { code: 'not_found', message: '未处理请求' } }, 404);
  }));

  const api = await import('./index');
  await expect(api.listApplicationBuilds('app_1')).resolves.toEqual([
    expect.objectContaining({ id: 'build_1', pipelineId: 'pipeline_1', status: '成功', ref: 'main', commit: 'abc123' })
  ]);
});

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}
