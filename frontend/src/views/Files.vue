<template>
  <div>
    <header-bar
      v-if="error || fileStore.req?.type === undefined"
      showMenu
      showLogo
    />

    <div class="breadcrumbs-row">
      <breadcrumbs base="/files" />
      <div v-if="showFilter" class="breadcrumbs-filters" role="group" aria-label="列表筛选">
        <MacOSSelect
          v-model="fileTypeFilter"
          :options="FILE_TYPE_FILTER_OPTIONS"
          class="breadcrumbs-filter"
          aria-label="文件类型筛选"
        />
        <MacOSSelect
          v-model="timeFilter"
          :options="TIME_FILTER_OPTIONS"
          class="breadcrumbs-filter"
          aria-label="修改时间筛选"
        />
      </div>
    </div>
    <errors v-if="error" :errorCode="error.status" />
    <component v-else-if="currentView" :is="currentView"></component>
    <div v-else>
      <h2 class="message delayed">
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
        <span>{{ t("files.loading") }}</span>
      </h2>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  computed,
  defineAsyncComponent,
  nextTick,
  onBeforeMount,
  onBeforeUnmount,
  onMounted,
  onUnmounted,
  ref,
  watch,
} from "vue";
import { files as api } from "@/api";
import { storeToRefs } from "pinia";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

import HeaderBar from "@/components/header/HeaderBar.vue";
import Breadcrumbs from "@/components/Breadcrumbs.vue";
import Errors from "@/views/Errors.vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import FileListing from "@/views/files/FileListing.vue";
import { StatusError } from "@/api/utils";
import { name } from "../utils/constants";
import MacOSSelect from "@/components/MacOSSelect.vue";
import {
  fileTypeFilter,
  FILE_TYPE_FILTER_OPTIONS,
} from "@/composables/fileTypeFilter";
import {
  timeFilter,
  TIME_FILTER_OPTIONS,
  resolveTimeFilter,
} from "@/composables/timeFilter";

const Editor = defineAsyncComponent(() => import("@/views/files/Editor.vue"));
const Preview = defineAsyncComponent(() => import("@/views/files/Preview.vue"));

const layoutStore = useLayoutStore();
const fileStore = useFileStore();

const viewportWidth = ref(typeof window !== "undefined" ? window.innerWidth : 1440);
onBeforeMount(() => {
  const onResize = () => (viewportWidth.value = window.innerWidth);
  window.addEventListener("resize", onResize);
  onBeforeUnmount(() => window.removeEventListener("resize", onResize));
});

/** 是否显示面包屑行的筛选下拉：仅在目录列表视图（FileListing）下显示 */
const showFilter = computed(
  () => !error.value && !!fileStore.req?.isDir && viewportWidth.value >= 900
);

const { reload } = storeToRefs(fileStore);

const route = useRoute();
const router = useRouter();

const { t } = useI18n({});

let fetchDataController = new AbortController();

const error = ref<StatusError | null>(null);

/* ------------------------------------------------------------------ */
/* 选中状态 ↔ URL 参数（?sel=文件名1,文件名2）双向同步                  */
/*  - 用户点击选中 → 写入 URL（router.replace，不产生历史记录）          */
/*  - 刷新/进入目录 → fetchData 成功后从 URL 恢复选中                   */
/*  - syncingSelection 锁：fetchData/恢复阶段的内部调整不回写 URL        */
/* ------------------------------------------------------------------ */
const encodeSel = (names: string[]) =>
  names.map(encodeURIComponent).join(",");
const decodeSel = (raw: unknown): string[] => {
  const s = Array.isArray(raw)
    ? raw.join(",")
    : typeof raw === "string"
      ? raw
      : "";
  if (!s) return [];
  try {
    return s.split(",").map(decodeURIComponent).filter(Boolean);
  } catch {
    return [];
  }
};
let syncingSelection = false;
/* lastSyncedPath 的作用：
 *   双击文件夹进入子目录的瞬间，两次 click 事件会把「第一下的选中修改」
 *   推到 Pinia，然后紧接着第二下触发 router.push。
 *   但 Vue 的 watcher 是在下一轮 flush（nextTick 之后）才跑，
 *   此时「第一下的 Pinia 变更」还没刷到 watcher，而 router.push 已经把路由改了。
 *   等 watcher 真正跑起来时：它读的是旧 route.query（push 前的快照）+
 *   旧 route.path（router 的 reactive 对象也是 push 前解析出来的？不完全是；
 *   但更重要的是——在路由还没完全 commit 的微任务间隙，watch 可能会触发），
 *   于是 `router.replace({ path: route.path, query: nextQuery })`
 *   这条「仅为了同步 query 参数」的 replace 就变成了「把用户弹回旧目录」，
 *   从而 **取消** 了之前的 push，表现为「点文件夹完全不动」。
 *
 *   解决：在 watch 里记录「最后一次成功同步时的 route.path」，
 *   如果当前 watch 触发时发现 route.path 已经不是上次的值（
 *   说明路径正在切换或已经切换完毕），直接跳过这次 replace。
 *   path 的切换自有另一个独立的 watcher 处理，不应该被 sel 同步打断。 */
