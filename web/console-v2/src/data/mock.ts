import { Activity, Box, GitBranch, Rocket, ShieldCheck, Workflow } from 'lucide-react';

export type Status = 'healthy' | 'warning' | 'danger' | 'running' | 'pending';

export const platformContexts = [
  {
    id: 'tenant-retail',
    name: '零售事业群',
    projects: [
      {
        id: 'project-order',
        name: '订单平台',
        applications: [
          {
            id: 'app-order-platform',
            name: '订单服务',
            code: 'order-platform',
            owner: '交付中台组',
            workloads: [
              { id: 'order-api', name: '订单 API', kind: 'Deployment', pipelineId: 'pipe-order-api' },
              { id: 'order-web', name: '订单前端', kind: 'Deployment', pipelineId: 'pipe-order-web' },
              { id: 'order-worker', name: '订单任务', kind: 'StatefulSet', pipelineId: 'pipe-order-worker' }
            ]
          },
          {
            id: 'app-inventory',
            name: '库存服务',
            code: 'inventory-service',
            owner: '供应链研发组',
            workloads: [
              { id: 'inventory-api', name: '库存 API', kind: 'Deployment', pipelineId: 'pipe-inventory-api' },
              { id: 'inventory-worker', name: '库存任务', kind: 'StatefulSet', pipelineId: 'pipe-inventory-worker' }
            ]
          }
        ]
      },
      {
        id: 'project-payment',
        name: '支付平台',
        applications: [
          {
            id: 'app-payment-gateway',
            name: '支付网关',
            code: 'payment-gateway',
            owner: '支付研发组',
            workloads: [
              { id: 'payment-api', name: '支付 API', kind: 'Deployment', pipelineId: 'pipe-payment-api' },
              { id: 'payment-reconcile', name: '对账任务', kind: 'StatefulSet', pipelineId: 'pipe-payment-reconcile' }
            ]
          }
        ]
      }
    ]
  },
  {
    id: 'tenant-platform',
    name: '平台工程部',
    projects: [
      {
        id: 'project-observability',
        name: '可观测平台',
        applications: [
          {
            id: 'app-metrics',
            name: '指标服务',
            code: 'metrics-hub',
            owner: '平台 SRE',
            workloads: [
              { id: 'metrics-api', name: '指标 API', kind: 'Deployment', pipelineId: 'pipe-metrics-api' },
              { id: 'metrics-collector', name: '采集器', kind: 'StatefulSet', pipelineId: 'pipe-metrics-collector' }
            ]
          }
        ]
      }
    ]
  }
];

export const overviewStats = [
  { label: '运行应用', value: '28', delta: '+3', note: '本周新增', icon: Box },
  { label: '今日构建', value: '146', delta: '92%', note: '成功率', icon: Workflow },
  { label: '部署中', value: '7', delta: '3', note: '等待审批', icon: Rocket },
  { label: '集群健康', value: '4/4', delta: '0', note: '严重告警', icon: ShieldCheck }
];

export const applications = [
  { name: 'order-api', displayName: '订单服务', project: '订单平台', env: 'prod', status: 'healthy' as Status, version: '20260622.4', owner: '李雷', updatedAt: '10 分钟前' },
  { name: 'order-web', displayName: '订单前端', project: '订单平台', env: 'stage', status: 'running' as Status, version: '20260622.2', owner: '王芳', updatedAt: '18 分钟前' },
  { name: 'payment-gateway', displayName: '支付网关', project: '支付平台', env: 'prod', status: 'warning' as Status, version: '20260621.9', owner: '赵敏', updatedAt: '34 分钟前' },
  { name: 'inventory-worker', displayName: '库存任务', project: '供应链', env: 'dev', status: 'healthy' as Status, version: '20260620.1', owner: '韩梅', updatedAt: '1 小时前' }
];

export const activities = [
  { title: '订单服务完成生产部署', desc: 'freight-20260622.4 已晋级 prod', time: '10:42', status: 'healthy' as Status },
  { title: '支付网关健康检查波动', desc: '上海集群 Ready 2/3，等待自动恢复', time: '10:18', status: 'warning' as Status },
  { title: '订单前端触发构建', desc: 'main @ 8c1a09f，正在执行镜像构建', time: '09:56', status: 'running' as Status },
  { title: '供应链项目新增成员', desc: '添加开发者角色：chenyu', time: '09:20', status: 'healthy' as Status }
];

