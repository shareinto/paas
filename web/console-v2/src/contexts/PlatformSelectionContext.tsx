import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useSearchParams } from 'react-router-dom';
import { platformContexts } from '../data/mock';
import { loadPlatformContexts, type PlatformContextSource, type PlatformTenant } from '../api/platform';

type Tenant = PlatformTenant;
type Project = Tenant['projects'][number];
type Application = Project['applications'][number];

type PlatformSelectionContextValue = {
  tenants: PlatformTenant[];
  currentTenant: Tenant;
  currentProject: Project;
  currentApplication: Application;
  loading: boolean;
  source: PlatformContextSource;
  error?: string;
  setTenant: (tenantId: string) => void;
  setProject: (projectId: string) => void;
  setApplication: (applicationId: string) => void;
  refreshContexts: () => Promise<void>;
};

const STORAGE_KEY = 'paas-console-v2-context';

const PlatformSelectionContext = createContext<PlatformSelectionContextValue | null>(null);

function readStoredSelection() {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) as Partial<Record<'tenantId' | 'projectId' | 'applicationId', string>> : {};
  } catch {
    return {};
  }
}

function resolveSelection(
  tenants: PlatformTenant[],
  ids: Partial<Record<'tenantId' | 'projectId' | 'applicationId', string>>
) {
  const safeTenants = hasUsableContext(tenants) ? tenants : platformContexts;
  const currentTenant = safeTenants.find((tenant) => tenant.id === ids.tenantId) || safeTenants[0];
  const currentProject = currentTenant.projects.find((project) => project.id === ids.projectId) || currentTenant.projects[0];
  const currentApplication =
    currentProject.applications.find((application) => application.id === ids.applicationId) || currentProject.applications[0];

  return { currentTenant, currentProject, currentApplication };
}

export function PlatformSelectionProvider({ children }: { children: ReactNode }) {
  const [searchParams, setSearchParams] = useSearchParams();
  const [tenants, setTenants] = useState<PlatformTenant[]>(platformContexts);
  const [source, setSource] = useState<PlatformContextSource>('mock');
  const [error, setError] = useState<string | undefined>();
  const [loading, setLoading] = useState(true);

  async function refreshContexts() {
    setLoading(true);
    const result = await loadPlatformContexts();
    setTenants(hasUsableContext(result.tenants) ? result.tenants : platformContexts);
    setSource(result.source);
    setError(result.error);
    setLoading(false);
  }

  useEffect(() => {
    let alive = true;
    setLoading(true);
    loadPlatformContexts()
      .then((result) => {
        if (!alive) return;
        setTenants(hasUsableContext(result.tenants) ? result.tenants : platformContexts);
        setSource(result.source);
        setError(result.error);
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);

  const storedSelection = useMemo(readStoredSelection, []);
  const ids = {
    tenantId: searchParams.get('tenantId') || storedSelection.tenantId,
    projectId: searchParams.get('projectId') || storedSelection.projectId,
    applicationId: searchParams.get('applicationId') || storedSelection.applicationId
  };
  const resolved = resolveSelection(tenants, ids);

  useEffect(() => {
    const payload = {
      tenantId: resolved.currentTenant.id,
      projectId: resolved.currentProject.id,
      applicationId: resolved.currentApplication.id
    };
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(payload));
  }, [resolved.currentTenant.id, resolved.currentProject.id, resolved.currentApplication.id]);

  useEffect(() => {
    const tenantId = searchParams.get('tenantId');
    const projectId = searchParams.get('projectId');
    const applicationId = searchParams.get('applicationId');
    if (
      tenantId === resolved.currentTenant.id &&
      projectId === resolved.currentProject.id &&
      applicationId === resolved.currentApplication.id
    ) {
      return;
    }
    const nextParams = new URLSearchParams(searchParams);
    nextParams.set('tenantId', resolved.currentTenant.id);
    nextParams.set('projectId', resolved.currentProject.id);
    nextParams.set('applicationId', resolved.currentApplication.id);
    setSearchParams(nextParams, { replace: true });
  }, [resolved.currentTenant.id, resolved.currentProject.id, resolved.currentApplication.id, searchParams, setSearchParams]);

  function commitSelection(nextIds: Partial<Record<'tenantId' | 'projectId' | 'applicationId', string>>) {
    const nextSelection = resolveSelection(tenants, nextIds);
    const nextParams = new URLSearchParams(searchParams);
    nextParams.set('tenantId', nextSelection.currentTenant.id);
    nextParams.set('projectId', nextSelection.currentProject.id);
    nextParams.set('applicationId', nextSelection.currentApplication.id);
    setSearchParams(nextParams, { replace: false });
  }

  const value = useMemo<PlatformSelectionContextValue>(() => ({
    tenants,
    ...resolved,
    loading,
    source,
    error,
    setTenant: (tenantId) => commitSelection({ tenantId }),
    setProject: (projectId) => commitSelection({ tenantId: resolved.currentTenant.id, projectId }),
    setApplication: (applicationId) =>
      commitSelection({
        tenantId: resolved.currentTenant.id,
        projectId: resolved.currentProject.id,
        applicationId
      }),
    refreshContexts
  }), [tenants, resolved, loading, source, error, searchParams, setSearchParams]);

  return <PlatformSelectionContext.Provider value={value}>{children}</PlatformSelectionContext.Provider>;
}

export function usePlatformSelection() {
  const context = useContext(PlatformSelectionContext);
  if (!context) {
    throw new Error('usePlatformSelection must be used within PlatformSelectionProvider');
  }
  return context;
}

function hasUsableContext(tenants: PlatformTenant[]) {
  return tenants.some((tenant) => tenant.projects.some((project) => project.applications.length > 0));
}
