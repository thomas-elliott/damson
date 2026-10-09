import { useState } from "react";

import { Shell } from "@/components/Shell";
import { ApiTokensSection } from "@/components/settings/ApiTokensSection";
import { LibraryMembersSection } from "@/components/settings/LibraryMembersSection";
import { LibraryProfileSection } from "@/components/settings/LibraryProfileSection";
import { ServerAdminSection } from "@/components/settings/ServerAdminSection";
import {
  Badge,
  Breadcrumb,
  BreadcrumbItem,
  Checkbox,
  EmptyState,
  Input,
  KeyValueList,
  Section,
  SegmentedControl,
  Toolbar,
} from "@/design-system";
import { useLibrarySettings, useMe } from "@/hooks/useDataQueries";
import { usePatchLibrarySettings, type LibrarySettingsPatch } from "@/hooks/useMutations";
import { useSavedFlash } from "@/hooks/useSavedFlash";

/**
 * Library settings surface. Admins can edit the toggles + theme/grid
 * defaults; members see the same values rendered read-only. All writes go
 * via PATCH /api/settings, which the handler gates with `requireAdmin`.
 *
 * Sysadmins also see the server-admin section (users, server settings).
 */
export function SettingsPage() {
  const { data: me } = useMe();
  const { data: settings, loading } = useLibrarySettings(!!me?.current_library);
  const isAdmin = me?.current_library?.role === "admin";
  const isSysadmin = me?.role === "sysadmin";

  if (!me) return null;

  return (
    <Shell
      main={
        <>
          <Toolbar>
            <Breadcrumb>
              <BreadcrumbItem current>Settings</BreadcrumbItem>
            </Breadcrumb>
          </Toolbar>
          {!me.current_library ? (
            <EmptyState
              title="No library"
              description="You need to be in a library to configure its settings."
            />
          ) : loading ? (
            <div className="flex flex-1 items-center justify-center text-ui text-subtle-foreground">
              Loading…
            </div>
          ) : !settings ? (
            <EmptyState title="Settings unavailable" />
          ) : (
            <div className="min-h-0 flex-1 overflow-y-auto">
              <div className="mx-auto flex max-w-[720px] flex-col gap-4 p-4">
                <header>
                  <h1 className="text-heading-semibold text-foreground">Library settings</h1>
                  <p className="mt-1 text-ui text-subtle-foreground">
                    Applies to <span className="font-mono">{me.current_library.name}</span>. Changes
                    are audited.
                    {!isAdmin && <> You have read-only access — ask a library admin to edit.</>}
                  </p>
                </header>

                {isAdmin ? (
                  <AdminSettingsForm settings={settings} />
                ) : (
                  <ReadOnlySettings settings={settings} />
                )}

                <LibraryMembersSection libraryId={me.current_library.id} isAdmin={isAdmin} />

                <LibraryProfileSection
                  key={me.current_library.id}
                  library={me.current_library}
                  isAdmin={isAdmin}
                />

                <ApiTokensSection />

                {isSysadmin && <ServerAdminSection />}
              </div>
            </div>
          )}
        </>
      }
    />
  );
}

interface SettingsView {
  grid_density_default: "small" | "medium" | "large";
  retain_original_packages: boolean;
  dedupe_enabled: boolean;
  thumbnails_raster_enabled: boolean;
  thumbnails_audio_enabled: boolean;
  thumbnails_font_enabled: boolean;
  thumbnails_unity_preview: boolean;
  auto_enrich_enabled: boolean;
  preferred_store: string;
  audit_retention_days: number | null;
}

const PREFERRED_STORE_OPTIONS = [
  { value: "unity", label: "Unity" },
  { value: "sketchfab", label: "Sketchfab" },
  { value: "itch", label: "itch.io" },
];

function ReadOnlySettings({ settings }: { settings: SettingsView }) {
  return (
    <>
      <Section title="Appearance" defaultOpen>
        <KeyValueList
          entries={[
            {
              key: "Grid density default",
              value: (
                <Badge size="sm" mono>
                  {settings.grid_density_default}
                </Badge>
              ),
            },
          ]}
        />
      </Section>
      <Section title="Storage" defaultOpen>
        <KeyValueList
          entries={[
            {
              key: "Retain original packages",
              value: boolBadge(settings.retain_original_packages),
            },
            { key: "Dedupe enabled", value: boolBadge(settings.dedupe_enabled) },
          ]}
        />
      </Section>
      <Section title="Thumbnails" defaultOpen>
        <KeyValueList
          entries={[
            { key: "Raster", value: boolBadge(settings.thumbnails_raster_enabled) },
            { key: "Audio waveform", value: boolBadge(settings.thumbnails_audio_enabled) },
            { key: "Font sample", value: boolBadge(settings.thumbnails_font_enabled) },
            { key: "Unity preview", value: boolBadge(settings.thumbnails_unity_preview) },
          ]}
        />
      </Section>
      <Section title="Metadata" defaultOpen>
        <KeyValueList
          entries={[
            { key: "Auto-enrich on import", value: boolBadge(settings.auto_enrich_enabled) },
            {
              key: "Preferred store",
              value: (
                <Badge size="sm" mono>
                  {settings.preferred_store}
                </Badge>
              ),
            },
          ]}
        />
      </Section>
      <Section title="Audit" defaultOpen>
        <KeyValueList
          entries={[
            {
              key: "Retention",
              value:
                settings.audit_retention_days == null
                  ? "Unbounded"
                  : `${settings.audit_retention_days} days`,
            },
          ]}
        />
      </Section>
    </>
  );
}

