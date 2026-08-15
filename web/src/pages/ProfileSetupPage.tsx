import { useState } from "react";

import { Button, FieldRow, Input } from "@/design-system";
import type { Me } from "@/hooks/useDataQueries";
import { usePatchMe } from "@/hooks/useMutations";

/**
 * ProfileSetupPage is the first-login step shown when a user has no email yet
 * — i.e. the IdP authenticated them by `sub` but sent no email claim. OIDC
 * identity is fixed; this collects the app-owned profile. Email is required;
 * display name is optional and defaults to the email.
 *
 * On success usePatchMe seeds the fresh Me into the query cache, so the
 * ProfileGate in AuthGuard falls through to the app without any navigation.
 */
export function ProfileSetupPage({ me }: { me: Me }) {
  const patchMe = usePatchMe();
  const [email, setEmail] = useState(me.email);
  const [displayName, setDisplayName] = useState(me.display_name);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    try {
      await patchMe.mutateAsync({
        email: email.trim(),
        display_name: displayName.trim() || undefined,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Couldn't save your profile.");
    }
  }

  return (
    <div className="grid min-h-screen place-items-center bg-background px-4">
      <div className="w-full max-w-sm rounded-lg border border-border bg-card p-6 shadow-md">
        <h1 className="text-subhead-semibold text-foreground">Finish setting up your profile</h1>
        <p className="mt-1 text-caption">
          Your sign-in didn't include an email address. Add one so teammates can recognise you.
        </p>
        <form onSubmit={onSubmit} className="mt-5 flex flex-col gap-3.5">
          <FieldRow label="Email">
            <Input
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </FieldRow>
          <FieldRow label="Display name" hint="Optional — defaults to your email.">
            <Input
              type="text"
              autoComplete="name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
            />
          </FieldRow>
          {error && <p className="text-label text-destructive">{error}</p>}
          <Button
            type="submit"
            variant="primary"
            disabled={patchMe.isPending}
            className="mt-1 w-full"
          >
            {patchMe.isPending ? "Saving…" : "Continue"}
          </Button>
        </form>
      </div>
    </div>
  );
}
