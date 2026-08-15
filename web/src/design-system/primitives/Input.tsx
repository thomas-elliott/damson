import { forwardRef, type InputHTMLAttributes, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  leadingIcon?: ReactNode;
  trailingSlot?: ReactNode;
  invalid?: boolean;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, leadingIcon, trailingSlot, invalid, disabled, ...props },
  ref,
) {
  return (
    <div
      data-invalid={invalid ? "true" : undefined}
      data-disabled={disabled ? "true" : undefined}
      className={cn(
        "flex items-center gap-1.5 rounded-sm border border-border bg-background px-2 py-1.5 text-field text-foreground transition-colors duration-fast ease-standard",
        "focus-within:border-primary focus-within:bg-muted focus-within:ring-2 focus-within:ring-inset focus-within:ring-ring",
        "data-[invalid=true]:border-destructive/60 data-[invalid=true]:focus-within:ring-destructive/50",
        "data-[disabled=true]:cursor-not-allowed data-[disabled=true]:opacity-50",
        className,
      )}
    >
      {leadingIcon && (
        <span className="grid place-items-center text-subtle-foreground">{leadingIcon}</span>
      )}
      <input
        ref={ref}
        disabled={disabled}
        className="min-w-0 flex-1 bg-transparent text-field leading-none text-foreground outline-hidden placeholder:text-faint-foreground disabled:cursor-not-allowed"
        {...props}
      />
      {trailingSlot && (
        <span className="grid place-items-center text-subtle-foreground">{trailingSlot}</span>
      )}
    </div>
  );
});
