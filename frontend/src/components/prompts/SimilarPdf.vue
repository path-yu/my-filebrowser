<template>
  <div class="card floating" id="similarPdfUploadPrompt">
    <div class="card-title">
      <h2>上传 PDF / 图片 搜索相似图纸</h2>
      <button
        v-if="uploading"
        class="action"
        @click="abortSearch"
        :aria-label="'取消'"
        :title="'取消'"
      >
        <i class="material-icons spin">autorenew</i>
      </button>
    </div>

    <!-- 上传区（没开始搜的时候显示；结果出来后也在顶部显示当前已上传的文件） -->
    <div class="card-content">
      <!-- 拖拽/点击选择区域 -->
      <div
        class="dropzone"
        :class="{ dragging, disabled: uploading, filled: !!selectedFile }"
        tabindex="0"
        role="button"
        @click="!uploading && fileInputRef?.click()"
        @keydown.enter.prevent="!uploading && fileInputRef?.click()"
        @dragover.prevent.stop="onDragOver"
        @dragleave.prevent.stop="onDragLeave"
        @drop.prevent.stop="onDrop"
      >
        <input
          ref="fileInputRef"
          type="file"
          accept=".pdf,application/pdf,.jpg,.jpeg,.png,.webp,.bmp,.tif,.tiff,.gif,image/*"
          style="display: none"
          @change="onFileSelect"
          :disabled="uploading"
        />
        <i class="material-icons dropzone-icon">
          {{
            uploading
              ? "cloud_upload"
              : selectedFile
              ? isImageFile(selectedFile.name)
                ? "image"
                : "picture_as_pdf"
              : "cloud_upload"
          }}
        </i>
        <div class="dropzone-text">
          <template v-if="selectedFile">
            <p class="dropzone-filename">
              <i class="material-icons">
                {{ isImageFile(selectedFile.name) ? "image" : "insert_drive_file" }}
              </i>
              {{ selectedFile.name }}
              <span class="dropzone-size">({{ formatBytes(selectedFile.size) }})</span>
            </p>
            <p v-if="!uploading" class="dropzone-hint">点击重新选择，或拖拽替换</p>
          </template>
          <template v-else>
            <p>点击选择 PDF 或图片，或 <strong>拖拽 PDF / 图片到这里</strong></p>
            <p class="dropzone-hint">
              支持 .pdf / .jpg / .jpeg / .png / .webp / .bmp / .tif / .tiff / .gif；最大 200MB
            </p>
          </template>
        </div>
      </div>

      <!-- 诊断 / 错误信息 -->
      <div v-if="errorText" class="error-block">
        <i class="material-icons">error</i>
        <span>{{ errorText }}</span>
      </div>
      <div v-else-if="diagnosis || uploading" class="info-block">
        <i class="material-icons" :class="uploading ? 'spin' : ''">
          {{ uploading ? "autorenew" : "info" }}
        </i>
        <span>
          <template v-if="uploading">
            <template v-if="isImageFile(selectedFile?.name || '')">
              正在上传图片 → 解码（透明通道贴白底）→ 提取 ResNet18 特征 → 全库相似度计算 ...
              （约 2~10 秒，具体取决于向量库规模）
            </template>
            <template v-else>
              正在上传 PDF → 转换第1页 → 提取 ResNet18 特征 → 全库相似度计算 ...
              （约 3~15 秒，具体取决于 PDF 大小和向量库规模）
            </template>
          </template>
          <template v-else-if="diagnosis">
            共 <strong>{{ totalInDB }}</strong> 条向量库记录 · 耗时 <strong>{{ elapsed }}</strong>
            <br />
            诊断：{{ diagnosis }}
          </template>
        </span>
      </div>

      <!-- Top-K 结果表 -->
      <div v-if="results.length > 0" class="results-wrap">
        <h3 class="results-title">
          Top-{{ results.length }} 相似图纸（按相似度降序）
          <span class="results-subtitle">点击任意行将结果渲染到主文件列表（自动关闭弹窗）</span>
        </h3>
        <ul class="results-list">
          <li
            v-for="(r, idx) in results"
            :key="r.path"
            class="result-row"
            tabindex="0"
            role="button"
            @click="goResult(r)"
            @keydown.enter.prevent="goResult(r)"
          >
            <span class="result-rank">{{ idx + 1 }}</span>
            <span class="result-sim" :class="simClass(r.similarity)">
              {{ (r.similarity * 100).toFixed(2) }}%
            </span>
            <div class="result-main">
              <span class="result-name" :title="r.name">
                <i class="material-icons">picture_as_pdf</i>
                <span class="result-name-text">{{ r.name }}</span>
              </span>
              <span class="result-path" :title="r.path">{{ r.path }}</span>
            </div>
            <span class="result-meta">
              {{ formatBytes(r.size) }} · {{ formatDate(r.modified) }}
            </span>
          </li>
        </ul>
      </div>
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="closeHovers"
        :aria-label="'关闭'"
        :title="'关闭'"
        :disabled="uploading"
        tabindex="2"
      >
        关闭
      </button>
      <button
        v-if="results.length > 0"
        class="button button--flat button--grey"
        @click="applyAllAsResults"
        :disabled="uploading"
        tabindex="3"
        title="将全部 Top-K 结果一次性渲染到主文件列表"
      >
        <i class="material-icons" style="font-size: 16px; vertical-align: -2px">
          list_alt
        </i>
        显示全部结果
      </button>
      <button
        id="focus-prompt"
        class="button button--flat button--blue"
        :disabled="!canSubmit"
        @click="submitSearch"
        tabindex="1"
      >
        <i class="material-icons" style="font-size: 16px; vertical-align: -2px">
          {{ uploading ? "autorenew" : "search" }}
        </i>
        {{ uploading ? "正在搜索…" : "开始检索相似图纸" }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onBeforeUnmount, ref, watch, nextTick } from "vue";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { similarPdf } from "@/api";
