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
  const fetchMock = vi.fn(async (url: string) => {
    if (url.endsWith('/api/auth/local/login')) return new Response(JSON.stringify({ token: 'token_1', userName: '李雷' }), { status: 200 });
    if (url.endsWith('/api/auth/oidc/start')) return new Response(JSON.stringify({ redirect_url: 'https://idp.example/login' }), { status: 200 });
    if (url.endsWith('/api/projects?page=1&page_size=50')) return new Response(JSON.stringify({ items: [{ id: 'project_1' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
    if (url.endsWith('/api/applications?page=1&page_size=50')) return new Response(JSON.stringify({ items: [{ id: 'app_1' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
    if (url.endsWith('/api/builds?page=1&page_size=50')) return new Response(JSON.stringify({ items: [{ id: 'build_1' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
    if (url.endsWith('/api/builds/build_128/logs/stream')) return new Response('日志', { status: 200 });
    if (url.endsWith('/api/builds/build_129/logs/stream')) return new Response('新日志', { status: 200 });
    if (url.endsWith('/api/audit/logs?page=1&page_size=50')) return new Response(JSON.stringify({ items: [{ id: 'audit_1', actor_id: 'usr_1', action: 'promotion.approve', resource_type: 'promotion', resource_id: 'promotion_1', result: 'succeeded', summary: '审批通过', occurred_at: '2026-05-30T10:00:00Z' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
    if (url.endsWith('/api/apps/app_1/freights?page=1&page_size=50')) return new Response(JSON.stringify({ items: [{ id: 'freight_1', name: 'v1.0.0', image_uri: 'registry.local/app:v1.0.0', image_digest: 'sha256:abc', commit_sha: 'abc123', created_at: '2026-05-30T10:00:00Z' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
    if (url.endsWith('/api/apps/app_2/freights?page=1&page_size=50')) return new Response(JSON.stringify({ items: [{ id: 'freight_2', name: 'v2.0.0', uri: 'registry.local/app:v2.0.0', digest: 'sha256:def', created_at: '2026-05-31T10:00:00Z' }], total: 1, page: 1, page_size: 50 }), { status: 200 });
    return new Response('', { status: 404 });
  });
  vi.stubGlobal('fetch', fetchMock);
  const api = await import('./index');
  await expect(api.login('admin', 'password')).resolves.toEqual({ token: 'token_1', userName: '李雷' });
  await expect(api.oidcLoginURL()).resolves.toBe('https://idp.example/login');
  await expect(api.listProjects()).resolves.toEqual([{ id: 'project_1' }]);
  await expect(api.listApplications()).resolves.toEqual([{ id: 'app_1' }]);
  await expect(api.listBuilds()).resolves.toEqual([{ id: 'build_1' }]);
  await expect(api.buildLog()).resolves.toBe('日志');
  await expect(api.buildLog('build_129')).resolves.toBe('新日志');
  await expect(api.listAuditLogs()).resolves.toMatchObject([{ id: 'audit_1', actor: 'usr_1', action: 'promotion.approve', resource: 'promotion promotion_1', result: '成功', summary: '审批通过' }]);
  await expect(api.listFreights()).resolves.toMatchObject([{ id: 'freight_1', version: 'v1.0.0', image: 'registry.local/app:v1.0.0', digest: 'sha256:abc', commit: 'abc123' }]);
  await expect(api.listFreights('app_2')).resolves.toMatchObject([{ id: 'freight_2', version: 'v2.0.0', image: 'registry.local/app:v2.0.0', digest: 'sha256:def', commit: '-' }]);
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
