import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";

import { invalidateAuthMode } from "@/lib/authMode";
import { localBootstrap } from "@/lib/localAuth";
import { queryKeys } from "@/lib/queryKeys";

// BootstrapPage is shown on a fresh local-mode deployment where no users
// exist yet. It creates the first admin via POST /api/auth/bootstrap and
// logs them straight in. Once any user exists the server flips
// needs_bootstrap → false and this page is no longer reachable.
export function BootstrapPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (password !== confirm) {
      setError("passwords don't match");
      return;
    }
    if (password.length < 8) {
      setError("password must be at least 8 characters");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await localBootstrap(email, password);
      invalidateAuthMode();
      await queryClient.invalidateQueries({ queryKey: queryKeys.authMode });
      navigate("/", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "bootstrap failed");
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
        <h1 className="mb-1 text-lg font-semibold">Create the first admin</h1>
        <p className="mb-6 text-xs text-text-3">
          Damson hasn't been set up yet. The account you create here becomes the
          server's first sysadmin.
        </p>

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

        <label className="mb-3 block text-xs font-medium text-text-3">
          Password
          <input
            type="password"
            autoComplete="new-password"
            required
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="mt-1 block w-full rounded border border-line bg-surface px-3 py-2 text-sm text-text placeholder:text-text-4 focus:border-accent focus:outline-none"
          />
        </label>

        <label className="mb-4 block text-xs font-medium text-text-3">
          Confirm password
          <input
            type="password"
            autoComplete="new-password"
            required
            minLength={8}
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
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
          {submitting ? "Creating account…" : "Create account"}
        </button>
      </form>
    </div>
  );
}
