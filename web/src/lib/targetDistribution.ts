// The distributions a project can target. The API stores the value; the UI
// shows the label.
export const TARGET_DISTRIBUTIONS = [
  { value: "unspecified", label: "Unspecified" },
  { value: "commercial", label: "Commercial" },
  { value: "open-source", label: "Open source" },
  { value: "personal", label: "Personal" },
] as const;

// targetDistributionLabel falls back to the raw value, so a value added
// server-side still renders.
export function targetDistributionLabel(value: string): string {
  return TARGET_DISTRIBUTIONS.find((d) => d.value === value)?.label ?? value;
}
