import { Navigate, Route, Routes } from "react-router-dom";

import { AuthGuard } from "@/components/AuthGuard";
import { LibraryPage } from "@/pages/LibraryPage";
import { LoginCallbackPage } from "@/pages/LoginCallbackPage";
import { LoginPage } from "@/pages/LoginPage";

export default function App() {
  return (
    <Routes>
      <Route path="/auth/callback" element={<LoginCallbackPage />} />
      <Route path="/login" element={<LoginPage />} />
      <Route element={<AuthGuard />}>
        <Route path="/" element={<Navigate to="/library" replace />} />
        <Route path="/library/*" element={<LibraryPage />} />
      </Route>
    </Routes>
  );
}
