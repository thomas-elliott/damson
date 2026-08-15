import { Fragment, forwardRef, type ReactNode } from "react";
import type { SyntaxNode } from "@lezer/common";
import { GFM, parser as baseParser } from "@lezer/markdown";

import { cn } from "@/design-system/utils";

const parser = baseParser.configure(GFM);

export interface MarkdownProps {
  /** CommonMark + GFM source. Untrusted input is safe: no raw HTML is rendered. */
  children: string;
  className?: string;
}

/**
 * Renders markdown as React elements using CodeMirror's own `@lezer/markdown`
 * parser — the same grammar {@link MarkdownEditor} highlights with, so the read
 * and edit views can never disagree about what a document means.
 *
 * Raw HTML in the source is emitted as literal text, and link targets are
 * limited to http/https/mailto, so store-scraped descriptions can be rendered
 * without a sanitiser. Images render as labelled links rather than `<img>` —
 * see the `Image` handler.
 */
export const Markdown = forwardRef<HTMLDivElement, MarkdownProps>(function Markdown(
  { children, className },
  ref,
) {
  const tree = parser.parse(children);
  return (
    <div ref={ref} className={cn("flex flex-col gap-2 text-ui text-muted-foreground", className)}>
      {renderChildren(tree.topNode, children, "inline")}
    </div>
  );
});

type Mode = "inline" | "block";

const SKIPPED = new Set([
  "HeaderMark",
  "ListMark",
  "QuoteMark",
  "EmphasisMark",
  "CodeMark",
  "LinkMark",
  "CodeInfo",
  "LinkTitle",
  "TableDelimiter",
  // Rows are pulled out by the Table handler so <thead>/<tbody> stay well-formed.
  "TableHeader",
  "TableRow",
  "TaskMarker",
  "Comment",
  "CommentBlock",
  "LinkReference",
]);

const HEADING_CLASS: Record<number, string> = {
  1: "text-subhead-semibold text-foreground",
  2: "text-ui font-semibold text-foreground",
  3: "text-ui font-medium text-foreground",
};

function renderChildren(node: SyntaxNode, src: string, mode: Mode): ReactNode[] {
  return renderRange(node, src, mode, node.from, node.to);
}

// Renders the children of `node` that fall inside [from, to) — a link's visible
// label is the slice between its brackets, not the whole node (which also holds
// the target).
function renderRange(node: SyntaxNode, src: string, mode: Mode, from: number, to: number) {
  const out: ReactNode[] = [];
  let pos = from;
  let key = 0;
  const pushText = (raw: string) => {
    const text = mode === "inline" ? raw.replace(/\s*\n\s*/g, " ") : raw;
    if (text) out.push(<Fragment key={key++}>{text}</Fragment>);
  };
  for (let child = node.firstChild; child; child = child.nextSibling) {
    if (child.to <= from) continue;
    if (child.from >= to) break;
    if (child.from > pos) pushText(src.slice(pos, child.from));
    const rendered = renderNode(child, src, key++);
    if (rendered !== null) out.push(rendered);
    pos = child.to;
  }
  if (pos < to) pushText(src.slice(pos, to));
  return out;
}

