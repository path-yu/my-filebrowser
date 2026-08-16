<template>
  <div id="previewer" @mousemove="toggleNavigation" @touchstart="toggleNavigation">
    <!-- v-if="isPdf || isEpub || isCsv || showNav" -->
    <header-bar>
      <action icon="close" :label="$t('buttons.close')" @action="close()" />
      <title>{{ name }}</title>
      <action :disabled="layoutStore.loading" v-if="isResizeEnabled && fileStore.req?.type === 'image'"
        :icon="fullSize ? 'photo_size_select_large' : 'hd'" @action="toggleSize" />

      <template #actions>
        <action :disabled="layoutStore.loading" v-if="authStore.user?.perm.rename" icon="mode_edit"
          :label="$t('buttons.rename')" show="rename" />
        <action :disabled="layoutStore.loading" v-if="isCsv && authStore.user?.perm.modify" icon="edit_note"
          :label="t('buttons.editAsText')" @action="editAsText" />
        <action :disabled="layoutStore.loading" v-if="authStore.user?.perm.delete" icon="delete"
          :label="$t('buttons.delete')" @action="deleteFile" id="delete-button" />
        <action :disabled="layoutStore.loading" v-if="authStore.user?.perm.download" icon="file_download"
          :label="$t('buttons.download')" @action="download" />
        <!-- 编辑产品编号：仅 PDF 且有修改权限时显示 -->
        <action :disabled="layoutStore.loading" v-if="isPdf && authStore.user?.perm.modify && !isShareMode" icon="sell"
          :label="$t('buttons.productCode')" show="productCode" />
        <action :disabled="layoutStore.loading" v-if="
          ['image', 'audio', 'video'].includes(fileStore.req?.type || '') &&
          authStore.user?.perm.download
        " icon="open_in_new" :label="t('buttons.openDirect')" @action="openDirect" />
        <action :disabled="layoutStore.loading" icon="info" :label="$t('buttons.info')" show="info" />
      </template>
    </header-bar>

    <div class="loading delayed" v-if="layoutStore.loading">
      <div class="spinner">
        <div class="bounce1"></div>
        <div class="bounce2"></div>
        <div class="bounce3"></div>
      </div>
    </div>
    <template v-else>
      <div class="preview">
        <div v-if="isEpub" class="epub-reader">
          <vue-reader :location="location" :url="previewUrl" :get-rendition="getRendition" :epubInitOptions="{
            requestCredentials: true,
          }" :epubOptions="{
              allowPopups: true,
            }" @update:location="locationChange" />
          <div class="size">
            <button @click="changeSize(Math.max(100, size - 10))" class="reader-button">
              <i class="material-icons">remove</i>
            </button>
            <button @click="changeSize(Math.min(150, size + 10))" class="reader-button">
              <i class="material-icons">add</i>
            </button>
            <span>{{ size }}%</span>
          </div>
        </div>
        <CsvViewer v-else-if="isCsv" :content="csvContent" :error="csvError" />
        <ExtendedImage v-else-if="fileStore.req?.type == 'image'" :src="previewUrl" />
          <MacOsAudioPlay size="md"  bottom="120px" 
  width="80%"  v-else-if="fileStore.req?.type == 'audio'" ref="player" :src="previewUrl" />

        <VideoPlayer v-else-if="fileStore.req?.type == 'video'" ref="player" :source="previewUrl" :subtitles="subtitles"
          :options="videoOptions">
        </VideoPlayer>
        <div v-else-if="isPdf" class="pdf-window" :class="{ 'pdf-fullscreen': pdfIsFullscreen }">
          <!-- macOS 风格标题栏 (红黄绿交通灯 + 居中文件名 + 侧边栏开关) -->
          <div class="pdf-titlebar">
            <div class="pdf-traffic-lights">
              <span class="pdf-tl pdf-tl-close" title="关闭"></span>
              <span class="pdf-tl pdf-tl-min" title="最小化"></span>
              <span class="pdf-tl pdf-tl-max" :class="{ active: pdfIsFullscreen }" title="全屏"
                @click="pdfToggleFullscreen"></span>
            </div>
            <div class="pdf-titlebar-center">
              <svg class="pdf-pdf-icon" viewBox="0 0 24 24" fill="currentColor">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8l-6-6z" opacity=".9" />
                <path d="M14 2v6h6" fill="#fff" opacity=".5" />
                <text x="7" y="17" fontSize="6" fontWeight="700" fill="#fff">PDF</text>
              </svg>
              <span class="pdf-doc-title" :title="name">{{ name }}</span>
            </div>
            <div class="pdf-titlebar-right">
              <button class="pdf-icon-btn" :class="{ active: pdfSidebarOpen }" title="缩略图侧边栏"
                @click="pdfSidebarOpen = !pdfSidebarOpen">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                  <rect x="3" y="4" width="18" height="16" rx="2" />
                  <path d="M9 4v16" />
                </svg>
              </button>
            </div>
          </div>

          <!-- 工具栏：翻页输入 + 缩放 + 适合模式 + 下载 -->
          <div class="pdf-toolbar2">
            <div class="pdf-toolbar-group">
              <button class="pdf-toolbar-btn" :disabled="pdfCurrentPage <= 1" title="上一页 (←)" @click="pdfPrevPage">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M15 18l-6-6 6-6" />
                </svg>
              </button>
              <form class="pdf-pageform" @submit.prevent="pdfSubmitPage">
                <input v-model="pdfPageInput" type="text" class="pdf-pageinput" @blur="pdfSubmitPage" />
                <span class="pdf-pageslash">/ {{ pdfTotalPages || "–" }}</span>
              </form>
              <button class="pdf-toolbar-btn" :disabled="!pdfTotalPages || pdfCurrentPage >= pdfTotalPages"
                title="下一页 (→)" @click="pdfNextPage">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M9 18l6-6-6-6" />
                </svg>
              </button>
            </div>

            <div class="pdf-toolbar-divider"></div>

            <div class="pdf-toolbar-group">
              <button class="pdf-toolbar-btn" title="缩小 (⌘-)" @click="pdfZoomOut">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="11" cy="11" r="7" />
                  <path d="M21 21l-4.3-4.3M8 11h6" />
                </svg>
              </button>
              <button class="pdf-scale-label" @click="pdfResetScale" title="重置缩放 (⌘0)">
                {{ pdfDisplayScale }}
              </button>
              <button class="pdf-toolbar-btn" title="放大 (⌘+)" @click="pdfZoomIn">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="11" cy="11" r="7" />
                  <path d="M21 21l-4.3-4.3M11 8v6M8 11h6" />
                </svg>
              </button>
            </div>

            <div class="pdf-toolbar-divider"></div>

            <button class="pdf-toolbar-btn" :class="{ active: pdfFitMode === 'width' }" title="适合页宽"
              @click="pdfSetFitMode('width')">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                <path d="M4 12h16M8 8l-4 4 4 4M16 8l4 4-4 4" />
              </svg>
            </button>
            <button class="pdf-toolbar-btn" :class="{ active: pdfFitMode === 'page' }" title="适合整页"
              @click="pdfSetFitMode('page')">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                <rect x="5" y="3" width="14" height="18" rx="1.5" />
                <path d="M9 8h6M9 12h6M9 16h4" />
              </svg>
            </button>

            <div class="pdf-toolbar-spacer"></div>

            <a :href="downloadUrl" class="pdf-download-link" title="下载 PDF">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 3v12M7 11l5 5 5-5M5 21h14" />
              </svg>
              下载
            </a>
          </div>

          <!-- 主体：侧边栏缩略图 + Canvas 内容 -->
          <div class="pdf-body">
            <aside v-if="pdfSidebarOpen" class="pdf-sidebar">
              <div class="pdf-sidebar-header">缩略图</div>
              <div class="pdf-sidebar-scroll">
                <button v-for="(thumb, idx) in pdfThumbnails" :key="idx" class="pdf-thumb-item"
                  :class="{ active: idx + 1 === pdfCurrentPage }" @click="pdfGoToPage(idx + 1)">
                  <div class="pdf-thumb-frame" :class="{ active: idx + 1 === pdfCurrentPage }">
                    <img v-if="thumb" :src="thumb" :alt="'第 ' + (idx + 1) + ' 页'" class="pdf-thumb-img" />
                    <div v-else class="pdf-thumb-empty">{{ idx + 1 }}</div>
                  </div>
                  <div class="pdf-thumb-number" :class="{ active: idx + 1 === pdfCurrentPage }">
                    {{ idx + 1 }}
                  </div>
                </button>
                <p v-if="pdfTotalPages > pdfThumbnails.length && pdfThumbnails.length > 0" class="pdf-thumb-notice">
                  仅显示前 {{ pdfThumbnails.length }} 页
                </p>
              </div>
            </aside>

            <div ref="pdfContainerRef" class="pdf-canvas-container">
              <!-- 加载遮罩 -->
              <div v-if="pdfLoading" class="pdf-loading-backdrop">
                <div class="pdf-loading-spinner"></div>
                <p class="pdf-loading-text">正在加载 PDF…</p>
              </div>

              <!-- 错误卡片 -->
              <div v-if="pdfError && !pdfLoading" class="pdf-error-backdrop">
                <div class="pdf-error-card">
                  <div class="pdf-error-icon">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                      <circle cx="12" cy="12" r="9" />
                      <path d="M12 8v5M12 16h.01" />
                    </svg>
                  </div>
                  <h3 class="pdf-error-title">无法打开 PDF 文档</h3>
                  <p class="pdf-error-message pdf-error-message--multiline">{{ pdfError }}</p>
                  <div class="pdf-error-actions">
                    <a :href="downloadUrl" class="pdf-error-btn pdf-error-btn--primary" title="下载 PDF">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M12 3v12M7 11l5 5 5-5M5 21h14" />
                      </svg>
                      下载原文件
                    </a>
                    <a target="_blank" :href="previewUrl" class="pdf-error-btn pdf-error-btn--ghost"
                      v-if="!fileStore.req?.isDir" title="新窗口打开">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                        <path d="M15 3h6v6" />
                        <path d="M10 14L21 3" />
                      </svg>
                      新窗口打开
                    </a>
                  </div>
                </div>
              </div>

              <!-- Canvas 绘制区域 -->
              <div v-if="!pdfLoading && !pdfError" class="pdf-canvas-shadow">
                <canvas ref="pdfCanvasRef" class="pdf-canvas-white"></canvas>
              </div>
            </div>
          </div>

          <!-- 状态栏 -->
          <div class="pdf-statusbar">
            <span>
              {{ pdfTotalPages > 0 ? '第 ' + pdfCurrentPage + ' 页，共 ' + pdfTotalPages + ' 页' : '就绪' }}
            </span>
            <span class="pdf-status-hint">← → 翻页 · ⌘± 缩放</span>
            <span>{{ pdfDisplayScale }}</span>
          </div>
        </div>
        <!-- ============ Word (docx) 预览：对齐 Example wordPview.tsx macOS 风格 ============ -->
        <div v-else-if="isWord" class="pdf-window" :class="{ 'pdf-fullscreen': wordIsFullscreen }">
          <!-- docx-preview 样式注入容器（隐藏） -->
          <div ref="wordStyleRef" class="pdf-word-style" aria-hidden="true"></div>

          <!-- macOS 标题栏：交通灯 + 蓝色 W 图标 + 文件名 + 页数徽标 -->
          <div class="pdf-titlebar">
            <div class="pdf-traffic-lights">
              <span class="pdf-tl pdf-tl-close" title="关闭"></span>
              <span class="pdf-tl pdf-tl-min" title="最小化"></span>
              <span class="pdf-tl pdf-tl-max" :class="{ active: wordIsFullscreen }" title="全屏"
                @click="wordIsFullscreen = !wordIsFullscreen"></span>
            </div>
            <div class="pdf-titlebar-center">
              <div class="pdf-word-icon">
                <span>W</span>
              </div>
              <span class="pdf-doc-title" :title="name">{{ name }}</span>
            </div>
            <div class="pdf-titlebar-right">
              <span v-if="wordPageCount > 0 && !wordLoading && !wordError" class="pdf-page-badge">
                {{ wordPageCount }} 页
              </span>
            </div>
          </div>

          <!-- 工具栏：缩放 + 适合页宽 + 实际大小 + 下载 -->
          <div class="pdf-toolbar2">
            <div class="pdf-toolbar-group">
              <button class="pdf-toolbar-btn" title="缩小 (⌘-)" @click="wordZoomOut">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="11" cy="11" r="7" />
                  <path d="M21 21l-4.3-4.3M8 11h6" />
                </svg>
              </button>
              <button class="pdf-scale-label" title="重置缩放 (⌘0)" @click="wordResetZoom">
                {{ wordDisplayScale }}
              </button>
              <button class="pdf-toolbar-btn" title="放大 (⌘+)" @click="wordZoomIn">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <circle cx="11" cy="11" r="7" />
                  <path d="M21 21l-4.3-4.3M11 8v6M8 11h6" />
                </svg>
              </button>
            </div>

            <div class="pdf-toolbar-divider"></div>

            <button class="pdf-toolbar-btn" :class="{ active: wordFitWidth }" title="适合页宽"
              @click="wordFitWidth = !wordFitWidth">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                <path d="M4 12h16M8 8l-4 4 4 4M16 8l4 4-4 4" />
              </svg>
            </button>
            <button class="pdf-toolbar-btn" :class="{ active: !wordFitWidth && wordScale === 1 }" title="实际大小"
              @click="wordResetZoom">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                <rect x="5" y="3" width="14" height="18" rx="1.5" />
                <path d="M9 8h6M9 12h6M9 16h4" />
              </svg>
            </button>

            <div class="pdf-toolbar-spacer"></div>

            <a :href="downloadUrl" class="pdf-download-link" title="下载文档">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 3v12M7 11l5 5 5-5M5 21h14" />
              </svg>
              下载
            </a>
          </div>

          <!-- 主体：滚动容器 + docx 渲染宿主 -->
          <div class="pdf-body">
            <div ref="wordBodyRef" class="pdf-word-container">
              <!-- 加载遮罩 -->
              <div v-if="wordLoading" class="pdf-loading-backdrop">
                <div class="pdf-loading-spinner pdf-word-spinner"></div>
                <p class="pdf-loading-text">{{ isLegacyDoc ? "正在转换并加载 .doc 文档…" : "正在加载 Word 文档…" }}</p>
              </div>

              <!-- 错误卡片 -->
              <div v-if="wordError && !wordLoading" class="pdf-error-backdrop">
                <div class="pdf-error-card">
                  <div class="pdf-error-icon pdf-error-icon--word">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                      <circle cx="12" cy="12" r="9" />
                      <path d="M12 8v5M12 16h.01" />
                    </svg>
                  </div>
                  <h3 class="pdf-error-title">无法打开 Word 文档</h3>
                  <p class="pdf-error-message pdf-error-message--multiline">{{ wordError }}</p>
                  <p class="pdf-error-sub">{{ isLegacyDoc ? "旧版 .doc 需要服务器转换后预览" : "仅支持 .docx / .doc 格式" }}</p>
                  <div class="pdf-error-actions">
                    <a :href="downloadUrl" class="pdf-error-btn pdf-error-btn--primary" title="下载文档">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M12 3v12M7 11l5 5 5-5M5 21h14" />
                      </svg>
                      下载原文件
                    </a>
                    <a target="_blank" :href="previewUrl" class="pdf-error-btn pdf-error-btn--ghost"
                      v-if="!fileStore.req?.isDir" title="新窗口打开（浏览器可能下载）">
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                        <path d="M15 3h6v6" />
                        <path d="M10 14L21 3" />
                      </svg>
                      新窗口打开
                    </a>
                  </div>
                </div>
              </div>

              <!-- 内容：居中 + 缩放 -->
              <div class="pdf-word-stage" :class="{ 'is-hidden': wordLoading || !!wordError }">
                <div class="pdf-word-scalewrap" :class="{ 'is-fitwidth': wordFitWidth }"
                  :style="wordFitWidth ? undefined : { transform: `scale(${wordScale})` }">
                  <div ref="wordHostRef" class="pdf-word-host"></div>
                </div>
              </div>
            </div>
          </div>

          <!-- 状态栏 -->
          <div class="pdf-statusbar">
            <span>
              {{ wordPageCount > 0 ? `共 ${wordPageCount} 页` : wordLoading ? "加载中…" : "就绪" }}
            </span>
            <span class="pdf-status-hint">⌘± 缩放 · ⌘0 重置</span>
            <span>{{ wordDisplayScale }}</span>
          </div>
        </div>
        <div v-else-if="fileStore.req?.type == 'blob'" class="info">
          <div class="title">
            <i class="material-icons">feedback</i>
            {{ $t("files.noPreview") }}
          </div>
          <div>
            <a target="_blank" :href="downloadUrl" class="button button--flat">
              <div>
                <i class="material-icons">file_download</i>{{ $t("buttons.download") }}
              </div>
            </a>
            <a target="_blank" :href="previewUrl" class="button button--flat" v-if="!fileStore.req?.isDir">
              <div>
                <i class="material-icons">open_in_new</i>{{ $t("buttons.openFile") }}
              </div>
            </a>
          </div>
        </div>
      </div>
    </template>

    <!-- 左右切换按钮：仅 MP3(audio) / MP4(video) 预览时显示，图片、PDF、文本等一律隐藏 -->
    <button @click="prev" @mouseover="hoverNav = true" @mouseleave="hoverNav = false"
      :class="{ hidden: !isMediaPreview || !hasPrevious || !(showNav || alwaysShowNav) }" :aria-label="$t('buttons.previous')"
      :title="$t('buttons.previous')">
      <i class="material-icons">chevron_left</i>
    </button>
    <button @click="next" @mouseover="hoverNav = true" @mouseleave="hoverNav = false"
      :class="{ hidden: !isMediaPreview || !hasNext || !(showNav || alwaysShowNav) }" :aria-label="$t('buttons.next')" :title="$t('buttons.next')">
      <i class="material-icons">chevron_right</i>
    </button>
    <link rel="prefetch" :href="previousRaw" />
    <link rel="prefetch" :href="nextRaw" />
  </div>
