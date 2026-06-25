import { useEffect, useState } from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import {
  Bell,
  Boxes,
  ChevronDown,
  ChevronsLeft,
  ChevronsRight,
  CircleHelp,
  Cloud,
  GitPullRequest,
  Home,
  Menu,
  Rocket,
  Route,
  Search,
  Settings,
  Shield,
  Workflow
} from 'lucide-react';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { usePlatformSelection } from '../contexts/PlatformSelectionContext';
import { cn } from '../lib/utils';

const navGroups = [
  {
    label: '平台',
    items: [
      { label: '首页', to: '/', icon: Home },
      { label: '应用', to: '/applications', icon: Boxes },
      { label: '流水线', to: '/pipelines', icon: Workflow },
      { label: '部署', to: '/deployments', icon: Rocket },
      { label: '发布单', to: '/freights', icon: GitPullRequest },
      { label: '审计', to: '/audit', icon: Shield }
    ]
  },
  {
    label: '平台管理',
    items: [
      { label: '交付模板', to: '/delivery-template', icon: Route },
      { label: '集群管理', to: '/clusters', icon: Cloud },
      { label: '设置', to: '/settings', icon: Settings }
    ]
  }
];

const NAV_COLLAPSED_STORAGE_KEY = 'paas-console-v2-nav-collapsed';

export function Shell() {
  const [navCollapsed, setNavCollapsed] = useState(() => {
    if (typeof window === 'undefined') {
      return false;
    }
    return window.localStorage.getItem(NAV_COLLAPSED_STORAGE_KEY) === 'true';
  });
  const {
    tenants,
    currentTenant,
    currentProject,
    currentApplication,
    loading,
    source,
    error,
    setTenant,
    setProject,
    setApplication
  } = usePlatformSelection();

  useEffect(() => {
    window.localStorage.setItem(NAV_COLLAPSED_STORAGE_KEY, String(navCollapsed));
  }, [navCollapsed]);

  return (
    <div className="min-h-screen bg-background">
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-30 hidden border-r border-white/10 bg-sidebar text-sidebar-foreground transition-[width] duration-200 lg:block',
          navCollapsed ? 'w-16' : 'w-56'
        )}
      >
        <div className={cn('flex h-14 items-center gap-3 border-b border-white/10', navCollapsed ? 'justify-center px-0' : 'px-5')}>
          <Menu className="h-5 w-5 text-slate-300" />
          {!navCollapsed && <div className="text-sm font-semibold tracking-wide">PaaS Console</div>}
        </div>
        <nav className={cn('space-y-6 py-5', navCollapsed ? 'px-2' : 'px-3')}>
          {navGroups.map((group) => (
            <div key={group.label} className="space-y-2">
              {!navCollapsed && (
                <div className="px-3 text-[11px] font-semibold uppercase tracking-[0.1em] text-slate-400">{group.label}</div>
              )}
              <div className="space-y-1">
                {group.items.map((item) => (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    title={navCollapsed ? item.label : undefined}
                    className={({ isActive }) =>
                      cn(
                        'flex h-10 items-center rounded-md text-sm text-slate-300 transition-colors hover:bg-sidebar-accent hover:text-white',
                        navCollapsed ? 'justify-center px-0' : 'gap-3 px-3',
                        isActive && 'bg-sidebar-accent text-white shadow-[inset_3px_0_0_hsl(var(--primary))]'
                      )
                    }
                  >
                    <item.icon className="h-4 w-4" />
                    {!navCollapsed && <span>{item.label}</span>}
                  </NavLink>
                ))}
              </div>
            </div>
          ))}
        </nav>
        <div className="absolute bottom-0 left-0 right-0 border-t border-white/10 p-3">
          <Button
            variant="ghost"
            className={cn(
              'w-full text-slate-300 hover:bg-sidebar-accent hover:text-white',
              navCollapsed ? 'justify-center px-0' : 'justify-start'
            )}
            title={navCollapsed ? '展开导航' : '收起导航'}
            aria-label={navCollapsed ? '展开导航' : '收起导航'}
            onClick={() => setNavCollapsed((collapsed) => !collapsed)}
          >
            {navCollapsed ? <ChevronsRight className="h-4 w-4" /> : <ChevronsLeft className="h-4 w-4" />}
            {!navCollapsed && <span className="ml-2">收起导航</span>}
          </Button>
        </div>
      </aside>

      <div className={cn('transition-[padding-left] duration-200', navCollapsed ? 'lg:pl-16' : 'lg:pl-56')}>
        <header className="sticky top-0 z-20 flex h-14 items-center gap-4 border-b bg-card/95 px-4 backdrop-blur lg:px-6">
          <Button variant="ghost" size="icon" className="lg:hidden">
            <Menu className="h-5 w-5" />
          </Button>
          <div className="flex min-w-0 items-center gap-2 rounded-md border bg-background/70 px-2 py-1 shadow-control">
            <ContextSelect
              label="租户"
              value={currentTenant.id}
              onChange={setTenant}
              options={tenants.map((tenant) => ({ value: tenant.id, label: tenant.name }))}
            />
            <span className="h-5 w-px bg-border" />
            <ContextSelect
              label="项目"
              value={currentProject.id}
              onChange={setProject}
              options={currentTenant.projects.map((project) => ({ value: project.id, label: project.name }))}
            />
            <span className="h-5 w-px bg-border" />
            <ContextSelect
              label="应用"
              value={currentApplication.id}
              onChange={setApplication}
              options={currentProject.applications.map((application) => ({ value: application.id, label: application.name }))}
            />
            <span
              className="ml-1 rounded border bg-muted px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground"
              title={error || (source === 'api' ? '当前上下文来自后端接口' : '当前上下文来自本地 mock 数据')}
            >
              {loading ? '加载中' : source === 'api' ? '后端' : 'Mock'}
            </span>
          </div>
          <div className="ml-auto flex flex-1 items-center justify-end gap-3">
            <div className="relative hidden w-full max-w-[420px] md:block">
              <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
              <Input className="pl-9 pr-12" placeholder="搜索应用、资源、文档..." />
              <kbd className="pointer-events-none absolute right-2 top-2 rounded border bg-muted px-1.5 text-[11px] text-muted-foreground">/</kbd>
            </div>
            <Button variant="ghost" size="icon">
              <CircleHelp className="h-5 w-5" />
            </Button>
            <Button variant="ghost" size="icon" className="relative">
              <Bell className="h-5 w-5" />
              <span className="absolute right-1.5 top-1.5 h-2 w-2 rounded-full bg-primary" />
            </Button>
            <button className="flex h-9 items-center gap-2 rounded-md border bg-card px-2 shadow-control">
              <span className="flex h-7 w-7 items-center justify-center rounded-full bg-slate-900 text-xs font-semibold text-white">AD</span>
              <ChevronDown className="h-4 w-4 text-muted-foreground" />
            </button>
          </div>
        </header>
        <main className="w-full min-w-0 px-4 py-5 lg:px-6 2xl:px-8">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

function ContextSelect({
  label,
  value,
  options,
  onChange
}: {
  label: string;
  value: string;
  options: Array<{ value: string; label: string }>;
  onChange: (value: string) => void;
}) {
  return (
    <label className="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
      <span className="shrink-0">{label}</span>
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-7 max-w-[160px] rounded border-0 bg-transparent px-1 text-sm font-medium text-foreground outline-none transition-colors hover:bg-muted focus:bg-muted"
        aria-label={label}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
}
