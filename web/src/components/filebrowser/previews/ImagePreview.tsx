import { FilePreview, type FileNode } from "@/design-system";

import { useFileObjectUrl } from "../useFileObjectUrl";

/**
 * Inline raster-image preview: resolves the file to an auth-blob object URL and
 * hands it to the presentational {@link FilePreview}. Falls back to a
 * placeholder while the bytes load.
 */
export function ImagePreview({ node }: { node: FileNode }) {
  const url = useFileObjectUrl(node);
  return <FilePreview node={node} previewUrl={url} large />;
}
