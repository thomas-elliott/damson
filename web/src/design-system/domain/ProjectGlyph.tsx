import { Glyph, type GlyphProps } from "@/design-system/primitives/Glyph";

export interface ProjectGlyphProps extends Omit<GlyphProps, "children"> {
  /** 1–2 character glyph, usually the project's first letter(s). */
  glyph: string;
}

/**
 * Coloured identity square for a project. Wraps the primitive Glyph with
 * project-specific semantics; callers provide a stable colour + glyph.
 */
export function ProjectGlyph({ glyph, ...props }: ProjectGlyphProps) {
  return <Glyph {...props}>{glyph}</Glyph>;
}
