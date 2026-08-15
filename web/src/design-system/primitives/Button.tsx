import {
  Button as RegistryButton,
  buttonVariants,
  type ButtonProps as RegistryButtonProps,
} from "@/components/ui/button";
import { cn } from "@/design-system/utils";

// A wrapper rather than an edit to components/ui/button.tsx, which the next
// `shadcn add` overwrites. Button.test.tsx fails if a call site bypasses this.
const SOFT_DESTRUCTIVE = [
  "bg-destructive-soft text-destructive border-destructive/40",
  "hover:bg-destructive/20 hover:border-destructive/60",
  "active:bg-destructive/25",
  "data-[pressed=true]:bg-destructive/25",
].join(" ");

export type ButtonProps = RegistryButtonProps;

export function Button({ variant, className, ...props }: ButtonProps) {
  if (variant === "destructive") {
    return (
      <RegistryButton variant="secondary" className={cn(SOFT_DESTRUCTIVE, className)} {...props} />
    );
  }
  return <RegistryButton variant={variant} className={className} {...props} />;
}

export { buttonVariants };
