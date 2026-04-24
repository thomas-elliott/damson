import { Shell } from "@/components/Shell";
import {
  Badge,
  Breadcrumb,
  BreadcrumbItem,
  EmptyState,
  KeyValueList,
  Section,
  Toolbar,
} from "@/design-system";
import { useLibrarySettings, useMe } from "@/hooks/useDataQueries";

/**
 * Library settings surface. Members see read-only values; admins will be
 * able to PATCH once the edit form lands — the PATCH handler is wired and
 * returns 403 for non-admins, so this page can get away with read-only for
 * the first M5 slice.
 */
export function SettingsPage() {
  const { data: me } = useMe();
  const { data: settings, loading } = useLibrarySettings(!!me?.current_library);

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
            <div className="flex flex-1 items-center justify-center text-sm-plus text-text-3">
              Loading…
            </div>
          ) : !settings ? (
            <EmptyState title="Settings unavailable" />
          ) : (
            <div className="mx-auto flex max-w-[720px] flex-col gap-4 p-4">
              <header>
                <h1 className="text-xl font-semibold text-text">Library settings</h1>
                <p className="mt-1 text-sm-plus text-text-3">
                  Applies to <span className="font-mono">{me.current_library.name}</span>. Changes are audited.
                  {me.current_library.role !== "admin" && (
                    <> You have read-only access — ask a library admin to edit.</>
                  )}
                </p>
              </header>

              <Section title="Appearance" defaultOpen>
                <KeyValueList
                  entries={[
                    { key: "Theme default", value: <Badge size="sm" mono>{settings.theme_default}</Badge> },
                    { key: "Grid density default", value: <Badge size="sm" mono>{settings.grid_density_default}</Badge> },
                  ]}
                />
              </Section>

              <Section title="Storage" defaultOpen>
                <KeyValueList
                  entries={[
                    { key: "Retain original packages", value: boolLabel(settings.retain_original_packages) },
                    { key: "Dedupe enabled", value: boolLabel(settings.dedupe_enabled) },
                  ]}
                />
              </Section>

              <Section title="Thumbnails" defaultOpen>
                <KeyValueList
                  entries={[
                    { key: "Raster", value: boolLabel(settings.thumbnails_raster_enabled) },
                    { key: "Audio waveform", value: boolLabel(settings.thumbnails_audio_enabled) },
                    { key: "Font sample", value: boolLabel(settings.thumbnails_font_enabled) },
                    { key: "Unity preview", value: boolLabel(settings.thumbnails_unity_preview) },
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
            </div>
          )}
        </>
      }
    />
  );
}

function boolLabel(b: boolean) {
  return (
    <Badge size="sm" tone={b ? "ok" : "unk"}>
      {b ? "On" : "Off"}
    </Badge>
  );
}
