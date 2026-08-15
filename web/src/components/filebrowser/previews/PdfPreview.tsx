import { useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import {
  GlobalWorkerOptions,
  getDocument,
  type PDFDocumentProxy,
  type RenderTask,
} from "pdfjs-dist";
// Served same-origin (bundled by Vite) so it satisfies the CSP's default-src
// 'self' worker fallback — no blob: worker needed.
import workerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";

import { fetchBlob } from "@/api";
import type { FileNode } from "@/design-system";

GlobalWorkerOptions.workerSrc = workerUrl;

/**
 * Interactive PDF preview via pdf.js, rendered page-by-page to a canvas from
 * the in-memory file bytes. Hardened for our CSP: isEvalSupported:false (no
 * Function-constructor codepaths, so no 'unsafe-eval') and enableScripting:false
 * (embedded PDF JavaScript never runs). Default export for lazy loading.
 */
export default function PdfPreview({ node }: { node: FileNode }) {
  const objectKey = node.objectKey;
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const docRef = useRef<PDFDocumentProxy | null>(null);
  const [numPages, setNumPages] = useState(0);
  const [page, setPage] = useState(1);
  const [error, setError] = useState(false);

  // Load the document once per file.
  useEffect(() => {
    if (!objectKey) return;
    let cancelled = false;
    // isEvalSupported:false keeps pdf.js off its Function-constructor codepaths
    // so it runs under our CSP without 'unsafe-eval'. v6 dropped it from the
    // public type but still honours it at runtime. v6's low-level getDocument
    // does not execute embedded PDF scripts (no scripting manager is wired).
    let task: ReturnType<typeof getDocument> | undefined;
    fetchBlob(`/api/files/${objectKey}`)
      .then((blob) => blob.arrayBuffer())
      .then((data) => {
        task = getDocument({ data, isEvalSupported: false } as Parameters<typeof getDocument>[0]);
        return task.promise;
      })
      .then((doc) => {
        if (cancelled) return;
        docRef.current = doc;
        setPage(1);
        setNumPages(doc.numPages);
      })
      .catch(() => {
        if (!cancelled) setError(true);
      });
    return () => {
      cancelled = true;
      void task?.destroy();
      docRef.current = null;
      setNumPages(0);
    };
  }, [objectKey]);

  // Render the current page whenever it (or the loaded doc) changes.
  useEffect(() => {
    const doc = docRef.current;
    const canvas = canvasRef.current;
    if (!doc || !canvas || numPages === 0) return;
    let cancelled = false;
    let task: RenderTask | undefined;
    doc
      .getPage(page)
      .then((pdfPage) => {
        if (cancelled) return;
        const viewport = pdfPage.getViewport({ scale: 1.5 });
        const ctx = canvas.getContext("2d");
        if (!ctx) return;
        canvas.width = viewport.width;
        canvas.height = viewport.height;
        task = pdfPage.render({ canvas, canvasContext: ctx, viewport });
        return task.promise;
      })
      .catch(() => {
        /* render cancellations are expected when paging quickly */
      });
    return () => {
      cancelled = true;
      task?.cancel();
    };
  }, [page, numPages]);

  if (error) {
    return (
      <div className="grid h-full w-full place-items-center p-4 text-center text-ui text-subtle-foreground">
        Couldn’t open this PDF.
      </div>
    );
  }

  return (
    <div className="flex h-full w-full flex-col bg-muted">
      <div className="min-h-0 flex-1 overflow-auto p-3">
        <canvas ref={canvasRef} className="mx-auto max-w-full shadow-md" />
      </div>
      {numPages > 1 && (
        <div className="flex shrink-0 items-center justify-center gap-3 border-t border-border py-1.5 text-label text-subtle-foreground">
          <button
            type="button"
            aria-label="Previous page"
            disabled={page <= 1}
            onClick={() => setPage((p) => Math.max(1, p - 1))}
            className="grid h-7 w-7 place-items-center rounded-md hover:bg-accent disabled:opacity-30"
          >
            <ChevronLeft size={15} />
          </button>
          <span className="tabular-nums">
            {page} / {numPages}
          </span>
          <button
            type="button"
            aria-label="Next page"
            disabled={page >= numPages}
            onClick={() => setPage((p) => Math.min(numPages, p + 1))}
            className="grid h-7 w-7 place-items-center rounded-md hover:bg-accent disabled:opacity-30"
          >
            <ChevronRight size={15} />
          </button>
        </div>
      )}
    </div>
  );
}