</template>

<script setup lang="ts">
import { useStorage } from "@vueuse/core";
import { useAuthStore } from "@/stores/auth";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

import { files as api, pub as pubApi } from "@/api";
import { createURL } from "@/api/utils";
import { resizePreview } from "@/utils/constants";
import url from "@/utils/url";
import { throttle } from "lodash-es";
import HeaderBar from "@/components/header/HeaderBar.vue";
import Action from "@/components/header/Action.vue";
import ExtendedImage from "@/components/files/ExtendedImage.vue";
import VideoPlayer from "@/components/files/VideoPlayer.vue";
import CsvViewer from "@/components/files/CsvViewer.vue";
import { VueReader } from "vue-reader";
import {
  computed,
  inject,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  shallowRef,
  watch,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import MacOsAudioPlay from "@/components/MacOsAudioPlay.vue";
import type { Rendition } from "epubjs";
import { getTheme } from "@/utils/theme";
import { useI18n } from "vue-i18n";
// pdf.js / docx-preview 改为按需加载（共享 @/utils/previewLoaders）：
// 这两个库每个数百 KB，静态导入会把它们打进 Preview 组件 chunk，
// 而首屏（文件列表）根本用不到。worker 初始化（同版本字节级拷贝 +
// CDN 兜底）也统一由 previewLoaders 负责，与 FileListing 共享一次初始化。
import { ensurePdfLib, ensureDocxLib } from "@/utils/previewLoaders";

// CSV file size limit for preview (5MB)
// Prevents browser memory issues with large files
const CSV_MAX_SIZE = 5 * 1024 * 1024;

const location = useStorage("book-progress", 0, undefined, {
  serializer: {
    read: (v) => JSON.parse(v),
    write: (v) => JSON.stringify(v),
  },
});
const size = useStorage("book-size", 120, undefined, {
  serializer: {
    read: (v) => JSON.parse(v),
    write: (v) => JSON.stringify(v),
  },
});

const locationChange = (epubcifi: number) => {
  location.value = epubcifi;
};
let rendition: Rendition | null = null;
const changeSize = (val: number) => {
  size.value = val;
  rendition?.themes.fontSize(`${val}%`);
};

const getRendition = (_rendition: Rendition) => {
  rendition = _rendition;
  switch (getTheme()) {
    case "dark": {
      rendition.themes.override("color", "rgba(255, 255, 255, 0.6)");
      break;
    }
    case "light": {
      rendition.themes.override("color", "rgb(111, 111, 111)");
      break;
    }
  }
  rendition.themes.registerRules("h2Transparent", {
    "h1,h2,h3,h4": {
      "background-color": "transparent !important",
    },
  });
  rendition?.themes.fontSize(`${size.value}%`);
  rendition.themes.select("h2Transparent");
  rendition.themes.override("background-color", "transparent", true);
};

const mediaTypes: ResourceType[] = ["image", "video", "audio", "blob"];

const previousLink = ref<string>("");
const nextLink = ref<string>("");
const listing = ref<ResourceItem[] | null>(null);
const name = ref<string>("");
const fullSize = ref<boolean>(false);
const showNav = ref<boolean>(true);
const navTimeout = ref<null | number>(null);
const hoverNav = ref<boolean>(false);
const autoPlay = ref<boolean>(false);
const previousRaw = ref<string>("");
const nextRaw = ref<string>("");
const csvContent = ref<ArrayBuffer | string>("");
const csvError = ref<string>("");

const player = ref<HTMLVideoElement | HTMLAudioElement | null>(null);

/* ---------- PDF 预览（完全参考 example/src/components/PdfPreview.tsx macOS 风格） ---------- */
type PdfFitMode = "auto" | "width" | "page";
const pdfContainerRef = ref<HTMLDivElement | null>(null);
const pdfCanvasRef = ref<HTMLCanvasElement | null>(null);
// 渲染任务引用（每次 render 前取消旧任务，对应 Example 的 renderTaskRef）
// 注意：pdfjs 的 doc / renderTask 内部使用 # 私有字段，必须用 shallowRef 存储；
// 深度响应式代理会让 this.#xxx 抛 "Private element is not present on this object"。
const _pdfRenderTaskRef = shallowRef<any>(null);
const _pdfResizeObserver = ref<ResizeObserver | null>(null);
const _pdfLoadCancelled = ref<boolean>(false);
const _pdfDocHandle = shallowRef<any>(null);

const pdfDoc = shallowRef<any>(null);
const pdfCurrentPage = ref(1);
const pdfTotalPages = ref(0);
// scale 用户比例（配合 fitMode 计算 effectiveScale）
const pdfScale = ref(1.0);
const pdfFitMode = ref<PdfFitMode>("auto");
const pdfLoading = ref(false);
const pdfError = ref<string | null>(null);
const pdfThumbnails = ref<string[]>([]);
const pdfPageInput = ref("1");
const pdfIsFullscreen = ref(false);
const pdfSidebarOpen = ref(true);

// 缩放显示文本：页宽/整页/百分比
const pdfDisplayScale = computed(() => {
  if (pdfFitMode.value === "width") return "页宽";
  if (pdfFitMode.value === "page") return "整页";
  return Math.round(pdfScale.value * 100) + "%";
});

// Worker 初始化（已写在 <script setup> 顶部，此处留空避免重复）

const pdfDestroy = () => {
  _pdfLoadCancelled.value = true;

  // 取消 renderTask
  try {
    if (_pdfRenderTaskRef.value && typeof _pdfRenderTaskRef.value.cancel === "function") {
      _pdfRenderTaskRef.value.cancel();
    }
  } catch {
    /* ignore */
  }
  _pdfRenderTaskRef.value = null;

  // 断开 ResizeObserver
  try {
    if (_pdfResizeObserver.value) {
      _pdfResizeObserver.value.disconnect();
      _pdfResizeObserver.value = null;
    }
  } catch {
    /* ignore */
  }

  // 销毁文档句柄
  try {
    const doc = _pdfDocHandle.value || pdfDoc.value;
    if (doc && typeof doc.destroy === "function") doc.destroy();
  } catch {
    /* ignore */
  }
  _pdfDocHandle.value = null;

  // 清空状态
  pdfDoc.value = null;
  pdfTotalPages.value = 0;
  pdfCurrentPage.value = 1;
  pdfScale.value = 1.0;
  pdfThumbnails.value = [];
  pdfPageInput.value = "1";
  pdfError.value = null;
  pdfLoading.value = false;

  // 清空画布
  if (pdfCanvasRef.value) {
    try {
      const ctx = pdfCanvasRef.value.getContext("2d");
      ctx?.clearRect(0, 0, pdfCanvasRef.value.width, pdfCanvasRef.value.height);
      pdfCanvasRef.value.width = 0;
      pdfCanvasRef.value.height = 0;
    } catch {
      /* ignore */
    }
  }
};

/**
 * 核心渲染函数（完全对应 Example renderPage useCallback）
 * - 取消前一次 renderTask
 * - 根据 fitMode + 容器尺寸计算 effectiveScale
 * - DPR 内联进 viewport scale（不用 transform 数组）
 */
const pdfRenderPage = async () => {
  if (!pdfDoc.value) return;
  const canvas = pdfCanvasRef.value;
  const container = pdfContainerRef.value;
  if (!canvas || !container) return;

  // 1) 取消旧渲染任务
  if (_pdfRenderTaskRef.value && typeof _pdfRenderTaskRef.value.cancel === "function") {
    try {
      _pdfRenderTaskRef.value.cancel();
    } catch {
      /* ignore */
    }
    _pdfRenderTaskRef.value = null;
  }

  try {
    const page = await pdfDoc.value.getPage(pdfCurrentPage.value);
    const baseViewport = page.getViewport({ scale: 1 });
    const padding = 48;
    const availableW = Math.max(container.clientWidth - padding, 100);
    const availableH = Math.max(container.clientHeight - padding, 100);

    // 2) 计算 effectiveScale（对应 Example 逻辑）
    let effectiveScale = pdfScale.value;
    if (pdfFitMode.value === "width") {
      effectiveScale = availableW / baseViewport.width;
    } else if (pdfFitMode.value === "page") {
      effectiveScale = Math.min(
        availableW / baseViewport.width,
        availableH / baseViewport.height
      );
    } else {
      // auto：按适合宽度并限制最高 1.5 倍，再乘以用户 scale
      effectiveScale = Math.min(availableW / baseViewport.width, 1.5) * pdfScale.value;
    }

    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    // DPR 直接内联进 viewport scale（对应 Example 写法）
    const viewport = page.getViewport({ scale: effectiveScale * dpr });
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    canvas.width = Math.floor(viewport.width);
    canvas.height = Math.floor(viewport.height);
    canvas.style.width = Math.floor(viewport.width / dpr) + "px";
    canvas.style.height = Math.floor(viewport.height / dpr) + "px";

    const task = page.render({
      canvasContext: ctx,
      viewport,
      canvas,
    });
    _pdfRenderTaskRef.value = task;
    await task.promise;
    _pdfRenderTaskRef.value = null;
  } catch (err: any) {
    const isCancel =
      err?.name === "RenderingCancelledException" ||
      (err && typeof err.message === "string" && /cancell/i.test(err.message));
    if (isCancel) return;
    throw err;
  }
};

/**
 * 加载 PDF 文档（对应 Example useEffect 内 load() 函数）
 * - 带 X-Auth JWT 头 fetch raw 字节
 * - 生成缩略图最多 40 张
 */
const loadPdf = async () => {
  pdfDestroy();
  if (!fileStore.req || !isPdf.value) return;
  _pdfLoadCancelled.value = false;

  const cancelled = () => _pdfLoadCancelled.value;

  pdfLoading.value = true;
  pdfError.value = null;

  try {
    const rawUrl = buildRawUrl(fileStore.req.path, { inline: "true" });
    // 分享模式：URL 已携带 ?token=；用户模式：携带 X-Auth JWT 头
    // 文件字节与 pdfjs 库并行加载（库首次按需 import，不阻塞首屏）
    const [resp, pdfjs] = await Promise.all([
      fetch(rawUrl, {
        method: "GET",
        credentials: "same-origin",
        cache: "no-store",
        headers: buildFetchHeaders(),
      }),
      ensurePdfLib(),
    ]);
    if (cancelled()) return;
    if (!resp.ok) {
      let detail = "";
      try {
        detail = await resp.text();
      } catch { /* ignore */ }
      detail = (detail || "").trim();
      const base = `HTTP ${resp.status} ${resp.statusText || ""}`;
      if (detail) {
        throw new Error(`${base}：${detail}`);
      }
      throw new Error(`${base}。若文件无法解析，请尝试下载后用本机 PDF 阅读器打开。`);
    }

    const buffer = await resp.arrayBuffer();
    if (cancelled()) return;

    const loadingTask = (pdfjs as any).getDocument({
      data: new Uint8Array(buffer),
      useWorkerFetch: false,
      isEvalSupported: false,
      disableFontFace: false,
    });
    const doc = await loadingTask.promise;
    if (cancelled()) {
      try { doc.destroy(); } catch { /* ignore */ }
      return;
    }

    _pdfDocHandle.value = doc;
    pdfDoc.value = doc;
    pdfTotalPages.value = doc.numPages || 0;
    pdfCurrentPage.value = 1;
    pdfPageInput.value = "1";
    pdfScale.value = 1.0;
    pdfFitMode.value = "auto";

    // 等待 DOM 更新完成
    await nextTick();

    // 3) 挂载 ResizeObserver 监听容器尺寸变化重绘
    try {
      if (_pdfResizeObserver.value) _pdfResizeObserver.value.disconnect();
      const el = pdfContainerRef.value;
      if (el && typeof ResizeObserver !== "undefined") {
        const ro = new ResizeObserver(() => {
          void pdfRenderPage();
        });
        ro.observe(el);
        _pdfResizeObserver.value = ro;
      }
    } catch {
      /* ignore */
    }

    // 4) 关闭加载遮罩 → canvas（v-if="!pdfLoading"）才会挂载，
    //    等待挂载后再首屏渲染，否则 pdfCanvasRef 为 null 会静默跳过
    pdfLoading.value = false;
    await nextTick();
    await pdfRenderPage();
    if (cancelled()) return;

    // 5) 生成缩略图（性能上限 40 张，对应 Example）
    const thumbs: string[] = [];
    const maxThumbs = Math.min(doc.numPages, 40);
    for (let i = 1; i <= maxThumbs; i++) {
      if (cancelled()) break;
      try {
        const p = await doc.getPage(i);
        const vp = p.getViewport({ scale: 0.2 });
        const off = document.createElement("canvas");
        off.width = Math.max(1, Math.floor(vp.width));
        off.height = Math.max(1, Math.floor(vp.height));
        const octx = off.getContext("2d");
        if (octx) {
          await p.render({ canvasContext: octx, viewport: vp, canvas: off }).promise;
          try {
            thumbs.push(off.toDataURL("image/jpeg", 0.7));
          } catch {
            thumbs.push("");
          }
        } else {
          thumbs.push("");
        }
      } catch {
        thumbs.push("");
      }
    }
    if (!cancelled()) pdfThumbnails.value = thumbs;
  } catch (err: any) {
    if (_pdfLoadCancelled.value) return;
    const msg = (err && err.message) ? String(err.message) : (err ? String(err) : "无法加载 PDF 文件");
    pdfError.value = msg;
    $showError(err);
  } finally {
    if (!_pdfLoadCancelled.value) pdfLoading.value = false;
  }
};

/* ---------- PDF 用户交互函数（对应 Example goToPage/zoomIn/zoomOut 等） ---------- */
const pdfGoToPage = (n: number) => {
  if (!pdfTotalPages.value) return;
  const next = Math.min(Math.max(1, n), pdfTotalPages.value);
  pdfCurrentPage.value = next;
  pdfPageInput.value = String(next);
  void pdfRenderPage();
};

const pdfPrevPage = () => pdfGoToPage(pdfCurrentPage.value - 1);
const pdfNextPage = () => pdfGoToPage(pdfCurrentPage.value + 1);

const pdfZoomIn = () => {
  pdfFitMode.value = "auto";
  pdfScale.value = Math.min(3, Math.round((pdfScale.value + 0.1) * 10) / 10);
  void pdfRenderPage();
};

const pdfZoomOut = () => {
  pdfFitMode.value = "auto";
  pdfScale.value = Math.max(0.4, Math.round((pdfScale.value - 0.1) * 10) / 10);
  void pdfRenderPage();
};

const pdfResetScale = () => {
  pdfFitMode.value = "auto";
  pdfScale.value = 1;
  void pdfRenderPage();
};

const pdfSetFitMode = (m: PdfFitMode) => {
  pdfFitMode.value = m;
  pdfScale.value = 1;
  void pdfRenderPage();
};

const pdfSubmitPage = () => {
  const n = parseInt(pdfPageInput.value, 10);
  if (!Number.isNaN(n)) {
    pdfGoToPage(n);
  } else {
    pdfPageInput.value = String(pdfCurrentPage.value);
  }
};

const pdfToggleFullscreen = () => { pdfIsFullscreen.value = !pdfIsFullscreen.value; };

/* ============================================================
 * Word (docx) 预览 —— 对齐 Example wordPview.tsx
 * docx-preview renderAsync 保真渲染 + macOS 风格 UI + 缩放/适合页宽
 * ============================================================ */
const wordBodyRef = ref<HTMLElement | null>(null);
const wordHostRef = ref<HTMLElement | null>(null);
const wordStyleRef = ref<HTMLElement | null>(null);

const wordLoading = ref(false);
const wordError = ref<string | null>(null);
const wordScale = ref(1);
const wordFitWidth = ref(false);
const wordIsFullscreen = ref(false);
const wordPageCount = ref(0);
let _wordLoadSeq = 0;

const wordDisplayScale = computed(() =>
  wordFitWidth.value ? "页宽" : `${Math.round(wordScale.value * 100)}%`
);

const wordZoomIn = () => {
  wordFitWidth.value = false;
  wordScale.value = Math.min(2.5, Math.round((wordScale.value + 0.1) * 10) / 10);
};
const wordZoomOut = () => {
  wordFitWidth.value = false;
  wordScale.value = Math.max(0.5, Math.round((wordScale.value - 0.1) * 10) / 10);
};
const wordResetZoom = () => {
  wordFitWidth.value = false;
  wordScale.value = 1;
};

const wordDestroy = () => {
  _wordLoadSeq++;
  try { wordHostRef.value && (wordHostRef.value.innerHTML = ""); } catch { /* ignore */ }
  try { wordStyleRef.value && (wordStyleRef.value.innerHTML = ""); } catch { /* ignore */ }
  wordPageCount.value = 0;
  wordError.value = null;
};

/**
 * 加载 Word 文档（对应 Example useEffect 内 load()）
 * - X-Auth fetch raw 字节 + 基本校验（空文件 / HTML 错误页）
 * - docx-preview renderAsync 保真渲染（分页 section）
 */
const loadWord = async () => {
  const seq = ++_wordLoadSeq;
  if (!fileStore.req || !isWord.value) return;

  wordLoading.value = true;
  wordError.value = null;
  wordPageCount.value = 0;
  try { wordHostRef.value && (wordHostRef.value.innerHTML = ""); } catch { /* ignore */ }
  try { wordStyleRef.value && (wordStyleRef.value.innerHTML = ""); } catch { /* ignore */ }

  try {
    const host = wordHostRef.value;
    const styleContainer = wordStyleRef.value;
    if (!host || !styleContainer) {
      await nextTick();
    }

    // 旧版 .doc → 后端 Word COM 转换端点；.docx → 原始文件
    // 分享模式：走 public 端点并带 ?token=；用户模式：走 /api 内部端点 + X-Auth 头
    const rawUrl = isLegacyDoc.value
      ? buildConvertDocUrl(fileStore.req.path)
      : buildRawUrl(fileStore.req.path, { inline: "true" });
    const resp = await fetch(rawUrl, {
      method: "GET",
      credentials: "same-origin",
      cache: "no-store",
      headers: buildFetchHeaders(),
    });
    if (seq !== _wordLoadSeq) return;
    if (!resp.ok) {
      // 优先使用后端响应体中的详细错误信息（含安装指引 / 具体原因）
      let detail = "";
      try {
        detail = await resp.text();
      } catch { /* ignore */ }
      detail = (detail || "").trim();
      if (detail) {
        throw new Error(detail);
      }
      if (isLegacyDoc.value && resp.status === 503) {
        throw new Error(
          "转换 .doc 失败：服务器既未安装 Microsoft Office（Word），也未安装 LibreOffice。\n" +
          "请安装其中任意一个后重启服务。临时替代：直接下载原文件用本机 Word/WPS 打开。"
        );
      }
      throw new Error(`无法获取文档（HTTP ${resp.status}）`);
    }

    const contentType = resp.headers.get("content-type") || "";
    const buffer = await resp.arrayBuffer();
    if (seq !== _wordLoadSeq) return;

    // 基本校验（对应 Example）
    if (buffer.byteLength < 100) throw new Error("文档内容为空或无效");
    if (
      contentType.includes("text/html") &&
      new TextDecoder().decode(buffer.slice(0, 64)).trimStart().startsWith("<")
    ) {
      throw new Error("返回的不是有效的 .docx 文件");
    }

    const h = wordHostRef.value;
    const sc = wordStyleRef.value;
    if (!h || !sc) return;

    // docx-preview 按需加载（首次使用时才下载该 chunk）
    const docxLib = await ensureDocxLib();
    if (seq !== _wordLoadSeq) return;
    await docxLib.renderAsync(buffer, h, sc, {
      className: "docx-mac-preview",
      // 禁用 docx-preview 自带的 wrapper：它会注入一套硬编码深色背景的工具栏
      // （页号/搜索/缩放/页宽等），在应用浅色模式下显得格格不入（纯黑条）。
      // 我们已经有自己的伪窗口 UI（pdf-titlebar + pdf-toolbar2），支持响应式主题。
      inWrapper: false,
      ignoreWidth: false,
      ignoreHeight: false,
      ignoreFonts: false,
      breakPages: true,
      ignoreLastRenderedPageBreak: true,
      experimental: true,
      trimXmlDeclaration: true,
      useBase64URL: true,
      renderChanges: false,
      renderHeaders: true,
      renderFooters: true,
      renderFootnotes: true,
      renderEndnotes: true,
    });
    if (seq !== _wordLoadSeq) return;

    // 统计渲染出的页（section）。inWrapper=false 时 docx-preview 直接
    // 在宿主内渲染 section（可能带或不带 .docx class，取决于库版本）。
    const pages = h.querySelectorAll(
      "section.docx, section"
    );
    wordPageCount.value = pages.length || 1;
  } catch (e: any) {
    if (seq !== _wordLoadSeq) return;
    const msg = String(e?.message || e || "");
    const friendly =
      msg.includes("Can't find end of central directory") ||
        msg.includes("Corrupted zip") ||
        msg.toLowerCase().includes("zip")
        ? "无法解析该文件，请确认是有效的 Word 文档"
        : msg || "无法加载 Word 文档";
    wordError.value = friendly;
  } finally {
    if (seq === _wordLoadSeq) wordLoading.value = false;
  }
};

const $showError = inject<IToastError>("$showError")!;

const authStore = useAuthStore();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

const { t } = useI18n();

const route = useRoute();
const router = useRouter();

/* ---------- 分享页面上下文（公开匿名访问 vs 已登录私有访问） ---------- */
/** 公开分享模式：URL 路由为 /share/:hash/... 或 req 上带 share hash（公开 token 鉴权） */
const isShareMode = computed(() => {
  if (route.path.startsWith("/share/")) return true;
  return !!fileStore.req?.hash;
});
/** 分享 hash（分享根路径） */
const shareHash = computed(() => fileStore.req?.hash || "");
/** 分享一次性访问 token，已通过密码验证后 pubApi.fetch 返回 */
const shareToken = computed(() => fileStore.req?.token || "");

/**
 * 构造 raw 文件访问 URL：
 *  - 分享模式：/api/public/dl/<hash>/<path>?token=xxx&其他参数
 *  - 用户模式：/api/raw/<path>?其他参数 + JWT X-Auth 头
 */
function buildRawUrl(path: string, extraParams?: Record<string, string | undefined>) {
  const params: Record<string, string | undefined> = { ...(extraParams || {}) };
  if (isShareMode.value) {
    if (shareToken.value) params.token = shareToken.value;
    return createURL(`api/public/dl/${shareHash.value}${path}`, params);
  }
  return createURL(`api/raw${path}`, params);
}
/** 构造 .doc 转换 URL：/api/public/convert/doc/<hash>/<path>?token=xxx vs /api/convert/doc/<path> */
function buildConvertDocUrl(path: string) {
  if (isShareMode.value) {
    const params: Record<string, string | undefined> = {};
    if (shareToken.value) params.token = shareToken.value;
    return createURL(`api/public/convert/doc/${shareHash.value}${path}`, params);
  }
  return createURL(`api/convert/doc${path}`, {});
}
/** 构造预览缩图 URL：分享模式直接用 public/dl（不走 preview 内部鉴权） */
function buildPreviewUrl(res: Pick<Resource, "hash" | "token" | "path" | "modified" | "size">, size: "big" | "small") {
  if (isShareMode.value) {
    const p: Record<string, string | undefined> = {};
    const token = (res as any).token || shareToken.value;
    const hash = (res as any).hash || shareHash.value;
    if (token) p.token = token;
    // 分享场景直接用原图 raw，避免 thumbnail 端点需要内部 JWT
    return createURL(`api/public/dl/${hash}${res.path}`, p);
  }
  return api.getPreviewURL(res as Resource, size);
}
/** 构造下载 URL：分享模式使用 pub API */
function buildDownloadURL(
  res: Pick<Resource, "hash" | "token" | "path">,
  inline = false
) {
  if (isShareMode.value) {
    return pubApi.getDownloadURL(
      {
        ...(res as Resource),
        hash: (res as any).hash || shareHash.value,
        token: (res as any).token || shareToken.value,
      } as Resource,
      inline
    );
  }
  return api.getDownloadURL(res as Resource, inline);
}
/** 用于 fetch 调用的 headers：用户模式用 X-Auth（JWT），分享模式用 URL 携带 token 已无额外头 */
function buildFetchHeaders(): Record<string, string> {
  if (!isShareMode.value && authStore?.jwt) {
    return { "X-Auth": authStore.jwt };
  }
  return {};
}

const hasPrevious = computed(() => previousLink.value !== "");

const hasNext = computed(() => nextLink.value !== "");

// 仅 MP3(audio) / MP4(video) 预览才展示左右切换按钮；
// 图片、PDF、文本、代码、epub 等一律隐藏该按钮组，避免非音视频场景误操作。
const isMediaPreview = computed(() =>
  ["audio", "video"].includes(fileStore.req?.type ?? "")
);

// 音视频（MP3/MP4 等）预览时左右切换按钮常显：
// 不跟随"鼠标静止 1.5s 自动隐藏"逻辑，保证切换入口始终可见。
const alwaysShowNav = computed(() => isMediaPreview.value);

const downloadUrl = computed(() =>
  fileStore.req ? buildDownloadURL(fileStore.req, false) : ""
);

const directUrl = computed(() =>
  fileStore.req ? buildDownloadURL(fileStore.req, true) : ""
);

const previewUrl = computed(() => {
  if (!fileStore.req) {
    return "";
  }

  if (fileStore.req.type === "image" && !fullSize.value) {
    return buildPreviewUrl(fileStore.req, "big");
  }

  if (isEpub.value) {
    return buildRawUrl(fileStore.req.path, {});
  }

  return buildDownloadURL(fileStore.req, true);
});

const isPdf = computed(() => fileStore.req?.extension.toLowerCase() == ".pdf");
// .docx 直接渲染；旧版 .doc 由后端 Word COM 转 .docx 后渲染
const isWord = computed(() => [".docx", ".doc"].includes(fileStore.req?.extension?.toLowerCase() ?? ""));
// 是否为旧版 .doc（需走 /api/convert/doc 转换）
const isLegacyDoc = computed(() => fileStore.req?.extension.toLowerCase() == ".doc");
const isEpub = computed(
  () => fileStore.req?.extension.toLowerCase() == ".epub"
);
const isCsv = computed(
  () =>
    fileStore.req?.extension.toLowerCase() == ".csv" &&
    fileStore.req.size <= CSV_MAX_SIZE
);

const isResizeEnabled = computed(() => resizePreview);

const subtitles = computed(() => {
  if (fileStore.req?.subtitles) {
    return api.getSubtitlesURL(fileStore.req);
  }
  return [];
});

const videoOptions = computed(() => {
  return { autoplay: autoPlay.value };
});

watch(route, () => {
  updatePreview();
  toggleNavigation();
});

// 标题实时切换：优先取资源元数据的真实文件名（/files 与 /share/:hash 两种上下文均正确），
// 元数据缺失时回退到 URL 最后一段。route 与 fileStore.req 的更新先后顺序不确定，
// 两者任一变化都触发本 watch，保证左右切换文件时 <title>{{ name }}</title> 立即更新。
// req 更新后还需重算 prev/next（watch(route) 先触发时 req 仍是旧文件），
// updatePreview 内部以 route 解析的最新文件名为准。
{
  // 初始 name：setup 时 req 尚未就绪，从 route 兜底解析（此时函数定义均已完成）
  const dirs = route.fullPath.split("/");
  name.value = decodeURIComponent(dirs[dirs.length - 1]);
}
watch(
  () => fileStore.req,
  () => {
    const reqName = fileStore.req?.name?.trim();
    if (reqName) {
      name.value = reqName;
    } else {
      const dirs = route.fullPath.split("/");
      name.value = decodeURIComponent(dirs[dirs.length - 1]);
    }
    void updatePreview();
  }
);

// Specify hooks
onMounted(async () => {
  window.addEventListener("keydown", key);
  listing.value = fileStore.oldReq?.items ?? null;
  updatePreview();
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", key);
  pdfDestroy();
  wordDestroy();
});

// Specify methods
const deleteFile = () => {
  layoutStore.showHover({
    prompt: "delete",
    confirm: () => {
      if (listing.value === null) {
        return;
      }

      const index = listing.value.findIndex((item) => item.name == name.value);
      listing.value.splice(index, 1);

      if (hasNext.value) {
        next();
      } else if (!hasPrevious.value && !hasNext.value) {
        const nearbyItem = listing.value[Math.max(0, index - 1)];
        fileStore.preselect = nearbyItem?.path;

        close();
      } else {
        prev();
      }
    },
  });
};

const prev = () => {
  hoverNav.value = false;
  router.replace({ path: previousLink.value });
};

const next = () => {
  hoverNav.value = false;
  router.replace({ path: nextLink.value });
};

const key = (event: KeyboardEvent) => {
  if (layoutStore.currentPrompt !== null) {
    return;
  }
  const targetTag = (event.target as HTMLElement)?.tagName;
  const isInput = targetTag === "INPUT" || targetTag === "TEXTAREA";

  // PDF 专用快捷键（对应 Example 实现）
  if (isPdf.value && !isInput) {
    const mod = event.metaKey || event.ctrlKey;
    if (event.key === "ArrowRight" || event.which === 39 || event.key === "PageDown" || event.key === " ") {
      event.preventDefault();
      pdfNextPage();
      return;
    }
    if (event.key === "ArrowLeft" || event.which === 37 || event.key === "PageUp") {
      event.preventDefault();
      pdfPrevPage();
      return;
    }
    if (mod && (event.key === "=" || event.key === "+")) {
      event.preventDefault();
      pdfZoomIn();
      return;
    }
    if (mod && event.key === "-") {
      event.preventDefault();
      pdfZoomOut();
      return;
    }
    if (mod && event.key === "0") {
      event.preventDefault();
      pdfResetScale();
      return;
    }
  }

  // Word 专用快捷键（对应 Example wordPview.tsx）
  if (isWord.value && !isInput) {
    const mod = event.metaKey || event.ctrlKey;
    if (mod && (event.key === "=" || event.key === "+")) {
      event.preventDefault();
      wordZoomIn();
      return;
    }
    if (mod && event.key === "-") {
      event.preventDefault();
      wordZoomOut();
      return;
    }
    if (mod && event.key === "0") {
      event.preventDefault();
      wordResetZoom();
      return;
    }
  }

  // When previewing a video, let arrow keys fall through to video.js for
  // seeking instead of switching to the prev/next file. Enter still advances.
  const isVideo = fileStore.req?.type === "video";
  if (event.which === 13) {
    // enter
    if (hasNext.value) next();
  } else if (event.which === 39) {
    // right arrow
    if (isVideo) return;
    if (hasNext.value) next();
  } else if (event.which === 37) {
    // left arrow
    if (isVideo) return;
    if (hasPrevious.value) prev();
  } else if (event.which === 27) {
    // esc
    close();
  }
};
const updatePreview = async () => {
  if (player.value && player.value.paused && !player.value.ended) {
    autoPlay.value = false;
  }

  // Prefer the resource's actual `name` (which already reflects the shared
  // file's real filename in both /files and /share/:hash contexts) over
  // the last URL segment. Fall back to the URL segment when the resource
  // metadata is missing, to preserve legacy behavior on pure URL navigation
  // where req has not (yet) been populated.
  // 实际更新逻辑由下方 watch(fileStore.req) 承担：route 变化与 req 变化
  // 的先后顺序不确定，watch 两条链路都触发，保证 title 实时切换。

  // Load CSV content if it's a CSV file
  if (isCsv.value && fileStore.req) {
    csvContent.value = "";
    csvError.value = "";

    if (fileStore.req.size > CSV_MAX_SIZE) {
      csvError.value = t("files.csvTooLarge");
    } else {
      if (fileStore.req.rawContent != null) {
        csvContent.value = fileStore.req.rawContent;
      } else {
        csvContent.value = fileStore.req.content ?? "";
      }
    }
  }

  // Load PDF via pdfjs (instead of <object> tag)
  if (isPdf.value && fileStore.req) {
    void loadPdf();
  }

  // Load Word via docx-preview
  if (isWord.value && fileStore.req) {
    void nextTick(() => loadWord());
  } else {
    wordDestroy();
  }

  if (!listing.value) {
    try {
      const path = url.removeLastDir(route.path);
      const res = await api.fetch(path);
      listing.value = res.items;
    } catch (e: any) {
      $showError(e);
    }
  }

  previousLink.value = "";
  nextLink.value = "";
  previousRaw.value = "";
  nextRaw.value = "";

  // 只有 MP3/MP4 预览才做"同目录内音视频循环切换"。
  // 图片、PDF、文本、代码、epub 等文件：previousLink / nextLink 保持为空，
  // 保证 L382/L388 两个按钮始终被 :class.hidden 隐藏。
  const currentType = fileStore.req?.type;
  if (!listing.value || (currentType !== "audio" && currentType !== "video")) {
    return;
  }

  // 音视频预览（MP3/MP4 等）：取当前目录内全部 audio + video 文件作为切换序列，
  // 首尾循环（第一个的上一个是最后一个）。这样只要目录里不止一个 mp3/mp4，
  // 左右切换按钮始终可用，可在所有音视频文件间连续轮转。
  // 注意：watch(route) 触发时 fileStore.req 可能仍是上一个文件（store 尚未更新），
  // 因此当前文件名优先从 route 解析（route 永远是最新的），避免用旧文件名算出指向自己的 next。
  {
    const mediaList = listing.value.filter(
      (item) => item.type === "audio" || item.type === "video"
    );
    const routeSegs = route.fullPath.split("/");
    const routeName = decodeURIComponent(routeSegs[routeSegs.length - 1]);
    const currentName = routeName || fileStore.req?.name?.trim() || name.value;
    const idx = mediaList.findIndex((item) => item.name === currentName);
    if (idx >= 0 && mediaList.length > 1) {
      const prevItem = mediaList[(idx - 1 + mediaList.length) % mediaList.length];
      const nextItem = mediaList[(idx + 1) % mediaList.length];
      previousLink.value = prevItem.url;
      nextLink.value = nextItem.url;
    }
  }
};

const prefetchUrl = (item: ResourceItem) => {
  if (item.type !== "image") {
    return "";
  }

  return fullSize.value
    ? buildDownloadURL(item, true)
    : buildPreviewUrl(item, "big");
};

const toggleSize = () => (fullSize.value = !fullSize.value);

const toggleNavigation = throttle(function () {
  showNav.value = true;

  if (navTimeout.value) {
    clearTimeout(navTimeout.value);
  }

  navTimeout.value = window.setTimeout(() => {
    showNav.value = false || hoverNav.value;
    navTimeout.value = null;
  }, 1500);
}, 500);

const close = () => {
  const uri = url.removeLastDir(route.path) + "/";
  router.push({ path: uri });
};

const download = () => window.open(downloadUrl.value);
const openDirect = () => window.open(directUrl.value);

const editAsText = () => {
  router.push({ path: route.path, query: { edit: "true" } });
};
</script>

<style lang="css">
/* ===========================================================
   PDF Preview — macOS/iOS 风格 UI
   完全对应 example/src/components/PdfPreview.tsx 的视觉设计。
   Example 使用 Tailwind，这里转换为等价的纯 CSS 类。
   =========================================================== */

#previewer .preview {
  height: 100%;
}

