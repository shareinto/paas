import { BranchesOutlined, CodeOutlined, SyncOutlined } from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Badge, Button, Card, Descriptions, Input, Space, Table, Tabs, Tag, Typography, message } from 'antd';
import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useParams } from 'react-router-dom';
import { getSourceRepository, listRepositoryApplications, scanRepositoryJava, syncRepositoryPermissions } from '../api';
import { PageHeader } from '../components/PageHeader';

export function SourceRepositoryDetailPage() {
  const { id = '' } = useParams();
  const queryClient = useQueryClient();
  const [ref, setRef] = useState('main');
  const { data: repo, isLoading } = useQuery({ queryKey: ['source-repository', id], queryFn: () => getSourceRepository(id), enabled: !!id });
  const { data: applications = [] } = useQuery({ queryKey: ['source-repository-applications', id], queryFn: () => listRepositoryApplications(id), enabled: !!id });
  const { data: suggestions = [], isFetching } = useQuery({ queryKey: ['source-repository-java-scan', id, ref], queryFn: () => scanRepositoryJava(id, ref), enabled: !!id });
  const syncMutation = useMutation({
    mutationFn: () => syncRepositoryPermissions(id),
    onSuccess: async () => {
      message.success('权限同步任务已提交');
      await queryClient.invalidateQueries({ queryKey: ['source-repository', id] });
    }
  });

  return (
    <>
      <PageHeader
        title={repo?.displayName || '源码仓库'}
        breadcrumb={[
          { title: <Link to="/projects">项目</Link> },
          { title: repo?.projectId ? <Link to={`/projects/${repo.projectId}`}>{repo?.projectName || repo.projectId}</Link> : (repo?.projectName || '-') },
          { title: repo?.displayName || '源码仓库' }
        ]}
        extra={<Button icon={<SyncOutlined />} loading={syncMutation.isPending} onClick={() => syncMutation.mutate()}>同步权限</Button>}
      />
      <Card className="summary-card" loading={isLoading}>
        <Descriptions column={3} size="small" items={[
          { key: 'name', label: '仓库标识', children: repo?.name || '-' },
          { key: 'project', label: '所属项目', children: repo?.projectId ? <Link to={`/projects/${repo.projectId}`}>{repo?.projectName || repo.projectId}</Link> : '-' },
          { key: 'provider', label: 'Provider', children: <Tag color="blue">{repo?.gitProvider || 'gitlab'}</Tag> },
          { key: 'branch', label: '默认分支', children: <Tag icon={<BranchesOutlined />}>{repo?.defaultBranch || 'main'}</Tag> },
          { key: 'status', label: '状态', children: <Badge status={repo?.status === 'ready' ? 'success' : 'warning'} text={repo?.status === 'ready' ? '可用' : repo?.status || '-'} /> },
          { key: 'updated', label: '更新时间', children: repo?.updatedAt || '-' }
        ]} />
      </Card>
      <Tabs className="detail-tabs" items={[
        { key: 'overview', label: '总览', children: <Overview repo={repo} /> },
        { key: 'applications', label: '关联应用', children: <Table rowKey="id" dataSource={applications} pagination={false} columns={[{ title: '应用名称', dataIndex: 'displayName', render: (text, record: any) => <Link to={`/apps/${record.id}`}>{text}</Link> }, { title: '标识', dataIndex: 'name' }]} /> },
        { key: 'scan', label: 'Java 扫描', children: <ScanPanel refName={ref} setRefName={setRef} loading={isFetching} suggestions={suggestions} /> }
      ]} />
    </>
  );
}

function Overview({ repo }: { repo: any }) {
  return (
    <Card>
      <Space direction="vertical" size={12} className="full-width">
        <Typography.Text><CodeOutlined /> 仓库地址</Typography.Text>
        <Input readOnly value={repo?.httpUrl || '-'} />
        <Input readOnly value={repo?.sshUrl || '-'} />
        <Typography.Text type="secondary">源码仓库由平台托管在 GitLab 中，构建、发布和权限同步仍由 PaaS 控制。</Typography.Text>
      </Space>
    </Card>
  );
}

function ScanPanel({ refName, setRefName, loading, suggestions }: { refName: string; setRefName: (value: string) => void; loading: boolean; suggestions: any[] }) {
  return (
    <Card>
      <Space direction="vertical" size={12} className="full-width">
        <Input addonBefore="扫描引用" value={refName} onChange={(event) => setRefName(event.target.value)} />
        <Table
          rowKey={(record) => `${record.sourcePath}-${record.buildCommand}`}
          loading={loading}
          dataSource={suggestions}
          pagination={false}
          columns={[
            { title: '源码路径', dataIndex: 'sourcePath' },
            { title: '构建命令', dataIndex: 'buildCommand' },
            { title: '产物拷贝命令', dataIndex: 'artifactCopyCommand' },
            { title: '运行时镜像', dataIndex: 'runtimeBaseImage' },
            { title: '证据', dataIndex: 'evidence', render: (items: string[]) => (items || []).join('、') || '-' }
          ]}
        />
      </Space>
    </Card>
  );
}
