import { Navigate, Route, Routes } from 'react-router-dom';
import { useSession } from './store';
import { ConsoleLayout } from '../components/ConsoleLayout';
import { LoginPage } from '../pages/LoginPage';
import { OIDCCallbackPage } from '../pages/OIDCCallbackPage';
import { ProjectsPage } from '../pages/ProjectsPage';
import { ProjectDetailPage } from '../pages/ProjectDetailPage';
import { ApplicationsPage } from '../pages/ApplicationsPage';
import { CreateApplicationPage } from '../pages/CreateApplicationPage';
import { EditApplicationPage } from '../pages/EditApplicationPage';
import { ApplicationDetailPage } from '../pages/ApplicationDetailPage';
import { SourceRepositoriesPage } from '../pages/SourceRepositoriesPage';
import { SourceRepositoryDetailPage } from '../pages/SourceRepositoryDetailPage';
import { BuildDetailPage } from '../pages/BuildDetailPage';
import { BuildsPage } from '../pages/BuildsPage';
import { FreightsPage } from '../pages/FreightsPage';
import { PromotionPage } from '../pages/PromotionPage';
import { AuditPage } from '../pages/AuditPage';
import { TemplateConfigPage } from '../pages/TemplateConfigPage';
import { JenkinsTemplatesPage } from '../pages/JenkinsTemplatesPage';
import { TenantsPage } from '../pages/TenantsPage';

function Guard({ children }: { children: JSX.Element }) {
  const token = useSession((state) => state.token);
  return token ? children : <Navigate to="/login" replace />;
}

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/oidc/callback" element={<OIDCCallbackPage />} />
      <Route path="/" element={<Guard><ConsoleLayout /></Guard>}>
        <Route index element={<Navigate to="/projects" replace />} />
        <Route path="projects" element={<ProjectsPage />} />
        <Route path="projects/:id" element={<ProjectDetailPage />} />
        <Route path="source-repositories" element={<SourceRepositoriesPage />} />
        <Route path="source-repositories/:id" element={<SourceRepositoryDetailPage />} />
        <Route path="apps" element={<ApplicationsPage />} />
        <Route path="apps/new" element={<CreateApplicationPage />} />
        <Route path="apps/:id/edit" element={<EditApplicationPage />} />
        <Route path="apps/:id/promotions" element={<PromotionPage />} />
        <Route path="apps/:id" element={<ApplicationDetailPage />} />
        <Route path="builds" element={<BuildsPage />} />
        <Route path="builds/:id" element={<BuildDetailPage />} />
        <Route path="freights" element={<FreightsPage />} />
        <Route path="promotions" element={<Navigate to="/apps" replace />} />
        <Route path="audit" element={<AuditPage />} />
        <Route path="templates" element={<TemplateConfigPage />} />
        <Route path="tenants" element={<TenantsPage />} />
        <Route path="jenkins-templates" element={<JenkinsTemplatesPage />} />
      </Route>
    </Routes>
  );
}
