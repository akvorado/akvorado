import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import { lezer } from "@lezer/generator/rollup";
import path from "path";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue(), lezer()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "@data": path.resolve(__dirname, "./data"),
    },
  },
  build: {
    outDir: "../data/frontend",
    emptyOutDir: true,
    chunkSizeWarningLimit: 2000,
  },
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
        headers: {
          "X-Akvorado-User-Login": "alfred",
          "X-Akvorado-User-Name": "Alfred Pennyworth",
          "X-Akvorado-User-Email": "alfred@dccomics.example.com",
          "X-Akvorado-User-Logout":
            "https://en.wikipedia.org/wiki/Alfred_Pennyworth",
        },
      },
    },
  },
});
