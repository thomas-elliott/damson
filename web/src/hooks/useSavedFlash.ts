import { useCallback, useEffect, useRef, useState } from "react";

// useSavedFlash drives a transient "Saved" indicator. Call `flash()` from a
// mutation's onSuccess callback (an event, not an effect) — it shows the badge
// and auto-hides it after `durationMs`. Keeping the state change in an event
// callback (rather than a useEffect watching the mutation result) is what keeps
// us clear of react-hooks/set-state-in-effect.
export function useSavedFlash(durationMs = 1500) {
  const [saved, setSaved] = useState(false);
  const timer = useRef<number | null>(null);

  const flash = useCallback(() => {
    setSaved(true);
    if (timer.current) window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setSaved(false), durationMs);
  }, [durationMs]);

  useEffect(
    () => () => {
      if (timer.current) window.clearTimeout(timer.current);
    },
    [],
  );

  return { saved, flash };
}
