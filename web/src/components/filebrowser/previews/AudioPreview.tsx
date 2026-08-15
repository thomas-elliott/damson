import { useEffect, useRef, useState } from "react";
import { Pause, Play } from "lucide-react";
import WaveSurfer from "wavesurfer.js";

import type { FileNode } from "@/design-system";

import { useObjectUrl } from "../useFileObjectUrl";
import { audioMime } from "./previewKind";

/**
 * Waveform + transport for natively-decodable audio (wav/mp3/ogg/flac). Builds
 * an auth-blob object URL and hands it to wavesurfer. Default export so the
 * detail panel can lazy-load it (wavesurfer stays out of the main bundle).
 */
export default function AudioPreview({ node }: { node: FileNode }) {
  const url = useObjectUrl(node.objectKey, audioMime(node));
  const containerRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WaveSurfer | null>(null);
  const [playing, setPlaying] = useState(false);
  const [ready, setReady] = useState(false);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const container = containerRef.current;
    if (!url || !container) return;
    const ws = WaveSurfer.create({
      container,
      url,
      height: 96,
      waveColor: "#6b6657",
      progressColor: "#caa84a",
      cursorColor: "#e8e2d4",
      barWidth: 2,
      barGap: 1,
      barRadius: 2,
    });
    wsRef.current = ws;
    ws.on("ready", () => setReady(true));
    ws.on("play", () => setPlaying(true));
    ws.on("pause", () => setPlaying(false));
    ws.on("finish", () => setPlaying(false));
    ws.on("error", () => setFailed(true));
    return () => {
      ws.destroy();
      wsRef.current = null;
      setReady(false);
      setPlaying(false);
      setFailed(false);
    };
  }, [url]);

  if (failed) {
    return (
      <div className="grid h-full w-full place-items-center p-4 text-center text-ui text-subtle-foreground">
        Couldn’t decode this audio file.
      </div>
    );
  }

  return (
    <div className="flex h-full w-full flex-col justify-center gap-4 bg-muted p-4">
      <div ref={containerRef} className="w-full" />
      <div className="flex items-center justify-center">
        <button
          type="button"
          aria-label={playing ? "Pause" : "Play"}
          disabled={!ready}
          onClick={() => wsRef.current?.playPause()}
          className="grid h-11 w-11 place-items-center rounded-full bg-primary text-primary-foreground transition hover:bg-primary-hover disabled:opacity-40"
        >
          {playing ? <Pause size={18} /> : <Play size={18} className="translate-x-0.5" />}
        </button>
      </div>
      {!ready && (
        <div className="text-center text-label text-faint-foreground">Decoding audio…</div>
      )}
    </div>
  );
}
