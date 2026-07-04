import type { NotifyInput } from "@/components/notifications/NotificationProvider";

let notifier: ((input: NotifyInput) => void) | null = null;

export function setNotifier(fn: ((input: NotifyInput) => void) | null) {
  notifier = fn;
}

export function emitNotification(input: NotifyInput) {
  notifier?.(input);
}