/* 外部窗口（圆角 + 毛玻璃 + 阴影环 + 边框） */
#previewer .pdf-window {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-height: 0;
  box-sizing: border-box;
  padding: 12px;
  background: transparent;
}

#previewer .pdf-window.pdf-fullscreen {
  padding: 0;
}

/* ---------- macOS 标题栏 ---------- */
#previewer .pdf-titlebar {
  position: relative;
  flex: 0 0 48px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  border-top-left-radius: 12px;
  border-top-right-radius: 12px;
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  background: linear-gradient(180deg, #f6f6f7 0%, #ebebed 100%);
  box-sizing: border-box;
}

html.dark #previewer .pdf-titlebar {
  border-color: rgba(255, 255, 255, 0.07);
  border-bottom-color: rgba(255, 255, 255, 0.04);
  background: linear-gradient(180deg, #3a3a3c 0%, #2c2c2e 100%);
}

#previewer .pdf-traffic-lights {
  display: flex;
  align-items: center;
  gap: 8px;
  z-index: 2;
}

#previewer .pdf-tl {
  display: inline-block;
  width: 12px;
  height: 12px;
  border-radius: 9999px;
  box-shadow: 0 1px 0 rgba(0, 0, 0, 0.12);
  outline: 1px solid rgba(0, 0, 0, 0.1);
  transition: transform 0.15s ease;
  cursor: default;
}

