import { forwardRef } from "react";
import { Search } from "lucide-react";

import { Input, type InputProps } from "./Input";
import { Kbd } from "./Kbd";

export interface SearchInputProps extends Omit<InputProps, "type"> {
  kbd?: string;
}

export const SearchInput = forwardRef<HTMLInputElement, SearchInputProps>(
  function SearchInput({ kbd, placeholder = "Search", ...props }, ref) {
    return (
      <Input
        ref={ref}
        type="search"
        placeholder={placeholder}
        leadingIcon={<Search size={13} />}
        trailingSlot={kbd ? <Kbd>{kbd}</Kbd> : undefined}
        {...props}
      />
    );
  },
);
