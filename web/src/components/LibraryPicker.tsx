import { Check, ChevronDown } from "lucide-react";

import {
  Badge,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/design-system";
import { useLibraries, type MeLibrary } from "@/hooks/useDataQueries";
import { useSwitchCurrentLibrary } from "@/hooks/useMutations";

export function LibraryPicker({ current }: { current: MeLibrary }) {
  const { data: libraries } = useLibraries();
  const switchLib = useSwitchCurrentLibrary();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="Switch library"
        className="group flex min-w-0 items-center gap-1.5 rounded-sm px-1 py-0.5 text-ui hover:bg-surface focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-accent"
      >
        <span className="truncate font-medium text-text">{current.name}</span>
        <Badge size="sm" mono>
          {current.role}
        </Badge>
        <ChevronDown
          size={12}
          className="shrink-0 text-text-3 transition-transform duration-std ease group-data-[state=open]:rotate-180"
        />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-[14rem]">
        <DropdownMenuLabel>Switch library</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {libraries.map((lib) => {
          const active = lib.library_id === current.id;
          return (
            <DropdownMenuItem
              key={lib.library_id}
              aria-label={`Switch to ${lib.name}`}
              disabled={active || switchLib.isPending}
              onSelect={() => {
                if (active) return;
                switchLib.mutate(lib.library_id);
              }}
              className="justify-between gap-3"
            >
              <span className="flex min-w-0 items-center gap-2">
                <span className="grid h-3.5 w-3.5 place-items-center text-accent">
                  {active ? <Check size={12} strokeWidth={3} /> : null}
                </span>
                <span className="truncate">{lib.name}</span>
              </span>
              <Badge size="sm" mono>
                {lib.role}
              </Badge>
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
