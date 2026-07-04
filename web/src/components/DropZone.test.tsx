import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { DropZone, useDropZoneOpen } from "./DropZone";

const startUpload = vi.fn();
let capturedOnDrop: ((files: File[]) => void) | undefined;

vi.mock("react-dropzone", () => ({
  useDropzone: (opts: { onDrop: (files: File[]) => void }) => {
    capturedOnDrop = opts.onDrop;
    return {
      getRootProps: () => ({}),
      getInputProps: () => ({}),
      isDragActive: false,
      open: vi.fn(),
    };
  },
}));

vi.mock("@/components/notifications/NotificationProvider", () => ({
  useNotifications: () => ({ startUpload }),
}));

function OpenProbe() {
  const dz = useDropZoneOpen();
  return <span>{dz ? "has-open" : "no-open"}</span>;
}

describe("DropZone", () => {
  it("hands dropped files to startUpload and exposes the picker via context", () => {
    render(
      <DropZone>
        <OpenProbe />
      </DropZone>,
    );

    expect(screen.getByText("has-open")).toBeInTheDocument();

    const files = [new File(["a"], "a.zip")];
    capturedOnDrop?.(files);
    expect(startUpload).toHaveBeenCalledWith(files);
  });

  it("ignores an empty drop", () => {
    render(
      <DropZone>
        <span>child</span>
      </DropZone>,
    );
    startUpload.mockClear();
    capturedOnDrop?.([]);
    expect(startUpload).not.toHaveBeenCalled();
  });
});
