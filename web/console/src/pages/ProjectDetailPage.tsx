import { useQuery } from '@tanstack/react-query';
import { AlertCircle, CheckCircle, Clock, GitBranch, GitMerge, Package, Plus } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { listApplications, listAuditLogs, listProjects, listSourceRepositories } from '../api';
import { MemberManagement } from '../components/MemberManagement';
import { SourceRepositoryList } from './SourceRepositoriesPage';

type ProjectTab = 'applications' | 'repositories' | 'members' | 'audit';

const tabs: { key: ProjectTab; label: string }[] = [
  { key: 'applications', label: '应用' },
  { key: 'repositories', label: '源码仓库' },
  { key: 'members', label: '成员权限' },
  { key: 'audit', label: '审计记录' }
];

export function ProjectDetailPage() {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<ProjectTab>('applications');
  const { data: projects = [], isLoading: isProjectLoading } = useQuery({ queryKey: ['projects'], queryFn: listProjects });
  const { data: applications = [], isLoading: isAppsLoading } = useQuery({ queryKey: ['apps', id], queryFn: () => listApplications(id), enabled: !!id });
  const { data: repositories = [] } = useQuery({ queryKey: ['source-repositories', id], queryFn: () => listSourceRepositories(id), enabled: !!id });
  const { data: auditLogs = [] } = useQuery({ queryKey: ['audit'], queryFn: listAuditLogs });
  const project = projects.find((item) => item.id === id);

  const stats = useMemo(() => {
    const healthyApps = applications.filter((app) => app.stageStatus === '运行中' || app.stageStatus === '运行正常').length;
    const abnormalStages = applications.filter((app) => app.stageStatus && !['运行中', '运行正常'].includes(app.stageStatus)).length;
    const recentBuilds = applications.filter((app) => app.build && app.build !== '-').length;
    const releasing = applications.filter((app) => app.release && app.release !== '-').length;
    return [
      { label: '应用数', value: applications.length, icon: Package, tone: 'blue', help: healthyApps ? `${healthyApps} 个运行正常` : '暂无运行数据' },
      { label: '构建次数（本周）', value: recentBuilds, icon: GitMerge, tone: 'green', help: '来自应用最近构建记录' },
      { label: '发布中', value: releasing, icon: Clock, tone: 'orange', help: '已有最近发布记录' },
      { label: '异常 Stage', value: abnormalStages, icon: AlertCircle, tone: abnormalStages > 0 ? 'red' : 'slate', help: abnormalStages > 0 ? '请查看应用状态' : '暂无异常' }
    ];
  }, [applications]);

  if (!project && !isProjectLoading) {
    return (
      <div className="figma-project-page">
        <div className="figma-project-empty">
          <h2>项目不存在</h2>
          <p>请返回项目列表选择可访问的项目。</p>
          <button className="figma-primary-button" type="button" onClick={() => navigate('/projects')}>返回项目列表</button>
        </div>
      </div>
    );
  }

  return (
    <div className="figma-project-page figma-project-workbench">
      <div className="figma-workbench-head">
        <div className="figma-workbench-title">
          <div className="figma-project-icon">
            <Package size={24} />
          </div>
          <div>
            <h1>{project?.displayName || '项目详情'}</h1>
            <p>{project?.description || project?.name || '正在加载项目...'}</p>
          </div>
        </div>
        <dl className="figma-workbench-meta">
          <div>
            <dt>项目标识</dt>
            <dd>{project?.name || '-'}</dd>
          </div>
          <div>
            <dt>所属租户</dt>
            <dd>{project?.tenant || '-'}</dd>
          </div>
          <div>
            <dt>负责人</dt>
            <dd>{project?.owner || '-'}</dd>
          </div>
          <div>
            <dt>更新时间</dt>
            <dd>{project?.updatedAt || '-'}</dd>
          </div>
        </dl>
      </div>

      <div className="figma-workbench-stats">
        {stats.map((stat) => {
          const Icon = stat.icon;
          return (
            <div key={stat.label} className="figma-stat-card">
              <div className={`figma-stat-icon figma-stat-${stat.tone}`}>
                <Icon size={20} />
              </div>
              <strong>{isAppsLoading ? '-' : stat.value}</strong>
              <span>{stat.label}</span>
              <small>{stat.help}</small>
            </div>
          );
        })}
      </div>

      <section className="figma-workbench-panel">
        <div className="figma-tab-list" role="tablist" aria-label="项目工作台">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              type="button"
              role="tab"
              aria-selected={activeTab === tab.key}
              className={activeTab === tab.key ? 'active' : ''}
              onClick={() => setActiveTab(tab.key)}
            >
              {tab.label}
            </button>
          ))}
        </div>

        <div className="figma-tab-panel" role="tabpanel">
          {activeTab === 'applications' && (
            <ApplicationWorkbenchList
              projectId={id}
              applications={applications}
              isLoading={isAppsLoading}
              onCreate={() => navigate(`/apps/new?projectId=${encodeURIComponent(id)}`)}
              onOpen={(appId) => navigate(`/apps/${appId}`)}
            />
          )}

          {activeTab === 'repositories' && (
            <div className="figma-embedded-section">
              <div className="figma-info-box">
                <AlertCircle size={18} />
                <div>
                  <strong>关于源码仓库登记</strong>
                  <p>源码仓库在项目级别统一登记，可被多个应用的 BuildPipeline 复用。创建应用时不需要绑定源码仓库。</p>
                </div>
              </div>
              <SourceRepositoryList projectId={id} hideProjectFilter />
            </div>
          )}

          {activeTab === 'members' && (
            <div className="figma-embedded-section">
              <MemberManagement scopeKind="project" scopeId={id} title="项目成员" />
            </div>
          )}

          {activeTab === 'audit' && (
            <div className="figma-audit-list">
              <div className="figma-section-heading">
                <div>
                  <h2>最近操作</h2>
                  <p>查看项目相关的操作记录</p>
                </div>
              </div>
              {auditLogs.length > 0 ? auditLogs.slice(0, 6).map((log) => (
                <div key={log.id} className="figma-audit-item">
                  <div className="figma-audit-avatar">{log.actor.slice(0, 1)}</div>
                  <div>
                    <p>
                      <strong>{log.actor}</strong>
                      {' '}{log.action}{' '}
                      <code>{log.resource}</code>
                    </p>
                    <span>{log.summary} · {log.time}</span>
                  </div>
                  <span className="figma-success-status">
                    <CheckCircle size={16} />
                    {log.result}
                  </span>
                </div>
              )) : (
                <div className="figma-project-empty figma-project-empty-compact">暂无审计记录</div>
              )}
            </div>
          )}
        </div>
      </section>
    </div>
  );
}