export const pipelines = [
  {
    id: 'pipe-order-api',
    name: '主流水线',
    applicationId: 'app-order-platform',
    workloadId: 'order-api',
    workloadName: '订单 API',
    workloadKind: 'Deployment',
    app: '订单服务',
    branch: 'main',
    commit: '8c1a09f',
    status: 'running' as Status,
    duration: '2 分 18 秒',
    progress: 68,
    stages: [
      { name: '拉取代码', status: 'healthy' as Status, duration: '12 秒' },
      { name: '单元测试', status: 'healthy' as Status, duration: '46 秒' },
      { name: '构建镜像', status: 'running' as Status, duration: '进行中' },
      { name: '推送制品', status: 'pending' as Status, duration: '-' }
    ]
  },
  {
    id: 'pipe-order-web',
    name: '前端发布流水线',
    applicationId: 'app-order-platform',
    workloadId: 'order-web',
    workloadName: '订单前端',
    workloadKind: 'Deployment',
    app: '订单前端',
    branch: 'release/2026-06',
    commit: 'a11b22c',
    status: 'healthy' as Status,
    duration: '4 分 03 秒',
    progress: 100,
    stages: [
      { name: '依赖安装', status: 'healthy' as Status, duration: '58 秒' },
      { name: '类型检查', status: 'healthy' as Status, duration: '34 秒' },
      { name: '构建静态资源', status: 'healthy' as Status, duration: '1 分 42 秒' },
      { name: '归档制品', status: 'healthy' as Status, duration: '49 秒' }
    ]
  },
  {
    id: 'pipe-order-worker',
    name: '异步任务流水线',
    applicationId: 'app-order-platform',
    workloadId: 'order-worker',
    workloadName: '订单任务',
    workloadKind: 'StatefulSet',
    app: '订单任务',
    branch: 'main',
    commit: '73ef021',
    status: 'healthy' as Status,
    duration: '3 分 12 秒',
    progress: 100,
    stages: [
      { name: '拉取代码', status: 'healthy' as Status, duration: '10 秒' },
      { name: '单元测试', status: 'healthy' as Status, duration: '39 秒' },
      { name: '构建镜像', status: 'healthy' as Status, duration: '1 分 31 秒' },
      { name: '推送制品', status: 'healthy' as Status, duration: '52 秒' }
    ]
  },
  {
    id: 'pipe-payment',
    name: '安全加固流水线',
    applicationId: 'app-payment-gateway',
    workloadId: 'payment-api',
    workloadName: '支付 API',
    workloadKind: 'Deployment',
    app: '支付网关',
    branch: 'main',
    commit: '61b9120',
    status: 'danger' as Status,
    duration: '1 分 51 秒',
    progress: 55,
    stages: [
      { name: '拉取代码', status: 'healthy' as Status, duration: '11 秒' },
      { name: '依赖扫描', status: 'danger' as Status, duration: '1 分 20 秒' },
      { name: '构建镜像', status: 'pending' as Status, duration: '-' },
      { name: '推送制品', status: 'pending' as Status, duration: '-' }
    ]
  }
];

function podDetails(prefix: string, image: string, count: number, status: Status = 'healthy') {
  return Array.from({ length: count }, (_, index) => {
    const ordinal = index + 1;
    const isWaiting = status === 'running' && index === count - 1;
    return {
      resourceId: `${prefix}-${String(ordinal).padStart(2, '0')}`,
      namespace: prefix.includes('prod') ? 'prod' : prefix.includes('pre') ? 'pre' : prefix.includes('test') ? 'test' : 'dev',
      containerName: 'app',
      name: `${prefix}-${String(ordinal).padStart(2, '0')}`,
      status: isWaiting ? 'running' as Status : status,
      ready: isWaiting ? '0/1' : '1/1',
      restarts: isWaiting ? 1 : 0,
      node: `node-${ordinal}.shanghai.internal`,
      podIp: `10.${20 + ordinal}.${index + 4}.${120 + ordinal}`,
      image,
      cpu: isWaiting ? '24m' : `${80 + index * 26}m`,
      memory: isWaiting ? '96Mi' : `${180 + index * 40}Mi`,
      age: isWaiting ? '2 分钟' : `${18 + index * 7} 分钟`
    };
  });
}