import type { SimilarPdfResult } from "@/api/similarPdf";
import { StatusError } from "@/api/utils";

const $showError = inject<IToastError>("$showError")!;
const layoutStore = useLayoutStore();
const fileStore = useFileStore();

const fileInputRef = ref<HTMLInputElement | null>(null);
const selectedFile = ref<File | null>(null);
const uploading = ref(false);
const errorText = ref<string>("");
const results = ref<SimilarPdfResult[]>([]);
const diagnosis = ref<string>("");
const totalInDB = ref<number>(0);
const elapsed = ref<string>("");
let searchAbort: AbortController | null = null;

const dragging = ref(false);
const canSubmit = computed(() => !!selectedFile.value && !uploading.value);

// 支持的图片扩展名（和后端 drawingsearch.go / api/similarPdf.ts 保持一致）
const SUPPORTED_IMAGE_EXTS = new Set([
  ".jpg", ".jpeg", ".png", ".webp", ".bmp", ".tif", ".tiff", ".gif",
]);
/** 仅根据文件名扩展名判断是否为图片（不依赖 MIME type，避免系统注册表缺失导致识别错） */
function isImageFile(name: string): boolean {
  const n = (name || "").toLowerCase();
  const i = n.lastIndexOf(".");
  if (i < 0) return false;
  return SUPPORTED_IMAGE_EXTS.has(n.slice(i));
}
function isPdfFile(name: string): boolean {
  return (name || "").toLowerCase().endsWith(".pdf");
}
function isSupportedFile(name: string): boolean {
  return isPdfFile(name) || isImageFile(name);
}

watch(
  () => layoutStore.currentPromptName,
  (n) => {
    if (n === "similarPdf") {
      nextTick(() => {
        const btn = document.getElementById("focus-prompt");
        btn?.focus();
      });
    }
  }
);

onBeforeUnmount(() => {
  searchAbort?.abort();
});

