import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router";
import { QueryClientProvider } from "@tanstack/react-query";

import App from "@/App";
import { TooltipProvider } from "@/design-system/primitives/Tooltip";
import { queryClient } from "@/lib/queryClient";
import { applyAndStore, readStoredTheme } from "@/lib/theme";
import "@/styles/globals.css";

// Apply the stored theme before React mounts so a `light`-themed user never
// sees a single dark frame. With nothing stored, useTheme defaults to dark.
const stored = readStoredTheme();
if (stored) applyAndStore(stored);

const root = document.getElementById("root");
if (!root) throw new Error("missing #root element");

ReactDOM.createRoot(root).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <TooltipProvider>
          <App />
        </TooltipProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>,
);
