export type Tenant = { id: string; name: string; displayName: string; description?: string; updatedAt: string };
export type Project = { id: string; tenantId: string; name: string; displayName: string; description?: string; tenant: string; owner: string; updatedAt: string };
export type Application = { id: string; name: string; displayName: string; project: string; projectId?: string; description?: string; runtimeEnvironmentId?: string; runtimeEnvironments?: ApplicationRuntimeEnvironment[]; status?: string; type: string; envStatus: string; build: string; release: string; owner: string; updatedAt: string };
export type ApplicationRuntimeEnvironment = { id: string; name: string; runtimeBaseImage: string; artifactDeployPath?: string; dockerfilePath?: string };
export type ApplicationSource = { id?: string; key: string; displayName: string; sourceRepositoryId: string; jenkinsTemplateId?: string; buildEnvironmentId?: string; sourcePath: string; defaultRef: string; isPrimary: boolean; buildSpec: { sourcePath: string; buildCommand: string; artifactCopyCommand: string; runtimeBaseImage?: string; artifactDeployPath?: string; defaultRef: string } };
export type BuildPipelineSource = ApplicationSource & { pipelineId?: string };
export type BuildPipeline = { id: string; applicationId: string; name: string; displayName: string; description?: string; status: string; externalJobName?: string; runtimeEnvironments?: RuntimeEnvironment[]; sources?: BuildPipelineSource[]; updatedAt: string };
export type BuildRun = { id: string; application: string; pipeline?: string; pipelineId?: string; status: string; ref: string; commit: string; startedAt: string; duration: string };
export type AuditLog = { id: string; actor: string; action: string; resource: string; result: string; summary: string; time: string };
export type Freight = { id: string; version: string; image: string; digest: string; commit: string; createdAt: string };
export type SourceRepository = { id: string; projectId: string; projectName: string; name: string; displayName: string; description: string; gitProvider: string; httpUrl: string; sshUrl: string; defaultBranch: string; status: string; associatedApplications: number; updatedAt: string };
export type RepositoryBranch = { name: string; default: boolean };
export type RepositoryTreeItem = { name: string; path: string; type: 'tree' | 'blob' };
export type BuildSpecSuggestion = { sourcePath: string; buildCommand: string; artifactCopyCommand?: string; runtimeBaseImage: string; evidence: string[] };
export type JenkinsJobTemplate = { id: string; name: string; version: number; status: string; isDefault: boolean; jenkinsfileContent?: string; xmlContent?: string; updatedAt: string };
export type BuildType = JenkinsJobTemplate;
export type BuildEnvironment = { id: string; name: string; description?: string; buildImage?: string; status: string; isDefault: boolean; updatedAt: string };
export type RuntimeEnvironment = { id: string; name: string; description?: string; runtimeBaseImage: string; artifactDeployPath?: string; dockerfilePath?: string; status: string; isDefault: boolean; updatedAt: string };
export type BuildTemplate = { id: string; name: string; version: number; content: string; updatedAt: string };