#previewer .pdf-tl:hover {
  transform: scale(1.1);
}

#previewer .pdf-tl-close {
  background: #ff5f57;
}

#previewer .pdf-tl-min {
  background: #febc2e;
}

#previewer .pdf-tl-max {
  background: #28c840;
  cursor: pointer;
}

#previewer .pdf-tl-max.active {
  outline-color: #006500;
}

#previewer .pdf-titlebar-center {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  pointer-events: none;
}

#previewer .pdf-pdf-icon {
  width: 16px;
  height: 16px;
  flex: 0 0 auto;
  color: #ff3b30;
}

#previewer .pdf-doc-title {
  max-width: 50%;
  font-size: 13px;
  font-weight: 500;
  letter-spacing: -0.01em;
  color: #1d1d1f;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

html.dark #previewer .pdf-doc-title {
  color: #f5f5f7;
}

#previewer .pdf-titlebar-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 4px;
  z-index: 2;
}

#previewer .pdf-icon-btn {
  display: inline-flex;
  width: 28px;
  height: 28px;
  align-items: center;
  justify-content: center;
  border: 0;
  background: transparent;
  color: #1d1d1f;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.15s;
}

#previewer .pdf-icon-btn svg {
  width: 16px;
  height: 16px;
}

#previewer .pdf-icon-btn:hover {
  background: rgba(255, 255, 255, 0.9);
}