export const freights = [
  {
    id: 'freight-20260623.1',
    name: '20260623-112458',
    createdAt: '2026-06-23 11:24:58',
    source: '流水线产出',
    commit: '2f91c7d',
    currentStages: [],
    eligibleStages: ['dev'],
    workloads: [
      {
        name: 'order-api',
        displayName: '订单 API',
        pipeline: '主流水线',
        version: '20260623.1',
        image: 'registry.local/order-api:20260623.1',
        digest: 'sha256:2f91c7d',
        status: 'healthy' as Status,
        containers: [
          { name: 'app', pipeline: '主流水线', version: '20260623.1', image: 'registry.local/order-api:20260623.1', digest: 'sha256:2f91c7d', status: 'healthy' as Status },
          { name: 'metrics', pipeline: '自定义镜像', version: '自定义', image: 'registry.local/order-api-metrics:1.0.0', digest: '-', status: 'healthy' as Status }
        ]
      },
      { name: 'order-web', displayName: '订单前端', pipeline: '前端发布流水线', version: '20260623.1', image: 'registry.local/order-web:20260623.1', digest: 'sha256:af6029e', status: 'healthy' as Status },
      { name: 'order-worker', displayName: '订单任务', pipeline: '异步任务流水线', version: '20260623.1', image: 'registry.local/order-worker:20260623.1', digest: 'sha256:cc81e20', status: 'healthy' as Status }
    ]
  },
  {
    id: 'freight-20260622.4',
    name: '20260622-102400',
    createdAt: '2026-06-22 10:24:00',
    source: '手动创建',
    commit: '8c1a09f',
    currentStages: ['dev', 'test', 'pre'],
    eligibleStages: ['prod-canary'],
    workloads: [
      { name: 'order-api', displayName: '订单 API', pipeline: '主流水线', version: '20260622.4', image: 'registry.local/order-api:20260622.4', digest: 'sha256:8c1a09f', status: 'running' as Status },
      { name: 'order-web', displayName: '订单前端', pipeline: '前端发布流水线', version: '20260622.2', image: 'registry.local/order-web:20260622.2', digest: 'sha256:a11b22c', status: 'healthy' as Status },
      { name: 'order-worker', displayName: '订单任务', pipeline: '异步任务流水线', version: '20260622.1', image: 'registry.local/order-worker:20260622.1', digest: 'sha256:73ef021', status: 'healthy' as Status }
    ]
  },
  {
    id: 'freight-20260621.8',
    name: '20260621-184200',
    createdAt: '2026-06-21 18:42:00',
    source: '回滚候选',
    commit: '61b9120',
    currentStages: ['prod-canary', 'prod'],
    eligibleStages: [],
    workloads: [
      { name: 'order-api', displayName: '订单 API', pipeline: '主流水线', version: '20260621.8', image: 'registry.local/order-api:20260621.8', digest: 'sha256:61b9120', status: 'healthy' as Status },
      { name: 'order-web', displayName: '订单前端', pipeline: '前端发布流水线', version: '20260621.6', image: 'registry.local/order-web:20260621.6', digest: 'sha256:b40d19a', status: 'healthy' as Status },
      { name: 'order-worker', displayName: '订单任务', pipeline: '异步任务流水线', version: '20260621.3', image: 'registry.local/order-worker:20260621.3', digest: 'sha256:ff38d91', status: 'healthy' as Status }
    ]
  },
  {
    id: 'freight-20260620.5',
    name: '20260620-173600',
    createdAt: '2026-06-20 17:36:00',
    source: '自定义镜像',
    commit: '24f6ab2',
    currentStages: ['dev'],
    eligibleStages: ['test'],
    workloads: [
      { name: 'order-api', displayName: '订单 API', pipeline: '主流水线', version: '20260620.5', image: 'registry.local/order-api:20260620.5', digest: 'sha256:24f6ab2', status: 'healthy' as Status },
      { name: 'order-web', displayName: '订单前端', pipeline: '前端发布流水线', version: '20260620.3', image: 'registry.local/order-web:20260620.3', digest: 'sha256:ab332c0', status: 'healthy' as Status },
      { name: 'order-worker', displayName: '订单任务', pipeline: '异步任务流水线', version: '20260620.2', image: 'registry.local/order-worker:20260620.2', digest: 'sha256:9ef17bd', status: 'healthy' as Status }
    ]
  }
];

export type ContainerImageSource = {
  mode: 'pipeline' | 'custom';
  pipelineId?: string;
  customImage?: string;
};

export type WorkloadProbeConfig = {
  enabled?: boolean;
  probeType?: 'http' | 'tcp';
  path: string;
  port?: number;
  initialDelaySeconds?: number;
  periodSeconds?: number;
  timeoutSeconds?: number;
  failureThreshold?: number;
  successThreshold?: number;
};

export type WorkloadContainerConfig = {
  id: string;
  name: string;
  imageSource: ContainerImageSource;
  port: number;
  cpu: string;
  memory: string;
  limitCpu?: string;
  limitMemory?: string;
  command?: string;
  livenessProbe?: WorkloadProbeConfig;
  readinessProbe?: WorkloadProbeConfig;
  startupProbe?: WorkloadProbeConfig;
  envVars?: WorkloadKeyValueConfig[];
  secretRefs?: WorkloadSecretRefConfig[];
  configFiles?: WorkloadConfigFileConfig[];
  writableDirs?: WorkloadWritableDirConfig[];
  nasMount?: WorkloadNasMountConfig;
};

export type WorkloadNasMountConfig = {
  enabled?: boolean;
  nasPath?: string;
  mountPath?: string;
};

export type WorkloadKeyValueConfig = {
  id?: string;
  name: string;
  value: string;
};

export type WorkloadSecretRefConfig = {
  id?: string;
  name: string;
  secretRef: string;
};

export type WorkloadConfigFileConfig = {
  id?: string;
  mountPath: string;
  content: string;
  base64Encoded?: boolean;
};

export type WorkloadWritableDirConfig = {
  id?: string;
  mountPath: string;
  ownerGroup?: string;
  mode?: string;
  sizeLimit?: string;
};

