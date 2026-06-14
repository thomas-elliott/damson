import { type LucideProps } from "lucide-react";

import { fileTypeMeta, type FileType } from "./fileTypes";

export interface FileGlyphProps extends LucideProps {
  ftype: FileType;
}

/** Renders the lucide icon for a file type. Falls back to a generic file. */
export function FileGlyph({ ftype, ...props }: FileGlyphProps) {
  const { Icon } = fileTypeMeta(ftype);
  return <Icon {...props} />;
}
