import { useMemo, useState } from "react";
import { UserPlus, Trash2 } from "lucide-react";

import {
  Badge,
  Button,
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
  useConfirm,
} from "@/design-system";
import { useAuthMode, useLibraryMembers } from "@/hooks/useDataQueries";
import { useInviteMember, usePatchMember, useRemoveMember } from "@/hooks/useMutations";

/**
 * Library members section. Lists every membership; admins can invite, change
 * roles, and remove. Local-mode invites can create a brand-new user when
 * the email isn't on file (admin sets the password). OIDC-mode invites
 * require the user to have signed in at least once first — the form
 * surfaces that as inline help.
 */
export function LibraryMembersSection({
  libraryId,
  isAdmin,
}: {
  libraryId: number;
  isAdmin: boolean;
}) {
  const { data: members } = useLibraryMembers(libraryId);
  const [inviteOpen, setInviteOpen] = useState(false);
  const patch = usePatchMember();
  const remove = useRemoveMember();
  const { confirm, dialog: confirmDialog } = useConfirm();
  // Server returns 409 on demote/remove of the last admin — disabling the
  // UI gives users that signal before they click. Defence-in-depth, not
  // the only gate.
  const adminCount = useMemo(() => members.filter((m) => m.role === "admin").length, [members]);

  return (
    <Section
      title={`Members (${members.length})`}
      defaultOpen
      accessory={
        isAdmin ? (
          <Button
            size="sm"
            variant="primary"
            leadingIcon={<UserPlus size={12} />}
            onClick={() => setInviteOpen(true)}
          >
            Invite
          </Button>
        ) : null
      }
    >
      {members.length === 0 ? (
        <EmptyState title="No members yet" description="Library memberships will appear here." />
      ) : (
        <div className="flex flex-col">
          {members.map((m) => {
            const isLastAdmin = m.role === "admin" && adminCount <= 1;
            const lastAdminMsg = "Last admin — promote another member first.";
            return (
              <div
                key={m.user_id}
                className="flex items-center gap-3 border-b border-border py-2 last:border-b-0"
              >
                <div className="min-w-0 flex-1">
                  <div className="truncate text-ui text-foreground">{m.display_name}</div>
                  <div className="truncate text-caption">{m.email}</div>
                </div>
                {m.global_role === "sysadmin" && (
                  <Badge size="sm" tone="warn">
                    sysadmin
                  </Badge>
                )}
                {isAdmin ? (
                  isLastAdmin ? (
                    <SimpleTooltip label={lastAdminMsg} side="top">
                      <span>
                        <Select value={m.role} disabled>
                          <SelectTrigger aria-label="Role" className="w-28">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="admin">admin</SelectItem>
                            <SelectItem value="member">member</SelectItem>
                          </SelectContent>
                        </Select>
                      </span>
                    </SimpleTooltip>
                  ) : (
                    <Select
                      value={m.role}
                      onValueChange={(role) =>
                        patch.mutate({
                          libraryId,
                          userId: m.user_id,
                          role: role as "admin" | "member",
                        })
                      }
                    >
                      <SelectTrigger aria-label="Role" className="w-28">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="admin">admin</SelectItem>
                        <SelectItem value="member">member</SelectItem>
                      </SelectContent>
                    </Select>
                  )
                ) : (
                  <Badge size="sm" mono>
                    {m.role}
                  </Badge>
                )}
                {isAdmin &&
                  (isLastAdmin ? (
                    <SimpleTooltip label={lastAdminMsg} side="top">
                      <span>
                        <Button
                          size="sm"
                          variant="ghost"
                          leadingIcon={<Trash2 size={12} />}
                          disabled
                        >
                          Remove
                        </Button>
                      </span>
                    </SimpleTooltip>
                  ) : (
                    <Button
                      size="sm"
                      variant="ghost"
                      leadingIcon={<Trash2 size={12} />}
                      onClick={async () => {
                        const ok = await confirm({
                          title: `Remove ${m.display_name}?`,
                          description: "They'll lose access to this library.",
                          confirmLabel: "Remove",
                          tone: "destructive",
                        });
                        if (ok) remove.mutate({ libraryId, userId: m.user_id });
                      }}
                    >
                      Remove
                    </Button>
                  ))}
              </div>
            );
          })}
        </div>
      )}

      <InviteMemberDialog libraryId={libraryId} open={inviteOpen} onOpenChange={setInviteOpen} />
      {confirmDialog}
    </Section>
  );
}

function InviteMemberDialog({
  libraryId,
  open,
  onOpenChange,
}: {
  libraryId: number;
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const { mode: authMode } = useAuthMode();
  const invite = useInviteMember();
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [role, setRole] = useState<"admin" | "member">("member");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);

  const submit = () => {
    setError(null);
    invite.mutate(
      {
        libraryId,
        email: email.trim(),
        display_name: displayName.trim() || undefined,
        role,
        password: authMode === "local" && password ? password : undefined,
      },
      {
        onSuccess: () => {
          onOpenChange(false);
          setEmail("");
          setDisplayName("");
          setPassword("");
          setRole("member");
        },
        onError: (e: unknown) => {
          setError(e instanceof Error ? e.message : "Failed");
        },
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Invite a member</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <FieldRow label="Email">
            <Input
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="user@example.com"
            />
          </FieldRow>
          <FieldRow label="Display name" hint="Optional. Defaults to email.">
            <Input
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="Optional"
            />
          </FieldRow>
          <FieldRow label="Role">
            <Select value={role} onValueChange={(v) => setRole(v as "admin" | "member")}>
              <SelectTrigger aria-label="Role">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="member">member</SelectItem>
                <SelectItem value="admin">admin</SelectItem>
              </SelectContent>
            </Select>
          </FieldRow>
          {authMode === "local" ? (
            <FieldRow
              label="Initial password"
              hint="Required when this email isn't already on file. Tell the user out-of-band."
            >
              <Input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Leave blank for existing users"
              />
            </FieldRow>
          ) : (
            <p className="text-caption">
              In OIDC mode, the user must sign in once before being invited — Damson can't create
              identities in your provider.
            </p>
          )}
          {error && <p className="text-label text-destructive">{error}</p>}
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="primary" onClick={submit} disabled={!email || invite.isPending}>
            Invite
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