function ApplicationWorkbenchList({
  projectId,
  applications,
  isLoading,
  onCreate,
  onOpen
}: {
  projectId: string;
  applications: Awaited<ReturnType<typeof listApplications>>;
  isLoading: boolean;
  onCreate: () => void;
  onOpen: (id: string) => void;
}) {
  return (
    <div className="figma-application-workbench">
      <div className="figma-section-heading">
        <div>
          <h2>应用列表</h2>
          <p>本项目下的所有应用</p>
        </div>
        <button className="figma-primary-button" type="button" onClick={onCreate} disabled={!projectId}>
          <Plus size={16} />
          创建应用
        </button>
      </div>

      {isLoading ? (
        <div className="figma-project-empty figma-project-empty-compact">正在加载应用...</div>
      ) : applications.length > 0 ? (
        <div className="figma-app-list">
          {applications.map((app) => {
            const healthy = app.stageStatus === '运行中' || app.stageStatus === '运行正常';
            return (
              <button key={app.id} className="figma-app-item" type="button" onClick={() => onOpen(app.id)}>
                <div className="figma-app-main">
                  <div className="figma-app-icon"><Package size={20} /></div>
                  <div>
                    <strong>{app.displayName || app.name}</strong>
                    <span>{app.name}</span>
                  </div>
                </div>
                <div className="figma-app-side">
                  <span className={healthy ? 'figma-running-status' : 'figma-warning-status'}>
                    <i />
                    {app.stageStatus || '暂无状态'}
                  </span>
                  <span>最近构建: {app.build || '-'}</span>
                  <span>最近发布: {app.release || '-'}</span>
                </div>
              </button>
            );
          })}
        </div>
      ) : (
        <div className="figma-project-empty figma-project-empty-compact">
          <h2>暂无应用</h2>
          <p>在当前项目下创建第一个应用。</p>
        </div>
      )}

      <div className="figma-repo-summary">
        <GitBranch size={16} />
        <span>源码仓库请在“源码仓库”页签中独立登记，构建流水线配置时再选择。</span>
      </div>
    </div>
  );
}
