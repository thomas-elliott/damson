import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useQueryClient } from "@tanstack/react-query";

import { ApiError, isUploadAbortedError, uploadFile } from "@/api";
import { useDeleteAsset } from "@/hooks/useMutations";
import {
  CHUNKED_UPLOAD_THRESHOLD,
  cancelChunkedUpload,
  uploadFileChunked,
  type UploadPhase,
} from "@/lib/chunkedUpload";
import { setNotifier } from "@/lib/notificationBridge";
import { queryKeys } from "@/lib/queryKeys";

import { NotificationCentre } from "./NotificationCentre";

export type NotificationTone = "info" | "success" | "warn" | "error";

export interface AppNotification {
  id: string;
  tone: NotificationTone;
  title: string;
  message?: string;
  createdAt: number;
  read: boolean;
  readAt?: number;
  source: "upload" | "toast";
  action?: { label: string };
}

export interface Upload {
  id: string;
  filename: string;
  size: number;
  progress: number;
  queued: boolean;
  phase?: UploadPhase;
}

export interface NotifyInput {
  tone: NotificationTone;
  title: string;
  message?: string;
}

interface NotificationContextValue {
  uploads: Upload[];
  notifications: AppNotification[];
  open: boolean;
  unreadCount: number;
  inFlightCount: number;
  startUpload: (files: File[]) => void;
  cancelUpload: (id: string) => void;
  cancelAllUploads: () => void;
  notify: (input: NotifyInput) => void;
  runAction: (id: string) => void;
  markRead: (id: string) => void;
  dismiss: (id: string) => void;
  clearRead: () => void;
  clearUnread: () => void;
  toggleOpen: () => void;
  setOpen: (open: boolean) => void;
}

const NotificationContext = createContext<NotificationContextValue | null>(null);

const STORAGE_KEY = "damson.notifications";
const READ_TTL_MS = 7 * 24 * 60 * 60 * 1000;
// Cap in-flight uploads so a large drop doesn't fire N requests at once and
// saturate the proxy into 502s; the rest queue.
const MAX_CONCURRENT_UPLOADS = 3;

function newId(): string {
  try {
    // crypto.randomUUID throws outside secure contexts (e.g. plain-http LAN hosts).
    return crypto.randomUUID();
  } catch {
    return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  }
}

function loadNotifications(): AppNotification[] {
  if (typeof window === "undefined") return [];
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as AppNotification[];
    if (!Array.isArray(parsed)) return [];
    const cutoff = Date.now() - READ_TTL_MS;
    // An action's handler is an in-memory closure that can't survive a reload,
    // so drop persisted action notifications rather than show a dead button.
    return parsed.filter((n) => !n.action && !(n.read && (n.readAt ?? 0) < cutoff));
  } catch {
    return [];
  }
}

