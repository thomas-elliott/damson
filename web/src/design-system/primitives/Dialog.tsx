import { forwardRef, type ComponentPropsWithoutRef } from "react";
import * as RadixDialog from "@radix-ui/react-dialog";
import { X } from "lucide-react";

import { cn } from "@/design-system/utils";

export const Dialog = RadixDialog.Root;
export const DialogTrigger = RadixDialog.Trigger;
export const DialogPortal = RadixDialog.Portal;

export const DialogOverlay = forwardRef<
  React.ElementRef<typeof RadixDialog.Overlay>,
  ComponentPropsWithoutRef<typeof RadixDialog.Overlay>
>(function DialogOverlay({ className, ...props }, ref) {
  return (
    <RadixDialog.Overlay
      ref={ref}
      className={cn(
        "fixed inset-0 z-dialog bg-black/50 backdrop-blur-[2px] data-[state=open]:animate-fade-in",
        className,
      )}
      {...props}
    />
  );
});

export const DialogContent = forwardRef<
  React.ElementRef<typeof RadixDialog.Content>,
  ComponentPropsWithoutRef<typeof RadixDialog.Content> & {
    width?: number | string;
  }
>(function DialogContent({ className, children, width = 440, ...props }, ref) {
  return (
    <DialogPortal>
      <DialogOverlay />
      <RadixDialog.Content
        ref={ref}
        className={cn(
          "fixed left-1/2 top-1/2 z-dialog -translate-x-1/2 -translate-y-1/2 rounded-lg border border-line bg-panel text-text shadow-elevated data-[state=open]:animate-pop-up",
          "flex max-h-[90vh] flex-col overflow-hidden",
          className,
        )}
        style={{ width: typeof width === "number" ? `${width}px` : width }}
        {...props}
      >
        {children}
        <RadixDialog.Close
          aria-label="Close"
          className="absolute right-3 top-3 grid h-6 w-6 place-items-center rounded-sm text-text-3 transition-colors duration-std ease hover:bg-surface hover:text-text"
        >
          <X size={14} />
        </RadixDialog.Close>
      </RadixDialog.Content>
    </DialogPortal>
  );
});

export function DialogHeader({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  // Stack the title + description vertically and reserve space on the right
  // for the absolute-positioned close button in DialogContent (24px button +
  // 12px inset). Using justify-between used to squish a long title against
  // the description.
  return (
    <div
      className={cn(
        "flex flex-col gap-1 border-b border-line px-4 py-3 pr-10",
        className,
      )}
      {...props}
    />
  );
}

export function DialogBody({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "flex flex-1 flex-col gap-3.5 overflow-y-auto px-4 py-4",
        className,
      )}
      {...props}
    />
  );
}

export function DialogFooter({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "flex items-center justify-end gap-2 border-t border-line bg-bg px-4 py-3",
        className,
      )}
      {...props}
    />
  );
}

export const DialogTitle = forwardRef<
  React.ElementRef<typeof RadixDialog.Title>,
  ComponentPropsWithoutRef<typeof RadixDialog.Title>
>(function DialogTitle({ className, ...props }, ref) {
  return (
    <RadixDialog.Title
      ref={ref}
      className={cn("text-subhead text-text", className)}
      {...props}
    />
  );
});

export const DialogDescription = forwardRef<
  React.ElementRef<typeof RadixDialog.Description>,
  ComponentPropsWithoutRef<typeof RadixDialog.Description>
>(function DialogDescription({ className, ...props }, ref) {
  return (
    <RadixDialog.Description
      ref={ref}
      className={cn("text-caption", className)}
      {...props}
    />
  );
});

export const DialogClose = RadixDialog.Close;
