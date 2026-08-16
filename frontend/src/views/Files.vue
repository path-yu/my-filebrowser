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
  onBeforeUnmount,
  onMounted,
  onUnmounted,
  ref,
  watch,
  onBeforeMount,
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

watch(
  () => [...fileStore.selected].sort((a, b) => a - b),
  (indices) => {
    if (syncingSelection) return;
    const items = fileStore.req?.items;
    if (!items) return;
    const names = indices
      .map((i) => items[i])
      .filter(Boolean)
      .map((it) => it.name);
    const encoded = encodeSel(names);
    if ((route.query.sel as string | undefined ?? "") === encoded) return;
    const nextQuery = { ...route.query } as Record<string, string>;
    if (encoded) nextQuery.sel = encoded;
    else delete nextQuery.sel;
    router.replace({ path: route.path, query: nextQuery });
  }
);

const restoreSelectionFromQuery = () => {
  const names = decodeSel(route.query.sel);
  if (names.length === 0) return;
  const items = fileStore.req?.items;
  if (!items) return;
  const indices = items
    .filter((it) => names.includes(it.name))
    .map((it) => it.index);
  if (indices.length === 0) return;
  fileStore.selected = indices;
  if (indices.length > 1) fileStore.multiple = true;
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

/* 只监听 path：query 变化（如 ?sel= 同步、?edit= 切换）不重新拉取目录，
 * 否则点击选中 → 写 URL → 重取数据 → 清空选中 会形成循环 */
watch(
  () => route.path,
  () => {
    fetchData();
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