export function NotificationProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const deleteAsset = useDeleteAsset();
  const [uploads, setUploads] = useState<Upload[]>([]);
  const [notifications, setNotifications] = useState<AppNotification[]>(loadNotifications);
  const [open, setOpen] = useState(false);
  const controllers = useRef(new Map<string, AbortController>());
  const uploadQueue = useRef<Array<{ id: string; start: () => void }>>([]);
  const activeUploads = useRef(0);
  const actionHandlers = useRef(new Map<string, () => void>());

  const pumpQueue = useCallback(() => {
    while (activeUploads.current < MAX_CONCURRENT_UPLOADS && uploadQueue.current.length > 0) {
      const next = uploadQueue.current.shift();
      if (!next) break;
      activeUploads.current += 1;
      next.start();
    }
  }, []);

  useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(notifications));
    } catch {
      // storage unavailable (private mode, quota) — notifications stay in-memory
    }
  }, [notifications]);

  const pushNotification = useCallback(
    (n: Omit<AppNotification, "id" | "createdAt" | "read">, onAction?: () => void) => {
      const id = newId();
      if (onAction) actionHandlers.current.set(id, onAction);
      setNotifications((prev) => [{ id, createdAt: Date.now(), read: false, ...n }, ...prev]);
    },
    [],
  );

  // The accept action re-invokes runUpload; routing through a ref keeps its
  // useCallback from having to depend on itself.
  const runUploadRef = useRef<(file: File, assetId?: number, onUploaded?: () => void) => void>(
    () => {},
  );
  const runUpload = useCallback(
    (file: File, assetId?: number, onUploaded?: () => void) => {
      const id = newId();
      const controller = new AbortController();
      controllers.current.set(id, controller);
      setUploads((prev) => [
        ...prev,
        { id, filename: file.name, size: file.size, progress: 0, queued: true },
      ]);

      const setProgress = (p: number) =>
        setUploads((prev) => prev.map((u) => (u.id === id ? { ...u, progress: p } : u)));
      const chunked = file.size >= CHUNKED_UPLOAD_THRESHOLD;

      const start = () => {
        setUploads((prev) =>
          prev.map((u) =>
            u.id === id ? { ...u, queued: false, phase: chunked ? "hashing" : "uploading" } : u,
          ),
        );
        const transfer = chunked
          ? uploadFileChunked({
              file,
              assetId,
              signal: controller.signal,
              onPhase: (phase) =>
                setUploads((prev) => prev.map((u) => (u.id === id ? { ...u, phase } : u))),
              onProgress: setProgress,
            })
          : uploadFile("/api/imports", file, setProgress, controller.signal, assetId);
        void transfer
          .then((resp) => {
            setUploads((prev) => prev.filter((u) => u.id !== id));
            pushNotification({
              tone: "success",
              title: assetId ? `Added version from ${file.name}` : `Uploaded ${file.name}`,
              source: "upload",
            });
            if (!assetId && resp.suggested_match) {
              const match = resp.suggested_match;
              const created = resp.asset_id;
              pushNotification(
                {
                  tone: "info",
                  title: `Possible new version of ${match.name}`,
                  message: `Add ${file.name} as a version of ${match.name} instead of a separate asset?`,
                  source: "upload",
                  action: { label: "Add as version" },
                },
                () =>
                  runUploadRef.current(file, match.asset_id, () => {
                    // The plain upload already created a standalone asset; once the
                    // file is re-attached to the match, remove it. Runs only on
                    // success, so a duplicate-version 409 leaves it in place.
                    if (created) deleteAsset.mutate(created);
                  }),
              );
            }
            onUploaded?.();
            void queryClient.invalidateQueries({ queryKey: queryKeys.assets() });
            void queryClient.invalidateQueries({ queryKey: queryKeys.imports });
          })
          .catch((err: unknown) => {
            setUploads((prev) => prev.filter((u) => u.id !== id));
            if (isUploadAbortedError(err)) {
              if (chunked) void cancelChunkedUpload(file);
              return;
            }
            const duplicate = err instanceof ApiError && err.status === 409;
            pushNotification({
              tone: duplicate ? "warn" : "error",
              title: duplicate
                ? `${file.name} already exists on that asset`
                : `Upload failed: ${file.name}`,
              message: err instanceof Error ? err.message : "upload failed",
              source: "upload",
            });
          })
          .finally(() => {
            controllers.current.delete(id);
            activeUploads.current -= 1;
            pumpQueue();
          });
      };

      uploadQueue.current.push({ id, start });
      pumpQueue();
    },
    [deleteAsset, pushNotification, queryClient, pumpQueue],
  );
  useEffect(() => {
    runUploadRef.current = runUpload;
  }, [runUpload]);

  const startUpload = useCallback(
    (files: File[]) => files.forEach((file) => runUpload(file)),
    [runUpload],
  );

  const runAction = useCallback((id: string) => {
    const handler = actionHandlers.current.get(id);
    actionHandlers.current.delete(id);
    setNotifications((prev) =>
      prev.map((n) =>
        n.id === id ? { ...n, read: true, readAt: Date.now(), action: undefined } : n,
      ),
    );
    handler?.();
  }, []);

  const cancelUpload = useCallback((id: string) => {
    controllers.current.get(id)?.abort();
    // A still-queued upload has no in-flight request to abort, so drop it from
    // the queue and the visible list directly.
    const queued = uploadQueue.current.some((q) => q.id === id);
    if (queued) {
      uploadQueue.current = uploadQueue.current.filter((q) => q.id !== id);
      controllers.current.delete(id);
      setUploads((prev) => prev.filter((u) => u.id !== id));
    }
  }, []);

  const cancelAllUploads = useCallback(() => {
    uploadQueue.current = [];
    controllers.current.forEach((c) => c.abort());
    controllers.current.clear();
    setUploads([]);
  }, []);

  const notify = useCallback(
    (input: NotifyInput) => pushNotification({ ...input, source: "toast" }),
    [pushNotification],
  );

  useEffect(() => {
    setNotifier(notify);
    return () => setNotifier(null);
  }, [notify]);

  const markRead = useCallback((id: string) => {
    setNotifications((prev) =>
      prev.map((n) => (n.id === id ? { ...n, read: true, readAt: Date.now() } : n)),
    );
  }, []);

  const dismiss = useCallback((id: string) => {
    actionHandlers.current.delete(id);
    setNotifications((prev) => prev.filter((n) => n.id !== id));
  }, []);

  const clearRead = useCallback(() => {
    setNotifications((prev) => prev.filter((n) => !n.read));
  }, []);

  const clearUnread = useCallback(() => {
    setNotifications((prev) => {
      prev.forEach((n) => {
        if (!n.read) actionHandlers.current.delete(n.id);
      });
      return prev.filter((n) => n.read);
    });
  }, []);

  const toggleOpen = useCallback(() => setOpen((o) => !o), []);

  const value = useMemo<NotificationContextValue>(() => {
    const unreadCount = notifications.reduce((n, item) => (item.read ? n : n + 1), 0);
    return {
      uploads,
      notifications,
      open,
      unreadCount,
      inFlightCount: uploads.length,
      startUpload,
      cancelUpload,
      cancelAllUploads,
      notify,
      runAction,
      markRead,
      dismiss,
      clearRead,
      clearUnread,
      toggleOpen,
      setOpen,
    };
  }, [
    uploads,
    notifications,
    open,
    startUpload,
    cancelUpload,
    cancelAllUploads,
    notify,
    runAction,
    markRead,
    dismiss,
    clearRead,
    clearUnread,
    toggleOpen,
  ]);

  return (
    <NotificationContext.Provider value={value}>
      {children}
      <NotificationCentre />
    </NotificationContext.Provider>
  );
}

export function useNotifications(): NotificationContextValue {
  const ctx = useContext(NotificationContext);
  if (!ctx) throw new Error("useNotifications must be used within a NotificationProvider");
  return ctx;
}
