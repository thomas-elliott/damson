import { useState } from "react";
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
} from "@/design-system";
import { useAuthMode, useLibraryMembers } from "@/hooks/useDataQueries";
import {
  useInviteMember,
  usePatchMember,
  useRemoveMember,
} from "@/hooks/useMutations";

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
          {members.map((m) => (
            <div
              key={m.user_id}
              className="flex items-center gap-3 border-b border-line py-2 last:border-b-0"
            >
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm-plus text-text">{m.display_name}</div>
                <div className="truncate text-xs-plus text-text-3">{m.email}</div>
              </div>
              {m.global_role === "sysadmin" && (
                <Badge size="sm" tone="warn">sysadmin</Badge>
              )}
              {isAdmin ? (
                <Select
                  value={m.role}
                  onValueChange={(role) =>
                    patch.mutate({ libraryId, userId: m.user_id, role: role as "admin" | "member" })
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
              ) : (
                <Badge size="sm" mono>{m.role}</Badge>
              )}
              {isAdmin && (
                <Button
                  size="sm"
                  variant="ghost"
                  leadingIcon={<Trash2 size={12} />}
                  onClick={() => {
                    if (confirm(`Remove ${m.display_name} from this library?`)) {
                      remove.mutate({ libraryId, userId: m.user_id });
                    }
                  }}
                >
                  Remove
                </Button>
              )}
            </div>
          ))}
        </div>
      )}

      <InviteMemberDialog
        libraryId={libraryId}
        open={inviteOpen}
        onOpenChange={setInviteOpen}
      />
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
            <Input value={email} onChange={(e) => setEmail(e.target.value)} placeholder="user@example.com" />
          </FieldRow>
          <FieldRow label="Display name" hint="Optional. Defaults to email.">
            <Input value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder="Optional" />
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
            <p className="text-xs-plus text-text-3">
              In OIDC mode, the user must sign in once before being invited — Damson can't create
              identities in your provider.
            </p>
          )}
          {error && <p className="text-xs-plus text-err">{error}</p>}
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
