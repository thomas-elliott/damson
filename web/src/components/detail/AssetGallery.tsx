import { useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, X } from "lucide-react";

import { cn, IconButton } from "@/design-system";
import { useBlobUrls, type BlobSource } from "@/components/filebrowser/useFileObjectUrl";
import type { AssetImageView } from "@/hooks/useDataQueries";

function thumbSource(image: AssetImageView): BlobSource {
  return { url: `/api/thumbs/${image.object_key}`, mime: image.content_type };
}

function wrapIndex(next: number, count: number): number {
  return ((next % count) + count) % count;
}

// AssetGallery shows the storefront gallery one image at a time — prev/next
// step through it, a click opens the lightbox on the image you're looking at.
// Every image stays mounted in the sliding track so stepping doesn't re-fetch.
export function AssetGallery({ images }: { images: AssetImageView[] }) {
  const urls = useBlobUrls(images.map(thumbSource));
  const [index, setIndex] = useState(0);
  const [zoomed, setZoomed] = useState(false);

  if (images.length === 0) return null;

  const current = Math.min(index, images.length - 1);
  const step = (delta: number) => setIndex(wrapIndex(current + delta, images.length));

  return (
    <div className="relative aspect-video w-full overflow-hidden rounded-md border border-border bg-muted">
      <div
        className="flex h-full w-full transition-transform duration-fast ease-standard"
        style={{ transform: `translateX(-${current * 100}%)` }}
      >
        {images.map((img, i) => (
          <button
            key={img.id}
            type="button"
            onClick={() => {
              setIndex(i);
              setZoomed(true);
            }}
            aria-label={`Enlarge preview image ${i + 1} of ${images.length}`}
            className="h-full w-full shrink-0 cursor-zoom-in"
          >
            <GalleryFrame url={urls[thumbSource(img).url]} className="object-cover" />
          </button>
        ))}
      </div>
      {images.length > 1 && (
        <>
          <GalleryArrow side="left" onClick={() => step(-1)} />
          <GalleryArrow side="right" onClick={() => step(1)} />
          <GalleryCount current={current} total={images.length} />
        </>
      )}
      {zoomed && (
        <GalleryLightbox
          images={images}
          urls={urls}
          index={current}
          onIndexChange={setIndex}
          onClose={() => setZoomed(false)}
        />
      )}
    </div>
  );
}

function GalleryFrame({ url, className }: { url?: string; className?: string }) {
  if (!url) return <div className="h-full w-full animate-pulse bg-muted" />;
  return <img src={url} alt="" className={cn("h-full w-full", className)} />;
}

function GalleryArrow({ side, onClick }: { side: "left" | "right"; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={side === "left" ? "Previous image" : "Next image"}
      className={cn(
        "absolute top-1/2 grid h-7 w-7 -translate-y-1/2 place-items-center rounded-full",
        "bg-black/55 text-white transition-colors duration-fast ease-standard hover:bg-black/80",
        "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring",
        side === "left" ? "left-1.5" : "right-1.5",
      )}
    >
      {side === "left" ? <ChevronLeft size={16} /> : <ChevronRight size={16} />}
    </button>
  );
}

function GalleryCount({ current, total }: { current: number; total: number }) {
  return (
    <span className="pointer-events-none absolute bottom-1.5 right-1.5 rounded-full bg-black/60 px-1.5 py-0.5 text-mono tabular-nums text-white">
      {current + 1} / {total}
    </span>
  );
}

// GalleryLightbox is the full-screen view of the same carousel. Keys are handled
// on the dialog itself (not the document) and propagation stopped, so the page's
// own arrow/escape shortcuts don't move the grid selection behind it.
function GalleryLightbox({
  images,
  urls,
  index,
  onIndexChange,
  onClose,
}: {
  images: AssetImageView[];
  urls: Record<string, string>;
  index: number;
  onIndexChange: (next: number) => void;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    ref.current?.focus();
  }, []);

  const step = (delta: number) => onIndexChange(wrapIndex(index + delta, images.length));

  return (
    <div
      ref={ref}
      role="dialog"
      aria-modal="true"
      aria-label={`Preview image ${index + 1} of ${images.length}`}
      tabIndex={-1}
      onClick={onClose}
      onKeyDown={(e) => {
        if (e.key === "Escape") onClose();
        else if (e.key === "ArrowLeft") step(-1);
        else if (e.key === "ArrowRight") step(1);
        else return;
        e.preventDefault();
        e.stopPropagation();
      }}
      style={{ zIndex: "var(--z-index-dialog)" }}
      className="fixed inset-0 flex items-center justify-center bg-black/80 p-6 focus-visible:outline-hidden"
    >
      <IconButton
        aria-label="Close preview"
        size="lg"
        onClick={onClose}
        // hover:text-white is not redundant: `ghost` carries hover:text-foreground,
        // which tailwind-merge keeps as a separate modifier, and that is near-black
        // in light theme.
        className="absolute right-4 top-4 text-white hover:bg-white/20 hover:text-white"
      >
        <X />
      </IconButton>
      <img
        src={urls[thumbSource(images[index]).url]}
        alt=""
        onClick={(e) => e.stopPropagation()}
        className="max-h-full max-w-full rounded-md object-contain"
      />
      {images.length > 1 && (
        <div onClick={(e) => e.stopPropagation()}>
          <LightboxArrow side="left" onClick={() => step(-1)} />
          <LightboxArrow side="right" onClick={() => step(1)} />
          <span className="pointer-events-none absolute bottom-6 left-1/2 -translate-x-1/2 rounded-full bg-black/60 px-2 py-0.5 text-mono tabular-nums text-white">
            {index + 1} / {images.length}
          </span>
        </div>
      )}
    </div>
  );
}

function LightboxArrow({ side, onClick }: { side: "left" | "right"; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={side === "left" ? "Previous image" : "Next image"}
      className={cn(
        "absolute top-1/2 grid h-10 w-10 -translate-y-1/2 place-items-center rounded-full",
        "bg-black/55 text-white transition-colors duration-fast ease-standard hover:bg-black/80",
        "focus-visible:outline-hidden focus-visible:ring-2 focus-visible:ring-ring",
        side === "left" ? "left-4" : "right-4",
      )}
    >
      {side === "left" ? <ChevronLeft size={22} /> : <ChevronRight size={22} />}
    </button>
  );
}