const wait = (ms = 120) => new Promise((resolve) => window.setTimeout(resolve, ms));
const tenants: Tenant[] = [
  { id: 'tenant_1', name: 'rnd', displayName: '研发中心', description: '默认开发租户', updatedAt: '2026-05-30 09:30' }
];
const projects: Project[] = [
  { id: 'project_1', tenantId: 'tenant_1', name: 'order', displayName: '订单平台', description: '订单业务应用', tenant: '研发中心', owner: '李雷', updatedAt: '2026-05-30 09:30' },
  { id: 'project_2', tenantId: 'tenant_1', name: 'payment', displayName: '支付平台', description: '支付业务应用', tenant: '研发中心', owner: '韩梅梅', updatedAt: '2026-05-29 17:12' }
];
const jenkinsJobTemplates: JenkinsJobTemplate[] = [
  { id: 'java-unified-v1', name: 'java-unified-v1', version: 1, status: 'enabled', isDefault: true, updatedAt: '2026-05-31 10:00' },
  { id: 'java-tomcat-v1', name: 'java-tomcat-v1', version: 2, status: 'enabled', isDefault: false, updatedAt: '2026-05-30 18:20' }
];
const buildEnvironments: BuildEnvironment[] = [
  { id: 'build_env_java_springboot', name: 'java-springboot', buildImage: 'maven:3.9.9-eclipse-temurin-17', status: 'enabled', isDefault: true, updatedAt: '2026-05-31 10:00' },
  { id: 'build_env_java_tomcat', name: 'java-tomcat', buildImage: 'maven:3.8.8-eclipse-temurin-8', status: 'enabled', isDefault: false, updatedAt: '2026-05-31 10:00' }
];
const runtimeEnvironments: RuntimeEnvironment[] = [
  { id: 'runtime_env_java17', name: 'java17', runtimeBaseImage: 'registry.example/runtime/java17:1.0', artifactDeployPath: '/app/', dockerfilePath: 'java/jar/Dockerfile', status: 'enabled', isDefault: true, updatedAt: '2026-05-31 10:00' },
  { id: 'runtime_env_tomcat8', name: 'tomcat8', runtimeBaseImage: 'registry.example/runtime/tomcat8:1.0', artifactDeployPath: '/usr/local/tomcat/webapps/', dockerfilePath: 'java/tomcat/Dockerfile', status: 'enabled', isDefault: false, updatedAt: '2026-05-31 10:00' }
];
let buildTemplate: BuildTemplate = {
  id: 'global-build-template',
  name: 'global-build-template',
  version: 1,
  content: "pipeline {\n  agent any\n  environment { PAAS_CALLBACK_URL = 'https://paas.example/api/builds/build_run_x/callback' }\n  stages {\n    stage('按代码源构建') {\n      steps { sh 'mvn clean package -DskipTests' }\n    }\n    stage('生成并推送多架构镜像') {\n      steps { sh 'docker buildx build --platform linux/amd64,linux/arm64 --push .' }\n    }\n  }\n  post {\n    success { sh 'curl -fsS -X POST \"$PAAS_CALLBACK_URL\" -H \"Content-Type: application/json\" --data-binary @report/callback-success.json' }\n  }\n}",
  updatedAt: '2026-05-31 10:00'
};
const sourceRepositories: SourceRepository[] = [
  { id: 'repo_1', projectId: 'project_1', projectName: '订单平台', name: 'order-api', displayName: '订单服务仓库', description: '平台托管 GitLab 源码仓库', gitProvider: 'gitlab', httpUrl: 'https://gitlab.example/rnd/order/order-api.git', sshUrl: 'git@gitlab.example:rnd/order/order-api.git', defaultBranch: 'main', status: 'ready', associatedApplications: 1, updatedAt: '2026-05-30 10:12' },
  { id: 'repo_2', projectId: 'project_1', projectName: '订单平台', name: 'order-platform', displayName: '订单平台 monorepo', description: '多个应用共用的托管仓库', gitProvider: 'gitlab', httpUrl: 'https://gitlab.example/rnd/order/order-platform.git', sshUrl: 'git@gitlab.example:rnd/order/order-platform.git', defaultBranch: 'main', status: 'ready', associatedApplications: 2, updatedAt: '2026-05-29 18:22' }
];
const applications: Application[] = [
  { id: 'app_1', name: 'order-api', displayName: '订单服务', project: '订单平台', projectId: 'project_1', description: '订单服务应用', runtimeEnvironmentId: 'runtime_env_java17', runtimeEnvironments: [{ id: 'runtime_env_java17', name: 'java17', runtimeBaseImage: 'registry.example/runtime/java17:1.0', artifactDeployPath: '/app/' }], status: 'active', type: 'Spring Boot', envStatus: '运行中', build: '#128 成功', release: 'v1.8.2', owner: '李雷', updatedAt: '2026-05-30 10:12' },
  { id: 'app_2', name: 'order-web', displayName: '订单前端', project: '订单平台', projectId: 'project_1', description: '', runtimeEnvironmentId: 'runtime_env_tomcat8', runtimeEnvironments: [{ id: 'runtime_env_tomcat8', name: 'tomcat8', runtimeBaseImage: 'registry.example/runtime/tomcat8:1.0', artifactDeployPath: '/usr/local/tomcat/webapps/' }], status: 'active', type: 'Tomcat', envStatus: '待绑定集群', build: '#42 成功', release: 'v0.9.4', owner: '王芳', updatedAt: '2026-05-29 18:22' }
];
const applicationSources: Record<string, ApplicationSource[]> = {
  app_1: [{ id: 'app_source_1', key: 'main', displayName: '主代码源', sourceRepositoryId: 'repo_1', buildEnvironmentId: 'build_env_java_springboot', sourcePath: 'services/order-api', defaultRef: 'main', isPrimary: true, buildSpec: { sourcePath: 'services/order-api', buildCommand: 'mvn clean package -DskipTests', artifactCopyCommand: 'cp -ar target/order-api.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"', defaultRef: 'main' } }]
};
const buildPipelines: Record<string, BuildPipeline[]> = {
  app_1: [{
    id: 'pipeline_1',
    applicationId: 'app_1',
    name: 'main',
    displayName: '主流水线',
    description: '后端服务构建',
    status: 'active',
    externalJobName: 'paas/rnd/order/order-api/main',
    updatedAt: '2026-05-30 10:12',
    runtimeEnvironments: [{ ...runtimeEnvironments[0] }],
    sources: [{ id: 'pipeline_source_1', pipelineId: 'pipeline_1', key: 'main', displayName: '主代码源', sourceRepositoryId: 'repo_1', buildEnvironmentId: 'build_env_java_springboot', sourcePath: 'services/order-api', defaultRef: 'main', isPrimary: true, buildSpec: { sourcePath: 'services/order-api', buildCommand: 'mvn clean package -DskipTests', artifactCopyCommand: 'cp -ar target/order-api.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"', runtimeBaseImage: 'registry.example/runtime/java17:1.0', artifactDeployPath: '/app/', defaultRef: 'main' } }]
  }]
};

