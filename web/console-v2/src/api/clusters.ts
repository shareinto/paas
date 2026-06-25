import { APIError, actorBody, actorQuery, hasAPIBaseURL, request, type PageResult } from './client';
import { platformContexts } from '../data/mock';

export type ClusterStatus = 'ready' | 'degraded' | 'unreachable' | 'draining' | 'disabled';

export type Cluster = {
  id: string;
  tenantId: string;
  name: string;
  region: string;
  labels: Record<string, string>;
  serverVersion: string;
  status: ClusterStatus;
  lastHeartbeatAt?: string;
  updatedAt?: string;
};

export type ClusterResult = {
  source: 'api' | 'mock';
  clusters: Cluster[];
  error?: string;
};

type BackendCluster = {
  id: string;
  tenant_id?: string;
  tenantId?: string;
  name?: string;
  region?: string;
  labels?: Record<string, string>;
  server_version?: string;
  serverVersion?: string;
  status?: ClusterStatus;
  last_heartbeat_at?: string;
  lastHeartbeatAt?: string;
  updated_at?: string;
  updatedAt?: string;
};

const mockClusters: Cluster[] = [
  {
    id: 'cluster-shanghai-dev',
    tenantId: 'tenant-retail',
    name: 'shanghai-dev-01',
    region: '华东-上海',
    labels: { env: 'dev', pool: 'shared' },
    serverVersion: 'v1.30.2',
    status: 'ready',
    lastHeartbeatAt: '2 分钟前',
    updatedAt: '今天 10:42'
  },
  {
    id: 'cluster-shanghai-test',
    tenantId: 'tenant-retail',
    name: 'shanghai-test-01',
    region: '华东-上海',
    labels: { env: 'test', pool: 'shared' },
    serverVersion: 'v1.30.2',
    status: 'ready',
    lastHeartbeatAt: '1 分钟前',
    updatedAt: '今天 10:43'
  },
  {
    id: 'cluster-shanghai-prod-01',
    tenantId: 'tenant-retail',
    name: 'shanghai-prod-01',
    region: '华东-上海',
    labels: { env: 'prod', zone: 'a' },
    serverVersion: 'v1.29.6',
    status: 'degraded',
    lastHeartbeatAt: '8 分钟前',
    updatedAt: '今天 10:31'
  },
  {
    id: 'cluster-platform-observe',
    tenantId: 'tenant-platform',
    name: 'platform-observe-01',
    region: '华东-杭州',
    labels: { env: 'prod', domain: 'observability' },
    serverVersion: 'v1.30.1',
    status: 'ready',
    lastHeartbeatAt: '刚刚',
    updatedAt: '今天 10:45'
  }
];

export async function loadClusters(tenantId: string): Promise<ClusterResult> {
  if (!hasAPIBaseURL()) {
    return { source: 'mock', clusters: mockClustersForTenant(tenantId) };
  }

  try {
    const data = await request<PageResult<BackendCluster>>(`/api/clusters?tenant_id=${encodeURIComponent(tenantId)}&page=1&page_size=100&${actorQuery()}`);
    return { source: 'api', clusters: data.items.map(mapCluster) };
  } catch (error) {
    const message =
      error instanceof APIError
        ? `后端集群接口请求失败（${error.status || '未知状态'}）：${error.message}，已回退 Mock 数据`
        : error instanceof Error
          ? `${error.message}，已回退 Mock 数据`
          : '集群列表加载失败，已回退 Mock 数据';
    return {
      source: 'mock',
      clusters: mockClustersForTenant(tenantId),
      error: message
    };
  }
}

export async function updateCluster(clusterId: string, input: Pick<Cluster, 'name' | 'region' | 'labels'>) {
  if (!hasAPIBaseURL()) return;
  await request(`/api/clusters/${encodeURIComponent(clusterId)}`, {
    method: 'PATCH',
    body: JSON.stringify({ actor: actorBody(), ...input })
  });
}

export async function updateClusterAction(clusterId: string, action: 'disable' | 'drain' | 'rotate-token') {
  if (!hasAPIBaseURL()) return undefined;
  return request(`/api/clusters/${encodeURIComponent(clusterId)}/${action}`, { method: 'POST', body: JSON.stringify({ actor: actorBody() }) });
}

function mockClustersForTenant(tenantId: string) {
  const known = mockClusters.filter((cluster) => cluster.tenantId === tenantId);
  if (known.length) return known;
  const tenant = platformContexts.find((item) => item.id === tenantId);
  return tenant ? [] : mockClusters;
}

function mapCluster(cluster: BackendCluster): Cluster {
  return {
    id: cluster.id,
    tenantId: cluster.tenant_id || cluster.tenantId || '',
    name: cluster.name || cluster.id,
    region: cluster.region || '-',
    labels: cluster.labels || {},
    serverVersion: cluster.server_version || cluster.serverVersion || '-',
    status: cluster.status || 'ready',
    lastHeartbeatAt: formatTime(cluster.last_heartbeat_at || cluster.lastHeartbeatAt),
    updatedAt: formatTime(cluster.updated_at || cluster.updatedAt)
  };
}

function formatTime(value: string | undefined) {
  if (!value) return undefined;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString('zh-CN', { hour12: false });
}
