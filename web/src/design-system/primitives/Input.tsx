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
        "flex items-center gap-1.5 rounded-sm border border-line bg-bg px-2 py-1.5 text-ui text-text transition-colors duration-std ease",
        "focus-within:border-accent focus-within:bg-surface focus-within:ring-2 focus-within:ring-inset focus-within:ring-accent-ring",
        "data-[invalid=true]:border-err/60 data-[invalid=true]:focus-within:ring-err/50",
        "data-[disabled=true]:cursor-not-allowed data-[disabled=true]:opacity-50",
        className,
      )}
    >
      {leadingIcon && (
        <span className="grid place-items-center text-text-3">
          {leadingIcon}
        </span>
      )}
      <input
        ref={ref}
        disabled={disabled}
        className="min-w-0 flex-1 bg-transparent text-ui leading-none text-text outline-hidden placeholder:text-text-4 disabled:cursor-not-allowed"
        {...props}
      />
      {trailingSlot && (
        <span className="grid place-items-center text-text-3">
          {trailingSlot}
        </span>
      )}
    </div>
  );
});
