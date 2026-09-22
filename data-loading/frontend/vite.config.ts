import { fileURLToPath, URL } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  base: "/seed-data/",
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  server: {
    fs: { allow: [fileURLToPath(new URL("..", import.meta.url))] },
    proxy: {
      "/api": {
        target: process.env.CARE_API_URL ?? "http://localhost:9000",
        changeOrigin: true,
        secure: false,
      },
    },
  },
  build: {
    outDir: fileURLToPath(new URL("../../deployments/seed-data", import.meta.url)),
    emptyOutDir: true,
    chunkSizeWarningLimit: 1200,
  },
  plugins: [react(), tailwindcss()],
});