#previewer .pdf-icon-btn.active {
  background: #ffffff;
  color: #007aff;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
  outline: 1px solid rgba(0, 0, 0, 0.08);
}

html.dark #previewer .pdf-icon-btn {
  color: #f5f5f7;
}

html.dark #previewer .pdf-icon-btn:hover {
  background: rgba(255, 255, 255, 0.08);
}

html.dark #previewer .pdf-icon-btn.active {
  background: rgba(255, 255, 255, 0.12);
  outline-color: rgba(255, 255, 255, 0.1);
}

/* ---------- 工具栏 ---------- */
#previewer .pdf-toolbar2 {
  flex: 0 0 44px;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 12px;
  border-left: 1px solid rgba(0, 0, 0, 0.08);
  border-right: 1px solid rgba(0, 0, 0, 0.08);
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  background: rgba(240, 240, 242, 0.9);
  box-sizing: border-box;
}

html.dark #previewer .pdf-toolbar2 {
  border-color: rgba(255, 255, 255, 0.06);
  background: rgba(28, 28, 30, 0.9);
}

#previewer .pdf-toolbar-group {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  background: rgba(0, 0, 0, 0.05);
  border-radius: 8px;
}

html.dark #previewer .pdf-toolbar-group {
  background: rgba(255, 255, 255, 0.05);
}

