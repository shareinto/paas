import { AppstoreOutlined, BuildOutlined, ClusterOutlined, DeploymentUnitOutlined, FileSearchOutlined, FolderOpenOutlined, LogoutOutlined, MenuOutlined, SafetyCertificateOutlined, SettingOutlined, TeamOutlined } from '@ant-design/icons';
import { createContext, useContext, useEffect, useMemo, useState } from 'react';
import { Button, Layout, Menu, Tabs } from 'antd';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import { useSession } from '../app/store';

const { Sider, Content } = Layout;

type ApplicationTab = { id: string; title: string };
type ApplicationTabsContextValue = {
  openApplicationTab: (tab: ApplicationTab) => void;
  updateApplicationTabTitle: (id: string, title: string) => void;
};

const ApplicationTabsContext = createContext<ApplicationTabsContextValue>({
  openApplicationTab: () => undefined,
  updateApplicationTabTitle: () => undefined
});

export function useApplicationTabs() {
  return useContext(ApplicationTabsContext);
}

const platformItems = [
  { key: '/projects', icon: <FolderOpenOutlined />, label: '项目' },
  { key: '/apps', icon: <DeploymentUnitOutlined />, label: '应用' }
];

const adminItems = [
  { key: '/tenants', icon: <TeamOutlined />, label: '租户管理' },
  { key: '/roles', icon: <SafetyCertificateOutlined />, label: '角色权限' },
  { key: '/clusters', icon: <ClusterOutlined />, label: '集群管理' },
  { key: '/delivery-flow-template', icon: <DeploymentUnitOutlined />, label: '交付流模板' },
  { key: '/jenkins-templates', icon: <BuildOutlined />, label: '构建管理' },
  { key: '/audit', icon: <FileSearchOutlined />, label: '审计日志' },
  { key: '/settings', icon: <SettingOutlined />, label: '设置' }
];

export function ConsoleLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const clearSession = useSession((state) => state.clear);
  const activeApplicationId = currentApplicationDetailId(location.pathname);
  const [applicationTabs, setApplicationTabs] = useState<ApplicationTab[]>([]);
  const tabsContext = useMemo<ApplicationTabsContextValue>(() => ({
    openApplicationTab: (tab) => {
      setApplicationTabs((current) => {
        if (current.some((item) => item.id === tab.id)) return current;
        return [...current, tab];
      });
    },
    updateApplicationTabTitle: (id, title) => {
      setApplicationTabs((current) => current.map((item) => item.id === id && item.title !== title ? { ...item, title } : item));
    }
  }), []);

  useEffect(() => {
    if (!activeApplicationId) return;
    tabsContext.openApplicationTab({ id: activeApplicationId, title: activeApplicationId });
  }, [activeApplicationId, tabsContext]);

  const handlePlatformMenuClick = ({ key }: { key: string }) => {
    navigate(key);
  };
  const handleLogout = () => {
    clearSession();
    navigate('/login', { replace: true });
  };
  const handleCloseApplicationTab = (targetKey: string) => {
    setApplicationTabs((current) => {
      const index = current.findIndex((tab) => tab.id === targetKey);
      const nextTabs = current.filter((tab) => tab.id !== targetKey);
      if (targetKey === activeApplicationId) {
        const nextActive = nextTabs[index - 1] || nextTabs[index] || null;
        navigate(nextActive ? `/apps/${nextActive.id}` : '/apps');
      }
      return nextTabs;
    });
  };

  return (
    <ApplicationTabsContext.Provider value={tabsContext}>
      <Layout className="console-shell">
        <Sider width={256} className="console-sider">
          <div className="brand"><MenuOutlined className="brand-menu" />CloudDeliver</div>
          <div className="sider-menu-stack">
            <div className="nav-section">平台</div>
            <Menu theme="dark" mode="inline" selectedKeys={[selectedKey(location.pathname)]} items={platformItems} onClick={handlePlatformMenuClick} />
            <div className="nav-section">平台管理</div>
            <Menu theme="dark" mode="inline" selectedKeys={[selectedKey(location.pathname)]} items={adminItems} onClick={({ key }) => navigate(key)} />
          </div>
          <div className="sider-footer">
            <div className="sider-footer-item"><AppstoreOutlined />收起导航</div>
            <Button className="sider-logout" type="text" icon={<LogoutOutlined />} onClick={handleLogout}>退出登录</Button>
          </div>
        </Sider>
        <Layout>
          <Content className="console-content">
            {applicationTabs.length > 0 && (
              <Tabs
                data-testid="application-detail-tabs"
                className="application-detail-tabs"
                type="editable-card"
                hideAdd
                activeKey={activeApplicationId || undefined}
                onChange={(key) => navigate(`/apps/${key}`)}
                onEdit={(targetKey, action) => {
                  if (action === 'remove' && typeof targetKey === 'string') handleCloseApplicationTab(targetKey);
                }}
                items={applicationTabs.map((tab) => ({ key: tab.id, label: tab.title, closable: true }))}
              />
            )}
            <Outlet />
          </Content>
        </Layout>
      </Layout>
    </ApplicationTabsContext.Provider>
  );
}

function selectedKey(pathname: string) {
  if (pathname.startsWith('/projects')) return '/projects';
  if (pathname.startsWith('/source-repositories')) return '/projects';
  if (pathname.startsWith('/apps/new')) return '/apps';
  if (pathname.startsWith('/apps')) return '/apps';
  if (pathname.startsWith('/freights')) return '/freights';
  if (pathname.startsWith('/audit')) return '/audit';
  if (pathname.startsWith('/tenants')) return '/tenants';
  if (pathname.startsWith('/roles')) return '/roles';
  if (pathname.startsWith('/clusters')) return '/clusters';
  if (pathname.startsWith('/delivery-flow-template')) return '/delivery-flow-template';
  if (pathname.startsWith('/jenkins-templates')) return '/jenkins-templates';
  if (pathname.startsWith('/settings')) return '/settings';
  return pathname;
}

function currentApplicationDetailId(pathname: string) {
  const match = pathname.match(/^\/apps\/([^/]+)(?:\/(build|deploy|config|promotions))?\/?$/);
  if (!match || match[1] === 'new') return '';
  return decodeURIComponent(match[1]);
}