export async function login(account: string, password: string) {
  await wait();
  if (!account || !password) {
    throw new Error('请输入账号和密码');
  }
  return { token: 'mock-token', userName: account };
}

export async function oidcLoginURL() {
  await wait();
  return '/oidc/callback?code=mock';
}

export async function currentUser() {
  await wait();
  return { name: '平台用户', permissions: ['application:create', 'deployment:create'] };
}

export async function listTenants(): Promise<Tenant[]> {
  await wait();
  return tenants.map((item) => ({ ...item }));
}

export async function createTenant(input: { name: string; displayName: string; description?: string }) {
  await wait();
  if (tenants.some((item) => item.name === input.name)) throw new Error('租户标识已存在');
  const tenant = { id: `tenant_${Date.now()}`, name: input.name, displayName: input.displayName || input.name, description: input.description || '', updatedAt: '刚刚' };
  tenants.unshift(tenant);
  return { ...tenant };
}

export async function updateTenant(id: string, input: { displayName: string; description?: string }) {
  await wait();
  const tenant = tenants.find((item) => item.id === id);
  if (!tenant) throw new Error('租户不存在');
  tenant.displayName = input.displayName || tenant.name;
  tenant.description = input.description || '';
  tenant.updatedAt = '刚刚';
  projects.forEach((project) => {
    if (project.tenantId === tenant.id) project.tenant = tenant.displayName;
  });
  return { ...tenant };
}

export async function listProjects(): Promise<Project[]> {
  await wait();
  return projects.map((item) => ({ ...item }));
}

export async function createProject(input: { tenantId: string; name: string; displayName: string; description?: string }) {
  await wait();
  const tenant = tenants.find((item) => item.id === input.tenantId);
  if (!tenant) throw new Error('租户不存在');
  const project = { id: `project_${Date.now()}`, tenantId: tenant.id, name: input.name, displayName: input.displayName || input.name, description: input.description || '', tenant: tenant.displayName, owner: '平台管理员', updatedAt: '刚刚' };
  projects.unshift(project);
  return { ...project };
}

export async function deleteProject(id: string) {
  await wait();
  const index = projects.findIndex((item) => item.id === id);
  if (index < 0) throw new Error('项目不存在');
  projects.splice(index, 1);
}

export async function listApplications(): Promise<Application[]> {
  await wait();
  return applications.map((item) => ({ ...item }));
}

export async function getApplication(id: string): Promise<Application> {
  await wait();
  const app = applications.find((item) => item.id === id);
  if (!app) throw new Error('应用不存在');
  return { ...app, runtimeEnvironments: app.runtimeEnvironments?.map((item) => ({ ...item })) };
}

export async function getApplicationSources(id: string): Promise<ApplicationSource[]> {
  await wait();
  return (applicationSources[id] || []).map((item) => ({ ...item, buildSpec: { ...item.buildSpec } }));
}

