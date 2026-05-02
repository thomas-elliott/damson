import { useState } from "react";
import { Navigate, useNavigate } from "react-router";

import { Button, FieldRow, Input } from "@/design-system";
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
      setError(err instanceof Error ? err.message : "Login failed.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="grid min-h-screen place-items-center bg-bg px-4">
      <div className="w-full max-w-sm rounded-lg border border-line bg-panel p-6 shadow-elevated">
        <div className="mb-5 flex items-center gap-2.5">
          <div className="grid h-[30px] w-[30px] place-items-center rounded-md bg-gradient-to-br from-accent to-accent-2 font-mono text-ui font-bold text-accent-fg">
            D
          </div>
          <h1 className="text-subhead text-text">Damson</h1>
        </div>
        <form onSubmit={onSubmit} className="flex flex-col gap-3.5">
          <FieldRow label="Email">
            <Input
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </FieldRow>
          <FieldRow label="Password">
            <Input
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </FieldRow>
          {error && <p className="text-small text-err">{error}</p>}
          <Button type="submit" variant="primary" disabled={submitting} className="mt-1 w-full">
            {submitting ? "Signing in…" : "Sign in"}
          </Button>
        </form>
      </div>
    </div>
  );
}
