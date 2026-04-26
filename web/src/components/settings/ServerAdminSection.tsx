import { useEffect, useState } from "react";

import {
  Badge,
  Button,
  Checkbox,
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  EmptyState,
  FieldRow,
  Input,
  Section,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/design-system";
import {
  useAdminUsers,
  useAuthMode,
  useServerSettings,
  type AdminUser,
} from "@/hooks/useDataQueries";
import {
  usePatchServerSettings,
  usePatchUserRole,
  useResetUserPassword,
} from "@/hooks/useMutations";

/**
 * Sysadmin-only sections rolled into one component so the SettingsPage can
 * mount them with a single import. Two surfaces: server settings (max
 * upload bytes + KB auto-update) and the user list (role changes + local
 * password resets).
 */
export function ServerAdminSection() {
  return (
    <>
      <ServerSettingsForm />
      <UsersSection />
    </>
  );
}

function ServerSettingsForm() {
  const { data: settings, loading } = useServerSettings();
  const patch = usePatchServerSettings();
  const [maxUpload, setMaxUpload] = useState("");
  const [savedAt, setSavedAt] = useState<number | null>(null);

  useEffect(() => {
    if (settings) setMaxUpload(String(settings.max_upload_bytes));
  }, [settings?.max_upload_bytes]);

  useEffect(() => {
    if (!patch.isSuccess) return;
    setSavedAt(Date.now());
    const t = window.setTimeout(() => setSavedAt(null), 1500);
    return () => window.clearTimeout(t);
  }, [patch.submittedAt, patch.isSuccess]);

  if (loading || !settings) {
    return (
      <Section title="Server settings" defaultOpen>
        <div className="text-caption">Loading…</div>
      </Section>
    );
  }
  return (
    <Section
      title="Server settings"
      defaultOpen
      accessory={savedAt ? <Badge size="sm" tone="ok">Saved</Badge> : null}
    >
      <FieldRow label="Max upload bytes" hint="Hard cap applied to all uploads. 2 GiB = 2147483648.">
        <Input
          type="number"
          min={1}
          value={maxUpload}
          onChange={(e) => setMaxUpload(e.target.value)}
          onBlur={() => {
            const n = Number(maxUpload);
            if (!Number.isFinite(n) || n <= 0) return;
            if (n !== settings.max_upload_bytes) {
              patch.mutate({ max_upload_bytes: n });
            }
          }}
          className="w-48"
        />
      </FieldRow>
      <div className="flex items-center justify-between gap-4 py-2">
        <div className="flex flex-col">
          <span className="text-ui text-text">Auto-update licence knowledge base</span>
          <span className="text-caption">
            When supported, periodically refresh the SPDX overlay from upstream.
          </span>
        </div>
        <Checkbox
          aria-label="Auto-update licence knowledge base"
          checked={settings.licence_kb_auto_update}
          onCheckedChange={(v) => patch.mutate({ licence_kb_auto_update: !!v })}
        />
      </div>
    </Section>
  );
}

function UsersSection() {
  const { data: users, loading } = useAdminUsers();
  const { mode: authMode } = useAuthMode();
  const patchRole = usePatchUserRole();
  const reset = useResetUserPassword();
  const [resetUser, setResetUser] = useState<AdminUser | null>(null);

  return (
    <Section title={`Users (${users.length})`} defaultOpen>
      {loading ? (
        <div className="text-caption">Loading…</div>
      ) : users.length === 0 ? (
        <EmptyState title="No users yet" />
      ) : (
        <div className="flex flex-col">
          {users.map((u) => (
            <div
              key={u.id}
              className="flex items-center gap-3 border-b border-line py-2 last:border-b-0"
            >
              <div className="min-w-0 flex-1">
                <div className="truncate text-ui text-text">{u.display_name}</div>
                <div className="truncate text-caption">{u.email}</div>
              </div>
              <Select
                value={u.role}
                onValueChange={(role) =>
                  patchRole.mutate({ userId: u.id, role: role as "user" | "sysadmin" })
                }
              >
                <SelectTrigger aria-label="Global role" className="w-32">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="user">user</SelectItem>
                  <SelectItem value="sysadmin">sysadmin</SelectItem>
                </SelectContent>
              </Select>
              {authMode === "local" && (
                <Button size="sm" variant="ghost" onClick={() => setResetUser(u)}>
                  Reset password
                </Button>
              )}
            </div>
          ))}
        </div>
      )}
      <ResetPasswordDialog
        user={resetUser}
        onClose={() => setResetUser(null)}
        onSubmit={(password) => {
          if (!resetUser) return;
          reset.mutate(
            { userId: resetUser.id, password },
            { onSuccess: () => setResetUser(null) },
          );
        }}
      />
    </Section>
  );
}

function ResetPasswordDialog({
  user,
  onClose,
  onSubmit,
}: {
  user: AdminUser | null;
  onClose: () => void;
  onSubmit: (password: string) => void;
}) {
  const [password, setPassword] = useState("");
  useEffect(() => setPassword(""), [user?.id]);

  return (
    <Dialog open={!!user} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Reset password</DialogTitle>
        </DialogHeader>
        <DialogBody>
          {user && (
            <p className="text-ui text-text-2">
              Set a new password for <span className="font-mono">{user.email}</span>. Their
              existing sessions will be invalidated.
            </p>
          )}
          <FieldRow label="New password">
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="At least 8 characters"
            />
          </FieldRow>
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="primary"
            onClick={() => onSubmit(password)}
            disabled={password.length < 8}
          >
            Reset
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
