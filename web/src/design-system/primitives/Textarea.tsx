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
      aria-invalid={invalid || undefined}
      className={cn(
        "min-h-[80px] w-full resize-y rounded-md bg-control px-2 py-2 text-field text-foreground outline-hidden transition-colors duration-fast ease-standard placeholder:text-faint-foreground",
        "focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background",
        "disabled:cursor-not-allowed disabled:opacity-50",
        "data-[invalid=true]:ring-2 data-[invalid=true]:ring-destructive",
        className,
      )}
      {...props}
    />
  );
});
