import { useState } from "react";
import { Check, Pencil, Search, X } from "lucide-react";

import { fetchJson } from "@/api";
import {
  Badge,
  Button,
  FieldRow,
  KeyValueList,
  LicencePill,
  SegmentedControl,
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

const ATTR_ROWS: { key: Exclude<keyof LicenceAttributes, "custom_terms">; label: string }[] = [
  { key: "commercial_use", label: "Commercial use" },
  { key: "modification", label: "Modification" },
  { key: "redistribution", label: "Redistribution" },
  { key: "attribution_required", label: "Attribution required" },
  { key: "share_alike", label: "Share-alike" },
  { key: "open_source_required", label: "Open-source required" },
];

// SegmentedControl takes string values, so the tri-state attributes map
// through these.
type Tri = "yes" | "no" | "unknown";

const TRI_OPTIONS: { value: Tri; label: string }[] = [
  { value: "yes", label: "Yes" },
  { value: "no", label: "No" },
  { value: "unknown", label: "Unknown" },
];

const toTri = (v: boolean | null | undefined): Tri =>
  v === true ? "yes" : v === false ? "no" : "unknown";
const fromTri = (t: Tri): boolean | null => (t === "yes" ? true : t === "no" ? false : null);

async function detect(text: string): Promise<DetectResponse> {
  return fetchJson<DetectResponse>(`/api/licences/detect`, {
    method: "POST",
    body: JSON.stringify({ text }),
  });
}

/**
 * Detail-panel licence section. If a licence exists, renders the attribute
 * matrix and original text, and Edit swaps in LicenceEditForm. If none exists,
 * shows a textarea + "Detect & save" button wired to /api/licences/detect.
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
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(existing?.original_text ?? "");
  const [detection, setDetection] = useState<DetectResponse | null>(null);
  const [detecting, setDetecting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const runDetection = async () => {
    if (!draft.trim()) return;
    setDetecting(true);
    setError(null);
    try {
      setDetection(await detect(draft));
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

  if (existing && editing) {
    return (
      <LicenceEditForm assetId={assetId} existing={existing} onDone={() => setEditing(false)} />
    );
  }

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
          {/* Overview has its own Edit button, so this one carries a distinct
              accessible name. */}
          <Button
            size="sm"
            variant="ghost"
            aria-label="Edit licence"
            leadingIcon={<Pencil size={12} />}
            onClick={() => setEditing(true)}
          >
            Edit
          </Button>
        </div>
        <AttributeGrid attrs={existing.attributes} />
        <details className="group">
          <summary className="cursor-pointer select-none text-label text-subtle-foreground hover:text-foreground">
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
      {error && <ErrorLine message={error} />}
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

/**
 * Edits a set licence's text and attributes. Detect replaces the drafted type
 * and attributes with the detector's; when it finds no match the draft becomes
 * custom and keeps the person's attributes. A save never stores "unknown", so
 * the attributes a person sets always count towards the licence's status.
 */
function LicenceEditForm({
  assetId,
  existing,
  onDone,
}: {
  assetId: number;
  existing: LicenceView;
  onDone: () => void;
}) {
  // Called here rather than in LicenceEditor so each opening of the form
  // starts without an earlier save's error.
  const put = usePutLicence();
  const [text, setText] = useState(existing.original_text);
  const [type, setType] = useState(existing.detected_type);
  const [attrs, setAttrs] = useState<LicenceAttributes>(existing.attributes);
  // The text each Detect was sent with: once one has run, Save needs the
  // current text to be that text, so a saved type always matches its text.
  const [detection, setDetection] = useState<{ text: string; result: DetectResponse } | null>(null);
  const [detecting, setDetecting] = useState(false);
  const [detectError, setDetectError] = useState<string | null>(null);

  const textChecked = detection ? detection.text === text : text === existing.original_text;

  const runDetection = async () => {
    if (!text.trim()) return;
    const sent = text;
    setDetecting(true);
    setDetectError(null);
    try {
      const result = await detect(sent);
      setDetection({ text: sent, result });
      if (result.detected_type === "unknown") {
        setType("custom");
      } else {
        setType(result.detected_type);
        setAttrs((prev) => ({ ...result.attributes, custom_terms: prev.custom_terms }));
      }
    } catch (err) {
      setDetectError(err instanceof Error ? err.message : "detection failed");
    } finally {
      setDetecting(false);
    }
  };

  const save = () => {
    put.mutate(
      {
        assetId,
        original_text: text,
        detected_type: !type || type === "unknown" ? "custom" : type,
        attributes: attrs,
      },
      { onSuccess: onDone },
    );
  };

  const result = detection?.result;
  const error = detectError ?? (put.error ? put.error.message : null);

  return (
    <div className="flex flex-col gap-3">
      <FieldRow label="Licence text">
        <Textarea value={text} onChange={(e) => setText(e.currentTarget.value)} rows={6} />
      </FieldRow>
      {result && (
        <div className="flex flex-wrap items-center gap-2">
          {result.detected_type === "unknown" ? (
            <span className="text-label text-subtle-foreground">No match: will save as custom</span>
          ) : (
            <>
              <LicencePill status={result.status} spdx={result.detected_type} />
              <Badge size="sm" mono>
                {Math.round(result.detection_confidence * 100)}% match
              </Badge>
            </>
          )}
        </div>
      )}
      <AttributeEditGrid
        attrs={attrs}
        onChange={(key, value) => setAttrs((prev) => ({ ...prev, [key]: value }))}
      />
      {!textChecked && text.trim() && (
        <div className="text-label text-subtle-foreground">
          Run Detect on the changed text before saving.
        </div>
      )}
      {error && <ErrorLine message={error} />}
      <div className="flex flex-wrap items-center gap-2">
        <Button
          variant="secondary"
          leadingIcon={<Search size={12} />}
          onClick={runDetection}
          disabled={!text.trim() || detecting}
        >
          {detecting ? "Detecting…" : "Detect"}
        </Button>
        {/* Overview's edit form has its own Save and Cancel, so these carry
            distinct accessible names. */}
        <Button
          variant="primary"
          aria-label="Save licence"
          onClick={save}
          disabled={!text.trim() || !textChecked || detecting || put.isPending}
        >
          Save
        </Button>
        <Button variant="ghost" aria-label="Cancel licence edit" onClick={onDone}>
          Cancel
        </Button>
      </div>
    </div>
  );
}

function ErrorLine({ message }: { message: string }) {
  return (
    <div className="flex items-center gap-1 text-label text-destructive">
      <X size={11} /> {message}
    </div>
  );
}

function AttributeEditGrid({
  attrs,
  onChange,
}: {
  attrs: LicenceAttributes;
  onChange: (key: (typeof ATTR_ROWS)[number]["key"], value: boolean | null) => void;
}) {
  const entries = ATTR_ROWS.map(({ key, label }) => ({
    key: label,
    value: (
      <SegmentedControl
        aria-label={label}
        value={toTri(attrs[key])}
        onValueChange={(t) => onChange(key, fromTri(t))}
        options={TRI_OPTIONS}
      />
    ),
  }));
  return <KeyValueList entries={entries} labelWidth="150px" />;
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
