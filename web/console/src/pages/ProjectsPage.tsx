import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Calendar, FolderKanban, GitBranch, Package, Plus, Search, Trash2, User, X } from 'lucide-react';
import { FormEvent, MouseEvent, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { message } from 'antd';
import { createProject, deleteProject, listApplications, listProjects, listSourceRepositories, listTenants, type Project } from '../api';

type CreateProjectForm = {
  tenantId: string;
  name: string;
  displayName: string;
  description: string;
};

type SortKey = 'updatedAt' | 'name' | 'appCount';

const initialCreateForm: CreateProjectForm = {
  tenantId: '',
  name: '',
  displayName: '',
  description: ''
};

export function ProjectsPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Project>();
  const [keyword, setKeyword] = useState('');
  const [tenantId, setTenantId] = useState<string>('');
  const [owner, setOwner] = useState<string>('');
  const [sortBy, setSortBy] = useState<SortKey>('updatedAt');
  const [form, setForm] = useState<CreateProjectForm>(initialCreateForm);
  const { data = [], isLoading } = useQuery({ queryKey: ['projects'], queryFn: listProjects });
  const { data: tenants = [] } = useQuery({ queryKey: ['tenants'], queryFn: listTenants });
  const { data: applications = [] } = useQuery({ queryKey: ['apps', 'all'], queryFn: () => listApplications() });
  const { data: repositories = [] } = useQuery({ queryKey: ['source-repositories', undefined], queryFn: () => listSourceRepositories() });

  const tenantOptions = useMemo(() => tenants.map((tenant) => ({ value: tenant.id, label: tenant.displayName || tenant.name })), [tenants]);
  const ownerOptions = useMemo(() => Array.from(new Set(data.map((project) => project.owner).filter(Boolean))), [data]);

  const projectStats = useMemo(() => {
    return data.reduce<Record<string, { appCount: number; repoCount: number; lastRelease: string }>>((acc, project) => {
      const projectApps = applications.filter((app) => app.projectId === project.id || app.project === project.displayName || app.project === project.name);
      const projectRepos = repositories.filter((repo) => repo.projectId === project.id);
      const releases = projectApps.map((app) => app.release).filter((release) => release && release !== '-');
      acc[project.id] = {
        appCount: projectApps.length,
        repoCount: projectRepos.length,
        lastRelease: releases[0] || '暂无发布'
      };
      return acc;
    }, {});
  }, [applications, data, repositories]);

  const filteredProjects = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    return [...data]
      .filter((project) => {
        const matchTenant = !tenantId || project.tenantId === tenantId;
        const matchOwner = !owner || project.owner === owner;
        const matchKeyword = !normalizedKeyword || [project.displayName, project.name, project.description].some((value) => (value || '').toLowerCase().includes(normalizedKeyword));
        return matchTenant && matchOwner && matchKeyword;
      })
      .sort((left, right) => {
        if (sortBy === 'name') return (left.displayName || left.name).localeCompare(right.displayName || right.name, 'zh-CN');
        if (sortBy === 'appCount') return (projectStats[right.id]?.appCount || 0) - (projectStats[left.id]?.appCount || 0);
        return (right.updatedAt || '').localeCompare(left.updatedAt || '');
      });
  }, [data, keyword, owner, projectStats, sortBy, tenantId]);

  const createMutation = useMutation({
    mutationFn: createProject,
    onSuccess: async () => {
      message.success('项目已创建');
      closeCreateModal();
      await queryClient.invalidateQueries({ queryKey: ['projects'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '项目创建失败')
  });
  const deleteMutation = useMutation({
    mutationFn: deleteProject,
    onSuccess: async () => {
      message.success('项目已删除');
      setDeleteTarget(undefined);
      await queryClient.invalidateQueries({ queryKey: ['projects'] });
    },
    onError: (error) => message.error(error instanceof Error ? error.message : '项目删除失败')
  });

  const openCreateModal = () => {
    setForm({ ...initialCreateForm, tenantId: tenantId || tenantOptions[0]?.value || '' });
    setOpen(true);
  };

  const closeCreateModal = () => {
    setOpen(false);
    setForm(initialCreateForm);
  };

  const handleCreate = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!form.tenantId) {
      message.error('请选择所属租户');
      return;
    }
    if (!/^[a-z][a-z0-9-]{1,62}$/.test(form.name)) {
      message.error('项目标识仅支持小写字母、数字和连字符');
      return;
    }
    if (!form.displayName.trim()) {
      message.error('请输入项目名称');
      return;
    }
    createMutation.mutate({
      tenantId: form.tenantId,
      name: form.name.trim(),
      displayName: form.displayName.trim(),
      description: form.description.trim()
    });
  };

  const handleDeleteClick = (event: MouseEvent<HTMLButtonElement>, project: Project) => {
    event.stopPropagation();
    setDeleteTarget(project);
  };

  return (
    <div className="figma-project-page">
      <div className="figma-project-header">
        <div>
          <h1>项目</h1>
          <p>管理您的应用交付项目</p>
        </div>
        <button className="figma-primary-button" type="button" onClick={openCreateModal}>
          <Plus size={16} />
          创建项目
        </button>
      </div>

      <div className="figma-project-filter-card">
        <div className="figma-search-input">
          <Search size={16} />
          <input
            type="search"
            placeholder="搜索项目名称或描述..."
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
          />
        </div>
        <select aria-label="选择租户" value={tenantId} onChange={(event) => setTenantId(event.target.value)}>
          <option value="">全部租户</option>
          {tenantOptions.map((tenant) => <option key={tenant.value} value={tenant.value}>{tenant.label}</option>)}
        </select>
        <select aria-label="选择负责人" value={owner} onChange={(event) => setOwner(event.target.value)}>
          <option value="">全部负责人</option>
          {ownerOptions.map((name) => <option key={name} value={name}>{name}</option>)}
        </select>
        <select aria-label="排序方式" value={sortBy} onChange={(event) => setSortBy(event.target.value as SortKey)}>
          <option value="updatedAt">按更新时间</option>
          <option value="name">按名称</option>
          <option value="appCount">按应用数</option>
        </select>
      </div>

      {isLoading ? (
        <div className="figma-project-empty">正在加载项目...</div>
      ) : filteredProjects.length > 0 ? (
        <div className="figma-project-grid">
          {filteredProjects.map((project) => {
            const stats = projectStats[project.id] || { appCount: 0, repoCount: 0, lastRelease: '暂无发布' };
            return (
              <article
                key={project.id}
                className="figma-project-card"
                onClick={() => navigate(`/projects/${project.id}`)}
                tabIndex={0}
                onKeyDown={(event) => {
                  if (event.key === 'Enter') navigate(`/projects/${project.id}`);
                }}
              >
                <div className="figma-project-card-head">
                  <div className="figma-project-title-row">
                    <div className="figma-project-icon">
                      <FolderKanban size={24} />
                    </div>
                    <div>
                      <h2>{project.displayName || project.name}</h2>
                      <p>{project.description || '暂无项目描述'}</p>
                    </div>
                  </div>
                </div>

                <div className="figma-project-metrics">
                  <div>
                    <span><Package size={13} /> 应用</span>
                    <strong>{stats.appCount}</strong>
                  </div>
                  <div>
                    <span><GitBranch size={13} /> 仓库</span>
                    <strong>{stats.repoCount}</strong>
                  </div>
                  <div>
                    <span>最近发布</span>
                    <strong className="figma-project-small-value">{stats.lastRelease}</strong>
                  </div>
                </div>

                <div className="figma-project-card-footer">
                  <div>
                    <User size={16} />
                    <span>{project.owner || '未设置'}</span>
                  </div>
                  <div>
                    <Calendar size={16} />
                    <span>{project.updatedAt || '-'}</span>
                  </div>
                  <button className="figma-danger-ghost" type="button" onClick={(event) => handleDeleteClick(event, project)}>
                    <Trash2 size={14} />
                    删除
                  </button>
                </div>
              </article>
            );
          })}
        </div>
      ) : (
        <div className="figma-project-empty">
          <div className="figma-project-empty-icon"><FolderKanban size={32} /></div>
          <h2>{keyword || tenantId || owner ? '未找到匹配的项目' : '暂无项目'}</h2>
          <p>{keyword || tenantId || owner ? '请调整筛选条件后重试' : '创建您的第一个项目，开始应用交付'}</p>
          {!keyword && !tenantId && !owner && (
            <button className="figma-primary-button" type="button" onClick={openCreateModal}>
              <Plus size={16} />
              创建项目
            </button>
          )}
        </div>
      )}

      {open && (
        <div className="figma-modal-backdrop" role="presentation">
          <div className="figma-modal" role="dialog" aria-modal="true" aria-labelledby="create-project-title">
            <div className="figma-modal-head">
              <h2 id="create-project-title">创建项目</h2>
              <button type="button" aria-label="关闭" onClick={closeCreateModal}><X size={18} /></button>
            </div>
            <form onSubmit={handleCreate}>
              <div className="figma-modal-body">
                <label>
                  所属租户 <span>*</span>
                  <select value={form.tenantId} onChange={(event) => setForm((current) => ({ ...current, tenantId: event.target.value }))}>
                    <option value="">选择租户</option>
                    {tenantOptions.map((tenant) => <option key={tenant.value} value={tenant.value}>{tenant.label}</option>)}
                  </select>
                </label>
                <label>
                  项目标识 <span>*</span>
                  <input
                    value={form.name}
                    placeholder="order"
                    onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))}
                  />
                  <small>只能包含小写字母、数字和连字符</small>
                </label>
                <label>
                  项目名称 <span>*</span>
                  <input
                    value={form.displayName}
                    placeholder="订单平台"
                    onChange={(event) => setForm((current) => ({ ...current, displayName: event.target.value }))}
                  />
                </label>
                <label>
                  项目描述
                  <textarea
                    rows={3}
                    value={form.description}
                    placeholder="说明项目用途"
                    onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))}
                  />
                </label>
              </div>
              <div className="figma-modal-foot">
                <button className="figma-secondary-button" type="button" onClick={closeCreateModal}>取消</button>
                <button className="figma-primary-button" type="submit" disabled={createMutation.isPending}>
                  {createMutation.isPending ? '创建中...' : '创建'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {deleteTarget && (
        <div className="figma-modal-backdrop" role="presentation">
          <div className="figma-modal figma-confirm-modal" role="dialog" aria-modal="true" aria-labelledby="delete-project-title">
            <div className="figma-modal-head">
              <h2 id="delete-project-title">删除项目</h2>
              <button type="button" aria-label="关闭" onClick={() => setDeleteTarget(undefined)}><X size={18} /></button>
            </div>
            <div className="figma-modal-body">
              <p>确认删除项目“{deleteTarget.displayName || deleteTarget.name}”？有关联应用或源码仓库时会被拒绝。</p>
            </div>
            <div className="figma-modal-foot">
              <button className="figma-secondary-button" type="button" onClick={() => setDeleteTarget(undefined)}>取消</button>
              <button className="figma-danger-button" type="button" disabled={deleteMutation.isPending} onClick={() => deleteMutation.mutate(deleteTarget.id)}>
                {deleteMutation.isPending ? '删除中...' : '删除'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