function renderNode(node: SyntaxNode, src: string, key: number): ReactNode {
  const name = node.name;
  if (SKIPPED.has(name)) return null;

  const inline = () => renderChildren(node, src, "inline");

  if (name === "Paragraph") {
    return <p key={key}>{inline()}</p>;
  }
  const heading = /^(?:ATX|Setext)Heading([1-6])$/.exec(name);
  if (heading) {
    const level = Number(heading[1]);
    const Tag = `h${Math.min(level, 6)}` as "h1";
    return (
      <Tag key={key} className={cn("mt-1", HEADING_CLASS[level] ?? HEADING_CLASS[3])}>
        {inline()}
      </Tag>
    );
  }
  if (name === "BulletList" || name === "OrderedList") {
    const Tag = name === "BulletList" ? "ul" : "ol";
    return (
      <Tag
        key={key}
        className={cn("flex flex-col gap-1 pl-4", Tag === "ul" ? "list-disc" : "list-decimal")}
      >
        {renderChildren(node, src, "block")}
      </Tag>
    );
  }
  if (name === "ListItem") {
    return (
      <li key={key} className="[&>p]:inline">
        {renderChildren(node, src, "block")}
      </li>
    );
  }
  if (name === "Task") {
    const done = /^\[[xX]\]/.test(src.slice(node.from, node.to));
    return (
      <Fragment key={key}>
        <span aria-hidden className="mr-1 text-mono text-subtle-foreground">
          {done ? "[x]" : "[ ]"}
        </span>
        <span className="sr-only">{done ? "Done: " : "To do: "}</span>
        {inline()}
      </Fragment>
    );
  }
  if (name === "Blockquote") {
    return (
      <blockquote
        key={key}
        className="flex flex-col gap-2 border-l-2 border-border-strong pl-2.5 italic"
      >
        {renderChildren(node, src, "block")}
      </blockquote>
    );
  }
  if (name === "FencedCode" || name === "CodeBlock") {
    return (
      <pre
        key={key}
        className="overflow-x-auto rounded-sm border border-border bg-muted p-2 text-mono text-muted-foreground"
      >
        <code>{codeText(node, src)}</code>
      </pre>
    );
  }
  if (name === "InlineCode") {
    return (
      <code key={key} className="rounded-sm bg-muted px-1 py-0.5 text-mono text-foreground">
        {stripMarks(node, src)}
      </code>
    );
  }
  if (name === "Emphasis") return <em key={key}>{inline()}</em>;
  if (name === "StrongEmphasis") {
    return (
      <strong key={key} className="font-semibold text-foreground">
        {inline()}
      </strong>
    );
  }
  if (name === "Strikethrough") {
    return (
      <del key={key} className="text-subtle-foreground">
        {inline()}
      </del>
    );
  }
  if (name === "Link" || name === "Autolink" || name === "URL") {
    const href = safeHref(linkTarget(node, src));
    const label =
      name === "Link" ? linkLabel(node, src) : src.slice(node.from, node.to).replace(/^<|>$/g, "");
    if (!href) return <Fragment key={key}>{label}</Fragment>;
    return (
      <a
        key={key}
        href={href}
        target="_blank"
        rel="noreferrer noopener"
        className="break-words text-primary hover:underline"
      >
        {label}
      </a>
    );
  }
  if (name === "Image") {
    // Store descriptions carry hot-linked third-party images; rendering them
    // inline would leak a request per open and break on mixed content. Show
    // the alt text as a link out instead.
    const href = safeHref(linkTarget(node, src));
    const alt = altText(node, src) || "image";
    if (!href) return <Fragment key={key}>{alt}</Fragment>;
    return (
      <a
        key={key}
        href={href}
        target="_blank"
        rel="noreferrer noopener"
        className="text-primary hover:underline"
      >
        {alt}
      </a>
    );
  }
  if (name === "HorizontalRule") return <hr key={key} className="border-border" />;
  if (name === "HardBreak") return <br key={key} />;
  if (name === "Escape") return <Fragment key={key}>{src.slice(node.from + 1, node.to)}</Fragment>;
  if (name === "Entity") return <Fragment key={key}>{src.slice(node.from, node.to)}</Fragment>;

  if (name === "Table") {
    const head: ReactNode[] = [];
    const body: ReactNode[] = [];
    let row = 0;
    for (let child = node.firstChild; child; child = child.nextSibling) {
      if (child.name === "TableHeader") {
        head.push(<tr key={row++}>{cellNodes(child, src, true)}</tr>);
      } else if (child.name === "TableRow") {
        body.push(<tr key={row++}>{cellNodes(child, src, false)}</tr>);
      }
    }
    return (
      <div key={key} className="overflow-x-auto">
        <table className="w-full border-separate border-spacing-0 text-label">
          {head.length > 0 && <thead>{head}</thead>}
          <tbody>{body}</tbody>
        </table>
      </div>
    );
  }

  // Unknown node (raw HTML, footnotes, …): fall through to its text so nothing
  // silently disappears from the document.
  return <Fragment key={key}>{renderChildren(node, src, "inline")}</Fragment>;
}

function cellNodes(row: SyntaxNode, src: string, header: boolean): ReactNode[] {
  const out: ReactNode[] = [];
  let key = 0;
  for (let child = row.firstChild; child; child = child.nextSibling) {
    if (child.name !== "TableCell") continue;
    const content = renderChildren(child, src, "inline");
    out.push(
      header ? (
        <th key={key++} className="border-b border-border px-2 py-1 text-left text-foreground">
          {content}
        </th>
      ) : (
        <td key={key++} className="border-b border-border px-2 py-1">
          {content}
        </td>
      ),
    );
  }
  return out;
}

function codeText(node: SyntaxNode, src: string): string {
  for (let child = node.firstChild; child; child = child.nextSibling) {
    if (child.name === "CodeText") return src.slice(child.from, child.to);
  }
  return stripMarks(node, src);
}

function stripMarks(node: SyntaxNode, src: string): string {
  let from = node.from;
  let to = node.to;
  for (let child = node.firstChild; child; child = child.nextSibling) {
    if (child.name !== "CodeMark") continue;
    if (child.from === from) from = child.to;
    if (child.to === to) to = child.from;
  }
  return src.slice(from, to);
}

function linkLabel(node: SyntaxNode, src: string): ReactNode[] {
  let start = node.from;
  let end = node.to;
  for (let child = node.firstChild; child; child = child.nextSibling) {
    if (child.name !== "LinkMark") continue;
    const mark = src.slice(child.from, child.to);
    if (mark === "[" || mark === "![") start = child.to;
    else if (mark === "]") {
      end = child.from;
      break;
    }
  }
  return renderRange(node, src, "inline", start, end);
}

function linkTarget(node: SyntaxNode, src: string): string {
  for (let child = node.firstChild; child; child = child.nextSibling) {
    if (child.name === "URL") return src.slice(child.from, child.to).trim();
  }
  if (node.name === "Autolink" || node.name === "URL") {
    return src.slice(node.from, node.to).replace(/^<|>$/g, "").trim();
  }
  return "";
}

function altText(node: SyntaxNode, src: string): string {
  const raw = src.slice(node.from, node.to);
  return /!\[([^\]]*)\]/.exec(raw)?.[1]?.trim() ?? "";
}

const SAFE_SCHEME = /^(https?:|mailto:)/i;

function safeHref(target: string): string | null {
  if (!target) return null;
  const url = target.replace(/^<|>$/g, "").trim();
  if (url.startsWith("//") || url.startsWith("/") || url.startsWith("#")) return null;
  return SAFE_SCHEME.test(url) ? url : null;
}