export async function updateApplication(id: string, input: any) {
  await wait();
  const index = applications.findIndex((item) => item.id === id);
  if (index < 0) throw new Error('应用不存在');
  applications[index] = {
    ...applications[index],
    displayName: input.displayName || applications[index].name,
    description: input.description || '',
    status: input.disabled ? 'disabled' : 'active',
    updatedAt: '刚刚'
  };
  return { ...applications[index] };
}

export async function deleteApplication(id: string) {
  await wait();
  const index = applications.findIndex((item) => item.id === id);
  if (index < 0) throw new Error('应用不存在');
  applications.splice(index, 1);
  delete buildPipelines[id];
}

export async function listBuilds(): Promise<BuildRun[]> {
  await wait();
  return [
    { id: 'build_128', application: '订单服务', status: '成功', ref: 'main', commit: '8c1a09f', startedAt: '2026-05-30 10:01', duration: '3 分 12 秒' },
    { id: 'build_127', application: '订单服务', status: '失败', ref: 'main', commit: '61b9120', startedAt: '2026-05-29 16:40', duration: '2 分 03 秒' }
  ];
}

export async function buildLog() {
  await wait();
  return [
    '[INFO] 检出平台托管源码仓库',
    '[INFO] 执行构建命令 mvn clean package -DskipTests',
    '[INFO] 校验产物 target/order-api.jar',
    '[INFO] 构建并推送镜像 registry.local/order-api:v1.8.2',
    '[INFO] 回调 PaaS 控制面完成'
  ].join('\n');
}

export async function listAuditLogs(): Promise<AuditLog[]> {
  await wait();
  return [
    { id: 'audit_1', actor: '李雷', action: 'promotion.approve', resource: '发布 promotion_18', result: '成功', summary: '审批通过生产发布', time: '2026-05-30 10:40' },
    { id: 'audit_2', actor: '平台管理员', action: 'cluster.disable', resource: '集群 prod-a', result: '成功', summary: '禁用异常集群', time: '2026-05-29 21:14' }
  ];
}

export async function listFreights(): Promise<Freight[]> {
  await wait();
  return [
    { id: 'freight_18', version: 'v1.8.2', image: 'registry.local/order-api:v1.8.2', digest: 'sha256:91ab', commit: '8c1a09f', createdAt: '2026-05-30 10:08' },
    { id: 'freight_17', version: 'v1.8.1', image: 'registry.local/order-api:v1.8.1', digest: 'sha256:7f02', commit: '61b9120', createdAt: '2026-05-29 16:45' }
  ];
}

export async function listSourceRepositories(projectId?: string): Promise<SourceRepository[]> {
  await wait();
  return sourceRepositories.filter((item) => !projectId || item.projectId === projectId).map((item) => ({ ...item }));
}

export async function getSourceRepository(id: string): Promise<SourceRepository> {
  const repos = await listSourceRepositories();
  const repo = repos.find((item) => item.id === id) || repos[0];
  return repo;
}

export async function createSourceRepository(input: { projectId: string; name: string; displayName: string; description?: string; defaultBranch: string }) {
  await wait();
  const project = projects.find((item) => item.id === input.projectId);
  const repo = { id: `repo_${Date.now()}`, projectId: input.projectId, projectName: project?.displayName || '', name: input.name, displayName: input.displayName || input.name, description: input.description || '', gitProvider: 'gitlab', httpUrl: `https://gitlab.example/rnd/${project?.name || 'project'}/${input.name}.git`, sshUrl: `git@gitlab.example:rnd/${project?.name || 'project'}/${input.name}.git`, defaultBranch: input.defaultBranch, status: 'ready', associatedApplications: 0, updatedAt: '刚刚' };
  sourceRepositories.unshift(repo);
  return { ...repo };
}

export async function deleteSourceRepository(id: string) {
  await wait();
  const index = sourceRepositories.findIndex((item) => item.id === id);
  if (index < 0) throw new Error('源码仓库不存在');
  if (sourceRepositories[index].associatedApplications > 0) throw new Error('存在关联应用，不能删除');
  sourceRepositories.splice(index, 1);
}

export async function listRepositoryApplications() {
  await wait();
  return [{ id: 'app_1', name: 'order-api', displayName: '订单服务' }];
}

