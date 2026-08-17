import path from "node:path";
import http from "node:http";
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
        host: '0.0.0.0', // 允许外部/穿透网关访问
        allowedHosts: true, // 允许所有 Host 域名访问
        proxy: {
          "/api/command": {
            target: "ws://127.0.0.1:8080",
            ws: true,
            // Make absolutely sure that raw `[` / `]` in the path never reach
            // Node's http.request (which would throw ERR_INVALID_URL for them)
            // or the Go backend gorilla/mux parser (403/404 for gen-delims in
            // the path segment). encodeURIComponent on the browser side is
            // the first line of defence; this proxy rewrite acts as a net.
            configure: (proxy) => {
              proxy.on("proxyReq", (proxyReq) => {
                if (proxyReq.path) {
                  proxyReq.path = proxyReq.path
                    .replace(/\[/g, "%5B")
                    .replace(/\]/g, "%5D");
                }
              });
            },
          },
          "/api": {
            target: "http://127.0.0.1:8080",
            changeOrigin: false,
            // 显式使用 http.Agent（不要 H2/KeepAlive 升级），避免
            // http-proxy 在转发 multipart/form-data POST 时偶发:
            //   HPE_INVALID_CONSTANT / ECONNRESET / Client closed socket
            // 最终在前端表现为 502 Bad Gateway（后端实际正常返回了）
            agent: new http.Agent({
              keepAlive: false,
              keepAliveMsecs: 0,
              maxSockets: 64,
              timeout: 10 * 60 * 1000, // 向量检索 10 分钟超时（留足 ONNX+转图）
            }),
            timeout: 10 * 60 * 1000,
            proxyTimeout: 10 * 60 * 1000,
            xfwd: false,
            selfHandleResponse: false,
            followRedirects: false,
            configure: (proxy, options) => {
              // 编码路径里的 [] 为 %5B/%5D，避免 Node http.request 抛
              // ERR_INVALID_URL 或 gorilla/mux 把 gen-delims 算成路径分隔
              proxy.on("proxyReq", (proxyReq) => {
                if (proxyReq.path) {
                  proxyReq.path = proxyReq.path
                    .replace(/\[/g, "%5B")
                    .replace(/\]/g, "%5D");
                }
                // 大文件上传时，http-proxy 不会主动把 Transfer-Encoding: chunked
                // 带原始 content-length 的 body 提前 flush；这里显式在有
                // Content-Length 且长度已知时 setHeader 一次，避免被当成 EOF。
                // （不做也没事，主要是保险。）
              });
              // 关键兜底：把代理层错误以明确 JSON 写回前端，
              // 而不是让 http-proxy 吞掉后默认返回 502 空响应。
              proxy.on("error", (err: any, req, res) => {
                try {
                  const msg: any = {
                    error: "代理层连接失败 (Vite -> 后端 127.0.0.1:8080)",
                    err: String(err?.message || err || ""),
                    url: (req as any)?.url || "",
                    target: String(options?.target || ""),
                  };
                  const body = JSON.stringify(msg);
                  (res as any).statusCode = (res as any).statusCode && (res as any).statusCode >= 400
                    ? (res as any).statusCode
                    : 502;
                  (res as any).setHeader?.("Content-Type", "application/json; charset=utf-8");
                  (res as any).setHeader?.("Content-Length", Buffer.byteLength(body));
                  (res as any).end?.(body);
                } catch {
                  /* ignore: 响应可能已经发了 */
                }
              });
              proxy.on("econnreset", (err: any, req, res) => {
                try {
                  const body = JSON.stringify({
                    error: "后端连接被重置（ECONNRESET）：请确认 filebrowser 服务仍在运行于 127.0.0.1:8080",
                    err: String(err?.message || err || ""),
                  });
                  (res as any).statusCode = 502;
                  (res as any).setHeader?.("Content-Type", "application/json; charset=utf-8");
                  (res as any).setHeader?.("Content-Length", Buffer.byteLength(body));
                  (res as any).end?.(body);
                } catch { /* ignore */ }
              });
            },
          },
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
