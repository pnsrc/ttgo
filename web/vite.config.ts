import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// API proxy для dev-режима. В production frontend будет на том же origin.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:9090",
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
