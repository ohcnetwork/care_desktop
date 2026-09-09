import { defineConfig } from "vite";
import { writeFileSync } from "node:fs";
import { fileURLToPath, URL } from "node:url";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// base "./" so the built assets work when embedded + served by Wails.
// keep-dist re-creates the tracked placeholder vite's emptyOutDir wipes, so a
// clean checkout still compiles the //go:embed all:frontend/dist.
export default defineConfig({
  base: "./",
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  build: { outDir: "dist", emptyOutDir: true },
  plugins: [
    react(),
    tailwindcss(),
    {
      name: "keep-dist",
      closeBundle() {
        writeFileSync("dist/.gitkeep", "");
      },
    },
  ],
});
