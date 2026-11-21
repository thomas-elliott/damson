import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";

// The same two variables `just dev` reads, so moving the API moves the proxy
// with it.
const apiPort = process.env.PORT ?? "8080";
const webPort = Number(process.env.WEB_PORT ?? "5173");

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
    port: webPort,
    // Fail on a taken port rather than drifting to the next one, so the URL
    // `just dev` prints is the one being served, and the one sign-in returns to.
    strictPort: true,
    allowedHosts: [".ts.net"],
    proxy: {
      "/api": {
        target: `http://127.0.0.1:${apiPort}`,
        changeOrigin: true,
      },
    },
  },
});
