import { Navigate, Route, Routes } from 'react-router-dom';
import { Shell } from './components/Shell';
import { PlatformSelectionProvider } from './contexts/PlatformSelectionContext';
import { HomePage } from './pages/HomePage';
import { PipelinePage } from './pages/PipelinePage';
import { DeploymentPage } from './pages/DeploymentPage';
import { DeliveryTemplatePage } from './pages/DeliveryTemplatePage';
import { ClustersPage } from './pages/ClustersPage';

export function App() {
  return (
    <PlatformSelectionProvider>
      <Routes>
        <Route element={<Shell />}>
          <Route index element={<HomePage />} />
          <Route path="/pipelines" element={<PipelinePage />} />
          <Route path="/deployments" element={<DeploymentPage />} />
          <Route path="/delivery-template" element={<DeliveryTemplatePage />} />
          <Route path="/clusters" element={<ClustersPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </PlatformSelectionProvider>
  );
}