export type VersionSourceWorkloadConfig = {
  id: string;
  name: string;
  kind: 'Deployment' | 'StatefulSet';
  replicas: number;
  serviceType?: 'None' | 'ClusterIP' | 'NodePort' | 'LoadBalancer';
  servicePort: number;
  enableDomainAccess?: boolean;
  serverName?: string;
  domain?: string;
  ingressPath?: string;
  ingressRewrite?: boolean;
  ingressRewritePath?: string;
  ingressTls?: boolean;
  ingressTlsRedirect?: boolean;
  probePath?: string;
  probePort?: number;
  livenessInitialDelaySeconds?: number;
  readinessInitialDelaySeconds?: number;
  probePeriodSeconds?: number;
  probeTimeoutSeconds?: number;
  terminationGracePeriodSeconds?: number;
  nodeType?: 'general' | 'network' | 'memory' | 'compute';
  exclusive?: boolean;
  envVars?: WorkloadKeyValueConfig[];
  secretRefs?: WorkloadSecretRefConfig[];
  configFiles?: WorkloadConfigFileConfig[];
  writableDirs?: WorkloadWritableDirConfig[];
  containers: WorkloadContainerConfig[];
};

export type VersionSourcePipeline = {
  id: string;
  name: string;
  description?: string;
  branch: string;
  runtime: string;
  runtimeEnvironmentIds?: string[];
  sourcePath: string;
  buildCommand: string;
  artifactCopyCommand: string;
  sources: Array<{
    id: string;
    key: string;
    name: string;
    sourceType?: 'git' | 'svn';
    repository: string;
    sourceUrl?: string;
    sourceRef?: string;
    svnRevision?: string;
    branch: string;
    sourcePath: string;
    buildEnvironment: string;
    buildEnvironmentId?: string;
    buildCommand: string;
    artifactCopyCommand: string;
    runtimeBaseImage?: string;
    artifactDeployPath?: string;
  }>;
  buildHistory: Array<{
    id: string;
    branch: string;
    status: Status;
    version: string;
    startedAt: string;
    duration: string;
  }>;
  logs: string[];
  latestVersion: string;
  status: Status;
};

export const versionSourcePipelines: VersionSourcePipeline[] = [
  {
    id: 'pipe-order-api',
    name: '订单 API 主流水线',
    branch: 'main',
    runtime: 'Java 17 / Maven',
    sourcePath: 'services/order-api',
    buildCommand: 'mvn clean package -DskipTests',
    artifactCopyCommand: 'cp -ar target/order-api.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"',
    sources: [
      {
        id: 'src-order-api-main',
        key: 'main',
        name: '主代码源',
        repository: 'gitlab.internal/retail/order-platform',
        branch: 'main',
        sourcePath: 'services/order-api',
        buildEnvironment: 'Maven JDK17 构建环境',
        buildCommand: 'mvn clean package -DskipTests',
        artifactCopyCommand: 'cp -ar target/order-api.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"'
      },
      {
        id: 'src-order-api-contract',
        key: 'contract',
        name: '契约定义',
        repository: 'gitlab.internal/retail/order-contracts',
        branch: 'main',
        sourcePath: 'proto/order',
        buildEnvironment: 'Maven JDK17 构建环境',
        buildCommand: 'mvn -pl contracts test',
        artifactCopyCommand: 'cp -ar target/generated "$PAAS_ARTIFACT_OUTPUT/contracts"'
      }
    ],
    buildHistory: [
      { id: 'build-order-api-1024', branch: 'main', status: 'healthy', version: '20260623.1', startedAt: '10:12', duration: '3 分 42 秒' },
      { id: 'build-order-api-1023', branch: 'main', status: 'healthy', version: '20260622.4', startedAt: '昨天 18:31', duration: '3 分 56 秒' },
      { id: 'build-order-api-1022', branch: 'feature/coupon', status: 'warning', version: '20260622.3', startedAt: '昨天 15:08', duration: '4 分 18 秒' }
    ],
    logs: [
      '[10:12:01] 拉取代码 gitlab.internal/retail/order-platform main',
      '[10:12:18] 执行构建命令 mvn clean package -DskipTests',
      '[10:14:42] 单元测试通过 286 个用例',
      '[10:15:19] 收集产物 target/order-api.jar',
      '[10:15:43] 回写镜像候选 registry.local/order-api:20260623.1'
    ],
    latestVersion: '20260623.1',
    status: 'healthy'
  },
  {
    id: 'pipe-order-web',
    name: '订单前端流水线',
    branch: 'release/2026-06',
    runtime: 'Node 22 / pnpm',
    sourcePath: 'web/order',
    buildCommand: 'pnpm install && pnpm build',
    artifactCopyCommand: 'cp -ar dist "$PAAS_ARTIFACT_OUTPUT/dist"',
    sources: [
      {
        id: 'src-order-web-main',
        key: 'main',
        name: '主代码源',
        repository: 'gitlab.internal/retail/order-frontend',
        branch: 'release/2026-06',
        sourcePath: 'web/order',
        buildEnvironment: 'Node 22 构建环境',
        buildCommand: 'pnpm install && pnpm build',
        artifactCopyCommand: 'cp -ar dist "$PAAS_ARTIFACT_OUTPUT/dist"'
      }
    ],
    buildHistory: [
      { id: 'build-order-web-815', branch: 'release/2026-06', status: 'healthy', version: '20260623.1', startedAt: '09:46', duration: '4 分 03 秒' },
      { id: 'build-order-web-814', branch: 'release/2026-06', status: 'healthy', version: '20260622.2', startedAt: '昨天 17:08', duration: '4 分 11 秒' }
    ],
    logs: [
      '[09:46:03] pnpm install',
      '[09:47:12] pnpm build',
      '[09:49:41] 产物目录 dist 已归档',
      '[09:50:06] 回写镜像候选 registry.local/order-web:20260623.1'
    ],
    latestVersion: '20260623.1',
    status: 'healthy'
  },
  {
    id: 'pipe-order-worker',
    name: '订单任务流水线',
    branch: 'main',
    runtime: 'Java 17 / Gradle',
    sourcePath: 'workers/order-worker',
    buildCommand: './gradlew clean build -x test',
    artifactCopyCommand: 'cp -ar build/libs/order-worker.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"',
    sources: [
      {
        id: 'src-order-worker-main',
        key: 'main',
        name: '主代码源',
        repository: 'gitlab.internal/retail/order-platform',
        branch: 'main',
        sourcePath: 'workers/order-worker',
        buildEnvironment: 'Gradle JDK17 构建环境',
        buildCommand: './gradlew clean build -x test',
        artifactCopyCommand: 'cp -ar build/libs/order-worker.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"'
      }
    ],
    buildHistory: [
      { id: 'build-order-worker-477', branch: 'main', status: 'running', version: '20260623.1', startedAt: '10:36', duration: '进行中' },
      { id: 'build-order-worker-476', branch: 'main', status: 'healthy', version: '20260622.1', startedAt: '昨天 16:22', duration: '3 分 12 秒' }
    ],
    logs: [
      '[10:36:02] 拉取代码 gitlab.internal/retail/order-platform main',
      '[10:36:24] 执行构建命令 ./gradlew clean build -x test',
      '[10:38:19] 正在执行集成测试',
      '[10:39:14] 等待回写镜像候选'
    ],
    latestVersion: '20260622.1',
    status: 'running'
  }
];

