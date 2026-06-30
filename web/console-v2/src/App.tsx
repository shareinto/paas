import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { Shell } from './components/Shell';
import { PlatformSelectionProvider } from './contexts/PlatformSelectionContext';
import { HomePage } from './pages/HomePage';
import { PipelinePage } from './pages/PipelinePage';
import { DeploymentPage } from './pages/DeploymentPage';
import { DeliveryTemplatePage } from './pages/DeliveryTemplatePage';
import { ClustersPage } from './pages/ClustersPage';
import { BuildManagementPage } from './pages/BuildManagementPage';
import { ApplicationsPage } from './pages/ApplicationsPage';
import { RBACManagementPage } from './pages/RBACManagementPage';
import { LoginPage } from './pages/LoginPage';

function RequireAuth({ children }: { children: React.ReactNode }) {
  const location = useLocation();
  const token = window.localStorage.getItem('paas_token');
  const expires = window.localStorage.getItem('paas_token_expires');

  const isValid = token && (!expires || new Date(expires) > new Date());

  if (!isValid) {
    // Clear stale token
    if (token && expires && new Date(expires) <= new Date()) {
      window.localStorage.removeItem('paas_token');
      window.localStorage.removeItem('paas_refresh_token');
      window.localStorage.removeItem('paas_token_expires');
    }
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <>{children}</>;
}

export function App() {
  return (
    <PlatformSelectionProvider>
      <Routes>
        {/* Login page — no Shell layout */}
        <Route path="/login" element={<LoginPage />} />

        {/* Protected routes — wrapped in Shell */}
        <Route
          element={
            <RequireAuth>
              <Shell />
            </RequireAuth>
          }
        >
          <Route index element={<HomePage />} />
          <Route path="/applications" element={<ApplicationsPage />} />
          <Route path="/pipelines" element={<PipelinePage />} />
          <Route path="/deployments" element={<DeploymentPage />} />
          <Route path="/rbac" element={<RBACManagementPage />} />
          <Route path="/build-management" element={<BuildManagementPage />} />
          <Route path="/delivery-template" element={<DeliveryTemplatePage />} />
          <Route path="/clusters" element={<ClustersPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </PlatformSelectionProvider>
  );
}
