import { useState, type ReactNode } from "react";
import { RotateCw, TriangleAlert } from "lucide-react";

import { EmptyState } from "@/design-system/compositions/EmptyState";
import { Button } from "@/design-system/primitives/Button";

export type ErrorStateProps = {
  title: ReactNode;
  description?: ReactNode;
  /** Retry stays disabled until a returned promise settles. */
  onRetry: () => unknown;
  className?: string;
};

/** A failed load, in place of the content: fixed wording and a Retry. */
export function ErrorState({ title, description, onRetry, className }: ErrorStateProps) {
  const [retrying, setRetrying] = useState(false);
  const retry = async () => {
    setRetrying(true);
    try {
      await onRetry();
    } finally {
      setRetrying(false);
    }
  };
  return (
    <EmptyState
      className={className}
      icon={<TriangleAlert size={32} />}
      title={title}
      description={description ?? "Something went wrong while loading. Try again."}
      action={
        <Button
          variant="secondary"
          leadingIcon={<RotateCw size={12} />}
          loading={retrying}
          onClick={() => void retry()}
        >
          Retry
        </Button>
      }
    />
  );
}
