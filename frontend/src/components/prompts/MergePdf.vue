<template>
  <div class="card floating merge-pdf">
    <div class="card-title">
      <h2>{{ $t("prompts.mergePdf") }}</h2>
    </div>

    <div class="card-content">
      <p class="merge-pdf__desc">
        {{ $t("prompts.mergePdfMessage", { count: order.length }) }}
      </p>

      <ul v-if="order.length > 0" class="merge-pdf__list">
        <li
          v-for="(item, idx) in order"
          :key="item.path || item.url"
          class="merge-pdf__row"
        >
          <span class="merge-pdf__index">{{ idx + 1 }}</span>
          <span class="merge-pdf__name" :title="item.name">{{ item.name }}</span>
          <div class="merge-pdf__actions">
            <button
              class="merge-pdf__icon"
              :disabled="idx === 0"
              :title="$t('prompts.mergePdfMoveUp')"
              :aria-label="$t('prompts.mergePdfMoveUp')"
              @click="move(idx, -1)"
            >
              <i class="material-icons">arrow_upward</i>
            </button>
            <button
              class="merge-pdf__icon"
              :disabled="idx === order.length - 1"
              :title="$t('prompts.mergePdfMoveDown')"
              :aria-label="$t('prompts.mergePdfMoveDown')"
              @click="move(idx, 1)"
            >
              <i class="material-icons">arrow_downward</i>
            </button>
          </div>
        </li>
      </ul>

      <p v-if="progress" class="merge-pdf__progress">
        {{ progress }}
      </p>
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="closeHovers"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button
        id="focus-prompt"
        @click="mergeAndDownload"
        class="button button--flat"
        type="submit"
        :aria-label="$t('buttons.mergePdf')"
        :title="$t('buttons.mergePdf')"
        :disabled="order.length < 2 || loading"
      >
        {{ loading ? $t("prompts.mergePdfMerging") : $t("buttons.mergePdf") }}
      </button>
    </div>
  </div>
</template>

<script>
import { mapActions, mapState } from "pinia";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { fetchURL, createURL } from "@/api/utils";
import { encodePath } from "@/utils/url";
import { baseURL } from "@/utils/constants";
import { getDownloadURL } from "@/api/files";

function ensureLeadingSlash(p) {
  if (!p) return "/";
  return p.startsWith("/") ? p : "/" + p;
}

/** 把 ArrayBuffer 前 N 字节转成十六进制串，便于肉眼识别魔数 */
function hexDump(buf, len = 8) {
  const head = new Uint8Array(buf, 0, Math.min(len, buf.byteLength));
  return Array.prototype.map.call(head, (b) => b.toString(16).padStart(2, "0")).join(" ");
}
/** 把字节数格式化成人类可读 */
function fmtBytes(n) {
  if (!Number.isFinite(n)) return String(n);
  if (n < 1024) return n + " B";
  if (n < 1024 * 1024) return (n / 1024).toFixed(2) + " KB";
  return (n / 1024 / 1024).toFixed(2) + " MB";
}
/** 校验 ArrayBuffer 的前 5 字节是不是 PDF 魔数 "%PDF-"（0x25 0x50 0x44 0x46 0x2D） */
function isPdfMagic(buf) {
  if (!buf || buf.byteLength < 5) return false;
  const head = new Uint8Array(buf, 0, 5);
  return (
    head[0] === 0x25 &&
    head[1] === 0x50 &&
    head[2] === 0x44 &&
    head[3] === 0x46 &&
    head[4] === 0x2d
  );
}

