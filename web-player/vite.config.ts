import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5174,
    proxy: {
      "/api": "http://127.0.0.1:8766",
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
