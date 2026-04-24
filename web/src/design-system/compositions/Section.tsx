import {
  forwardRef,
  useId,
  useState,
  type HTMLAttributes,
  type ReactNode,
} from "react";
import { ChevronRight } from "lucide-react";

import { cn } from "@/design-system/utils";

export interface SectionProps extends Omit<HTMLAttributes<HTMLElement>, "title"> {
  title: ReactNode;
  /** Right-aligned accessory — counts, action buttons. */
  accessory?: ReactNode;
  /** Start closed? Defaults to open. */
  defaultOpen?: boolean;
  /** Controlled open state; omit to use internal state. */
  open?: boolean;
  onOpenChange?: (next: boolean) => void;
  children: ReactNode;
}

/**
 * Collapsible section used in filter sidebars and detail panels. Header
 * stays visible; body hides when collapsed. Caret rotates 90deg on open.
 */
export const Section = forwardRef<HTMLElement, SectionProps>(function Section(
  {
    className,
    title,
    accessory,
    defaultOpen = true,
    open: controlledOpen,
    onOpenChange,
    children,
    ...props
  },
  ref,
) {
  const [internalOpen, setInternalOpen] = useState(defaultOpen);
  const open = controlledOpen ?? internalOpen;
  const setOpen = (next: boolean) => {
    if (controlledOpen === undefined) setInternalOpen(next);
    onOpenChange?.(next);
  };
  const bodyId = useId();

  return (
    <section
      ref={ref}
      className={cn("border-b border-line", className)}
      {...props}
    >
      <div className="flex items-center">
        <button
          type="button"
          aria-expanded={open}
          aria-controls={bodyId}
          onClick={() => setOpen(!open)}
          className={cn(
            "flex flex-1 items-center gap-1.5 px-4 py-3 text-left text-sm-plus text-text-2 transition-colors duration-std ease",
            "hover:text-text",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-ring",
          )}
        >
          <ChevronRight
            size={12}
            className={cn(
              "text-text-3 transition-transform duration-slow ease",
              open && "rotate-90",
            )}
          />
          <span className="font-medium">{title}</span>
        </button>
        {accessory && <div className="px-4">{accessory}</div>}
      </div>
      {open && (
        <div id={bodyId} className="px-4 pb-3 pt-0">
          {children}
        </div>
      )}
    </section>
  );
});