export default {
  name: "mergePdf",
  data() {
    return {
      /** 按合并顺序排列的 ResourceItem 列表，初始化时取当前选中项 */
      order: [],
      loading: false,
      progress: "",
    };
  },
  inject: ["$showError", "$showSuccess"],
  computed: {
    ...mapState(useFileStore, ["selectedItems", "visibleItemAt", "selected"]),
  },
  created() {
    // 打开对话框时取 store 中已选中的 PDF 项作为初始顺序
    const items = (this.selectedItems || []).filter(
      (it) => it && !it.isDir && (it.type === "pdf" || (it.extension || "").toLowerCase() === "pdf")
    );
    this.order = items.slice();
  },
  methods: {
    ...mapActions(useLayoutStore, ["closeHovers"]),
    move(idx, delta) {
      const target = idx + delta;
      if (target < 0 || target >= this.order.length) return;
      const arr = this.order.slice();
      const [it] = arr.splice(idx, 1);
      arr.splice(target, 0, it);
      this.order = arr;
    },
    downloadBlob(blob, filename) {
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = filename;
      document.body.appendChild(a);
      a.click();
      a.remove();
      setTimeout(() => URL.revokeObjectURL(url), 2000);
    },
    async mergeAndDownload() {
      if (this.order.length < 2 || this.loading) return;
      const LOG = "[MergePdf]";
      const t0 = performance.now();
      console.groupCollapsed(
        `${LOG} start merge  total=${this.order.length}  order=%o`,
        this.order.map((o, i) => `[${i}] ${o.name}`)
      );
      this.order.forEach((it, i) => {
        console.log(`${LOG}   item[${i}] name=${it.name}  size=${it.size ?? "?"}  path=${it.path}  type=${it.type}`);
      });
      console.groupEnd();

      this.loading = true;
      this.progress = "";
      try {
        // 动态导入，减小首屏包体
        const tImport = performance.now();
        const { PDFDocument } = await import("pdf-lib");
        console.log(`${LOG} pdf-lib imported  took=${(performance.now() - tImport).toFixed(1)}ms`);
        const mergedDoc = await PDFDocument.create();

        for (let i = 0; i < this.order.length; i++) {
          const it = this.order[i];
          this.progress = this.$t("prompts.mergePdfProgress", {
            current: i + 1,
            total: this.order.length,
            name: it.name,
          });

          // 与项目 getDownloadURL(file, true) 完全一致的 URL 构造方式：
          //   1. endpoint = "api/raw" + ensureLeadingSlash(it.path)
          //   2. 调用 createURL(endpoint, { inline: "true" })
          //      createURL 内部做了 encodePath + 方括号二次编码 (%5B/%5D)，
          //      这是 Vite proxy / Go net/http 处理 [ ] 等特殊字符时不 panic 的关键。
          //   3. createURL 返回的是 "${origin}${baseURL}${pathPart}?qs" 的完整 URL，
          //      而 fetchURL(relative) 需要 "${baseURL}${relative}"，所以把前缀剥掉得到 relative。
          const endpoint = "api/raw" + ensureLeadingSlash(it.path || "");
          const fullURL = createURL(endpoint, { inline: "true" });
          const originBase = typeof origin !== "undefined" ? origin : "";
          const stripPrefix = originBase + (baseURL.endsWith("/") ? baseURL.slice(0, -1) : baseURL);
          const relative = fullURL.startsWith(stripPrefix)
            ? fullURL.slice(stripPrefix.length)
            : fullURL;
          console.groupCollapsed(
            `${LOG} [${i + 1}/${this.order.length}] fetch  name=${it.name}`
          );
          console.log(`${LOG}   item.path       = ${it.path || ""}`);
          console.log(`${LOG}   endpoint        = ${endpoint}`);
          console.log(`${LOG}   fullURL(create) = ${fullURL}`);
          console.log(`${LOG}   getDownloadURL  = ${getDownloadURL(it, true)}    (should be IDENTICAL to fullURL(create))`);
          console.log(`${LOG}   stripPrefix     = ${stripPrefix}   (will be removed to get relative for fetchURL)`);
          console.log(`${LOG}   relative (final arg to fetchURL) = ${relative}`);
          const tFetch = performance.now();
          const resp = await fetchURL(relative, {
            headers: { Accept: "application/pdf" },
          });
          const dtFetch = performance.now() - tFetch;
          const cl = resp.headers.get("Content-Length");
          const ct = resp.headers.get("Content-Type") || "";
          console.log(
            `${LOG}   resp status=${resp.status}  ok=${resp.ok}  took=${dtFetch.toFixed(1)}ms  Content-Type=${ct || "(none)"}  Content-Length=${cl ?? "(missing)"}`
          );
          if (!resp.ok) {
            // 非 2xx 先抓一些响应 body 方便排查（例如 401 HTML 登录页 / 500 / 403）
            const preview = await (async () => {
              try {
                const t = await resp.clone().text();
                return t.length > 300 ? t.slice(0, 300) + "…[truncated]" : t;
              } catch (e) {
                return "(unable to read body as text: " + e + ")";
              }
            })();
            console.error(`${LOG}   !resp.ok  body preview:\n${preview}`);
            console.groupEnd();
            throw new Error(
              this.$t("prompts.mergePdfInvalid", {
                name: it.name,
                type: `HTTP ${resp.status}`,
              })
            );
          }

          if (ct && !ct.toLowerCase().includes("pdf") && !ct.includes("octet-stream")) {
            console.warn(
              `${LOG}   suspicious Content-Type="${ct}" (neither pdf nor octet-stream); body preview follows`
            );
            try {
              const preview = await resp.clone().text();
              console.warn(
                `${LOG}   body:\n` +
                  (preview.length > 500 ? preview.slice(0, 500) + "…[truncated]" : preview)
              );
            } catch (e) {
              console.warn(`${LOG}   (unable to preview text body: ${e})`);
            }
            console.groupEnd();
            throw new Error(
              this.$t("prompts.mergePdfInvalid", { name: it.name, type: ct || "(none)" })
            );
          }

          const tBuf = performance.now();
          const buf = await resp.arrayBuffer();
          console.log(
            `${LOG}   arrayBuffer size=${fmtBytes(buf.byteLength)}  read took=${(performance.now() - tBuf).toFixed(1)}ms  head bytes(12)=${hexDump(buf, 12)}`
          );
          if (!isPdfMagic(buf)) {
            // 前 5 字节不是 %PDF- —— 几乎肯定是拿到了登录页/错误响应，再打一些前 200 字符方便定位
            const previewText = (() => {
              try {
                const dec = new TextDecoder("utf-8", { fatal: false });
                const head = new Uint8Array(buf, 0, Math.min(512, buf.byteLength));
                const s = dec.decode(head);
                return s.length > 300 ? s.slice(0, 300) + "…[truncated]" : s;
              } catch (e) {
                return "";
              }
            })();
            console.error(
              `${LOG}   !isPdfMagic  first bytes NOT "%PDF-".  Head bytes: ${hexDump(buf, 16)}` +
                (previewText ? `\nText preview (UTF-8, up to 300 chars):\n${previewText}` : "")
            );
            console.groupEnd();
            throw new Error(
              this.$t("prompts.mergePdfInvalid", { name: it.name, type: "NOT_PDF_HEADER" })
            );
          }

          const tLoad = performance.now();
          let srcDoc;
          try {
            srcDoc = await PDFDocument.load(buf);
          } catch (e) {
            console.error(`${LOG}   PDFDocument.load failed  err=${e?.message || e}\n`, e);
            console.groupEnd();
            throw e;
          }
          const pageIndices = srcDoc.getPageIndices();
          console.log(
            `${LOG}   PDFDocument.load OK  pages=${pageIndices.length}  took=${(performance.now() - tLoad).toFixed(1)}ms`
          );

          const tCopy = performance.now();
          const copied = await mergedDoc.copyPages(srcDoc, pageIndices);
          copied.forEach((page) => mergedDoc.addPage(page));
          console.log(
            `${LOG}   merged copied pages into doc  added=${copied.length}  cumulative total=${mergedDoc.getPageCount()}  took=${(performance.now() - tCopy).toFixed(1)}ms`
          );
          console.groupEnd();
        }

        this.progress = this.$t("prompts.mergePdfSaving");
        console.groupCollapsed(`${LOG} saving merged document...`);
        const tSave = performance.now();
        const mergedBytes = await mergedDoc.save();
        console.log(
          `${LOG} mergedDoc.save OK  total pages=${mergedDoc.getPageCount()}  size=${fmtBytes(mergedBytes.byteLength)}  took=${(performance.now() - tSave).toFixed(1)}ms`
        );
        console.groupEnd();
        const blob = new Blob([mergedBytes], { type: "application/pdf" });

        const ts = new Date();
        const pad = (n) => String(n).padStart(2, "0");
        const filename = `merged_${ts.getFullYear()}${pad(ts.getMonth() + 1)}${pad(
          ts.getDate()
        )}_${pad(ts.getHours())}${pad(ts.getMinutes())}${pad(ts.getSeconds())}.pdf`;
        console.log(
          `${LOG} trigger download  filename=${filename}  total took=${(performance.now() - t0).toFixed(1)}ms`
        );
        this.downloadBlob(blob, filename);

        this.$showSuccess?.(this.$t("prompts.mergePdfSuccess"));
      } catch (e) {
        console.error(`${LOG} FAILED  err=${e?.message || e}\n`, e);
        this.$showError(e);
      } finally {
        this.loading = false;
      }
    },
  },
};
</script>

