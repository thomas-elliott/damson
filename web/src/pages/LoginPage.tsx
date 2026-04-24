import { useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";

import { useAuthMode } from "@/hooks/useDataQueries";
import { localLogin } from "@/lib/localAuth";

export function LoginPage() {
  const navigate = useNavigate();
  const { mode, needsBootstrap, loading: modeLoading } = useAuthMode();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // On a fresh local-mode deployment, there's nothing to log in as yet.
  // Bounce the user to the bootstrap flow so they create the first admin.
  if (!modeLoading && mode === "local" && needsBootstrap) {
    return <Navigate to="/bootstrap" replace />;
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await localLogin(email, password);
      navigate("/", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "login failed");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg">
      <form
        onSubmit={onSubmit}
        className="w-full max-w-sm rounded-md border border-line bg-panel p-6"
      >
        <h1 className="mb-6 text-lg font-semibold">Damson</h1>

        <label className="mb-3 block text-xs font-medium text-text-3">
          Email
          <input
            type="email"
            autoComplete="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            className="mt-1 block w-full rounded border border-line bg-surface px-3 py-2 text-sm text-text placeholder:text-text-4 focus:border-accent focus:outline-none"
          />
        </label>

        <label className="mb-4 block text-xs font-medium text-text-3">
          Password
          <input
            type="password"
            autoComplete="current-password"
            required
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="mt-1 block w-full rounded border border-line bg-surface px-3 py-2 text-sm text-text placeholder:text-text-4 focus:border-accent focus:outline-none"
          />
        </label>

        {error && (
          <p className="mb-3 text-xs text-err">{error}</p>
        )}

        <button
          type="submit"
          disabled={submitting}
          className="w-full rounded bg-accent px-3 py-2 text-sm font-medium text-accent-fg disabled:opacity-60"
        >
          {submitting ? "Signing in…" : "Sign in"}
        </button>
      </form>
    </div>
  );
}