function closeHovers() {
  layoutStore.closeHovers();
}

function formatBytes(b: number | undefined | null): string {
  if (!b) return "-";
  const kb = b / 1024;
  if (kb < 1024) return `${kb.toFixed(1)} KB`;
  const mb = kb / 1024;
  if (mb < 1024) return `${mb.toFixed(2)} MB`;
  return `${(mb / 1024).toFixed(2)} GB`;
}
function formatDate(iso: string | undefined | null): string {
  if (!iso) return "-";
  try {
    const d = new Date(iso);
    if (isNaN(d.getTime())) return "-";
    return d.toLocaleString();
  } catch {
    return "-";
  }
}
function simClass(sim: number): string {
  if (sim >= 0.95) return "sim-excellent";
  if (sim >= 0.85) return "sim-good";
  if (sim >= 0.7) return "sim-medium";
  return "sim-low";
}

function validateAndSetFile(f: File | null | undefined): boolean {
  if (!f) return false;
  if (!isSupportedFile(f.name)) {
    const ext = f.name.includes(".") ? f.name.slice(f.name.lastIndexOf(".")) : "(无扩展名)";
    errorText.value = `文件类型不支持（${ext}）：仅支持 PDF 或图片（JPG / PNG / WebP / BMP / TIFF / GIF），当前文件为 "${f.name}"`;
    return false;
  }
  if (f.size > 200 * 1024 * 1024) {
    errorText.value = isPdfFile(f.name)
      ? "上传 PDF 超过 200MB 限制"
      : "上传图片超过 200MB 限制";
    return false;
  }
  if (f.size === 0) {
    errorText.value = "上传的文件为空";
    return false;
  }
  selectedFile.value = f;
  errorText.value = "";
  return true;
}
function onFileSelect(e: Event) {
  const t = e.target as HTMLInputElement;
  const f = t.files?.[0];
  validateAndSetFile(f);
  if (t) t.value = ""; // 允许再次选同一个文件
}
function onDragOver() {
  if (!uploading.value) dragging.value = true;
}
function onDragLeave() {
  dragging.value = false;
}
function onDrop(e: DragEvent) {
  dragging.value = false;
  if (uploading.value) return;
  const files = e.dataTransfer?.files;
  if (!files || files.length === 0) return;
  if (!validateAndSetFile(files[0])) return;
  if (files.length > 1) {
    $showError?.(`仅处理第 1 个文件：${files[0].name}`);
  }
}

function abortSearch() {
  searchAbort?.abort();
  uploading.value = false;
}

async function submitSearch() {
  if (!selectedFile.value || uploading.value) return;
  uploading.value = true;
  errorText.value = "";
  results.value = [];
  diagnosis.value = "";
  totalInDB.value = 0;
  elapsed.value = "";
  searchAbort = new AbortController();
  try {
    const res = await similarPdf.searchSimilarPdf(
      selectedFile.value,
      10,
      searchAbort.signal
    );
    results.value = res.results ?? [];
    diagnosis.value = res.diagnosis ?? "";
    totalInDB.value = res.totalInDB ?? 0;
    elapsed.value = res.elapsed ?? "";
  } catch (err: any) {
    if (err instanceof StatusError && err.is_canceled) {
      errorText.value = "已取消搜索";
    } else if (err instanceof StatusError) {
      const st = err.status ?? 0;
      // 501 Not Implemented：filebrowser 默认编译未启用相似PDF检索
      if (st === 501) {
        const extra = err.hint ? `\n\n修复方法：${err.hint}` : "";
        errorText.value = `（HTTP 501）${err.message}${extra}`;
      }
      // 502 Bad Gateway：Vite 代理（5173）无法转发到后端 8080
      else if (st === 502) {
        const errInner = (err.body?.["err"] as string) || "";
        const target = (err.body?.["target"] as string) || "http://127.0.0.1:8080";
        const errMsg = err.message || "代理层连接失败";
        let tail = `\n\n排查步骤：\n1) 确认 filebrowser 后端是否还在运行（原 go run 进程的终端是否仍在、无报错）；\n2) 确认后端监听端口和 Vite 代理目标一致：当前 Vite target = ${target}`;
        if (errInner) tail += `\n3) 代理原始错误：${errInner}`;
        errorText.value = `（HTTP 502）${errMsg}${tail}`;
      }
      // 0：fetch 阶段根本没连上（后端已退出 / 5173 Vite 本身没启动）
      else if (st === 0) {
        errorText.value =
          "（HTTP 0 / No connection）浏览器根本没有收到响应。\n请确认：\n1) filebrowser 后端仍在运行（8080 端口 LISTEN）\n2) 前端 dev server 仍在运行（5173 端口）\n3) 如果是生产环境，请重新刷新页面重试。";
      } else {
        // 其他状态码：复用 StatusError 解析出的中文错误
        errorText.value = `（HTTP ${st}）${err.message}`;
        if (err.hint) errorText.value += `\n\n修复说明：${err.hint}`;
        $showError?.(err);
      }
    } else {
      errorText.value = err?.message || String(err) || "未知错误";
      $showError?.(err);
    }
  } finally {
    uploading.value = false;
    searchAbort = null;
  }
}