#previewer .pdf-toolbar-divider {
  width: 1px;
  height: 20px;
  background: rgba(0, 0, 0, 0.1);
  margin: 0 8px;
}

html.dark #previewer .pdf-toolbar-divider {
  background: rgba(255, 255, 255, 0.1);
}

#previewer .pdf-toolbar-btn {
  display: inline-flex;
  width: 28px;
  height: 28px;
  align-items: center;
  justify-content: center;
  border: 0;
  background: transparent;
  color: #1d1d1f;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.15s;
}

#previewer .pdf-toolbar-btn svg {
  width: 16px;
  height: 16px;
}

#previewer .pdf-toolbar-btn:hover {
  background: rgba(255, 255, 255, 0.85);
}

#previewer .pdf-toolbar-btn:disabled {
  pointer-events: none;
  opacity: 0.3;
}

#previewer .pdf-toolbar-btn.active {
  background: #ffffff;
  color: #007aff;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
  outline: 1px solid rgba(0, 0, 0, 0.06);
}

html.dark #previewer .pdf-toolbar-btn {
  color: #f5f5f7;
}

html.dark #previewer .pdf-toolbar-btn:hover {
  background: rgba(255, 255, 255, 0.08);
}

html.dark #previewer .pdf-toolbar-btn.active {
  background: rgba(255, 255, 255, 0.12);
  outline-color: rgba(255, 255, 255, 0.08);
}

