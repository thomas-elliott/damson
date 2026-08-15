import { useState } from "react";
import { Copy, KeyRound, Trash2 } from "lucide-react";

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
  useConfirm,
} from "@/design-system";
import { useApiTokens, type ApiToken } from "@/hooks/useDataQueries";
import { useCreateApiToken, useRevokeApiToken } from "@/hooks/useMutations";

/**
 * Personal access tokens. Lists the signed-in user's tokens and mints new
 * ones — the only way to obtain the credential the dmsn CLI needs, since
 * token auth is refused on this endpoint by design.
 *
 * A new token is pinned to the current library at creation and its secret is
 * shown exactly once.
 */
export function ApiTokensSection() {
  const { data: tokens, loading, error } = useApiTokens();
  const [createOpen, setCreateOpen] = useState(false);
  const [issued, setIssued] = useState<(ApiToken & { token: string }) | null>(null);
  const [revokeError, setRevokeError] = useState<string | null>(null);
  const revoke = useRevokeApiToken();
  const { confirm, dialog: confirmDialog } = useConfirm();

  const active = tokens.filter((t) => !isDead(t));

  return (
    <Section
      title={`Access tokens (${active.length})`}
      accessory={
        <Button
          size="sm"
          variant="primary"
          leadingIcon={<KeyRound size={12} />}
          onClick={() => setCreateOpen(true)}
        >
          New token
        </Button>
      }
    >
      {error ? (
        <EmptyState
          title="Couldn't load tokens"
          description="Your tokens are still there — this list just failed to load. Reload to try again."
        />
      ) : loading ? (
        <div className="py-2 text-ui text-subtle-foreground">Loading…</div>
      ) : tokens.length === 0 ? (
        <EmptyState
          title="No access tokens"
          description="Tokens let the dmsn CLI and other unattended clients reach this library without a browser session."
        />
      ) : (
        <div className="flex flex-col">
          {tokens.map((t) => (
            <div
              key={t.id}
              className="flex items-center gap-3 border-b border-border py-2 last:border-b-0"
            >
              <div className="min-w-0 flex-1">
                <div className="truncate text-ui text-foreground">{t.label}</div>
                <div className="truncate text-caption">
                  <span className="font-mono">{t.prefix}…</span>
                  {t.library_name && <> · {t.library_name}</>} · {describeUse(t)}
                </div>
              </div>
              <Badge size="sm" mono tone={t.scope === "read" ? "unk" : undefined}>
                {t.scope}
              </Badge>
              {isDead(t) ? (
                <Badge size="sm" tone="unk">
                  {t.revoked_at ? "Revoked" : "Expired"}
                </Badge>
              ) : (
                <Button
                  size="sm"
                  variant="ghost"
                  leadingIcon={<Trash2 size={12} />}
                  onClick={async () => {
                    const ok = await confirm({
                      title: `Revoke ${t.label}?`,
                      description: "Any client using this token stops working immediately.",
                      confirmLabel: "Revoke",
                      tone: "danger",
                    });
                    if (!ok) return;
                    setRevokeError(null);
                    revoke.mutate(
                      { id: t.id },
                      {
                        onError: (e: unknown) =>
                          setRevokeError(
                            e instanceof Error
                              ? `Couldn't revoke ${t.label}: ${e.message}`
                              : `Couldn't revoke ${t.label}.`,
                          ),
                      },
                    );
                  }}
                >
                  Revoke
                </Button>
              )}
            </div>
          ))}
        </div>
      )}

      {revokeError && <p className="mt-2 text-small text-destructive">{revokeError}</p>}

      <CreateTokenDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onIssued={(t) => {
          setCreateOpen(false);
          setIssued(t);
        }}
      />
      <IssuedTokenDialog token={issued} onClose={() => setIssued(null)} />
      {confirmDialog}
    </Section>
  );
}