export const versionSourceConfig = {
  updatedAt: '10:58',
  freightSerial: 24,
  workloads: [
    {
      id: 'order-api',
      name: '订单 API',
      kind: 'Deployment',
      replicas: 2,
      servicePort: 8080,
      serverName: '订单 API',
      probePath: '/actuator/health',
      containers: [
        {
          id: 'api-main',
          name: 'app',
          imageSource: { mode: 'pipeline', pipelineId: 'pipe-order-api' },
          port: 8080,
          cpu: '500m',
          memory: '768Mi'
        },
        {
          id: 'api-metrics',
          name: 'metrics',
          imageSource: { mode: 'custom', customImage: 'registry.local/order-api-metrics:1.0.0' },
          port: 9100,
          cpu: '100m',
          memory: '128Mi'
        }
      ]
    },
    {
      id: 'order-web',
      name: '订单前端',
      kind: 'Deployment',
      replicas: 2,
      servicePort: 80,
      serverName: '订单前端',
      probePath: '/healthz',
      containers: [
        {
          id: 'web-main',
          name: 'web',
          imageSource: { mode: 'pipeline', pipelineId: 'pipe-order-web' },
          port: 80,
          cpu: '200m',
          memory: '256Mi'
        }
      ]
    },
    {
      id: 'order-worker',
      name: '订单任务',
      kind: 'StatefulSet',
      replicas: 1,
      servicePort: 9090,
      serverName: '订单 Worker',
      probePath: '/readyz',
      containers: [
        {
          id: 'worker-main',
          name: 'worker',
          imageSource: { mode: 'pipeline', pipelineId: 'pipe-order-worker' },
          port: 9090,
          cpu: '300m',
          memory: '512Mi'
        }
      ]
    }
  ] satisfies VersionSourceWorkloadConfig[]
};