<style scoped>
.merge-pdf {
  min-width: 380px;
  max-width: 520px;
}
.merge-pdf__desc {
  margin: 0 0 12px 0;
  color: var(--theme-text-sub, #666);
  font-size: 13px;
}
.merge-pdf__list {
  list-style: none;
  padding: 0;
  margin: 0;
  max-height: 320px;
  overflow-y: auto;
  border: 1px solid var(--theme-border, #e5e7eb);
  border-radius: 8px;
  background: var(--theme-bg-elevated, #fafafa);
}
.merge-pdf__row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--theme-border, #e5e7eb);
}
.merge-pdf__row:last-child {
  border-bottom: none;
}
.merge-pdf__index {
  flex: 0 0 28px;
  height: 28px;
  border-radius: 50%;
  background: var(--theme-primary, #2196f3);
  color: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
}
.merge-pdf__name {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
}
.merge-pdf__actions {
  flex: 0 0 auto;
  display: flex;
  gap: 4px;
}
.merge-pdf__icon {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  border: 1px solid var(--theme-border, #e5e7eb);
  background: var(--theme-btn-bg, #fff);
  color: var(--theme-text, #333);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background 0.15s;
}
.merge-pdf__icon:hover:not(:disabled) {
  background: var(--theme-primary-soft, #e3f2fd);
  color: var(--theme-primary, #2196f3);
}
.merge-pdf__icon:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.merge-pdf__progress {
  margin: 12px 0 0 0;
  font-size: 12px;
  color: var(--theme-text-sub, #666);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
