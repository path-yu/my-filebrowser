import path from "node:path";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import VueI18nPlugin from "@intlify/unplugin-vue-i18n/vite";
import legacy from "@vitejs/plugin-legacy";
import { compression } from "vite-plugin-compression2";

console.log(path.resolve(__dirname, "./src/i18n/**/*.json"));
const plugins = [
  vue(),
  VueI18nPlugin({
    include: ['./src/i18n/**/*.json'],
  }),
  legacy({
    // defaults already drop IE support
    targets: ["defaults"],
  }),
  compression({ include: /\.js$/, deleteOriginalAssets: false }),
];

const resolve = {
  alias: {
    // vue: "@vue/compat",
    "@/": `${path.resolve(__dirname, "src")}/`,
  },
};

// https://vitejs.dev/config/
export default defineConfig(({ command }) => {
  if (command === "serve") {
    return {
      plugins,
      resolve,
      server: {
        proxy: {
          "/api/command": {
            target: "ws://127.0.0.1:8080",
            ws: true,
          },
          "/api": "http://127.0.0.1:8080",
        },
      },
    };
  } else {
    // command === 'build'
    return {
      plugins,
      resolve,
      base: "",
      build: {
        rollupOptions: {
          input: {
            index: path.resolve(__dirname, "./public/index.html"),
          },
          output: {
            manualChunks: (id) => {
              // bundle dayjs files in a single chunk
              // this avoids having small files for each locale
              if (id.includes("dayjs/")) {
                return "dayjs";
                // bundle i18n in a separate chunk
              } else if (id.includes("i18n/")) {
                return "i18n";
              }

              // Heavy viewer/editor libraries are only imported dynamically
              // (preview pane / editor / settings pages). Grouping each
              // family into its own named chunk prevents duplicate copies
              // across async chunks and improves long-term caching.
              if (id.includes("pdfjs-dist")) {
                return "pdfjs";
              } else if (id.includes("ace-builds")) {
                return "ace";
              } else if (
                id.includes("video.js") ||
                id.includes("videojs-")
              ) {
                return "videojs";
              } else if (id.includes("docx-preview")) {
                return "docx";
              } else if (
                id.includes("marked") ||
                id.includes("katex") ||
                id.includes("dompurify")
              ) {
                return "markdown";
              } else if (
                id.includes("epubjs") ||
                id.includes("vue-reader") ||
                id.includes("jszip")
              ) {
                return "epub";
              }
            },
          },
        },
      },
      experimental: {
        renderBuiltUrl(filename, { hostType }) {
          if (hostType === "js") {
            return { runtime: `window.__prependStaticUrl("${filename}")` };
          } else if (hostType === "html") {
            return `[{[ .StaticURL ]}]/${filename}`;
          } else {
            return { relative: true };
          }
        },
      },
    };
  }
});
