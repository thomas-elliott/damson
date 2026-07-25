import { useNavigate } from "react-router";
import { LogOut, Moon, Settings as SettingsIcon, Sun } from "lucide-react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  NavRailItem,
  type ThemeMode,
} from "@/design-system";
import { useTheme } from "@/hooks/useTheme";
import { logout } from "@/lib/logout";

function initialsFor(displayName: string, email: string): string {
  const words = displayName.trim().split(/\s+/).filter(Boolean);
  if (words.length >= 2) return (words[0][0] + words[1][0]).toUpperCase();
  const single = words[0] ?? email.trim();
  if (!single) return "?";
  return single.slice(0, 2).toUpperCase();
}

export function UserMenu({ displayName, email }: { displayName: string; email: string }) {
  const { mode, setTheme } = useTheme();
  const navigate = useNavigate();

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <NavRailItem
          label="Account"
          icon={
            <span className="grid h-[22px] w-[22px] place-items-center rounded-full bg-surface-2 text-mono font-semibold text-text-2">
              {initialsFor(displayName, email)}
            </span>
          }
        />
      </DropdownMenuTrigger>
      <DropdownMenuContent side="right" align="end" className="min-w-[15rem]">
        <DropdownMenuLabel className="text-ui text-text">
          <span className="block truncate">{displayName || email}</span>
          {displayName && (
            <span className="block truncate font-mono text-small text-text-3">{email}</span>
          )}
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuLabel>Theme</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={mode} onValueChange={(next) => setTheme(next as ThemeMode)}>
          <DropdownMenuRadioItem value="light">
            <Sun size={13} className="text-text-3" />
            Light
          </DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="dark">
            <Moon size={13} className="text-text-3" />
            Dark
          </DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => navigate("/settings")}>
          <SettingsIcon size={13} className="text-text-3" />
          Settings
        </DropdownMenuItem>
        <DropdownMenuItem aria-label="Sign out" onSelect={() => void logout()}>
          <LogOut size={13} className="text-text-3" />
          Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
