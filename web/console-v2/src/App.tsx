import { Navigate, Route, Routes } from 'react-router-dom';
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

export function App() {
  return (
    <PlatformSelectionProvider>
      <Routes>
        <Route element={<Shell />}>
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
