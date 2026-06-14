import { AppstoreOutlined, BuildOutlined, CloudUploadOutlined, DeploymentUnitOutlined, FileSearchOutlined, FolderOpenOutlined, MenuOutlined, SettingOutlined, TeamOutlined } from '@ant-design/icons';
import { Layout, Menu } from 'antd';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

const { Sider, Content } = Layout;

const platformItems = [
  { key: '/projects', icon: <FolderOpenOutlined />, label: '项目' },
  {
    key: '/apps',
    icon: <DeploymentUnitOutlined />,
    label: '应用',
    children: [
      { key: 'app-build', icon: <BuildOutlined />, label: '构建' },
      { key: 'app-deploy', icon: <CloudUploadOutlined />, label: '部署' },
      { key: 'app-config', icon: <SettingOutlined />, label: '配置' }
    ]
  }
];

const adminItems = [
  { key: '/tenants', icon: <TeamOutlined />, label: '租户管理' },
  { key: '/delivery-flow-template', icon: <DeploymentUnitOutlined />, label: '交付流模板' },
  { key: '/jenkins-templates', icon: <BuildOutlined />, label: '构建管理' },
  { key: '/audit', icon: <FileSearchOutlined />, label: '审计日志' },
  { key: '/settings', icon: <SettingOutlined />, label: '设置' }
];

export function ConsoleLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const appId = currentApplicationId(location.pathname);
  const handlePlatformMenuClick = ({ key }: { key: string }) => {
    if (key === 'app-build' || key === 'app-deploy' || key === 'app-config') {
      if (!appId) {
        navigate('/apps');
        return;
      }
      const section = key.replace('app-', '');
      navigate(`/apps/${appId}/${section}`);
      return;
    }
    navigate(key);
  };

  return (
    <Layout className="console-shell">
      <Sider width={256} className="console-sider">
        <div className="brand"><MenuOutlined className="brand-menu" />平台控制台</div>
        <div className="nav-section">平台</div>
        <Menu theme="dark" mode="inline" selectedKeys={[selectedKey(location.pathname)]} defaultOpenKeys={['/apps']} items={platformItems} onClick={handlePlatformMenuClick} />
        <div className="nav-section">平台管理</div>
        <Menu theme="dark" mode="inline" selectedKeys={[selectedKey(location.pathname)]} items={adminItems} onClick={({ key }) => navigate(key)} />
        <div className="sider-footer"><AppstoreOutlined />收起导航</div>
      </Sider>
      <Layout>
        <Content className="console-content">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}

function selectedKey(pathname: string) {
  if (pathname.startsWith('/projects')) return '/projects';
  if (pathname.startsWith('/source-repositories')) return '/projects';
  if (pathname.startsWith('/apps/new')) return '/apps';
  if (/^\/apps\/[^/]+\/build/.test(pathname)) return 'app-build';
  if (/^\/apps\/[^/]+\/deploy/.test(pathname)) return 'app-deploy';
  if (/^\/apps\/[^/]+\/config/.test(pathname)) return 'app-config';
  if (/^\/apps\/[^/]+\/promotions/.test(pathname)) return 'app-deploy';
  if (pathname.startsWith('/apps')) return '/apps';
  if (pathname.startsWith('/freights')) return '/freights';
  if (pathname.startsWith('/audit')) return '/audit';
  if (pathname.startsWith('/tenants')) return '/tenants';
  if (pathname.startsWith('/delivery-flow-template')) return '/delivery-flow-template';
  if (pathname.startsWith('/jenkins-templates')) return '/jenkins-templates';
  if (pathname.startsWith('/settings')) return '/settings';
  return pathname;
}

function currentApplicationId(pathname: string) {
  const match = pathname.match(/^\/apps\/([^/]+)/);
  if (!match || match[1] === 'new') return '';
  return decodeURIComponent(match[1]);
}