let lastSyncedPath: string | null = null;

watch(
  () => route.path,
  (p) => {
    lastSyncedPath = p;
  },
  { immediate: true }
);

watch(
  () => [...fileStore.selected].sort((a, b) => a - b),
  (indices) => {
    if (syncingSelection) return;
    // 路径正在/已经切换：取消本次 sel 写入避免 replace 与 push 互相取消
    if (lastSyncedPath !== route.path) return;
    // 关键：搜索模式下用户点击选中的是 searchResults（过滤后的列表）里的索引，
    //       不能拿全量 req.items 去映射（否则选中的文件完全错位，sel 里会写错文件名）。
    const items = fileStore.searchMode
      ? fileStore.searchResults
      : fileStore.req?.items;
    if (!items) return;
    const names = indices
      .map((i) => items[i])
      .filter(Boolean)
      .map((it) => it.name);
    /** encodeSel 先对每个文件名单独 encodeURIComponent（逗号变成 %2C，
     *  避免文件名本身含逗号时 split(",") 被拆坏），再用 "," 拼接多个文件。
     *  Vue Router 4 的 router.replace/push 会对 query value 再做一次
     *  encodeURIComponent，为了避免双重 encode（用户 URL 里看到 %25 等），
     *  这里在写入前先 decode 一次，最终地址栏恰好保留一次 encode（中文变 %XX）。 */
    const encoded = encodeSel(names);
    if ((route.query.sel as string | undefined ?? "") === encoded) return;
    const nextQuery = { ...route.query } as Record<string, any>;
    if (encoded) {
      try {
        nextQuery.sel = decodeURIComponent(encoded);
      } catch {
        nextQuery.sel = encoded;
      }
    } else {
      delete nextQuery.sel;
    }
    router.replace({ path: route.path, query: nextQuery });
  }
);

const restoreSelectionFromQuery = () => {
  const names = decodeSel(route.query.sel);
  if (names.length === 0) return;
  // 兼容两种模式：搜索模式下从 searchResults 按名字匹配（和写入时保持一致），
  // 目录模式下从 req.items 匹配（req.items 里有原顺序 index，searchResults 也有 normalize 时补的 index）
  const items = fileStore.searchMode
    ? fileStore.searchResults
    : fileStore.req?.items;
  if (!items) return;
  const indices = items
    .filter((it: any) => names.includes(it.name))
    .map((it: any) => it.index);
  if (indices.length === 0) return;
  // 加锁：restore 期间对 selected 的修改不要回写 URL（否则会触发 watch→replace→watch 的死循环）
  syncingSelection = true;
  try {
    fileStore.selected = indices;
    if (indices.length > 1) fileStore.multiple = true;
  } finally {
    // 下一个 tick 后再解锁，保证当前微任务队列里的 watch 回调全都被拦住
    nextTick(() => {
      syncingSelection = false;
    });
  }
};

const currentView = computed(() => {
  if (fileStore.req?.type === undefined) {
    return null;
  }

  if (fileStore.req.isDir) {
    return FileListing;
  } else if (fileStore.req.extension.toLowerCase() === ".csv") {
    // CSV files use Preview for table view, unless ?edit=true
    if (route.query.edit === "true") {
      return Editor;
    }
    return Preview;
  } else if (
    fileStore.req.type === "text" ||
    fileStore.req.type === "textImmutable"
  ) {
    return Editor;
  } else {
    return Preview;
  }
});

// Define hooks
onMounted(() => {
  fetchData();
  fileStore.isFiles = true;
  window.addEventListener("keydown", keyEvent);
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", keyEvent);
});

onUnmounted(() => {
  fileStore.isFiles = false;
  if (layoutStore.showShell) {
    layoutStore.toggleShell();
  }
  fileStore.updateRequest(null);
  fetchDataController.abort();
});