export const deliveryTopology = {
  tenantId: 'tenant-retail',
  tenantName: '零售事业群',
  applicationId: 'app-order-platform',
  applicationName: '订单平台',
  topologyVersion: 'topology-20260623-03',
  generatedAt: '10:48',
  stages: [
    {
      id: 'dev',
      key: 'dev',
      name: '开发联调',
      colorToken: 'geek-blue',
      x: 11,
      y: 38,
      row: 1,
      col: 0,
      lane: 0,
      freightId: 'freight-20260622.4',
      promotionPolicy: 'auto',
      requiresVerification: false,
      verificationStatus: '' as string,
      configOutdated: false,
      clusterBindingId: 'bind-dev-shanghai',
      cluster: 'shanghai-dev-01',
      namespace: 'order-dev',
      sync: '已同步',
      syncStatus: 'Synced',
      healthStatus: 'Healthy',
      status: 'healthy' as Status,
      progress: 100,
      configurableWorkloadIds: ['order-api', 'order-web', 'order-worker'],
      checks: ['自动准入通过', '配置继承默认值'],
      workloads: [
        { name: 'order-api', displayName: '订单 API', kind: 'Deployment', image: '20260622.4', replicas: '2/2', pods: '2 Running', cpu: '410m', memory: '620Mi', status: 'healthy' as Status, podDetails: podDetails('order-api-dev', '20260622.4', 2) },
        { name: 'order-web', displayName: '订单前端', kind: 'Deployment', image: '20260622.2', replicas: '2/2', pods: '2 Running', cpu: '180m', memory: '280Mi', status: 'healthy' as Status, podDetails: podDetails('order-web-dev', '20260622.2', 2) },
        { name: 'order-worker', displayName: '订单任务', kind: 'StatefulSet', image: '20260622.1', replicas: '1/1', pods: '1 Running', cpu: '120m', memory: '256Mi', status: 'healthy' as Status, podDetails: podDetails('order-worker-dev', '20260622.1', 1) }
      ]
    },
    {
      id: 'test',
      key: 'test',
      name: '集成测试',
      colorToken: 'mint-green',
      x: 33,
      y: 38,
      row: 1,
      col: 1,
      lane: 1,
      freightId: 'freight-20260622.4',
      promotionPolicy: 'manual',
      requiresVerification: false,
      configOutdated: false,
      clusterBindingId: 'bind-test-shanghai',
      cluster: 'shanghai-test-01',
      namespace: 'order-test',
      sync: '已同步',
      syncStatus: 'Synced',
      healthStatus: 'Healthy',
      status: 'healthy' as Status,
      progress: 100,
      configurableWorkloadIds: ['order-api', 'order-web', 'order-worker'],
      checks: ['上游 Stage 已通过', '集成测试通过'],
      workloads: [
        { name: 'order-api', displayName: '订单 API', kind: 'Deployment', image: '20260622.4', replicas: '2/2', pods: '2 Running', cpu: '520m', memory: '760Mi', status: 'healthy' as Status, podDetails: podDetails('order-api-test', '20260622.4', 2) },
        { name: 'order-web', displayName: '订单前端', kind: 'Deployment', image: '20260622.2', replicas: '2/2', pods: '2 Running', cpu: '220m', memory: '310Mi', status: 'healthy' as Status, podDetails: podDetails('order-web-test', '20260622.2', 2) },
        { name: 'order-worker', displayName: '订单任务', kind: 'StatefulSet', image: '20260622.1', replicas: '1/1', pods: '1 Running', cpu: '160m', memory: '280Mi', status: 'healthy' as Status, podDetails: podDetails('order-worker-test', '20260622.1', 1) }
      ]
    },
    {
      id: 'pre',
      key: 'pre',
      name: '预发验证',
      colorToken: 'lemon-yellow',
      x: 55,
      y: 38,
      row: 1,
      col: 2,
      lane: 2,
      freightId: 'freight-20260622.4',
      promotionPolicy: 'approval_required',
      requiresVerification: true,
      configOutdated: false,
      clusterBindingId: 'bind-pre-shanghai',
      cluster: 'shanghai-pre-01',
      namespace: 'order-pre',
      sync: '等待人工验证',
      syncStatus: 'OutOfSync',
      healthStatus: 'Progressing',
      status: 'running' as Status,
      progress: 78,
      configurableWorkloadIds: ['order-api', 'order-web', 'order-worker'],
      checks: ['需要测试负责人验证', '灰度域名已生成'],
      workloads: [
        { name: 'order-api', displayName: '订单 API', kind: 'Deployment', image: '20260622.4', replicas: '3/3', pods: '3 Running', cpu: '720m', memory: '1.1Gi', status: 'healthy' as Status, podDetails: podDetails('order-api-pre', '20260622.4', 3) },
        { name: 'order-web', displayName: '订单前端', kind: 'Deployment', image: '20260622.2', replicas: '2/2', pods: '2 Running', cpu: '260m', memory: '390Mi', status: 'healthy' as Status, podDetails: podDetails('order-web-pre', '20260622.2', 2) },
        { name: 'order-worker', displayName: '订单任务', kind: 'StatefulSet', image: '20260622.1', replicas: '1/1', pods: '0/1 Running', cpu: '210m', memory: '360Mi', status: 'running' as Status, podDetails: podDetails('order-worker-pre', '20260622.1', 1, 'running') }
      ]
    },
    {
      id: 'prod-canary',
      key: 'prod-canary',
      name: '生产灰度',
      colorToken: 'amber-orange',
      x: 84,
      y: 24,
      row: 0,
      col: 3,
      lane: 3,
      freightId: 'freight-20260621.8',
      promotionPolicy: 'approval_required',
      requiresVerification: true,
      configOutdated: false,
      clusterBindingId: 'bind-prod-shanghai-a',
      cluster: 'shanghai-prod-01',
      namespace: 'order-prod-canary',
      sync: '等待晋级',
      syncStatus: 'OutOfSync',
      healthStatus: 'Unknown',
      status: 'pending' as Status,
      progress: 0,
      configurableWorkloadIds: ['order-api', 'order-web'],
      checks: ['生产审批未提交', '上游验证已通过'],
      workloads: [
        { name: 'order-api', displayName: '订单 API', kind: 'Deployment', image: '20260621.8', replicas: '1/1', pods: '1 Running', cpu: '460m', memory: '700Mi', status: 'healthy' as Status, podDetails: podDetails('order-api-canary', '20260621.8', 1) },
        { name: 'order-web', displayName: '订单前端', kind: 'Deployment', image: '20260621.6', replicas: '1/1', pods: '1 Running', cpu: '160m', memory: '260Mi', status: 'healthy' as Status, podDetails: podDetails('order-web-canary', '20260621.6', 1) },
        { name: 'order-worker', displayName: '订单任务', kind: 'StatefulSet', image: '20260621.3', replicas: '1/1', pods: '1 Running', cpu: '130m', memory: '250Mi', status: 'healthy' as Status, podDetails: podDetails('order-worker-canary', '20260621.3', 1) }
      ]
    },
    {
      id: 'prod',
      key: 'prod',
      name: '生产全量',
      colorToken: 'wine-purple',
      x: 84,
      y: 70,
      row: 2,
      col: 3,
      lane: 3,
      freightId: 'freight-20260621.8',
      promotionPolicy: 'approval_required',
      requiresVerification: true,
      configOutdated: false,
      clusterBindingId: 'bind-prod-shanghai-b',
      cluster: 'shanghai-prod-02',
      namespace: 'order-prod',
      sync: '已同步',
      syncStatus: 'Synced',
      healthStatus: 'Healthy',
      status: 'healthy' as Status,
      progress: 100,
      configurableWorkloadIds: ['order-api', 'order-web', 'order-worker'],
      checks: ['生产审批已完成', '回滚点已创建'],
      workloads: [
        { name: 'order-api', displayName: '订单 API', kind: 'Deployment', image: '20260621.8', replicas: '4/4', pods: '4 Running', cpu: '1.6', memory: '2.8Gi', status: 'healthy' as Status, podDetails: podDetails('order-api-prod', '20260621.8', 4) },
        { name: 'order-web', displayName: '订单前端', kind: 'Deployment', image: '20260621.6', replicas: '3/3', pods: '3 Running', cpu: '560m', memory: '920Mi', status: 'healthy' as Status, podDetails: podDetails('order-web-prod', '20260621.6', 3) },
        { name: 'order-worker', displayName: '订单任务', kind: 'StatefulSet', image: '20260621.3', replicas: '2/2', pods: '2 Running', cpu: '430m', memory: '720Mi', status: 'healthy' as Status, podDetails: podDetails('order-worker-prod', '20260621.3', 2) }
      ]
    }
  ],
  edges: [
    { fromStageId: 'dev', toStageId: 'test', rule: '自动准入' },
    { fromStageId: 'test', toStageId: 'pre', rule: '手动晋级' },
    { fromStageId: 'pre', toStageId: 'prod-canary', rule: '审批后灰度' },
    { fromStageId: 'pre', toStageId: 'prod', rule: '审批后全量' },
    { fromStageId: 'prod-canary', toStageId: 'prod', rule: '灰度验证通过' }
  ],
  stageConfigSchema: [
    { key: 'replicas', label: '实例数', value: '3', unit: '个' },
    { key: 'cpu', label: 'CPU Request', value: '750', unit: 'm' },
    { key: 'memory', label: '内存 Request', value: '1024', unit: 'Mi' },
    { key: 'domain', label: '访问域名', value: 'pre-order.internal.example.com', unit: '' },
    { key: 'probePath', label: '健康检查路径', value: '/readyz', unit: '' }
  ]
};

