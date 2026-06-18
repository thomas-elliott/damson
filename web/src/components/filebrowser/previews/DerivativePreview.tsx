import { FilePreview, type FileNode } from "@/design-system";

import { useBlobUrl } from "../useFileObjectUrl";

/**
 * Renders a file's backend-generated preview derivative (a flattened/rasterised
 * image served from GET /api/previews/:key). Used for browser-unopenable types
 * the import worker pre-rendered (pro/HDR images, SVG raster, PDF poster).
 * Falls back to the placeholder while the bytes load. Default export for lazy
 * loading.
 */
export default function DerivativePreview({ node }: { node: FileNode }) {
  const url = useBlobUrl(
    node.previewObjectKey ? `/api/previews/${node.previewObjectKey}` : undefined,
  );
  if (!url) return <FilePreview node={node} large />;
  return (
    <img src={url} alt={node.name} loading="lazy" className="block h-full w-full object-contain" />
  );
}