export async function scanRepositoryJava(): Promise<BuildSpecSuggestion[]> {
  await wait();
  return [
    { sourcePath: '.', buildCommand: 'mvn clean package -DskipTests', artifactCopyCommand: 'cp -ar target/order-api.jar "$PAAS_ARTIFACT_OUTPUT/app.jar"', runtimeBaseImage: 'registry.example/runtime/java17:1.0', evidence: ['pom.xml', 'target/order-api.jar'] }
  ];
}

export async function listRepositoryBranches(repositoryId: string): Promise<RepositoryBranch[]> {
  await wait();
  const repo = sourceRepositories.find((item) => item.id === repositoryId);
  const defaultBranch = repo?.defaultBranch || 'main';
  return [
    { name: defaultBranch, default: true },
    { name: 'develop', default: false },
    { name: 'feature/order-preview', default: false }
  ].filter((branch, index, branches) => branches.findIndex((item) => item.name === branch.name) === index);
}

export async function listRepositoryTree(_repositoryId: string, _ref: string, path = ''): Promise<RepositoryTreeItem[]> {
  await wait();
  const tree: Record<string, RepositoryTreeItem[]> = {
    '': [
      { name: 'services', path: 'services', type: 'tree' },
      { name: 'frontend', path: 'frontend', type: 'tree' },
      { name: 'README.md', path: 'README.md', type: 'blob' }
    ],
    services: [
      { name: 'order-api', path: 'services/order-api', type: 'tree' },
      { name: 'payment-api', path: 'services/payment-api', type: 'tree' }
    ],
    'services/order-api': [
      { name: 'pom.xml', path: 'services/order-api/pom.xml', type: 'blob' },
      { name: 'src', path: 'services/order-api/src', type: 'tree' },
      { name: 'target', path: 'services/order-api/target', type: 'tree' }
    ],
    'services/order-api/target': [
      { name: 'order-api.jar', path: 'services/order-api/target/order-api.jar', type: 'blob' }
    ],
    frontend: [
      { name: 'admin-web', path: 'frontend/admin-web', type: 'tree' }
    ]
  };
  return (tree[path] || []).map((item) => ({ ...item }));
}

export async function syncRepositoryPermissions() {
  await wait();
  return { id: 'repo_perm_sync_1', status: 'succeeded' };
}

export async function listJenkinsJobTemplates(): Promise<JenkinsJobTemplate[]> {
  await wait();
  return jenkinsJobTemplates.filter((item) => item.status === 'enabled').map((item) => ({ ...item }));
}

export const listBuildTypes = listJenkinsJobTemplates;
export const listAdminBuildTypes = listAdminJenkinsJobTemplates;

export async function listBuildEnvironments(): Promise<BuildEnvironment[]> {
  await wait();
  return buildEnvironments.filter((item) => item.status === 'enabled').map((item) => ({ ...item }));
}

export async function listAdminBuildEnvironments(): Promise<BuildEnvironment[]> {
  await wait();
  return buildEnvironments.map((item) => ({ ...item }));
}

export async function listRuntimeEnvironments(): Promise<RuntimeEnvironment[]> {
  await wait();
  return runtimeEnvironments.filter((item) => item.status === 'enabled').map((item) => ({ ...item }));
}

export async function listAdminRuntimeEnvironments(): Promise<RuntimeEnvironment[]> {
  await wait();
  return runtimeEnvironments.map((item) => ({ ...item }));
}

export async function createBuildEnvironment(input: Partial<BuildEnvironment>) {
  await wait();
  if (input.isDefault) buildEnvironments.forEach((item) => { item.isDefault = false; });
  const item: BuildEnvironment = { id: `build_env_${Date.now()}`, name: input.name || 'custom-build', description: input.description || '', buildImage: input.buildImage || 'maven:3.9.9-eclipse-temurin-17', status: 'enabled', isDefault: !!input.isDefault, updatedAt: '刚刚' };
  buildEnvironments.unshift(item);
  return { ...item };
}

export async function updateBuildEnvironment(id: string, input: Partial<BuildEnvironment>) {
  await wait();
  const index = buildEnvironments.findIndex((item) => item.id === id);
  if (index < 0) throw new Error('构建环境不存在');
  if (input.isDefault) buildEnvironments.forEach((item) => { item.isDefault = false; });
  buildEnvironments[index] = { ...buildEnvironments[index], ...input, updatedAt: '刚刚' };
  return { ...buildEnvironments[index] };
}

