import { useMemo, useState } from "react";

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
  SimpleTooltip,
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
import { useSavedFlash } from "@/hooks/useSavedFlash";

import { StorageCleanupSection } from "./StorageCleanupSection";

type ServerSettings = NonNullable<ReturnType<typeof useServerSettings>["data"]>;

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
      <StorageCleanupSection />
    </>
  );
}

function ServerSettingsForm() {
  const { data: settings, loading } = useServerSettings();
  if (loading || !settings) {
    return (
      <Section title="Server settings" defaultOpen>
        <div className="text-caption">Loading…</div>
      </Section>
    );
  }
  // Mount the body only once settings are loaded so maxUpload can seed from
  // them in a useState initializer instead of a sync effect.
  return <ServerSettingsFormBody settings={settings} />;
}

function ServerSettingsFormBody({ settings }: { settings: ServerSettings }) {
  const patch = usePatchServerSettings();
  const [maxUpload, setMaxUpload] = useState(String(settings.max_upload_bytes));
  // Flash a "Saved" indicator after each successful save (event-driven).
  const { saved, flash } = useSavedFlash();

  return (
    <Section
      title="Server settings"
      defaultOpen
      accessory={
        saved ? (
          <Badge size="sm" tone="ok">
            Saved
          </Badge>
        ) : null
      }
    >
      <FieldRow
        label="Max upload bytes"
        hint="Hard cap applied to all uploads. 2 GiB = 2147483648."
      >
        <Input
          type="number"
          min={1}
          value={maxUpload}
          onChange={(e) => setMaxUpload(e.target.value)}
          onBlur={() => {
            const n = Number(maxUpload);
            if (!Number.isFinite(n) || n <= 0) return;
            if (n !== settings.max_upload_bytes) {
              patch.mutate({ max_upload_bytes: n }, { onSuccess: flash });
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
          onCheckedChange={(v) =>
            patch.mutate({ licence_kb_auto_update: !!v }, { onSuccess: flash })
          }
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
  // The server returns 409 on demote-to-user when this is the only
  // sysadmin; surface that rule in the UI so admins find out before
  // clicking. Defence-in-depth, not the only gate.
  const sysadminCount = useMemo(() => users.filter((u) => u.role === "sysadmin").length, [users]);

  return (
    <Section title={`Users (${users.length})`} defaultOpen>
      {loading ? (
        <div className="text-caption">Loading…</div>
      ) : users.length === 0 ? (
        <EmptyState title="No users yet" />
      ) : (
        <div className="flex flex-col">
          {users.map((u) => {
            const isLastSysadmin = u.role === "sysadmin" && sysadminCount <= 1;
            return (
              <div
                key={u.id}
                className="flex items-center gap-3 border-b border-line py-2 last:border-b-0"
              >
                <div className="min-w-0 flex-1">
                  <div className="truncate text-ui text-text">{u.display_name}</div>
                  <div className="truncate text-caption">{u.email}</div>
                </div>
                {isLastSysadmin ? (
                  <SimpleTooltip label="Last sysadmin — promote another user first." side="top">
                    <span>
                      <Select value={u.role} disabled>
                        <SelectTrigger aria-label="Global role" className="w-32">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="user">user</SelectItem>
                          <SelectItem value="sysadmin">sysadmin</SelectItem>
                        </SelectContent>
                      </Select>
                    </span>
                  </SimpleTooltip>
                ) : (
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
                )}
                {authMode === "local" && (
                  <Button size="sm" variant="ghost" onClick={() => setResetUser(u)}>
                    Reset password
                  </Button>
                )}
              </div>
            );
          })}
        </div>
      )}
      <ResetPasswordDialog
        user={resetUser}
        onClose={() => setResetUser(null)}
        onSubmit={(password) => {
          if (!resetUser) return;
          reset.mutate({ userId: resetUser.id, password }, { onSuccess: () => setResetUser(null) });
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
  // The password field lives in ResetPasswordForm, which Radix only mounts
  // while the dialog is open — so it starts empty on every open (one user at a
  // time, dialog closes between them) without a reset effect.
  return (
    <Dialog open={!!user} onOpenChange={(v) => !v && onClose()}>
      <DialogContent>
        {user && <ResetPasswordForm user={user} onClose={onClose} onSubmit={onSubmit} />}
      </DialogContent>
    </Dialog>
  );
}

function ResetPasswordForm({
  user,
  onClose,
  onSubmit,
}: {
  user: AdminUser;
  onClose: () => void;
  onSubmit: (password: string) => void;
}) {
  const [password, setPassword] = useState("");

  return (
    <>
      <DialogHeader>
        <DialogTitle>Reset password</DialogTitle>
      </DialogHeader>
      <DialogBody>
        <p className="text-ui text-text-2">
          Set a new password for <span className="font-mono">{user.email}</span>. Their existing
          sessions will be invalidated.
        </p>
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
        <Button variant="primary" onClick={() => onSubmit(password)} disabled={password.length < 8}>
          Reset
        </Button>
      </DialogFooter>
    </>
  );
}
