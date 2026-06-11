import { useQuery } from '@tanstack/react-query';
import { Card, Descriptions, Tabs, Tag } from 'antd';
import { useParams } from 'react-router-dom';
import { listProjects } from '../api';
import { PageHeader } from '../components/PageHeader';
import { ApplicationsPage } from './ApplicationsPage';
import { SourceRepositoryList } from './SourceRepositoriesPage';

export function ProjectDetailPage() {
  const { id = '' } = useParams();
  const { data: projects = [], isLoading } = useQuery({ queryKey: ['projects'], queryFn: listProjects });
  const project = projects.find((item) => item.id === id);

  return (
    <>
      <PageHeader title={project?.displayName || '项目详情'} />
      <Card className="summary-card" loading={isLoading}>
        <Descriptions column={4} size="small" items={[
          { key: 'name', label: '项目标识', children: project?.name || '-' },
          { key: 'tenant', label: '所属租户', children: project?.tenant ? <Tag>{project.tenant}</Tag> : '-' },
          { key: 'owner', label: '负责人', children: project?.owner || '-' },
          { key: 'updated', label: '更新时间', children: project?.updatedAt || '-' }
        ]} />
      </Card>
      <Tabs className="detail-tabs" items={[
        { key: 'applications', label: '应用', children: <ApplicationsPage projectId={id} embedded /> },
        { key: 'source-repositories', label: '源码仓库', children: <SourceRepositoryList projectId={id} hideProjectFilter /> }
      ]} />
    </>
  );
}
