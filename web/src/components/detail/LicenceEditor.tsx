import { useState } from "react";
import { Check, Search, X } from "lucide-react";

import { fetchJson } from "@/api";
import {
  Badge,
  Button,
  FieldRow,
  KeyValueList,
  LicencePill,
  StatusDot,
  Textarea,
} from "@/design-system";
import type { LicenceAttributes, LicenceStatus, LicenceView } from "@/hooks/useDataQueries";
import { usePutLicence } from "@/hooks/useMutations";

interface DetectResponse {
  detected_type: string;
  detection_confidence: number;
  canonical_diff?: string;
  attributes: LicenceAttributes;
  status: LicenceStatus;
}

const ATTR_ROWS: { key: keyof LicenceAttributes; label: string }[] = [
  { key: "commercial_use", label: "Commercial use" },
  { key: "modification", label: "Modification" },
  { key: "redistribution", label: "Redistribution" },
  { key: "attribution_required", label: "Attribution required" },
  { key: "share_alike", label: "Share-alike" },
  { key: "open_source_required", label: "Open-source required" },
];

/**
 * Detail-panel licence section. If a licence exists, renders the attribute
 * matrix and original text (read-only). If none exists, shows a textarea +
 * "Detect & save" button wired to /api/licences/detect.
 */
export function LicenceEditor({
  assetId,
  existing,
  onReviewed,
}: {
  assetId: number;
  existing: LicenceView | null;
  onReviewed?: () => void;
}) {
  const put = usePutLicence();
  const [draft, setDraft] = useState(existing?.original_text ?? "");
  const [detection, setDetection] = useState<DetectResponse | null>(null);
  const [detecting, setDetecting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const runDetection = async () => {
    if (!draft.trim()) return;
    setDetecting(true);
    setError(null);
    try {
      const r = await fetchJson<DetectResponse>(`/api/licences/detect`, {
        method: "POST",
        body: JSON.stringify({ text: draft }),
      });
      setDetection(r);
    } catch (err) {
      setError(err instanceof Error ? err.message : "detection failed");
    } finally {
      setDetecting(false);
    }
  };

  const saveDetected = () => {
    if (!draft.trim()) return;
    put.mutate({
      assetId,
      original_text: draft,
      use_detected: true,
    });
    setDetection(null);
  };

  const saveAsCustom = () => {
    if (!draft.trim()) return;
    put.mutate({
      assetId,
      original_text: draft,
      detected_type: "custom",
      attributes: {},
    });
    setDetection(null);
  };

  if (existing) {
    return (
      <div className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-1.5">
          <LicencePill status={existing.status} spdx={existing.detected_type || "Unknown"} />
          {existing.detection_confidence > 0 && (
            <Badge size="sm" mono>
              {Math.round(existing.detection_confidence * 100)}% match
            </Badge>
          )}
          {existing.canonical_diff && (
            <Badge size="sm" tone="warn">
              modified from canonical
            </Badge>
          )}
          {existing.reviewed_at ? (
            <Badge size="sm" tone="ok">
              reviewed
            </Badge>
          ) : (
            <Button size="sm" variant="secondary" onClick={onReviewed}>
              <Check size={11} /> Mark reviewed
            </Button>
          )}
        </div>
        <AttributeGrid attrs={existing.attributes} />
        <details className="group">
          <summary className="cursor-pointer select-none text-small text-subtle-foreground hover:text-foreground">
            Original text
          </summary>
          <pre className="mt-2 max-h-[240px] overflow-auto rounded-sm border border-border bg-background p-2 text-mono text-muted-foreground whitespace-pre-wrap">
            {existing.original_text}
          </pre>
        </details>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      <FieldRow
        label="Paste licence text"
        description="Damson will try to match against SPDX + the Unity Asset Store EULA before saving."
      >
        <Textarea
          value={draft}
          onChange={(e) => setDraft(e.currentTarget.value)}
          placeholder="MIT License...&#10;Copyright (c) ..."
          rows={6}
        />
      </FieldRow>
      {detection && (
        <div className="flex flex-col gap-2 rounded-md border border-border-strong bg-background p-2.5">
          <div className="flex items-center gap-2">
            <LicencePill status={detection.status} spdx={detection.detected_type || "Unknown"} />
            <Badge size="sm" mono>
              {Math.round(detection.detection_confidence * 100)}% match
            </Badge>
            {detection.canonical_diff && (
              <Badge size="sm" tone="warn">
                modified
              </Badge>
            )}
          </div>
          <AttributeGrid attrs={detection.attributes} />
        </div>
      )}
      {error && (
        <div className="flex items-center gap-1 text-small text-destructive">
          <X size={11} /> {error}
        </div>
      )}
      <div className="flex flex-wrap items-center gap-2">
        <Button
          variant="secondary"
          leadingIcon={<Search size={12} />}
          onClick={runDetection}
          disabled={!draft.trim() || detecting}
        >
          {detecting ? "Detecting…" : "Detect"}
        </Button>
        <Button variant="primary" onClick={saveDetected} disabled={!draft.trim() || put.isPending}>
          Save detected
        </Button>
        <Button variant="ghost" onClick={saveAsCustom} disabled={!draft.trim()}>
          Save as custom
        </Button>
      </div>
    </div>
  );
}

function AttributeGrid({ attrs }: { attrs: LicenceAttributes }) {
  const entries = ATTR_ROWS.map(({ key, label }) => {
    const v = attrs[key];
    let tone: LicenceStatus = "unk";
    let value: string = "Unknown";
    if (v === true) {
      tone = "ok";
      value = "Yes";
    } else if (v === false) {
      tone = "err";
      value = "No";
    }
    return {
      key: label,
      value: (
        <span className="inline-flex items-center gap-1.5">
          <StatusDot tone={tone} />
          <span className={tone === "unk" ? "text-subtle-foreground" : "text-muted-foreground"}>
            {value}
          </span>
        </span>
      ),
    };
  });
  return <KeyValueList entries={entries} labelWidth="150px" />;
}
