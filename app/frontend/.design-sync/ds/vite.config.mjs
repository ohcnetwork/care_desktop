import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

const pkg = fileURLToPath(new URL("../.cache/pkg", import.meta.url));

export default defineConfig({
  root: fileURLToPath(new URL("../..", import.meta.url)),
  resolve: { alias: { "@": fileURLToPath(new URL("../../src", import.meta.url)) } },
  plugins: [react(), tailwindcss()],
  build: {
    outDir: `${pkg}/dist`,
    emptyOutDir: true,
    copyPublicDir: false,
    cssCodeSplit: false,
    minify: false,
    lib: {
      entry: fileURLToPath(new URL("./entry.ts", import.meta.url)),
      formats: ["es"],
      fileName: () => "index.js",
      cssFileName: "ds",
    },
    rollupOptions: { external: (id) => /^[a-z@][^:]*$/i.test(id) && !id.startsWith("@/") },
  },
});
