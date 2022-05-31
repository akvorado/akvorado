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
          "Remote-User": "alfred",
          "Remote-Name": "Alfred Pennyworth",
          "Remote-Email": "alfred@dccomics.example.com",
          "X-Logout-URL": "https://en.wikipedia.org/wiki/Alfred_Pennyworth",
        },
      },
    },
  },
});
