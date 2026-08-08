// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import tailwindcss from "@tailwindcss/vite";
import { lezer } from "@lezer/generator/rollup";
import { fileURLToPath, URL } from "node:url";

// Backend used by the development server for API requests. Point it to a
// public instance, like https://demo.akvorado.net, to work on the frontend
// without running a full Akvorado setup.
const apiTarget = process.env.AKVORADO_API_URL || "http://127.0.0.1:8080";

// https://vitejs.dev/config/
export default defineConfig({
  base: "",
  plugins: [vue(), tailwindcss(), lezer()],
  optimizeDeps: {
    // Fix dev server lazy init.
    exclude: ["@headlessui/vue"],
  },
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  build: {
    outDir: "../data/frontend",
    emptyOutDir: true,
    chunkSizeWarningLimit: 2000,
  },
  test: {
    reporters: ["default", "junit"],
    outputFile: "../../test/js/tests.xml",
    coverage: {
      reporter: ["text-summary", "html", "cobertura"],
      reportsDirectory: "../../test/js",
      all: false,
    },
  },
  server: {
    proxy: {
      "/api": {
        target: apiTarget,
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
