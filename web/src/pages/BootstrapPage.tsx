import { useState } from "react";
import { useNavigate } from "react-router";
import { useQueryClient } from "@tanstack/react-query";

import { Button, FieldRow, Input } from "@/design-system";
import { localBootstrap } from "@/lib/localAuth";
import { queryKeys } from "@/lib/queryKeys";
import { type AuthModeInfo } from "@/lib/authMode";

/**
 * BootstrapPage is shown on a fresh local-mode deployment where no users
 * exist yet. It creates the first admin via POST /api/auth/bootstrap and
 * logs them straight in. Once any user exists the server flips
 * needs_bootstrap → false and this page is no longer reachable.
 */
export function BootstrapPage() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<{
    password?: string;
    confirm?: string;
  }>({});

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setFieldErrors({});
    if (password !== confirm) {
      setFieldErrors({ confirm: "Passwords don't match." });
      return;
    }
    if (password.length < 8) {
      setFieldErrors({ password: "Password must be at least 8 characters." });
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      await localBootstrap(email, password);
      // Synchronously seed the new auth-mode value so AuthGuard sees
      // needs_bootstrap=false on the next render. Awaiting
      // invalidateQueries here previously hung under React 19 + RQ5 when
      // the authMode query had no active subscriber, leaving the user on
      // a frozen submit button (PER-68). setQueryData is synchronous and
      // avoids that path entirely; the no-await invalidate keeps any
      // future subscribers honest.
      queryClient.setQueryData<AuthModeInfo>(queryKeys.authMode, {
        mode: "local",
        needs_bootstrap: false,
      });
      void queryClient.invalidateQueries({ queryKey: queryKeys.authMode });
      navigate("/", { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Bootstrap failed.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <AuthCard
      title="Create the first admin"
      description="Damson hasn't been set up yet. The account you create here becomes the server's first sysadmin."
    >
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
        <FieldRow label="Password" hint="At least 8 characters." error={fieldErrors.password}>
          <Input
            type="password"
            autoComplete="new-password"
            required
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </FieldRow>
        <FieldRow label="Confirm password" error={fieldErrors.confirm}>
          <Input
            type="password"
            autoComplete="new-password"
            required
            minLength={8}
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
          />
        </FieldRow>
        {error && <p className="text-small text-err">{error}</p>}
        <Button type="submit" variant="primary" disabled={submitting} className="mt-1 w-full">
          {submitting ? "Creating account…" : "Create account"}
        </Button>
      </form>
    </AuthCard>
  );
}

function AuthCard({
  title,
  description,
  children,
}: {
  title: string;
  description?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="grid min-h-screen place-items-center bg-bg px-4">
      <div className="w-full max-w-sm rounded-lg border border-line bg-panel p-6 shadow-elevated">
        <h1 className="text-subhead text-text">{title}</h1>
        {description && <p className="mt-1 text-caption">{description}</p>}
        <div className="mt-5">{children}</div>
      </div>
    </div>
  );
}
