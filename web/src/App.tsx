import { Navigate, Route, Routes } from "react-router";

import { AdminAuditPage } from "@/pages/AdminAuditPage";
import { AuditPage } from "@/pages/AuditPage";
import { AuthGuard } from "@/components/AuthGuard";
import { BootstrapPage } from "@/pages/BootstrapPage";
import { CollectionDetailPage } from "@/pages/CollectionDetailPage";
import { CollectionsPage } from "@/pages/CollectionsPage";
import { LibraryPage } from "@/pages/LibraryPage";
import { LoginCallbackPage } from "@/pages/LoginCallbackPage";
import { LoginPage } from "@/pages/LoginPage";
import { ProjectDetailPage } from "@/pages/ProjectDetailPage";
import { ProjectsPage } from "@/pages/ProjectsPage";
import { QueuePage } from "@/pages/QueuePage";
import { SettingsPage } from "@/pages/SettingsPage";

export default function App() {
  return (
    <Routes>
      <Route path="/auth/callback" element={<LoginCallbackPage />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/bootstrap" element={<BootstrapPage />} />
      <Route element={<AuthGuard />}>
        <Route path="/" element={<Navigate to="/library" replace />} />
        <Route path="/library" element={<LibraryPage />} />
        <Route path="/library/:assetId" element={<LibraryPage />} />
        <Route path="/queue" element={<QueuePage />} />
        <Route path="/projects" element={<ProjectsPage />} />
        <Route path="/projects/:id" element={<ProjectDetailPage />} />
        <Route path="/collections" element={<CollectionsPage />} />
        <Route path="/collections/:id" element={<CollectionDetailPage />} />
        <Route path="/audit" element={<AuditPage />} />
        <Route path="/admin/audit" element={<AdminAuditPage />} />
        <Route path="/settings" element={<SettingsPage />} />
      </Route>
    </Routes>
  );
}
