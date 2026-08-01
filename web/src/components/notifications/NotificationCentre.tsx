import { useState } from "react";
import { AlertCircle, AlertTriangle, CheckCircle2, Info, Minus, UploadCloud } from "lucide-react";

import {
  Button,
  IconButton,
  QueueItem,
  StatusDot,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  TabCount,
  cn,
  formatBytes,
} from "@/design-system";

import {
  useNotifications,
  type AppNotification,
  type NotificationTone,
  type Upload,
} from "./NotificationProvider";

type TabKey = "uploads" | "notifications" | "read";

function uploadStatusLine(u: Upload): string {
  if (u.queued) return `Queued · ${formatBytes(u.size)}`;
  if (u.phase === "hashing") return `Hashing · ${formatBytes(u.size)}`;
  return formatBytes(u.size);
}

export function NotificationCentre() {
  const {
    open,
    setOpen,
    uploads,
    notifications,
    inFlightCount,
    cancelUpload,
    cancelAllUploads,
    runAction,
    markRead,
    dismiss,
    clearRead,
    clearUnread,
  } = useNotifications();
  const [tab, setTab] = useState<TabKey>("notifications");

  if (!open) return null;

  const unread = notifications.filter((n) => !n.read);
  const read = notifications.filter((n) => n.read);
  const showUploads = inFlightCount > 0;
  const activeTab: TabKey = tab === "uploads" && !showUploads ? "notifications" : tab;

  return (
    <section
      aria-label="Notification centre"
      className="fixed bottom-4 right-4 z-selbar flex max-h-[70vh] w-[340px] flex-col overflow-hidden rounded-lg border border-line-2 bg-panel shadow-elevated"
    >
      <header className="flex items-center justify-between border-b border-line px-3 py-2.5">
        <h2 className="text-ui font-semibold text-text">Notifications</h2>
        <IconButton aria-label="Minimise notifications" onClick={() => setOpen(false)}>
          <Minus size={15} />
        </IconButton>
      </header>

      <Tabs
        value={activeTab}
        onValueChange={(v) => setTab(v as TabKey)}
        className="flex min-h-0 flex-1 flex-col"
      >
        <TabsList className="px-2">
          {showUploads && (
            <TabsTrigger value="uploads">
              Uploads <TabCount>{inFlightCount}</TabCount>
            </TabsTrigger>
          )}
          <TabsTrigger value="notifications">
            Notifications {unread.length > 0 && <TabCount>{unread.length}</TabCount>}
          </TabsTrigger>
          <TabsTrigger value="read">Read</TabsTrigger>
        </TabsList>

        {showUploads && (
          <TabsContent value="uploads" className="min-h-0 flex-1 overflow-y-auto p-2.5">
            <div className="mb-2 flex justify-end">
              <Button size="sm" variant="ghost" onClick={cancelAllUploads}>
                Cancel all
              </Button>
            </div>
            <ul className="flex flex-col gap-2">
              {uploads.map((u) => (
                <li key={u.id}>
                  <QueueItem
                    name={u.filename}
                    thumb={<UploadCloud size={20} className="text-text-3" />}
                    sub={<span className="text-mono text-text-3">{uploadStatusLine(u)}</span>}
                    progress={u.progress * 100}
                    actions={
                      <Button size="sm" variant="ghost" onClick={() => cancelUpload(u.id)}>
                        Cancel
                      </Button>
                    }
                  />
                </li>
              ))}
            </ul>
          </TabsContent>
        )}

        <TabsContent value="notifications" className="min-h-0 flex-1 overflow-y-auto p-2.5">
          {unread.length === 0 ? (
            <EmptyRow>No new notifications</EmptyRow>
          ) : (
            <>
              <div className="mb-2 flex justify-end">
                <Button size="sm" variant="ghost" onClick={clearUnread}>
                  Clear all
                </Button>
              </div>
              <ul className="flex flex-col gap-2">
                {unread.map((n) => (
                  <NotificationRow
                    key={n.id}
                    n={n}
                    onAction={runAction}
                    onMarkRead={markRead}
                    onDismiss={dismiss}
                  />
                ))}
              </ul>
            </>
          )}
        </TabsContent>

        <TabsContent value="read" className="min-h-0 flex-1 overflow-y-auto p-2.5">
          {read.length === 0 ? (
            <EmptyRow>No read notifications</EmptyRow>
          ) : (
            <>
              <div className="mb-2 flex justify-end">
                <Button size="sm" variant="ghost" onClick={clearRead}>
                  Clear all
                </Button>
              </div>
              <ul className="flex flex-col gap-2">
                {read.map((n) => (
                  <NotificationRow key={n.id} n={n} onDismiss={dismiss} />
                ))}
              </ul>
            </>
          )}
        </TabsContent>
      </Tabs>
    </section>
  );
}

const TONE_ICON: Record<NotificationTone, typeof Info> = {
  info: Info,
  success: CheckCircle2,
  warn: AlertTriangle,
  error: AlertCircle,
};

const TONE_TEXT: Record<NotificationTone, string> = {
  info: "text-text-3",
  success: "text-ok",
  warn: "text-warn",
  error: "text-err",
};

function NotificationRow({
  n,
  onAction,
  onMarkRead,
  onDismiss,
}: {
  n: AppNotification;
  onAction?: (id: string) => void;
  onMarkRead?: (id: string) => void;
  onDismiss: (id: string) => void;
}) {
  const Icon = TONE_ICON[n.tone];
  return (
    <li className="flex flex-col gap-1.5 rounded-md border border-line bg-panel p-2.5">
      <div className="flex items-start justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <Icon size={14} className={cn("shrink-0", TONE_TEXT[n.tone])} />
          <span className="truncate text-ui font-medium text-text">{n.title}</span>
        </div>
        <span className="shrink-0 text-mono text-small text-text-4">
          {relativeTime(n.createdAt)}
        </span>
      </div>
      {n.message && <span className="pl-6 text-small text-text-3">{n.message}</span>}
      <div className="flex gap-1.5 pl-6">
        {n.action && onAction && (
          <Button size="sm" variant="primary" onClick={() => onAction(n.id)}>
            {n.action.label}
          </Button>
        )}
        {onMarkRead && (
          <Button size="sm" variant="ghost" onClick={() => onMarkRead(n.id)}>
            Mark read
          </Button>
        )}
        <Button size="sm" variant="ghost" onClick={() => onDismiss(n.id)}>
          Dismiss
        </Button>
      </div>
    </li>
  );
}

function EmptyRow({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-center gap-2 py-8 text-small text-text-4">
      <StatusDot tone="neutral" />
      {children}
    </div>
  );
}

function relativeTime(ts: number): string {
  const diff = Date.now() - ts;
  const sec = Math.round(diff / 1000);
  if (sec < 45) return "just now";
  const min = Math.round(sec / 60);
  if (min < 60) return `${min}m`;
  const hr = Math.round(min / 60);
  if (hr < 24) return `${hr}h`;
  const day = Math.round(hr / 24);
  return `${day}d`;
}
