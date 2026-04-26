import { useId, type HTMLAttributes, type ReactElement, type ReactNode } from "react";

import { cn } from "@/design-system/utils";

export interface FieldRowProps extends HTMLAttributes<HTMLDivElement> {
  label: ReactNode;
  /** Short description rendered under the label. */
  description?: ReactNode;
  /** Error message shown below the input; sets aria-invalid on the child. */
  error?: ReactNode;
  /** Single form control — Input/Textarea/Select. Must accept `id` + `aria-describedby`. */
  children: ReactElement;
  /** Optional help/hint line between input and error. */
  hint?: ReactNode;
}

/**
 * Labeled form field row. Wires `htmlFor` ↔ child `id`, generates stable ids
 * for description + hint, sets `aria-invalid` on error.
 */
export function FieldRow({
  className,
  label,
  description,
  error,
  hint,
  children,
  ...props
}: FieldRowProps) {
  const autoId = useId();
  const child = children as ReactElement<{
    id?: string;
    "aria-describedby"?: string;
    "aria-invalid"?: boolean;
  }>;
  const controlId = child.props.id ?? `field-${autoId}`;
  const descId = description ? `${controlId}-desc` : undefined;
  const hintId = hint ? `${controlId}-hint` : undefined;
  const errorId = error ? `${controlId}-error` : undefined;
  const describedBy =
    [descId, hintId, errorId].filter(Boolean).join(" ") || undefined;
  return (
    <div className={cn("flex flex-col gap-1.5", className)} {...props}>
      <label htmlFor={controlId} className="text-small text-text-3">
        {label}
      </label>
      {description && (
        <p id={descId} className="text-caption">
          {description}
        </p>
      )}
      {wireChild(child, {
        id: controlId,
        "aria-describedby": describedBy,
        "aria-invalid": error ? true : undefined,
      })}
      {hint && (
        <p id={hintId} className="text-small text-text-4">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} className="text-small text-err">
          {error}
        </p>
      )}
    </div>
  );
}

function wireChild<
  P extends {
    id?: string;
    "aria-describedby"?: string;
    "aria-invalid"?: boolean;
  },
>(child: ReactElement<P>, extra: P): ReactElement<P> {
  const merged = { ...child.props, ...extra };
  return { ...child, props: merged };
}
