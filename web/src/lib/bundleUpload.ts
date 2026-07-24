import { Zip, ZipPassThrough } from "fflate";

export const LARGE_BUNDLE_BYTES = 2 * 1024 * 1024 * 1024;
export const LARGE_BUNDLE_COUNT = 500;

const ARCHIVE_RE = /\.(zip|unitypackage)$/i;

export function isArchiveFile(file: File): boolean {
  return ARCHIVE_RE.test(file.name);
}

function relativePath(file: File): string {
  const withPath = file as File & { path?: string };
  const raw = withPath.path || file.webkitRelativePath || file.name;
  return raw.replace(/^\.?\/+/, "");
}

export function hasFolderStructure(files: File[]): boolean {
  return files.some((f) => relativePath(f).includes("/"));
}

export function topFolderName(files: File[]): string | undefined {
  for (const f of files) {
    const p = relativePath(f);
    if (p.includes("/")) return p.slice(0, p.indexOf("/"));
  }
  return undefined;
}

export function bundleName(files: File[]): string {
  return `${topFolderName(files) ?? "bundle"}.zip`;
}

export function bundleFilesToZip(files: File[], name: string): Promise<File> {
  return new Promise((resolve, reject) => {
    const chunks: Uint8Array[] = [];
    const zip = new Zip((err, chunk, final) => {
      if (err) {
        reject(err);
        return;
      }
      if (chunk) chunks.push(chunk);
      if (final) resolve(new File(chunks as BlobPart[], name, { type: "application/zip" }));
    });

    void (async () => {
      try {
        for (const file of files) {
          const entry = new ZipPassThrough(relativePath(file));
          entry.mtime = file.lastModified;
          zip.add(entry);
          await pushFile(entry, file);
        }
        zip.end();
      } catch (e) {
        reject(e instanceof Error ? e : new Error(String(e)));
      }
    })();
  });
}

async function pushFile(entry: ZipPassThrough, file: File): Promise<void> {
  // Stream in the browser (bounded memory for large files); fall back to
  // arrayBuffer() where File.stream() is absent (jsdom, older runtimes).
  if (typeof file.stream === "function") {
    const reader = file.stream().getReader();
    for (;;) {
      const { done, value } = await reader.read();
      if (done) {
        entry.push(new Uint8Array(0), true);
        return;
      }
      entry.push(value);
    }
  }
  entry.push(new Uint8Array(await file.arrayBuffer()), true);
}