export type DeliveryTemplateStage = {
  id: string;
  stageKey: string;
  displayName: string;
  description: string;
  colorToken: string;
  order: number;
  layoutColumn: number;
  layoutRow: number;
  status: 'enabled' | 'disabled';
  requiresApproval: boolean;
  requiresVerification: boolean;
  approveRoles: string[];
  verifyRoles: string[];
  clusterBinding: {
    clusterId: string;
    clusterName: string;
    region: string;
    status: 'active' | 'empty' | 'disabled';
  } | null;
};

export type DeliveryTemplateEdge = {
  fromStageKey: string;
  toStageKey: string;
  rule: string;
};

export const clusterOptions = [
  { id: 'cluster-shanghai-dev', name: 'shanghai-dev-01', region: '华东-上海', labels: ['dev', 'shared'] },
  { id: 'cluster-shanghai-test', name: 'shanghai-test-01', region: '华东-上海', labels: ['test', 'shared'] },
  { id: 'cluster-shanghai-pre', name: 'shanghai-pre-01', region: '华东-上海', labels: ['pre', 'isolated'] },
  { id: 'cluster-shanghai-prod-a', name: 'shanghai-prod-01', region: '华东-上海', labels: ['prod', 'zone-a'] },
  { id: 'cluster-shanghai-prod-b', name: 'shanghai-prod-02', region: '华东-上海', labels: ['prod', 'zone-b'] },
  { id: 'cluster-hangzhou-dr', name: 'hangzhou-dr-01', region: '华东-杭州', labels: ['dr', 'standby'] }
];

