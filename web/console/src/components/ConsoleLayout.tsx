import { AppstoreOutlined, BellOutlined, BuildOutlined, CloudServerOutlined, CodeOutlined, DeploymentUnitOutlined, DownOutlined, FileSearchOutlined, FolderOpenOutlined, MenuOutlined, PlusCircleOutlined, QuestionCircleOutlined, SearchOutlined, SettingOutlined, TagsOutlined, TeamOutlined } from '@ant-design/icons';
import { Avatar, Badge, Input, Layout, Menu, Space, Typography } from 'antd';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useSession } from '../app/store';

const { Header, Sider, Content } = Layout;

const platformItems = [
  { key: '/projects', icon: <FolderOpenOutlined />, label: '项目' },
  { key: '/source-repositories', icon: <CodeOutlined />, label: '源码仓库' },
  { key: '/apps', icon: <DeploymentUnitOutlined />, label: '应用' },
  { key: '/apps/new', icon: <PlusCircleOutlined />, label: '创建应用' },
  { key: '/builds', icon: <BuildOutlined />, label: '构建' },
  { key: '/freights', icon: <TagsOutlined />, label: '版本' },
  { key: '/promotions', icon: <CloudServerOutlined />, label: '发布晋级' },
  { key: '/templates', icon: <CodeOutlined />, label: '部署模板' },
  { key: '/audit', icon: <FileSearchOutlined />, label: '审计日志' }
];

const adminItems = [
  { key: '/tenants', icon: <TeamOutlined />, label: '租户管理' },
  { key: '/jenkins-templates', icon: <BuildOutlined />, label: '构建管理' },
  { key: '/settings', icon: <SettingOutlined />, label: '设置' }
];

export function ConsoleLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const session = useSession();
  return (
    <Layout className="console-shell">
      <Sider width={256} className="console-sider">
        <div className="brand"><MenuOutlined className="brand-menu" />平台控制台</div>
        <div className="nav-section">平台</div>
        <Menu theme="dark" mode="inline" selectedKeys={[selectedKey(location.pathname)]} items={platformItems} onClick={({ key }) => navigate(key)} />
        <div className="nav-section">平台管理</div>
        <Menu theme="dark" mode="inline" selectedKeys={[selectedKey(location.pathname)]} items={adminItems} onClick={({ key }) => navigate(key)} />
        <div className="sider-footer"><AppstoreOutlined />收起导航</div>
      </Sider>
      <Layout>
        <Header className="console-header">
          <div className="header-left">
            <div className="org-context">
              <span className="org-label">组织：</span>
              <Typography.Text strong>{session.tenantName}</Typography.Text>
              <DownOutlined />
            </div>
            <Input prefix={<SearchOutlined />} placeholder="搜索应用、资源、文档" className="global-search" />
          </div>
          <Space size={20} className="header-tools">
            <QuestionCircleOutlined />
            <Badge dot><BellOutlined /></Badge>
            <div className="user-chip">
              <Avatar>{session.userName.slice(0, 1)}</Avatar>
              <Typography.Text>{session.userName}</Typography.Text>
              <DownOutlined />
            </div>
          </Space>
        </Header>
        <Content className="console-content">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}

function selectedKey(pathname: string) {
  if (pathname.startsWith('/apps/new')) return '/apps/new';
  if (pathname.startsWith('/source-repositories')) return '/source-repositories';
  if (pathname.startsWith('/apps')) return '/apps';
  if (pathname.startsWith('/builds')) return '/builds';
  if (pathname.startsWith('/tenants')) return '/tenants';
  if (pathname.startsWith('/jenkins-templates')) return '/jenkins-templates';
  return pathname;
}