/* 判断 path 是否指向「目录列表」而不是单个文件预览：
 *  Files 路由下，目录路径都以 "/" 结尾（/files/、/files/图纸/），
 *  文件预览路径结尾是文件名 .pdf/.png 等（/files/图纸/a.pdf 不带尾斜杠）。
 *  只有「目录 ↔ 目录」之间切换（跨目录）才需要清 sel；
 *  只要一方是预览页就不要碰 sel——典型是用户关预览返回列表时，
 *  router.push 回来的 URL 是带着 sel 的，若这时误删，后续 restore 就找不到了。 */
const isDirListingPath = (p: string | undefined): boolean => {
  if (!p) return false;
  // 非 /files 路由不算（share / settings 等不走这里）
  if (!p.startsWith("/files")) return false;
  // 目录路径一定以 "/" 结尾；/files 本身 redirect 到 /files/，所以这里严格判
  return p === "/files" || p.endsWith("/");
};

/* 只监听 path：query 变化（如 ?sel= 同步、?edit= 切换）不重新拉取目录，
 * 否则点击选中 → 写 URL → 重取数据 → 清空选中 会形成循环 */
watch(
  () => route.path,
  (newPath, oldPath) => {
    if (newPath !== oldPath) {
      // ⚠️ 跨目录才清 sel；只要一侧是文件预览返回/进入就保留 URL 里的 sel
      if (
        route.query.sel !== undefined &&
        isDirListingPath(newPath) &&
        isDirListingPath(oldPath)
      ) {
        const nextQuery = { ...route.query } as Record<string, any>;
        delete nextQuery.sel;
        router
          .replace({ path: newPath, query: nextQuery, hash: route.hash })
          .catch(() => {
            /* ignore aborted / duplicate */
          });
      }
      fetchData();
    }
  }
);
// 时间筛选变化 → 重新拉取文件列表 / 搜索也会被 Search.vue 自身 watch 触发重搜
watch(timeFilter, () => {
  if (!fileStore.searchMode) {
    fetchData();
  }
});
watch(reload, (newValue) => {
  newValue && fetchData();
});

// Define functions

const applyPreSelection = () => {
  const preselect = fileStore.preselect;
  fileStore.preselect = null;

  if (!fileStore.req?.isDir || fileStore.oldReq === null) return;

  let index = -1;
  if (preselect) {
    // Find item with the specified path
    index = fileStore.req.items.findIndex((item) => item.path === preselect);
  } else if (fileStore.oldReq.path.startsWith(fileStore.req.path)) {
    // Get immediate child folder of the previous path
    const name = fileStore.oldReq.path
      .substring(fileStore.req.path.length)
      .split("/")
      .shift();

    index = fileStore.req.items.findIndex(
      (val) => val.path == fileStore.req!.path + name
    );
  }

  if (index === -1) return;
  fileStore.selected.push(index);
};

const fetchData = async () => {
  // Reset view information.
  // 加锁：fetchData 全过程对 selected 的调整（清空/恢复）都不回写 URL
  syncingSelection = true;
  fileStore.reload = false;
  fileStore.selected = [];
  fileStore.multiple = false;
  layoutStore.closeHovers();

  // Set loading to true and reset the error.
  layoutStore.loading = true;
  error.value = null;

  let url = route.path;
  if (url === "") url = "/";
  if (url[0] !== "/") url = "/" + url;
  // Cancel the ongoing request
  fetchDataController.abort();
  fetchDataController = new AbortController();
  try {
    const range = resolveTimeFilter(timeFilter.value);
    const query = {
      modified_after: range.modifiedAfter,
      modified_before: range.modifiedBefore,
    };
    const res = await api.fetch(url, fetchDataController.signal, query);
    fileStore.updateRequest(res);
    document.title = `${res.name || t("sidebar.myFiles")} - ${t("files.files")} - ${name}`;
    layoutStore.loading = false;

    // Selects the post-reload target item or the previously visited child folder
    applyPreSelection();
    // 从 URL ?sel= 恢复选中（优先级高于 preselect）
    restoreSelectionFromQuery();
  } catch (err) {
    if (err instanceof StatusError && err.is_canceled) {
      return;
    }
    if (err instanceof Error) {
      error.value = err;
    }
    layoutStore.loading = false;
  } finally {
    syncingSelection = false;
  }
};
const keyEvent = (event: KeyboardEvent) => {
  if (event.key === "F1") {
    event.preventDefault();
    layoutStore.showHover("help");
  }
};
</script>