async function goResult(r: SimilarPdfResult) {
  // 用户需求：点击单个结果时，把所有 Top-K 结果渲染到主文件列表中
  // （而不是单独跳转到某个文件），然后自动关闭弹窗
  applyAllAsResults();
}

/** 把当前 results 作为搜索结果注入主文件列表（复用 fileStore 的 searchMode 机制） */
function applyAllAsResults() {
  if (results.value.length === 0) return;
  const userRoot = fileStore.req?.path ?? "/";
  const items: ResourceItem[] = results.value.map((r) => {
    // r.path 是真实磁盘绝对路径（D:\BaiduNetdiskDownload\...），
    // 需要映射成相对 fileStore 用户根目录的虚拟路径（如 /图纸/ZKG2.pdf），
    // 供后续 fileStore.selectedItems / listItem / preview 渲染使用。
    const virtualPath = mapDiskPathToVirtual(r.path, userRoot, fileStore.req);
    const nameOnly = r.name || fileNameFromPath(virtualPath || r.path);
    const rawUrl = virtualPath || (r.path.startsWith("/") ? r.path : "/" + r.path.replace(/\\/g, "/"));
    // 构造成 normalize 能识别的形状（size / modified / dir 字段名直接兼容）
    return {
      path: rawUrl,
      name: nameOnly,
      size: r.size,
      modified: r.modified,
      isDir: !!r.dir,
      dir: !!r.dir,
      url: rawUrl,
      type: (r.dir ? "dir" : "blob") as any,
      subtitles: [
        `相似度 ${(r.similarity * 100).toFixed(2)}%`,
        r.path,
      ],
    } as any;
  });
  const queryLabel = `相似图纸${isImageFile(selectedFile.value?.name || "") ? "（图片）" : "（PDF）"}：${selectedFile.value?.name ?? "已上传"} （Top-${items.length}）`;
  fileStore.setSearchResults(queryLabel, items);
  closeHovers();
}

