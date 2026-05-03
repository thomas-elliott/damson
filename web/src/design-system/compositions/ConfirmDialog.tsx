import { useCallback, useRef, useState, type ReactNode } from "react";

import {
  Button,
  Dialog,
  DialogBody,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/design-system/primitives";

export type ConfirmTone = "default" | "danger";

export type ConfirmOptions = {
  title: string;
  description?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  tone?: ConfirmTone;
};

export type ConfirmDialogProps = ConfirmOptions & {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
};

/**
 * Styled confirmation dialog used to replace native window.confirm().
 * Prefer the useConfirm hook below for the imperative if-await-confirm
 * pattern; reach for this controlled component only when you need finer
 * control over open/close.
 */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  tone = "default",
  onConfirm,
}: ConfirmDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        {description !== undefined && (
          <DialogBody>
            {typeof description === "string" ? (
              <p className="text-ui text-text-2">{description}</p>
            ) : (
              description
            )}
          </DialogBody>
        )}
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {cancelLabel}
          </Button>
          <Button
            variant={tone === "danger" ? "danger" : "primary"}
            onClick={() => {
              onConfirm();
              onOpenChange(false);
            }}
          >
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

/**
 * Hook returning a `confirm` function that resolves to true/false and a
 * `dialog` element to render in the consumer's tree.
 *
 *   const { confirm, dialog } = useConfirm();
 *   ...
 *   onClick={async () => {
 *     if (await confirm({ title: "Delete?", tone: "danger" })) {
 *       del.mutate(id);
 *     }
 *   }}
 *   ...
 *   return <>{dialog}{...}</>;
 */
export function useConfirm() {
  const [open, setOpen] = useState(false);
  const [options, setOptions] = useState<ConfirmOptions | null>(null);
  // Resolver lives in a ref so resolving doesn't depend on stale closures
  // when callers fire confirm() back-to-back.
  const resolverRef = useRef<((value: boolean) => void) | null>(null);

  const settle = useCallback((value: boolean) => {
    const resolve = resolverRef.current;
    resolverRef.current = null;
    resolve?.(value);
  }, []);

  const confirm = useCallback((opts: ConfirmOptions) => {
    // If a previous prompt is still pending (shouldn't happen with normal
    // flows, but guard anyway), resolve it as cancelled before replacing.
    resolverRef.current?.(false);
    return new Promise<boolean>((resolve) => {
      resolverRef.current = resolve;
      setOptions(opts);
      setOpen(true);
    });
  }, []);

  const handleOpenChange = useCallback(
    (next: boolean) => {
      setOpen(next);
      if (!next) settle(false);
    },
    [settle],
  );

  const dialog = options ? (
    <ConfirmDialog
      open={open}
      onOpenChange={handleOpenChange}
      title={options.title}
      description={options.description}
      confirmLabel={options.confirmLabel}
      cancelLabel={options.cancelLabel}
      tone={options.tone}
      onConfirm={() => settle(true)}
    />
  ) : null;

  return { confirm, dialog };
}
