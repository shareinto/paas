import { afterEach, expect, test, vi } from 'vitest';

afterEach(() => {
  vi.unstubAllEnvs();
  vi.restoreAllMocks();
  vi.resetModules();
  window.localStorage.clear();
});

test('request 注入 token 并解析成功响应', async () => {
  vi.stubGlobal('fetch', vi.fn(async (_url, init) => {
    expect((init?.headers as Record<string, string>).Authorization).toBe('Bearer token_1');
    return new Response(JSON.stringify({ ok: true }), { status: 200 });
  }));
  window.localStorage.setItem('paas_token', 'token_1');
  const { request } = await import('./client');
  await expect(request('/api/ping')).resolves.toEqual({ ok: true });
});

test('request 将 401 转为会话过期并清理登录态', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ error: { code: 'unauthenticated', message: 'expired' } }), { status: 401 })));
  window.localStorage.setItem('paas_token', 'token_1');
  const { request, APIError } = await import('./client');
  const { useSession } = await import('../app/store');
  await expect(request('/api/ping')).rejects.toBeInstanceOf(APIError);
  expect(useSession.getState().token).toBe('');
});

test('request 映射后端错误响应', async () => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ error: { code: 'invalid_argument', message: '参数错误' } }), { status: 400 })));
  const { request } = await import('./client');
  await expect(request('/api/fail')).rejects.toMatchObject({ code: 'invalid_argument', message: '参数错误' });
});