#previewer .pdf-pageform {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 4px;
  margin: 0;
}

#previewer .pdf-pageinput {
  width: 40px;
  height: 24px;
  padding: 0 4px;
  border: 0;
  background: #ffffff;
  color: #1d1d1f;
  text-align: center;
  font-size: 12px;
  font-weight: 500;
  border-radius: 6px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
  outline: 1px solid rgba(0, 0, 0, 0.1);
}

#previewer .pdf-pageinput:focus {
  outline: 2px solid #007aff;
}

#previewer .pdf-pageslash {
  font-size: 12px;
  color: #86868b;
}

html.dark #previewer .pdf-pageinput {
  background: #3a3a3c;
  color: #ffffff;
  outline-color: rgba(255, 255, 255, 0.1);
}

html.dark #previewer .pdf-pageslash {
  color: #8e8e93;
}

#previewer .pdf-scale-label {
  min-width: 56px;
  height: 28px;
  padding: 0 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  background: transparent;
  color: #1d1d1f;
  font-size: 12px;
  font-weight: 500;
  border-radius: 6px;
  cursor: pointer;
}

#previewer .pdf-scale-label:hover {
  background: rgba(255, 255, 255, 0.8);
}

html.dark #previewer .pdf-scale-label {
  color: #f5f5f7;
}

html.dark #previewer .pdf-scale-label:hover {
  background: rgba(255, 255, 255, 0.08);
}

#previewer .pdf-toolbar-spacer {
  flex: 1 1 auto;
}

#previewer .pdf-download-link {
  display: inline-flex;
  height: 28px;
  align-items: center;
  gap: 6px;
  padding: 0 10px;
  font-size: 12px;
  font-weight: 500;
  color: #007aff;
  border-radius: 6px;
  text-decoration: none;
  transition: background 0.15s;
}

#previewer .pdf-download-link:hover {
  background: rgba(0, 122, 255, 0.1);
}

#previewer .pdf-download-link svg {
  width: 14px;
  height: 14px;
}

/* ---------- 主体 ---------- */
#previewer .pdf-body {
  flex: 1 1 auto;
  display: flex;
  min-height: 0;
  border-left: 1px solid rgba(0, 0, 0, 0.08);
  border-right: 1px solid rgba(0, 0, 0, 0.08);
  background: #e8e8ed;
}

html.dark #previewer .pdf-body {
  border-color: rgba(255, 255, 255, 0.06);
  background: #2c2c2e;
}

/* ---------- 缩略图侧边栏 ---------- */
#previewer .pdf-sidebar {
  flex: 0 0 140px;
  display: flex;
  flex-direction: column;
  border-right: 1px solid rgba(0, 0, 0, 0.05);
  background: rgba(245, 245, 247, 0.8);
  box-sizing: border-box;
}

html.dark #previewer .pdf-sidebar {
  border-color: rgba(255, 255, 255, 0.04);
  background: rgba(28, 28, 30, 0.8);
}

#previewer .pdf-sidebar-header {
  padding: 8px 12px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.05);
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: #86868b;
}

html.dark #previewer .pdf-sidebar-header {
  border-color: rgba(255, 255, 255, 0.05);
  color: #8e8e93;
}

#previewer .pdf-sidebar-scroll {
  flex: 1 1 auto;
  overflow-y: auto;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-sizing: border-box;
}

#previewer .pdf-thumb-item {
  display: block;
  width: 100%;
  background: transparent;
  border: 0;
  padding: 0;
  margin: 0;
  cursor: pointer;
  text-align: left;
  transition: transform 0.15s;
}

#previewer .pdf-thumb-item:hover {
  transform: scale(1.02);
}

#previewer .pdf-thumb-frame {
  overflow: hidden;
  border-radius: 6px;
  background: #ffffff;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
  outline: 1px solid rgba(0, 0, 0, 0.1);
  transition: all 0.2s;
}

html.dark #previewer .pdf-thumb-frame {
  background: #2c2c2e;
  outline-color: rgba(255, 255, 255, 0.1);
}

#previewer .pdf-thumb-frame.active {
  outline: 2px solid #007aff;
  box-shadow: 0 4px 12px rgba(0, 122, 255, 0.2);
}

#previewer .pdf-thumb-img {
  display: block;
  width: 100%;
  height: auto;
}

#previewer .pdf-thumb-empty {
  aspect-ratio: 3 / 4;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  color: #86868b;
}

#previewer .pdf-thumb-number {
  margin-top: 4px;
  text-align: center;
  font-size: 11px;
  color: #86868b;
}

#previewer .pdf-thumb-number.active {
  font-weight: 600;
  color: #007aff;
}

#previewer .pdf-thumb-notice {
  padding-bottom: 8px;
  margin: 0;
  text-align: center;
  font-size: 10px;
  color: #86868b;
}

/* ---------- Canvas 容器 ---------- */
#previewer .pdf-canvas-container {
  position: relative;
  flex: 1 1 auto;
  overflow: auto;
  min-height: 0;
  padding: 24px;
  box-sizing: border-box;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  background-color: #d1d1d6;
  background-image: radial-gradient(circle at center, #b8b8be 1px, transparent 1px);
  background-size: 16px 16px;
}

