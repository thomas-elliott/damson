import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { createContext, useContext, useState, type ComponentProps } from "react";

import { Button } from "@/design-system/primitives/Button";
import { cn } from "@/design-system/utils";

// Behaviour and a11y come from Radix — focus trap, scroll lock, Escape,
// aria-labelledby/describedby wiring off DialogTitle/DialogDescription.
// Focus return on close is Damson's own; see Dialog.

// Radix returns focus on close only to a DialogTrigger, and Damson opens its
// dialogs from state, so focus fell to <body>. Dialog notes what had focus
// when `open` turns true, during render, before a field's autoFocus or Radix
// moves focus in, and DialogContent returns focus to it on close.
const ReturnFocusContext = createContext<HTMLElement | null>(null);

function focusedElement() {
  return typeof document === "undefined" ? null : (document.activeElement as HTMLElement | null);
}

function Dialog({ open, ...props }: ComponentProps<typeof DialogPrimitive.Root>) {
  const [wasOpen, setWasOpen] = useState(open);
  const [returnFocus, setReturnFocus] = useState(() => (open ? focusedElement() : null));
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setReturnFocus(focusedElement());
  }
  return (
    <ReturnFocusContext.Provider value={returnFocus}>
      <DialogPrimitive.Root data-slot="dialog" open={open} {...props} />
    </ReturnFocusContext.Provider>
  );
}

function DialogTrigger(props: ComponentProps<typeof DialogPrimitive.Trigger>) {
  return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />;
}

function DialogClose(props: ComponentProps<typeof DialogPrimitive.Close>) {
  return <DialogPrimitive.Close data-slot="dialog-close" {...props} />;
}

function DialogOverlay({ className, ...props }: ComponentProps<typeof DialogPrimitive.Overlay>) {
  return (
    <DialogPrimitive.Overlay
      data-slot="dialog-overlay"
      className={cn(
        "fixed inset-0 z-50 bg-black/50",
        "data-[state=open]:animate-in data-[state=closed]:animate-out",
        "data-[state=open]:fade-in-0 data-[state=closed]:fade-out-0",
        className,
      )}
      {...props}
    />
  );
}

function DialogContent({
  className,
  children,
  onCloseAutoFocus,
  ...props
}: ComponentProps<typeof DialogPrimitive.Content>) {
  const returnFocus = useContext(ReturnFocusContext);
  const restoreFocus = (event: Event) => {
    onCloseAutoFocus?.(event);
    if (event.defaultPrevented) return;
    if (returnFocus && returnFocus !== document.body && returnFocus.isConnected) {
      event.preventDefault();
      returnFocus.focus();
    }
  };
  return (
    <DialogPrimitive.Portal data-slot="dialog-portal">
      <DialogOverlay />
      <DialogPrimitive.Content
        data-slot="dialog-content"
        className={cn(
          "fixed top-1/2 left-1/2 z-50 -translate-x-1/2 -translate-y-1/2",
          // A column, so DialogBody takes the slack and the footer stays put.
          "flex flex-col overflow-hidden p-0",
          "rounded-lg border border-border bg-popover text-popover-foreground shadow-lg",
          "w-[calc(100vw-3rem)] max-h-[calc(100vh-3rem)] sm:w-lg",
          "data-[state=open]:animate-in data-[state=closed]:animate-out",
          className,
        )}
        onCloseAutoFocus={restoreFocus}
        {...props}
      >
        {children}
        <DialogPrimitive.Close asChild>
          <Button
            variant="ghost"
            size="sm"
            className="absolute top-3 right-3 h-6 w-6 p-0"
            aria-label="Close"
          >
            <X aria-hidden />
          </Button>
        </DialogPrimitive.Close>
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}

function DialogHeader({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-header"
      className={cn("flex shrink-0 flex-col gap-1.5 px-5 pt-5 pr-12 pb-4", className)}
      {...props}
    />
  );
}

/** The scroll region. Wrap body content in this whenever the dialog can outgrow
 * the viewport: it is what keeps the footer pinned instead of scrolling away,
 * and `min-h-0` is what lets it shrink inside the column rather than taking its
 * content height. */
function DialogBody({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-body"
      className={cn("min-h-0 flex-1 overflow-auto px-5 pb-5", className)}
      {...props}
    />
  );
}

function DialogFooter({ className, ...props }: ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-footer"
      className={cn(
        // No top padding: DialogHeader's pb-4 or DialogBody's pb-5 supplies the gap.
        "flex shrink-0 flex-col-reverse gap-2 px-5 pt-0 pb-5 sm:flex-row sm:justify-end",
        className,
      )}
      {...props}
    />
  );
}

function DialogTitle({ className, ...props }: ComponentProps<typeof DialogPrimitive.Title>) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn("text-title font-semibold", className)}
      {...props}
    />
  );
}

function DialogDescription({
  className,
  ...props
}: ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn("text-body text-muted-foreground", className)}
      {...props}
    />
  );
}

export {
  Dialog,
  DialogBody,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogTitle,
  DialogTrigger,
};