test('真实 API 分支使用 VITE_API_BASE_URL', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/api/auth/local/login')) return new Response(JSON.stringify({ token: 'token_1', userName: '李雷' }), { status: 200 });
    if (url.endsWith('/api/auth/oidc/start')) return new Response(JSON.stringify({ redirect_url: 'https://idp.example/login' }), { status: 200 });
    if (url.endsWith('/api/tenants?page=1&page_size=100')) return new Response(JSON.stringify({ items: [{ id: 'tenant_1', name: 'rnd', displayName: '研发中心', description: '默认租户', updatedAt: '2026-05-30 10:00' }], total: 1, page: 1, page_size: 100 }), { status: 200 });
    if (url.endsWith('/api/tenants') && init?.method === 'POST') return new Response(JSON.stringify({ id: 'tenant_2', name: 'ops', display_name: '运维中心', description: '平台运维', updated_at: '2026-05-30T10:00:00Z' }), { status: 201 });
    if (url.endsWith('/api/tenants/tenant_2') && init?.method === 'PATCH') return new Response(JSON.stringify({ id: 'tenant_2', name: 'ops', display_name: '平台运维', description: '统一运维租户', updated_at: '2026-05-30T11:00:00Z' }), { status: 200 });
    if (url.endsWith('/api/projects?page=1&page_size=50')) return new Response(JSON.stringify({ items: [{ id: 'project_1' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
    if (url.endsWith('/api/applications?page=1&page_size=50')) return new Response(JSON.stringify({ items: [{ id: 'app_1' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
    if (url.endsWith('/api/builds?page=1&page_size=50')) return new Response(JSON.stringify({ items: [{ id: 'build_1', status: 'queued' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
    if (url.endsWith('/api/builds/build_128/logs/stream')) return new Response('日志', { status: 200 });
    if (url.endsWith('/api/builds/build_129/logs/stream')) return new Response('新日志', { status: 200 });
    if (url.endsWith('/api/builds/build_130/cancel') && init?.method === 'POST') return new Response(JSON.stringify({ id: 'build_130', status: 'aborted' }), { status: 200 });
    if (url.endsWith('/api/audit/logs?page=1&page_size=50')) return new Response(JSON.stringify({ items: [{ id: 'audit_1', actor_id: 'usr_1', action: 'promotion.approve', resource_type: 'promotion', resource_id: 'promotion_1', result: 'succeeded', summary: '审批通过', occurred_at: '2026-05-30T10:00:00Z' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
    if (url.endsWith('/api/apps/app_1/freights?page=1&page_size=50')) return new Response(JSON.stringify({ items: [{ id: 'freight_1', name: 'v1.0.0', image_uri: 'registry.local/app:v1.0.0', image_digest: 'sha256:abc', commit_sha: 'abc123', created_at: '2026-05-30T10:00:00Z' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
    if (url.endsWith('/api/apps/app_2/freights?page=1&page_size=50')) return new Response(JSON.stringify({ items: [{ id: 'freight_2', name: 'v2.0.0', uri: 'registry.local/app:v2.0.0', digest: 'sha256:def', created_at: '2026-05-31T10:00:00Z' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
    return new Response('', { status: 404 });
  });
  vi.stubGlobal('fetch', fetchMock);
  const api = await import('./index');
  await expect(api.login('admin', 'password')).resolves.toEqual({ token: 'token_1', userName: '李雷' });
  await expect(api.oidcLoginURL()).resolves.toBe('https://idp.example/login');
  await expect(api.listTenants()).resolves.toMatchObject([{ id: 'tenant_1', displayName: '研发中心' }]);
  await expect(api.createTenant({ name: 'ops', displayName: '运维中心', description: '平台运维' })).resolves.toMatchObject({ id: 'tenant_2', name: 'ops', displayName: '运维中心' });
  await expect(api.updateTenant('tenant_2', { displayName: '平台运维', description: '统一运维租户' })).resolves.toMatchObject({ id: 'tenant_2', displayName: '平台运维', description: '统一运维租户' });
  await expect(api.listProjects()).resolves.toEqual([{ id: 'project_1' }]);
  await expect(api.listApplications()).resolves.toMatchObject([{ id: 'app_1' }]);
  await expect(api.listBuilds()).resolves.toMatchObject([{ id: 'build_1', status: '构建中' }]);
  await expect(api.buildLog()).resolves.toBe('日志');
  await expect(api.buildLog('build_129')).resolves.toBe('新日志');
  await expect(api.cancelBuild('build_130')).resolves.toMatchObject({ id: 'build_130', status: '已取消' });
  await expect(api.listAuditLogs()).resolves.toMatchObject([{ id: 'audit_1', actor: 'usr_1', action: 'promotion.approve', resource: 'promotion promotion_1', result: '成功', summary: '审批通过' }]);
  await expect(api.listFreights()).resolves.toMatchObject([{ id: 'freight_1', version: 'v1.0.0', image: 'registry.local/app:v1.0.0', digest: 'sha256:abc', commit: 'abc123' }]);
  await expect(api.listFreights('app_2')).resolves.toMatchObject([{ id: 'freight_2', version: 'v2.0.0', image: 'registry.local/app:v2.0.0', digest: 'sha256:def', commit: '-' }]);
});

test('真实 API 分支查询流水线时同时加载代码源', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  const fetchMock = vi.fn(async (url: string) => {
    if (url.endsWith('/api/apps/app_1/build-pipelines?page=1&page_size=50')) {
      return new Response(JSON.stringify({ items: [{ id: 'pipeline_1', application_id: 'app_1', name: 'main', display_name: '主流水线', status: 'active', runtime_environments: [{ ID: 'runtime_env_java17', Name: 'java17', RuntimeBaseImage: 'registry.example/runtime/java17:1.0', ArtifactDeployPath: '/app/', DockerfilePath: 'java/jar/Dockerfile' }] }], total: 1, page: 1, page_size: 50 }), { status: 200 });
    }
    if (url.endsWith('/api/build-pipelines/pipeline_1/sources')) {
      return new Response(JSON.stringify({ items: [{ id: 'source_1', pipeline_id: 'pipeline_1', key: 'main', display_name: '主代码源', source_repository_id: 'repo_1', source_path: '.', build_spec: { source_path: '.', build_command: 'mvn clean package -DskipTests', artifact_copy_command: 'cp target/*.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"', runtime_base_image: 'registry.example/runtime/java17:1.0', artifact_deploy_path: '/app/', default_ref: 'main' } }] }), { status: 200 });
    }
    return new Response('', { status: 404 });
  });
  vi.stubGlobal('fetch', fetchMock);

  const api = await import('./index');
  await expect(api.listBuildPipelines('app_1')).resolves.toMatchObject([
    { id: 'pipeline_1', name: 'main', displayName: '主流水线', runtimeEnvironments: [{ id: 'runtime_env_java17', runtimeBaseImage: 'registry.example/runtime/java17:1.0', artifactDeployPath: '/app/' }], sources: [{ key: 'main', displayName: '主代码源', pipelineId: 'pipeline_1' }] }
  ]);
});

test('真实 API 更新流水线时过滤空运行时环境 ID', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  let patchBody: any;
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/api/build-pipelines/pipeline_1') && init?.method === 'PATCH') {
      patchBody = JSON.parse(String(init.body));
      return new Response(JSON.stringify({ id: 'pipeline_1', application_id: 'app_1', name: 'main', display_name: '主流水线', runtime_environments: [{ id: 'runtime_env_java17', name: 'java17' }] }), { status: 200 });
    }
    return new Response('', { status: 404 });
  }));

  const api = await import('./index');
  await api.updateBuildPipeline('pipeline_1', {
    displayName: '主流水线',
    runtimeEnvironmentIds: ['', 'runtime_env_java17', '  '],
    sources: [{
      id: 'source_1',
      applicationId: 'app_1',
      pipelineId: 'pipeline_1',
      key: 'main',
      displayName: '主代码源',
      sourceRepositoryId: 'repo_1',
      buildEnvironmentId: 'build_env_maven',
      sourcePath: '.',
      defaultRef: 'main',
      isPrimary: true,
      buildSpec: {
        sourcePath: '.',
        buildCommand: 'mvn clean package -DskipTests',
        artifactCopyCommand: 'cp target/*.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"',
        runtimeBaseImage: 'registry.example/runtime/java17:1.0',
        artifactDeployPath: '/app/',
        defaultRef: 'main'
      }
    }]
  });
  expect(patchBody.runtime_environment_ids).toEqual(['runtime_env_java17']);
});

test('真实 API 创建流水线提交绑定的 Workload ID', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  let postBody: any;
  vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
    if (url.endsWith('/api/apps/app_1/build-pipelines') && init?.method === 'POST') {
      postBody = JSON.parse(String(init.body));
      return new Response(JSON.stringify({ id: 'pipeline_1', application_id: 'app_1', workload_id: 'workload_api', name: 'main', display_name: '主流水线', status: 'active' }), { status: 201 });
    }
    return new Response('', { status: 404 });
  }));

  const api = await import('./index');
  await expect(api.createBuildPipeline('app_1', {
    workloadId: 'workload_api',
    name: 'main',
    displayName: '主流水线',
    runtimeEnvironmentIds: ['runtime_env_java17'],
    sources: [{
      id: 'source_1',
      applicationId: 'app_1',
      pipelineId: 'pipeline_1',
      key: 'main',
      displayName: '主代码源',
      sourceRepositoryId: 'repo_1',
      buildEnvironmentId: 'build_env_maven',
      sourcePath: '.',
      defaultRef: 'main',
      isPrimary: true,
      buildSpec: {
        sourcePath: '.',
        buildCommand: 'mvn clean package -DskipTests',
        artifactCopyCommand: 'cp target/*.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"',
        runtimeBaseImage: 'registry.example/runtime/java17:1.0',
        artifactDeployPath: '/app/',
        defaultRef: 'main'
      }
    }]
  })).resolves.toMatchObject({ id: 'pipeline_1', workloadId: 'workload_api' });
  expect(postBody.workload_id).toBe('workload_api');
});

test('streamBuildLog 使用 fetch 流式读取 SSE 并携带 token', async () => {
  vi.stubEnv('VITE_API_BASE_URL', 'https://paas.example');
  window.localStorage.setItem('paas_token', 'token_1');
  const encoder = new TextEncoder();
  const fetchMock = vi.fn(async (_url: string, init?: RequestInit) => {
    expect((init?.headers as Record<string, string>).Authorization).toBe('Bearer token_1');
    return new Response(new ReadableStream({
      start(controller) {
        controller.enqueue(encoder.encode('event: log\ndata: 第一行\n\n'));
        controller.enqueue(encoder.encode('event: status\ndata: succeeded\n\n'));
        controller.close();
      }
    }), { status: 200 });
  });
  vi.stubGlobal('fetch', fetchMock);
  const api = await import('./index');
  const logs: string[] = [];
  const statuses: string[] = [];
  api.streamBuildLog('build_1', (chunk) => logs.push(chunk), (status) => statuses.push(status));
  await vi.waitFor(() => expect(statuses).toContain('succeeded'));
  expect(logs).toEqual(['第一行']);
  expect(fetchMock).toHaveBeenCalledWith('https://paas.example/api/builds/build_1/logs/stream', expect.objectContaining({ signal: expect.any(AbortSignal) }));
});
