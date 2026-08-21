import {
  EmptyState as RegistryEmptyState,
  type EmptyStateProps as RegistryEmptyStateProps,
} from "@telliott/shared-ui";

export type EmptyStateProps = Omit<RegistryEmptyStateProps, "title"> & {
  title?: RegistryEmptyStateProps["title"];
};

export function EmptyState({ title, ...props }: EmptyStateProps) {
  return <RegistryEmptyState role="status" title={title ?? ""} {...props} />;
}
