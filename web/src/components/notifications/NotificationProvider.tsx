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

import { isUploadAbortedError, uploadFile } from "@/api";
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
}

export interface Upload {
  id: string;
  filename: string;
  size: number;
  progress: number;
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
  notify: (input: NotifyInput) => void;
  markRead: (id: string) => void;
  dismiss: (id: string) => void;
  clearRead: () => void;
  toggleOpen: () => void;
  setOpen: (open: boolean) => void;
}

const NotificationContext = createContext<NotificationContextValue | null>(null);

const STORAGE_KEY = "damson.notifications";
const READ_TTL_MS = 7 * 24 * 60 * 60 * 1000;

function newId(): string {
  try {
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
    return parsed.filter((n) => !(n.read && (n.readAt ?? 0) < cutoff));
  } catch {
    return [];
  }
}

export function NotificationProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [uploads, setUploads] = useState<Upload[]>([]);
  const [notifications, setNotifications] = useState<AppNotification[]>(loadNotifications);
  const [open, setOpen] = useState(false);
  const controllers = useRef(new Map<string, AbortController>());

  useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(notifications));
    } catch {
      // storage unavailable (private mode, quota) — notifications stay in-memory
    }
  }, [notifications]);

  const pushNotification = useCallback((n: Omit<AppNotification, "id" | "createdAt" | "read">) => {
    setNotifications((prev) => [
      { id: newId(), createdAt: Date.now(), read: false, ...n },
      ...prev,
    ]);
  }, []);

  const startUpload = useCallback(
    (files: File[]) => {
      files.forEach((file) => {
        const id = newId();
        const controller = new AbortController();
        controllers.current.set(id, controller);
        setUploads((prev) => [...prev, { id, filename: file.name, size: file.size, progress: 0 }]);
        void uploadFile(
          "/api/imports",
          file,
          (p) => setUploads((prev) => prev.map((u) => (u.id === id ? { ...u, progress: p } : u))),
          controller.signal,
        )
          .then(() => {
            controllers.current.delete(id);
            setUploads((prev) => prev.filter((u) => u.id !== id));
            pushNotification({
              tone: "success",
              title: `Uploaded ${file.name}`,
              source: "upload",
            });
            void queryClient.invalidateQueries({ queryKey: queryKeys.assets() });
            void queryClient.invalidateQueries({ queryKey: queryKeys.imports });
          })
          .catch((err: unknown) => {
            controllers.current.delete(id);
            setUploads((prev) => prev.filter((u) => u.id !== id));
            if (isUploadAbortedError(err)) return;
            pushNotification({
              tone: "error",
              title: `Upload failed: ${file.name}`,
              message: err instanceof Error ? err.message : "upload failed",
              source: "upload",
            });
          });
      });
    },
    [pushNotification, queryClient],
  );

  const cancelUpload = useCallback((id: string) => {
    controllers.current.get(id)?.abort();
  }, []);

  const notify = useCallback(
    (input: NotifyInput) => pushNotification({ ...input, source: "toast" }),
    [pushNotification],
  );

  const markRead = useCallback((id: string) => {
    setNotifications((prev) =>
      prev.map((n) => (n.id === id ? { ...n, read: true, readAt: Date.now() } : n)),
    );
  }, []);

  const dismiss = useCallback((id: string) => {
    setNotifications((prev) => prev.filter((n) => n.id !== id));
  }, []);

  const clearRead = useCallback(() => {
    setNotifications((prev) => prev.filter((n) => !n.read));
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
      notify,
      markRead,
      dismiss,
      clearRead,
      toggleOpen,
      setOpen,
    };
  }, [
    uploads,
    notifications,
    open,
    startUpload,
    cancelUpload,
    notify,
    markRead,
    dismiss,
    clearRead,
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