export const deliveryFlowTemplates = [
  {
    id: 'delivery-template-retail',
    tenantId: 'tenant-retail',
    tenantName: '零售事业群',
    name: '零售标准交付流',
    version: 'template-20260623-02',
    updatedAt: '10:52',
    effectiveApps: 18,
    status: 'enabled' as const,
    stages: [
      {
        id: 'tmpl-dev',
        stageKey: 'dev',
        displayName: '开发联调',
        description: '开发与特性分支联调入口，允许自动准入。',
        colorToken: 'geek-blue',
        order: 1,
        layoutColumn: 0,
        layoutRow: 1,
        status: 'enabled' as const,
        requiresApproval: false,
        requiresVerification: false,
        approveRoles: [],
        verifyRoles: [],
        clusterBinding: { clusterId: 'cluster-shanghai-dev', clusterName: 'shanghai-dev-01', region: '华东-上海', status: 'active' as const }
      },
      {
        id: 'tmpl-test',
        stageKey: 'test',
        displayName: '集成测试',
        description: '自动化测试与集成环境，要求上游 Freight 全量通过。',
        colorToken: 'mint-green',
        order: 2,
        layoutColumn: 1,
        layoutRow: 1,
        status: 'enabled' as const,
        requiresApproval: false,
        requiresVerification: true,
        approveRoles: [],
        verifyRoles: ['developer', 'operator'],
        clusterBinding: { clusterId: 'cluster-shanghai-test', clusterName: 'shanghai-test-01', region: '华东-上海', status: 'active' as const }
      },
      {
        id: 'tmpl-pre',
        stageKey: 'pre',
        displayName: '预发验证',
        description: '准生产验证环境，允许按 Workload 覆盖实例数、域名和探针。',
        colorToken: 'lemon-yellow',
        order: 3,
        layoutColumn: 2,
        layoutRow: 1,
        status: 'enabled' as const,
        requiresApproval: true,
        requiresVerification: true,
        approveRoles: ['tenant_admin', 'operator'],
        verifyRoles: ['operator'],
        clusterBinding: { clusterId: 'cluster-shanghai-pre', clusterName: 'shanghai-pre-01', region: '华东-上海', status: 'active' as const }
      },
      {
        id: 'tmpl-prod-canary',
        stageKey: 'prod-canary',
        displayName: '生产灰度',
        description: '小流量生产灰度，仅允许生产审批人触发。',
        colorToken: 'amber-orange',
        order: 4,
        layoutColumn: 3,
        layoutRow: 0,
        status: 'enabled' as const,
        requiresApproval: true,
        requiresVerification: true,
        approveRoles: ['prod_approver'],
        verifyRoles: ['operator', 'prod_approver'],
        clusterBinding: { clusterId: 'cluster-shanghai-prod-a', clusterName: 'shanghai-prod-01', region: '华东-上海', status: 'active' as const }
      },
      {
        id: 'tmpl-prod',
        stageKey: 'prod',
        displayName: '生产全量',
        description: '核心生产环境，全量发布前必须完成灰度或预发验证。',
        colorToken: 'wine-purple',
        order: 5,
        layoutColumn: 3,
        layoutRow: 2,
        status: 'enabled' as const,
        requiresApproval: true,
        requiresVerification: false,
        approveRoles: ['prod_approver'],
        verifyRoles: [],
        clusterBinding: { clusterId: 'cluster-shanghai-prod-b', clusterName: 'shanghai-prod-02', region: '华东-上海', status: 'active' as const }
      }
    ] satisfies DeliveryTemplateStage[],
    edges: [
      { fromStageKey: 'dev', toStageKey: 'test', rule: '自动准入' },
      { fromStageKey: 'test', toStageKey: 'pre', rule: '人工晋级' },
      { fromStageKey: 'pre', toStageKey: 'prod-canary', rule: '审批后灰度' },
      { fromStageKey: 'pre', toStageKey: 'prod', rule: '审批后全量' },
      { fromStageKey: 'prod-canary', toStageKey: 'prod', rule: '灰度验证通过' }
    ] satisfies DeliveryTemplateEdge[]
  }
];

export const topology = [
  { label: 'GitLab', status: 'healthy' as Status, icon: GitBranch },
  { label: 'Jenkins', status: 'healthy' as Status, icon: Workflow },
  { label: 'Argo CD', status: 'running' as Status, icon: Activity },
  { label: 'PaaS Agent', status: 'healthy' as Status, icon: ShieldCheck }
];
