import { forwardRef } from "react";

import { Badge, type BadgeProps } from "./Badge";

/**
 * Pill is Badge with the pill shape preset. Kept as a separate export
 * because it is used heavily in the UI (licence pills, smart-collection
 * badges) and the name reads better at call-sites.
 */
export const Pill = forwardRef<HTMLSpanElement, Omit<BadgeProps, "shape">>(
  function Pill(props, ref) {
    return <Badge ref={ref} shape="pill" {...props} />;
  },
);