html.dark #previewer .pdf-canvas-container {
  background-color: #000000;
  background-image: radial-gradient(circle at center, #3a3a3c 1px, transparent 1px);
}

#previewer .pdf-canvas-shadow {
  margin: auto 0;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.3), 0 1px 4px rgba(0, 0, 0, 0.1);
  outline: 1px solid rgba(0, 0, 0, 0.1);
  background: #ffffff;
}

#previewer .pdf-canvas-white {
  display: block;
  background: #ffffff;
  max-width: 100%;
  height: auto;
}

/* ---------- 加载遮罩 ---------- */
#previewer .pdf-loading-backdrop {
  position: absolute;
  inset: 0;
  z-index: 10;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  background: rgba(209, 209, 214, 0.8);
  backdrop-filter: blur(4px);
}

html.dark #previewer .pdf-loading-backdrop {
  background: rgba(0, 0, 0, 0.6);
}

#previewer .pdf-loading-spinner {
  width: 32px;
  height: 32px;
  border-radius: 9999px;
  border: 2px solid #007aff;
  border-top-color: transparent;
  animation: pdf-spin 0.8s linear infinite;
}

@keyframes pdf-spin {
  to {
    transform: rotate(360deg);
  }
}

#previewer .pdf-loading-text {
  margin: 0;
  font-size: 14px;
  font-weight: 500;
  color: #1d1d1f;
}

html.dark #previewer .pdf-loading-text {
  color: #f5f5f7;
}

/* ---------- 错误卡片 ---------- */
#previewer .pdf-error-backdrop {
  position: absolute;
  inset: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px;
  box-sizing: border-box;
}

#previewer .pdf-error-card {
  max-width: 400px;
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(10px);
  border-radius: 16px;
  padding: 24px;
  text-align: center;
  box-shadow: 0 20px 48px rgba(0, 0, 0, 0.25);
  outline: 1px solid rgba(0, 0, 0, 0.1);
}

html.dark #previewer .pdf-error-card {
  background: rgba(44, 44, 46, 0.95);
  outline-color: rgba(255, 255, 255, 0.08);
}

#previewer .pdf-error-icon {
  width: 48px;
  height: 48px;
  margin: 0 auto 12px;
  border-radius: 9999px;
  background: rgba(255, 59, 48, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #ff3b30;
}

#previewer .pdf-error-icon svg {
  width: 24px;
  height: 24px;
}

#previewer .pdf-error-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #1d1d1f;
}

html.dark #previewer .pdf-error-title {
  color: #ffffff;
}

#previewer .pdf-error-message {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: #86868b;
}

/* 多行错误（带安装指引）：保留换行与缩进 */
#previewer .pdf-error-message--multiline {
  white-space: pre-wrap;
  word-break: break-word;
  text-align: left;
  padding: 0 4px;
}

#previewer .pdf-error-sub {
  margin: 6px 0 0;
  font-size: 12px;
  line-height: 1.5;
  color: #a1a1a6;
}

/* 错误卡片底部 fallback 操作按钮 */
#previewer .pdf-error-actions {
  margin-top: 16px;
  display: flex;
  gap: 10px;
  justify-content: center;
  flex-wrap: wrap;
}

#previewer .pdf-error-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 980px;
  /* iOS 胶囊 */
  font-size: 13px;
  font-weight: 500;
  text-decoration: none;
  cursor: pointer;
  transition: all 0.15s ease;
  border: 1px solid transparent;
  white-space: nowrap;
}

#previewer .pdf-error-btn svg {
  width: 16px;
  height: 16px;
}

#previewer .pdf-error-btn--primary {
  background: #007aff;
  color: #fff;
  border-color: #007aff;
}

#previewer .pdf-error-btn--primary:hover {
  background: #0062cc;
  border-color: #0062cc;
}

#previewer .pdf-error-btn--primary:active {
  transform: scale(0.97);
}

#previewer .pdf-error-btn--ghost {
  background: #fff;
  color: #007aff;
  border-color: rgba(0, 122, 255, 0.3);
}

#previewer .pdf-error-btn--ghost:hover {
  background: rgba(0, 122, 255, 0.06);
  border-color: #007aff;
}

#previewer .pdf-error-btn--ghost:active {
  transform: scale(0.97);
}

html.dark #previewer .pdf-error-message--multiline {
  color: #8e8e93;
}

html.dark #previewer .pdf-error-sub {
  color: #636366;
}

html.dark #previewer .pdf-error-btn--ghost {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(10, 132, 255, 0.4);
  color: #0a84ff;
}

html.dark #previewer .pdf-error-btn--ghost:hover {
  background: rgba(10, 132, 255, 0.12);
  border-color: #0a84ff;
}

html.dark #previewer .pdf-error-btn--primary {
  background: #0a84ff;
  border-color: #0a84ff;
}

html.dark #previewer .pdf-error-btn--primary:hover {
  background: #409cff;
  border-color: #409cff;
}

/* ---------- 状态栏 ---------- */
#previewer .pdf-statusbar {
  flex: 0 0 28px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
  border-bottom-left-radius: 12px;
  border-bottom-right-radius: 12px;
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-top: 1px solid rgba(0, 0, 0, 0.05);
  background: rgba(240, 240, 242, 0.9);
  font-size: 11px;
  color: #86868b;
  box-sizing: border-box;
}

html.dark #previewer .pdf-statusbar {
  border-color: rgba(255, 255, 255, 0.06);
  background: rgba(28, 28, 30, 0.9);
  color: #8e8e93;
}

#previewer .pdf-status-hint {
  display: none;
}

@media (min-width: 640px) {
  #previewer .pdf-status-hint {
    display: inline;
  }
}

/* ===========================================================
   Word 预览 —— 对齐 example wordPview.tsx 的 macOS 风格
   =========================================================== */

/* docx-preview 样式注入容器（隐藏但保持可渲染） */
#previewer .pdf-word-style {
  position: absolute;
  width: 0;
  height: 0;
  overflow: hidden;
  visibility: hidden;
}

/* 标题栏 Word 蓝色 W 图标 */
#previewer .pdf-word-icon {
  width: 16px;
  height: 16px;
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 3px;
  /* background: linear-gradient(135deg, #2b579a, #185abd); */
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
}

#previewer .pdf-word-icon span {
  font-size: 8px;
  font-weight: 700;
  line-height: 1;
  color: #ffffff;
}

/* 标题栏右侧页数徽标 */
#previewer .pdf-page-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 6px;
  background: rgba(0, 0, 0, 0.05);
  font-size: 11px;
  color: #86868b;
}

html.dark #previewer .pdf-page-badge {
  background: rgba(255, 255, 255, 0.08);
}

/* 滚动容器（圆点网格背景，与 PDF 一致） */
#previewer .pdf-word-container {
  position: relative;
  flex: 1 1 auto;
  overflow: auto;
  min-height: 0;
  background-color: #d1d1d6;
  background-image: radial-gradient(circle at center, #b8b8be 1px, transparent 1px);
  background-size: 16px 16px;
}

html.dark #previewer .pdf-word-container {
  background-color: #000000;
  background-image: radial-gradient(circle at center, #3a3a3c 1px, transparent 1px);
}

/* 内容舞台：居中 + 内边距 */
#previewer .pdf-word-stage {
  display: flex;
  justify-content: center;
  padding: 24px;
  transition: opacity 0.15s;
}

#previewer .pdf-word-stage.is-hidden {
  pointer-events: none;
  opacity: 0;
}

/* 缩放包装：origin-top transform；fitWidth 时 100% */
#previewer .pdf-word-scalewrap {
  transform-origin: top center;
  transition: transform 0.15s ease-out;
}

#previewer .pdf-word-scalewrap.is-fitwidth {
  width: 100%;
  max-width: 100%;
  transform: none;
}

/* docx 渲染宿主：白纸卡片 */
#previewer .pdf-word-host {
  background: #ffffff;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.3), 0 1px 4px rgba(0, 0, 0, 0.1);
  outline: 1px solid rgba(0, 0, 0, 0.1);
}

#previewer .pdf-word-scalewrap.is-fitwidth #previewer .pdf-word-host,
#previewer .pdf-word-scalewrap.is-fitwidth .pdf-word-host {
  width: 100%;
}

/* docx-preview 输出的 section 兜底 */
#previewer .pdf-word-host .docx-wrapper,
#previewer .pdf-word-host .docx-mac-preview-wrapper {
  background: transparent;
  padding: 0;
}

#previewer .pdf-word-host section.docx {
  box-shadow: none;
  margin-bottom: 16px;
  background: #ffffff;
}

html.dark #previewer .pdf-word-host {
  outline-color: rgba(255, 255, 255, 0.1);
}

/* Word 加载 spinner（Word 蓝） */
#previewer .pdf-word-spinner {
  border-color: #2b579a;
}

/* Word 错误图标（Word 蓝） */
#previewer .pdf-error-icon--word {
  background: rgba(43, 87, 154, 0.1);
  color: #2b579a;
}

/* 错误卡片副标题 */
#previewer .pdf-error-sub {
  margin: 10px 0 0;
  font-size: 12px;
  color: #86868b;
}
</style>
