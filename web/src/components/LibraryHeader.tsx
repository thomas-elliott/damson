import type { Me } from "@/hooks/useDataQueries";

// A minimal top strip showing the current library name + the user's email.
// Placeholder for the full nav rail + library switcher; the important bit is
// that the library name is surfaced somewhere so it's obvious which
// workspace you're acting in.
export function LibraryHeader({ me }: { me: Me }) {
  return (
    <header className="flex items-center justify-between border-b border-line bg-panel px-4 py-2 text-xs">
      <div className="flex items-center gap-2">
        <span className="font-medium text-text">
          {me.current_library?.name ?? "No library"}
        </span>
        {me.current_library && (
          <span className="rounded bg-surface px-1.5 py-0.5 font-mono text-text-3">
            {me.current_library.role}
          </span>
        )}
      </div>
      <div className="font-mono text-text-3">{me.email}</div>
    </header>
  );
}
