import { useEffect, useMemo, useRef, useState } from 'react';
import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import {
  Bell,
  Boxes,
  ChevronDown,
  ChevronsLeft,
  ChevronsRight,
  CircleHelp,
  Cloud,
  GitPullRequest,
  Hammer,
  Home,
  LogOut,
  Menu,
  Rocket,
  Route,
  Search,
  Settings,
  Shield,
  ShieldCheck,
  User,
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
      { label: '权限管理', to: '/rbac', icon: ShieldCheck },
      { label: '构建管理', to: '/build-management', icon: Hammer },
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
              label="云环境"
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
            <ApplicationSearchSelect
              label="应用"
              value={currentApplication.id}
              onChange={setApplication}
              options={currentProject.applications.map((application) => ({ value: application.id, label: application.name }))}
            />
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
            <UserMenu />
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

function ApplicationSearchSelect({
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
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const rootRef = useRef<HTMLLabelElement | null>(null);
  const selectedLabel = options.find((option) => option.value === value)?.label || '未选择';
  const filteredOptions = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    if (!keyword) return options;
    return options.filter((option) => option.label.toLowerCase().includes(keyword));
  }, [options, query]);

  useEffect(() => {
    if (!open) return;
    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
        setQuery('');
      }
    };
    window.addEventListener('mousedown', closeOnOutsideClick);
    return () => window.removeEventListener('mousedown', closeOnOutsideClick);
  }, [open]);

  return (
    <label ref={rootRef} className="relative flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
      <span className="shrink-0">{label}</span>
      <button
        type="button"
        onClick={() => setOpen((nextOpen) => !nextOpen)}
        className="flex h-7 max-w-[180px] items-center gap-1 rounded border-0 bg-transparent px-1 text-sm font-medium text-foreground outline-none transition-colors hover:bg-muted focus:bg-muted"
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <span className="truncate">{selectedLabel}</span>
        <ChevronDown className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
      </button>
      {open && (
        <div className="absolute left-0 top-8 z-50 w-72 rounded-md border bg-popover p-2 text-popover-foreground shadow-lg">
          <Input
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === 'Escape') {
                setOpen(false);
                setQuery('');
              }
            }}
            placeholder="搜索应用"
            className="h-8"
          />
          <div className="mt-2 max-h-64 overflow-y-auto" role="listbox" aria-label="应用">
            {filteredOptions.length > 0 ? (
              filteredOptions.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  role="option"
                  aria-selected={option.value === value}
                  onClick={() => {
                    onChange(option.value);
                    setOpen(false);
                    setQuery('');
                  }}
                  className={cn(
                    'flex w-full items-center rounded px-2 py-1.5 text-left text-sm transition-colors hover:bg-muted',
                    option.value === value && 'bg-muted font-medium text-foreground'
                  )}
                >
                  <span className="truncate">{option.label}</span>
                </button>
              ))
            ) : (
              <div className="px-2 py-6 text-center text-sm text-muted-foreground">没有匹配的应用</div>
            )}
          </div>
        </div>
      )}
    </label>
  );
}

function UserMenu() {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const navigate = useNavigate();

  const userName = localStorage.getItem('paas_user_name') || '';
  const userId = localStorage.getItem('paas_actor_id') || '';

  // Generate initials from userName or userId
  const initials = useMemo(() => {
    if (userName) {
      const parts = userName.trim().split(/[\s_-]+/);
      if (parts.length >= 2) {
        return (parts[0][0] + parts[1][0]).toUpperCase();
      }
      return userName.slice(0, 2).toUpperCase();
    }
    if (userId) {
      return userId.slice(0, 2).toUpperCase();
    }
    return '?';
  }, [userName, userId]);

  useEffect(() => {
    if (!open) return;
    const closeOnOutsideClick = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    window.addEventListener('mousedown', closeOnOutsideClick);
    return () => window.removeEventListener('mousedown', closeOnOutsideClick);
  }, [open]);

  const handleLogout = () => {
    localStorage.removeItem('paas_token');
    localStorage.removeItem('paas_refresh_token');
    localStorage.removeItem('paas_token_expires');
    localStorage.removeItem('paas_user_name');
    localStorage.removeItem('paas_actor_id');
    navigate('/login', { replace: true });
  };

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        className="flex h-9 items-center gap-2 rounded-md border bg-card px-2 shadow-control transition-colors hover:bg-muted"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="用户菜单"
      >
        <span className="flex h-7 w-7 items-center justify-center rounded-full bg-slate-900 text-xs font-semibold text-white">
          {initials}
        </span>
        <ChevronDown className={cn('h-4 w-4 text-muted-foreground transition-transform', open && 'rotate-180')} />
      </button>
      {open && (
        <div className="absolute right-0 top-11 z-50 w-64 rounded-md border bg-popover text-popover-foreground shadow-lg" role="menu">
          {/* User info section */}
          <div className="border-b px-4 py-3">
            <div className="flex items-center gap-3">
              <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-slate-900 text-sm font-semibold text-white">
                {initials}
              </span>
              <div className="min-w-0">
                <div className="truncate text-sm font-medium text-foreground">
                  {userName || '未知用户'}
                </div>
                <div className="truncate text-xs text-muted-foreground">
                  {userId || '—'}
                </div>
              </div>
            </div>
          </div>
          {/* Menu items */}
          <div className="py-1">
            <button
              type="button"
              role="menuitem"
              onClick={() => {
                setOpen(false);
                // Future: navigate to profile page
              }}
              className="flex w-full items-center gap-2 px-4 py-2 text-sm text-foreground transition-colors hover:bg-muted"
            >
              <User className="h-4 w-4 text-muted-foreground" />
              <span>个人信息</span>
            </button>
            <button
              type="button"
              role="menuitem"
              onClick={handleLogout}
              className="flex w-full items-center gap-2 px-4 py-2 text-sm text-red-600 transition-colors hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-950/20"
            >
              <LogOut className="h-4 w-4" />
              <span>退出登录</span>
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