export async function deleteBuildEnvironment(id: string) {
  await wait();
  const index = buildEnvironments.findIndex((item) => item.id === id);
  if (index < 0) throw new Error('构建环境不存在');
  buildEnvironments.splice(index, 1);
}

export async function createRuntimeEnvironment(input: Partial<RuntimeEnvironment>) {
  await wait();
  if (input.isDefault) runtimeEnvironments.forEach((item) => { item.isDefault = false; });
  const item: RuntimeEnvironment = { id: `runtime_env_${Date.now()}`, name: input.name || 'custom-runtime', runtimeBaseImage: input.runtimeBaseImage || 'registry.example/runtime/java17:1.0', artifactDeployPath: input.artifactDeployPath || '/app/', dockerfilePath: input.dockerfilePath || 'java/jar/Dockerfile', status: 'enabled', isDefault: !!input.isDefault, updatedAt: '刚刚' };
  runtimeEnvironments.unshift(item);
  return { ...item };
}

export async function updateRuntimeEnvironment(id: string, input: Partial<RuntimeEnvironment>) {
  await wait();
  const index = runtimeEnvironments.findIndex((item) => item.id === id);
  if (index < 0) throw new Error('运行时环境不存在');
  if (input.isDefault) runtimeEnvironments.forEach((item) => { item.isDefault = false; });
  runtimeEnvironments[index] = { ...runtimeEnvironments[index], ...input, updatedAt: '刚刚' };
  return { ...runtimeEnvironments[index] };
}

export async function deleteRuntimeEnvironment(id: string) {
  await wait();
  const index = runtimeEnvironments.findIndex((item) => item.id === id);
  if (index < 0) throw new Error('运行时环境不存在');
  runtimeEnvironments.splice(index, 1);
}

export async function getBuildTemplate(): Promise<BuildTemplate> {
  await wait();
  return { ...buildTemplate };
}

export async function updateBuildTemplate(input: { content: string }) {
  await wait();
  buildTemplate = { ...buildTemplate, content: input.content, version: buildTemplate.version + 1, updatedAt: '刚刚' };
  return { ...buildTemplate };
}

export async function listAdminJenkinsJobTemplates(): Promise<JenkinsJobTemplate[]> {
  await wait();
  return jenkinsJobTemplates.map((item) => ({ ...item }));
}

export async function createJenkinsJobTemplate(input: { name: string; jenkinsfileContent?: string; xmlContent?: string; isDefault?: boolean }) {
  await wait();
  if (input.isDefault) {
    jenkinsJobTemplates.forEach((item) => { item.isDefault = false; });
  }
  const template = { id: `jenkins_template_${Date.now()}`, name: input.name, version: 1, status: 'enabled', isDefault: !!input.isDefault, jenkinsfileContent: input.jenkinsfileContent || input.xmlContent, updatedAt: '刚刚' };
  jenkinsJobTemplates.unshift(template);
  return { ...template };
}

export async function updateJenkinsJobTemplate(id: string, input: { status: string; isDefault?: boolean }) {
  await wait();
  const index = jenkinsJobTemplates.findIndex((item) => item.id === id);
  if (index < 0) throw new Error('模板不存在');
  if (input.isDefault) {
    jenkinsJobTemplates.forEach((item) => { item.isDefault = false; });
  }
  jenkinsJobTemplates[index] = { ...jenkinsJobTemplates[index], status: input.status, isDefault: !!input.isDefault, updatedAt: '刚刚' };
  return { ...jenkinsJobTemplates[index] };
}

export const createBuildType = createJenkinsJobTemplate;
export const updateBuildType = updateJenkinsJobTemplate;
export const uploadBuildTypeRevision = uploadJenkinsJobTemplateRevision;
export const deleteBuildType = deleteJenkinsJobTemplate;

export async function uploadJenkinsJobTemplateRevision(id: string, input: { jenkinsfileContent?: string; xmlContent?: string }) {
  await wait();
  const index = jenkinsJobTemplates.findIndex((item) => item.id === id);
  if (index < 0) throw new Error('模板不存在');
  jenkinsJobTemplates[index] = { ...jenkinsJobTemplates[index], version: jenkinsJobTemplates[index].version + 1, jenkinsfileContent: input.jenkinsfileContent || input.xmlContent, updatedAt: '刚刚' };
  return { ...jenkinsJobTemplates[index] };
}

