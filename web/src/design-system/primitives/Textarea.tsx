import { forwardRef, type TextareaHTMLAttributes } from "react";

import { cn } from "@/design-system/utils";

export interface TextareaProps
  extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  invalid?: boolean;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(
  function Textarea({ className, invalid, ...props }, ref) {
    return (
      <textarea
        ref={ref}
        data-invalid={invalid ? "true" : undefined}
        className={cn(
          "min-h-[80px] w-full resize-y rounded-sm border border-line-2 bg-bg px-2 py-2 text-sm-plus text-text outline-none transition-colors duration-std ease placeholder:text-text-4",
          "focus:border-transparent focus:outline focus:outline-2 focus:outline-accent-ring",
          "disabled:cursor-not-allowed disabled:opacity-50",
          "data-[invalid=true]:border-err/60 data-[invalid=true]:focus:outline-err/50",
          className,
        )}
        {...props}
      />
    );
  },
);
