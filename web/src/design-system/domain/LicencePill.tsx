import { forwardRef } from "react";

import { Pill, type BadgeProps } from "@/design-system/primitives";
import { StatusDot } from "@/design-system/primitives/StatusDot";
import type { LicenceStatus } from "@/design-system/tokens";

export interface LicencePillProps extends Omit<BadgeProps, "tone" | "shape" | "mono"> {
  status: LicenceStatus;
  /** SPDX identifier or free-form label. */
  spdx: string;
  /** Hide the leading status dot. */
  noDot?: boolean;
}

/**
 * Canonical licence badge — tone derived from status, always mono (SPDX
 * identifiers read cleaner as mono), optional status dot on the left.
 */
export const LicencePill = forwardRef<HTMLSpanElement, LicencePillProps>(function LicencePill(
  { status, spdx, noDot, children, ...props },
  ref,
) {
  return (
    <Pill ref={ref} tone={status} mono {...props}>
      {!noDot && <StatusDot tone={status} />}
      {spdx}
      {children}
    </Pill>
  );
});