export async function deleteJenkinsJobTemplate(id: string) {
  await wait();
  const index = jenkinsJobTemplates.findIndex((item) => item.id === id);
  if (index < 0) throw new Error('构建类型不存在');
  jenkinsJobTemplates.splice(index, 1);
}

export async function createApplication(input: any) {
  await wait();
  const id = `app_${Date.now()}`;
  const project = projects.find((item) => item.id === input.projectId);
  const app: Application = { id, name: input.name, displayName: input.displayName || input.name, project: project?.displayName || '', projectId: input.projectId, description: input.description || '', status: 'active', type: '-', envStatus: '待绑定集群', build: '-', release: '-', owner: '平台管理员', updatedAt: '刚刚' };
  applications.unshift(app);
  applicationSources[id] = [];
  buildPipelines[id] = [];
  return { id, name: input.name, displayName: input.displayName || input.name };
}

export async function listBuildPipelines(applicationId: string): Promise<BuildPipeline[]> {
  await wait();
  return (buildPipelines[applicationId] || []).map((pipeline) => ({ ...pipeline, runtimeEnvironments: pipeline.runtimeEnvironments?.map((runtime) => ({ ...runtime })), sources: pipeline.sources?.map((source) => ({ ...source, buildSpec: { ...source.buildSpec } })) }));
}

export async function createBuildPipeline(applicationId: string, input: Omit<BuildPipeline, 'id' | 'applicationId' | 'status' | 'updatedAt' | 'runtimeEnvironments'> & { runtimeEnvironmentIds?: string[]; sources: BuildPipelineSource[] }) {
  await wait();
  const id = `pipeline_${Date.now()}`;
  const runtimeSnapshots = (input.runtimeEnvironmentIds || []).map((runtimeId) => runtimeEnvironments.find((item) => item.id === runtimeId)).filter(Boolean) as RuntimeEnvironment[];
  const pipeline: BuildPipeline = {
    id,
    applicationId,
    name: input.name,
    displayName: input.displayName || input.name,
    description: input.description || '',
    status: 'active',
    externalJobName: '',
    updatedAt: '刚刚',
    runtimeEnvironments: runtimeSnapshots.map((runtime) => ({ ...runtime })),
    sources: input.sources.map((source, index) => ({ ...source, id: `pipeline_source_${Date.now()}_${index}`, pipelineId: id, displayName: source.displayName || source.key, isPrimary: index === 0, buildSpec: { ...source.buildSpec } }))
  };
  buildPipelines[applicationId] = [pipeline, ...(buildPipelines[applicationId] || [])];
  return { ...pipeline };
}

export async function listBuildPipelineSources(pipelineId: string): Promise<BuildPipelineSource[]> {
  await wait();
  const pipeline = Object.values(buildPipelines).flat().find((item) => item.id === pipelineId);
  if (!pipeline) throw new Error('流水线不存在');
  return (pipeline.sources || []).map((source) => ({ ...source, buildSpec: { ...source.buildSpec } }));
}

export async function deleteBuildPipeline(pipelineId: string) {
  await wait();
  for (const [applicationId, pipelines] of Object.entries(buildPipelines)) {
    const index = pipelines.findIndex((pipeline) => pipeline.id === pipelineId);
    if (index >= 0) {
      buildPipelines[applicationId].splice(index, 1);
      return;
    }
  }
  throw new Error('流水线不存在');
}

export async function triggerBuildPipeline(pipelineId: string, input: { gitRef?: string; commitSha?: string } = {}) {
  await wait();
  const pipeline = Object.values(buildPipelines).flat().find((item) => item.id === pipelineId);
  if (!pipeline) throw new Error('流水线不存在');
  return { id: `build_${Date.now()}`, application: pipeline.applicationId, pipeline: pipeline.displayName, pipelineId, status: '排队中', ref: input.gitRef || pipeline.sources?.[0]?.defaultRef || 'main', commit: input.commitSha || '', startedAt: '刚刚', duration: '-' };
}

export async function triggerBuild(applicationId: string, input: { gitRef?: string }) {
  await wait();
  const pipeline = (buildPipelines[applicationId] || [])[0];
  if (!pipeline) throw new Error('请先创建构建流水线');
  return triggerBuildPipeline(pipeline.id, input);
}