/** 磁盘绝对路径 → 虚拟路径（相对于 fileStore.req.path） */
function mapDiskPathToVirtual(
  diskPath: string,
  userRootVirtual: string,
  req: Resource | null
): string {
  try {
    if (!diskPath) return "";
    // 归一化正反斜杠
    const dp = diskPath.replace(/\\/g, "/");
    // 后端 req 对象中有时会带 user.rootFs（挂载的真实根）
    const realRoot: string | undefined =
      (req as any)?.["user"]?.["rootFs"] ??
      (req as any)?.["rootFs"] ??
      (globalThis as any).__FB_REAL_ROOT__;
    // 若能拿到真实根，直接截掉根前缀，再拼到 userRootVirtual 上
    if (realRoot) {
      const rr = realRoot.replace(/\\/g, "/").replace(/\/+$/, "");
      if (dp.toLowerCase().startsWith(rr.toLowerCase())) {
        const suffix = dp.slice(rr.length).replace(/^\/+/, "");
        const base = (userRootVirtual || "/").replace(/\/+$/, "");
        return suffix === "" ? (base === "" ? "/" : base) : base + "/" + suffix;
      }
    }
    // fallback：纯按路径名，拆出最后一段的文件名，前面加上用户当前目录，
    // 这样至少能显示正确的名字与点击预览（真正的 API 请求会按 url 走，经过后端鉴权后仍可命中磁盘文件）
    const name = fileNameFromPath(dp);
    const base = (userRootVirtual || "/").replace(/\/+$/, "");
    return base === "" ? "/" + name : base + "/" + name;
  } catch {
    return "";
  }
}
function fileNameFromPath(p: string): string {
  if (!p) return "";
  const s = p.replace(/\\/g, "/").replace(/\/+$/, "");
  const i = s.lastIndexOf("/");
  return i >= 0 ? s.slice(i + 1) : s;
}
function normalizeDiskPath(p: string): string {
  return String(p || "").replace(/\\/g, "/").toLowerCase();
}
</script>

<style scoped>
/* ------------------------------------------------------------------ */
/*  弹窗尺寸：覆盖全局 .card.floating 的 max-width:25em（仅 400px），   */
/*  相似 PDF 结果表格至少需要 640px+ 才能正常显示列，否则 grid 1fr 列   */
/*  会被压缩到接近 0，配合 word-break:break-all 导致每个字母单独一行，  */
/*  呈现用户看到的"文件名竖排"假象。                                    */
/* ------------------------------------------------------------------ */
:deep(#similarPdfUploadPrompt.card.floating),
#similarPdfUploadPrompt.card.floating {
  min-width: 420px;
  width: min(92vw, 880px);
  max-width: 880px;
}
@media (max-width: 480px) {
  :deep(#similarPdfUploadPrompt.card.floating),
  #similarPdfUploadPrompt.card.floating {
    min-width: unset;
    width: calc(100vw - 24px);
    max-width: calc(100vw - 24px);
  }
}

