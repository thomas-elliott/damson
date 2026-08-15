import { forwardRef, type TextareaHTMLAttributes } from "react";

import { cn } from "@/design-system/utils";

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea(
  { className, invalid, ...props },
  ref,
) {
  return (
    <textarea
      ref={ref}
      data-invalid={invalid ? "true" : undefined}
      className={cn(
        "min-h-[80px] w-full resize-y rounded-sm border border-border-strong bg-background px-2 py-2 text-field text-foreground outline-hidden transition-colors duration-fast ease-standard placeholder:text-faint-foreground",
        "focus:border-primary focus:ring-2 focus:ring-inset focus:ring-ring",
        "disabled:cursor-not-allowed disabled:opacity-50",
        "data-[invalid=true]:border-destructive/60 data-[invalid=true]:focus:ring-destructive/50",
        className,
      )}
      {...props}
    />
  );
});
