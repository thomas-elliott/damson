import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  build: {
    outDir: "static",
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    allowedHosts: [".ts.net"],
    proxy: {
      "/api": {
        target: process.env.DAMSON_API_PROXY ?? "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