function CreateTokenDialog({
  open,
  onOpenChange,
  onIssued,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  onIssued: (t: ApiToken & { token: string }) => void;
}) {
  const create = useCreateApiToken();
  const [label, setLabel] = useState("");
  const [scope, setScope] = useState<"read" | "write">("write");
  const [expires, setExpires] = useState("");
  const [error, setError] = useState<string | null>(null);

  const submit = () => {
    setError(null);
    create.mutate(
      {
        label: label.trim(),
        scope,
        // Local end-of-day, not UTC: the picker shows the user's own date, so
        // interpreting it in UTC can hand the server an already-past instant.
        expires_at: expires ? new Date(`${expires}T23:59:59`).toISOString() : undefined,
      },
      {
        onSuccess: (t) => {
          setLabel("");
          setScope("write");
          setExpires("");
          onIssued(t);
        },
        onError: (e: unknown) => setError(e instanceof Error ? e.message : "Failed"),
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New access token</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <FieldRow
            label="Label"
            hint="What this token is for. Shown in the list and the audit log."
          >
            <Input
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder="laptop sync"
            />
          </FieldRow>
          <FieldRow label="Scope" hint="Read tokens can only perform GET requests.">
            <Select value={scope} onValueChange={(v) => setScope(v as "read" | "write")}>
              <SelectTrigger aria-label="Scope">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="write">write</SelectItem>
                <SelectItem value="read">read</SelectItem>
              </SelectContent>
            </Select>
          </FieldRow>
          <FieldRow label="Expires" hint="Optional. Leave blank for a token that never expires.">
            <Input
              type="date"
              min={tomorrow()}
              value={expires}
              onChange={(e) => setExpires(e.target.value)}
            />
          </FieldRow>
          <p className="text-caption">
            The token is pinned to your current library and acts as you. It cannot create other
            tokens or reach server administration.
          </p>
          {error && <p className="text-small text-destructive">{error}</p>}
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="primary" onClick={submit} disabled={!label.trim() || create.isPending}>
            Create
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function IssuedTokenDialog({
  token,
  onClose,
}: {
  token: (ApiToken & { token: string }) | null;
  onClose: () => void;
}) {
  const [copied, setCopied] = useState(false);
  const [copyFailed, setCopyFailed] = useState(false);

  // navigator.clipboard is undefined outside a secure context — plain http on
  // a LAN is a normal way to run damson — so a failure here is expected, not
  // exceptional, and has to say so rather than no-op.
  const copy = async () => {
    if (!token) return;
    try {
      await navigator.clipboard.writeText(token.token);
      setCopied(true);
      setCopyFailed(false);
    } catch {
      setCopied(false);
      setCopyFailed(true);
    }
  };

  return (
    <Dialog
      open={token != null}
      onOpenChange={(v) => {
        if (!v) {
          setCopied(false);
          setCopyFailed(false);
          onClose();
        }
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Copy your token now</DialogTitle>
        </DialogHeader>
        <DialogBody>
          <p className="text-ui text-foreground">
            This is the only time the token is shown. Damson stores a digest, not the value — if you
            lose it, revoke this token and create another.
          </p>
          <div className="mt-3 flex items-center gap-2">
            <Input
              readOnly
              aria-label="Access token"
              value={token?.token ?? ""}
              className="font-mono"
              onFocus={(e) => e.currentTarget.select()}
            />
            <Button variant="ghost" leadingIcon={<Copy size={12} />} onClick={copy}>
              {copied ? "Copied" : "Copy"}
            </Button>
          </div>
          {copyFailed && (
            <p className="mt-2 text-small text-destructive">
              Couldn&rsquo;t reach the clipboard — browsers only allow it over HTTPS or on
              localhost. Select the value above and copy it manually.
            </p>
          )}
          <p className="mt-3 text-caption">
            Use it with <span className="font-mono">dmsn login --token …</span>, or as{" "}
            <span className="font-mono">Authorization: Bearer …</span>.
          </p>
        </DialogBody>
        <DialogFooter>
          <Button variant="primary" onClick={onClose}>
            Done
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function tomorrow(): string {
  const d = new Date();
  d.setDate(d.getDate() + 1);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function isDead(t: ApiToken): boolean {
  if (t.revoked_at) return true;
  return t.expires_at != null && new Date(t.expires_at).getTime() <= Date.now();
}

function describeUse(t: ApiToken): string {
  if (!t.last_used_at) return "never used";
  return `last used ${new Date(t.last_used_at).toLocaleDateString()}`;
}
