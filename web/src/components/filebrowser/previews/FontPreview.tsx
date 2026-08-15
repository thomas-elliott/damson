import { useEffect, useId, useState } from "react";

import type { FileNode } from "@/design-system";

import { useObjectUrl } from "../useFileObjectUrl";

const DEFAULT_SPECIMEN = "The quick brown fox jumps over the lazy dog";
const SIZES = [48, 32, 24, 16];

/**
 * Live font specimen: loads the file as a `FontFace` from an auth-blob object
 * URL under a unique family name (so it never collides or caches across files)
 * and renders editable sample text at several sizes. Default export for lazy
 * loading. No third-party library — the browser's own font engine renders it.
 */
export default function FontPreview({ node }: { node: FileNode }) {
  // font/ttf is a safe generic the browser sniffs past; the FontFace API reads
  // the actual sfnt/woff format from the bytes regardless.
  const url = useObjectUrl(node.objectKey, "font/ttf");
  const family = `preview-${useId().replace(/[:]/g, "")}`;
  const [loaded, setLoaded] = useState(false);
  const [failed, setFailed] = useState(false);
  const [text, setText] = useState(DEFAULT_SPECIMEN);

  useEffect(() => {
    if (!url) return;
    let cancelled = false;
    const face = new FontFace(family, `url(${url})`);
    face
      .load()
      .then((f) => {
        if (cancelled) return;
        document.fonts.add(f);
        setLoaded(true);
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });
    return () => {
      cancelled = true;
      document.fonts.delete(face);
      setLoaded(false);
      setFailed(false);
    };
  }, [url, family]);

  if (failed) {
    return (
      <div className="grid h-full w-full place-items-center p-4 text-center text-ui text-subtle-foreground">
        Couldn’t load this font.
      </div>
    );
  }

  return (
    <div className="flex h-full w-full flex-col gap-3 overflow-auto bg-muted p-4">
      <input
        aria-label="Specimen text"
        value={text}
        onChange={(e) => setText(e.target.value)}
        className="w-full rounded-md border border-border bg-background px-2.5 py-1.5 text-ui text-foreground outline-none focus:border-primary-hover"
      />
      <div style={{ fontFamily: loaded ? family : undefined }} className="flex flex-col gap-2">
        {SIZES.map((size) => (
          <div key={size} className="flex items-baseline gap-3">
            <span className="w-9 shrink-0 font-mono text-[10px] text-faint-foreground">{size}</span>
            <span
              style={{ fontSize: size, lineHeight: 1.2 }}
              className="break-words text-foreground"
            >
              {text || DEFAULT_SPECIMEN}
            </span>
          </div>
        ))}
      </div>
      {!loaded && <div className="text-small text-faint-foreground">Loading font…</div>}
    </div>
  );
}