function AdminSettingsForm({ settings }: { settings: SettingsView }) {
  const patch = usePatchLibrarySettings();
  // Flash a "Saved" indicator after each successful save — driven from the
  // mutation's onSuccess callback rather than an effect watching its result.
  const { saved, flash } = useSavedFlash();

  const apply = (p: LibrarySettingsPatch) => patch.mutate(p, { onSuccess: flash });

  return (
    <>
      <Section
        title="Appearance"
        defaultOpen
        accessory={
          saved ? (
            <Badge size="sm" tone="ok">
              Saved
            </Badge>
          ) : null
        }
      >
        <FormRow label="Grid density default">
          <SegmentedControl
            value={settings.grid_density_default}
            onValueChange={(v) => apply({ grid_density_default: v })}
            options={[
              { value: "small", label: "Small" },
              { value: "medium", label: "Medium" },
              { value: "large", label: "Large" },
            ]}
          />
        </FormRow>
      </Section>

      <Section title="Storage" defaultOpen>
        <ToggleRow
          label="Retain original packages"
          help="Keep the uploaded archive after extraction so re-extraction is possible."
          checked={settings.retain_original_packages}
          onChange={(v) => apply({ retain_original_packages: v })}
        />
        <ToggleRow
          label="Dedupe enabled"
          help="Within this library, identical re-uploads collapse onto the same asset."
          checked={settings.dedupe_enabled}
          onChange={(v) => apply({ dedupe_enabled: v })}
        />
      </Section>

      <Section title="Thumbnails" defaultOpen>
        <ToggleRow
          label="Raster"
          help="PNG / JPEG / WebP / GIF / BMP / TIFF inputs."
          checked={settings.thumbnails_raster_enabled}
          onChange={(v) => apply({ thumbnails_raster_enabled: v })}
        />
        <ToggleRow
          label="Audio waveform"
          help="WAV / MP3 / OGG inputs render as an amplitude strip."
          checked={settings.thumbnails_audio_enabled}
          onChange={(v) => apply({ thumbnails_audio_enabled: v })}
        />
        <ToggleRow
          label="Font sample"
          help="TTF / OTF inputs render a sample-text PNG."
          checked={settings.thumbnails_font_enabled}
          onChange={(v) => apply({ thumbnails_font_enabled: v })}
        />
        <ToggleRow
          label="Unity preview harvest"
          help="Use a package's embedded preview.png as its cover. Off: the store cover if there is one, otherwise the placeholder. Applies to new imports."
          checked={settings.thumbnails_unity_preview}
          onChange={(v) => apply({ thumbnails_unity_preview: v })}
        />
      </Section>

      <Section title="Metadata" defaultOpen>
        <ToggleRow
          label="Auto-enrich on import"
          help="After import, search storefronts by name and auto-fill blank fields (description, licence, cover) on a high-confidence match. Manual fetch stays available regardless."
          checked={settings.auto_enrich_enabled}
          onChange={(v) => apply({ auto_enrich_enabled: v })}
        />
        <FormRow
          label="Preferred store"
          help="Listings from this store sort first in search results and win ties on auto-enrich."
        >
          <SegmentedControl
            value={settings.preferred_store}
            onValueChange={(v) => apply({ preferred_store: v })}
            options={PREFERRED_STORE_OPTIONS}
          />
        </FormRow>
      </Section>

      <Section title="Audit" defaultOpen>
        <FormRow
          label="Retention (days)"
          help="Leave blank for unbounded. The retention policy is recorded but not yet enforced."
        >
          <RetentionInput
            key={String(settings.audit_retention_days)}
            value={settings.audit_retention_days}
            onCommit={(v) => apply({ audit_retention_days: v })}
          />
        </FormRow>
      </Section>
    </>
  );
}

function FormRow({
  label,
  help,
  children,
}: {
  label: string;
  help?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-4 py-2">
      <div className="flex flex-col">
        <span className="text-ui text-foreground">{label}</span>
        {help && <span className="text-caption">{help}</span>}
      </div>
      {children}
    </div>
  );
}

function ToggleRow({
  label,
  help,
  checked,
  onChange,
}: {
  label: string;
  help?: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <FormRow label={label} help={help}>
      <Checkbox aria-label={label} checked={checked} onCheckedChange={(v) => onChange(!!v)} />
    </FormRow>
  );
}

function RetentionInput({
  value,
  onCommit,
}: {
  value: number | null;
  onCommit: (v: number | null) => void;
}) {
  // Re-seeded from `value` via the key={value} the caller sets, so a committed
  // change remounts this input instead of syncing through an effect.
  const [draft, setDraft] = useState(value == null ? "" : String(value));
  return (
    <Input
      type="number"
      min={1}
      placeholder="Unbounded"
      value={draft}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={() => {
        if (draft.trim() === "") {
          if (value !== null) onCommit(null);
          return;
        }
        const n = Number(draft);
        if (!Number.isFinite(n) || n <= 0) return;
        if (n !== value) onCommit(n);
      }}
      className="w-32"
    />
  );
}

function boolBadge(b: boolean) {
  return (
    <Badge size="sm" tone={b ? "ok" : "unk"}>
      {b ? "On" : "Off"}
    </Badge>
  );
}