#similarPdfUploadPrompt .dropzone {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 16px;
  padding: 18px 20px;
  border: 2px dashed rgba(25, 118, 210, 0.45);
  border-radius: 12px;
  background: rgba(25, 118, 210, 0.05);
  cursor: pointer;
  transition: all 0.15s ease;
  color: var(--theme-text, #333);
}
#similarPdfUploadPrompt .dropzone:hover {
  border-color: var(--theme-color, #1976d2);
  background: rgba(25, 118, 210, 0.09);
}
#similarPdfUploadPrompt .dropzone.dragging {
  border-color: var(--theme-color, #1976d2);
  border-style: solid;
  background: rgba(25, 118, 210, 0.14);
  transform: scale(1.005);
}
#similarPdfUploadPrompt .dropzone.disabled {
  opacity: 0.6;
  cursor: not-allowed;
}
#similarPdfUploadPrompt .dropzone.filled {
  border-color: rgba(46, 125, 50, 0.6);
  background: rgba(46, 125, 50, 0.06);
}
.dropzone-icon {
  flex: none;
  font-size: 42px;
  color: var(--theme-color, #1976d2);
  opacity: 0.85;
}
.dropzone.filled .dropzone-icon {
  color: #2e7d32;
}
.dropzone-text {
  flex: 1 1 auto;
  min-width: 0;
}
.dropzone-text p {
  margin: 2px 0;
  line-height: 1.5;
  font-size: 14px;
}
.dropzone-filename {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  word-break: break-all;
}
.dropzone-filename .material-icons {
  font-size: 18px;
  color: #b71c1c;
}
.dropzone-size {
  color: #666;
  font-weight: 400;
}
.dropzone-hint {
  font-size: 12px !important;
  color: #666 !important;
}

.error-block,
.info-block {
  margin-top: 12px;
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 10px 12px;
  border-radius: 8px;
  font-size: 13px;
  line-height: 1.55;
}
.error-block {
  background: rgba(211, 47, 47, 0.08);
  color: #b71c1c;
  border: 1px solid rgba(211, 47, 47, 0.2);
}
.info-block {
  background: rgba(25, 118, 210, 0.08);
  color: #0d47a1;
  border: 1px solid rgba(25, 118, 210, 0.2);
}
.error-block .material-icons,
.info-block .material-icons {
  flex: none;
  font-size: 18px;
  margin-top: 1px;
}

.results-wrap {
  margin-top: 16px;
}
.results-title {
  margin: 0 0 10px;
  font-size: 14px;
  font-weight: 600;
  color: #333;
  display: flex;
  align-items: baseline;
  gap: 10px;
}
.results-subtitle {
  font-size: 12px;
  font-weight: 400;
  color: #777;
}
.results-list {
  list-style: none;
  padding: 0;
  margin: 0;
  max-height: 420px;
  overflow-y: auto;
  border: 1px solid var(--border, rgba(0, 0, 0, 0.08));
  border-radius: 10px;
  background: var(--surface-raised, #fff);
  /* 防止外层容器过窄时表格被压成"竖排文字" */
  min-width: 560px;
  width: 100%;
  box-sizing: border-box;
}
.result-row {
  display: grid;
  grid-template-columns: 40px 86px minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border, rgba(0, 0, 0, 0.05));
  cursor: pointer;
  transition: background 0.12s ease;
  width: 100%;
  box-sizing: border-box;
}
.result-row:last-child {
  border-bottom: none;
}
.result-row:hover,
.result-row:focus {
  background: rgba(25, 118, 210, 0.07);
  outline: none;
}
.result-rank {
  text-align: center;
  color: #888;
  font-weight: 700;
  font-size: 13px;
  flex-shrink: 0;
}
.result-sim {
  display: inline-block;
  padding: 3px 8px;
  border-radius: 999px;
  font-weight: 700;
  font-size: 12px;
  text-align: center;
  line-height: 1;
  flex-shrink: 0;
  min-width: 70px;
}
.result-sim.sim-excellent {
  background: rgba(46, 125, 50, 0.14);
  color: #1b5e20;
}
.result-sim.sim-good {
  background: rgba(25, 118, 210, 0.14);
  color: #0d47a1;
}
.result-sim.sim-medium {
  background: rgba(255, 152, 0, 0.16);
  color: #e65100;
}
.result-sim.sim-low {
  background: rgba(158, 158, 158, 0.16);
  color: #424242;
}
.result-main {
  min-width: 0;              /* 关键：允许 grid 子项收缩，配合 overflow 隐藏 */
  max-width: 100%;
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow: hidden;          /* 极端窄屏时强制截断，不让文字变成每个字母一行 */
}
.result-name {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: #222;
  /* 优先省略号：文件名很长时，用省略号 + title tooltip，而不是暴力 break-all 导致每字母换行 */
  min-width: 0;
  overflow: hidden;
}
.result-name .material-icons {
  flex: none;
  font-size: 16px;
  color: #b71c1c;
}
.result-name-text {
  /* 允许在词/路径段边界折行；极端窄屏仍可显示而不竖排 */
  display: inline-block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
  vertical-align: bottom;
}
.result-path {
  font-size: 11.5px;
  color: #777;
  /* 长路径：允许在斜杠/反斜杠处换行，但永远不会在单个字母处断开导致竖排 */
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}
.result-meta {
  font-size: 12px;
  color: #777;
  white-space: nowrap;
  text-align: right;
  flex-shrink: 0;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
.spin {
  animation: spin 900ms linear infinite;
}
</style>
