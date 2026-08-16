<template>
  <div id="editor-container">
    <header-bar>
      <action icon="close" :label="t('buttons.close')" @action="close()" />
      <title>{{ fileStore.req?.name ?? "" }}</title>

      <action
        icon="add"
        @action="increaseFontSize"
        :label="t('buttons.increaseFontSize')"
      />
      <span class="editor-font-size">{{ fontSize }}px</span>
      <action
        icon="remove"
        @action="decreaseFontSize"
        :label="t('buttons.decreaseFontSize')"
      />

      <action
        v-if="canSave"
        id="save-button"
        icon="save"
        :label="t('buttons.save')"
        @action="save()"
      />

      <action
        icon="preview"
        :label="t('buttons.preview')"
        @action="preview()"
        v-show="isMarkdownFile"
      />
    </header-bar>

    <!-- preview container -->
    <div class="loading delayed" v-if="layoutStore.loading">
      <div class="spinner">
        <div class="bounce1"></div>
        <div class="bounce2"></div>
        <div class="bounce3"></div>
      </div>
    </div>
    <template v-else>
      <div class="editor-header">
        <Breadcrumbs :base="breadcrumbsBase" noLink />

        <div>
          <button
            :disabled="isSelectionEmpty"
            @click="executeEditorCommand('copy')"
          >
            <span><i class="material-icons">content_copy</i></span>
          </button>
          <button
            :disabled="isSelectionEmpty"
            @click="executeEditorCommand('cut')"
          >
            <span><i class="material-icons">content_cut</i></span>
          </button>
          <button @click="executeEditorCommand('paste')">
            <span><i class="material-icons">content_paste</i></span>
          </button>
          <button @click="executeEditorCommand('openCommandPalette')">
            <span><i class="material-icons">more_vert</i></span>
          </button>
        </div>
      </div>

      <div
        v-show="isPreview && isMarkdownFile"
        id="preview-container"
        class="md_preview"
        v-html="previewContent"
      ></div>
      <form v-show="!isPreview || !isMarkdownFile" id="editor" ref="editorElRef"></form>
    </template>
  </div>
</template>

<script setup lang="ts">
import { files as api } from "@/api";
import buttons from "@/utils/buttons";
import url from "@/utils/url";
import ace, { Ace, version as ace_version } from "ace-builds";
import "ace-builds/src-noconflict/ext-language_tools";
import modelist from "ace-builds/src-noconflict/ext-modelist";
import DOMPurify from "dompurify";

import Breadcrumbs from "@/components/Breadcrumbs.vue";
import Action from "@/components/header/Action.vue";
import HeaderBar from "@/components/header/HeaderBar.vue";
import { useAuthStore } from "@/stores/auth";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import { getEditorTheme } from "@/utils/theme";
import { marked } from "marked";
import markedKatex from "marked-katex-extension";
import {
  computed,
  inject,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watchEffect,
} from "vue";
import { useI18n } from "vue-i18n";
import { onBeforeRouteUpdate, useRoute, useRouter } from "vue-router";
import { read, copy } from "@/utils/clipboard";

const $showError = inject<IToastError>("$showError")!;

const fileStore = useFileStore();
const authStore = useAuthStore();
const layoutStore = useLayoutStore();

const { t } = useI18n();

const route = useRoute();
const router = useRouter();

const editor = ref<Ace.Editor | null>(null);
// #editor 表单的模板引用：v-if="layoutStore.loading" 的 v-else 分支渲染，
// loading 翻转后需等 DOM 刷新（nextTick）才可用
const editorElRef = ref<HTMLFormElement | null>(null);
const fontSize = ref(parseInt(localStorage.getItem("editorFontSize") || "14"));

const isPreview = ref(false);
const previewContent = ref("");
const isMarkdownFile =
  fileStore.req?.name.endsWith(".md") ||
  fileStore.req?.name.endsWith(".markdown");

// Share mode detection & helpers — mirrors Preview.vue so raw file fetch
// works both on /files and /share/:hash routes.
const isShareMode = computed(() => {
  if (route.path.startsWith("/share/")) return true;
  if (fileStore.req && "hash" in fileStore.req) return true;
  return false;
});

const shareHash = computed(() => {
  if (route.path.startsWith("/share/")) return route.params.hash as string;
  return (fileStore.req as Resource & { hash?: string })?.hash ?? "";
});

const shareToken = computed(() => {
  return (fileStore.req as Resource & { token?: string })?.token ?? "";
});

// In share mode the viewer is always read-only: public shares never expose
// write APIs. Also respect the original textImmutable flag.
const isReadOnly = computed(() => {
  return (
    isShareMode.value || fileStore.req?.type === "textImmutable"
  );
});

const canSave = computed(() => {
  if (isShareMode.value) return false;
  return !!authStore.user?.perm.modify;
});

const breadcrumbsBase = computed(() => {
  if (isShareMode.value) return `/share/${shareHash.value}`;
  return "/files";
});

const createURL = (
  url: string,
  params?: Record<string, string | undefined>
) => {
  const u = new URL(url, window.location.origin);
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== null && v !== "") u.searchParams.set(k, v);
    }
  }
  return u.toString().replace(u.origin, "");
};

const buildRawUrl = (path: string) => {
  if (isShareMode.value) {
    return createURL(`api/public/dl/${shareHash.value}${path}`, {
      token: shareToken.value,
    });
  }
  return createURL(`api/raw${path}`);
};

const buildFetchHeaders = () => {
  const headers: Record<string, string> = {};
  if (!isShareMode.value && authStore.jwt) {
    headers["X-Auth"] = authStore.jwt;
  }
  return headers;
};

// Editor operates on a single file — resolve the path to that file so we can
// fetch its raw bytes as text when the pre-loaded `content` metadata field
// was omitted (common when navigating to a file entry that came from a
// directory listing response where Expand=false, e.g. share viewport
// entries populated via Share.vue).
const resolveFilePath = () => {
  const maybe = fileStore.req;
  if (!maybe) return "";
  // `req.path` is scoped path (starts with `/`). For share mode we still
  // need the scoped part (relative to share root) which matches
  // `buildRawUrl` concatenation semantics above.
  return maybe.path || "";
};

const loadRawContent = async (): Promise<string> => {
  const target = resolveFilePath();
  if (!target) return "";
  const resp = await fetch(buildRawUrl(target), {
    method: "GET",
    headers: buildFetchHeaders(),
    credentials: "same-origin",
  });
  if (!resp.ok) {
    throw new Error(`HTTP ${resp.status} ${resp.statusText}`);
  }
  return await resp.text();
};
const katexOptions = {
  output: "mathml" as const,
  throwOnError: false,
};
marked.use(markedKatex(katexOptions));

const isSelectionEmpty = ref(true);

const executeEditorCommand = (name: string) => {
  if (name == "paste") {
    read()
      .then((data) => {
        editor.value?.execCommand("paste", {
          text: data,
        });
      })
      .catch((e) => {
        if (
          document.queryCommandSupported &&
          document.queryCommandSupported("paste")
        ) {
          document.execCommand("paste");
        } else {
          console.warn("the clipboard api is not supported", e);
        }
      });
    return;
  }
  if (name == "copy" || name == "cut") {
    const selectedText = editor.value?.getCopyText();
    copy({ text: selectedText });
  }
  editor.value?.execCommand(name);
};

onMounted(() => {
  window.addEventListener("keydown", keyEvent);
  window.addEventListener("beforeunload", handlePageChange);

  watchEffect(async () => {
    if (isMarkdownFile && isPreview.value) {
      const new_value = editor.value?.getValue() || "";
      try {
        previewContent.value = DOMPurify.sanitize(await marked(new_value));
      } catch (error) {
        console.error("Failed to convert content to HTML:", error);
        previewContent.value = "";
      }
    }
  });

  ace.config.set(
    "basePath",
    `https://cdn.jsdelivr.net/npm/ace-builds@${ace_version}/src-min-noconflict/`
  );

  const bootstrap = async () => {
    // Prefer the preloaded metadata content when it's already present
    // (regular /files flow with Expand=true on a single resource). For
    // share viewport entries the metadata came from a directory listing,
    // which omits `content`, so fall back to fetching the raw bytes via
    // the raw download endpoint (public or user-specific, depending on
    // the current view mode).
    let fileContent = fileStore.req?.content ?? "";
    if (!fileContent) {
      try {
        layoutStore.loading = true;
        fileContent = await loadRawContent();
      } catch (e: any) {
        $showError(`Failed to load file content: ${e?.message ?? e}`);
        fileContent = "";
      } finally {
        layoutStore.loading = false;
      }
    }

    const tryInit = () => {
      if (!layoutStore.loading) {
        // loading 由 true 翻转为 false 时，v-else 里的 #editor 表单还没渲染
        // 到 DOM（Vue 的更新在微任务中刷新；watchEffect flush:"pre" 也在
        // DOM 更新之前）。必须等 nextTick 后再初始化 ace，否则
        // "ace.edit can't find div #editor"。
        void nextTick().then(() => initEditor(fileContent));
        return true;
      }
      return false;
    };

    if (!tryInit()) {
      const unwatch = watchEffect(() => {
        if (tryInit()) unwatch();
      });
    }
  };

  bootstrap();
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", keyEvent);
  window.removeEventListener("beforeunload", handlePageChange);
  editor.value?.destroy();
});

onBeforeRouteUpdate((to, from) => {
  if (editor.value?.session.getUndoManager().isClean()) {
    return true;
  }

  return new Promise<boolean>((resolve) => {
    layoutStore.showHover({
      prompt: "discardEditorChanges",
      confirm: (event: Event) => {
        event.preventDefault();
        resolve(true);
      },
      saveAction: async () => {
        await save();
        resolve(true);
      },
    });
  });
});

const initEditor = (fileContent: string) => {
  // 优先模板 ref；组件已卸载 / DOM 未就绪时直接跳过，避免抛错
  const el = editorElRef.value ?? document.getElementById("editor");
  if (!el) return;
  editor.value = ace.edit(el as HTMLFormElement, {
    value: fileContent,
    showPrintMargin: false,
    readOnly: isReadOnly.value,
    theme: getEditorTheme(authStore.user?.aceEditorTheme ?? ""),
    mode: modelist.getModeForPath(fileStore.req!.name).mode,
    wrap: true,
    enableBasicAutocompletion: true,
    enableLiveAutocompletion: true,
    enableSnippets: true,
  });

  editor.value.setFontSize(fontSize.value);
  editor.value.focus();

  const selection = editor.value?.getSelection();
  selection.on("changeSelection", function () {
    isSelectionEmpty.value = selection.isEmpty();
  });
};

const keyEvent = (event: KeyboardEvent) => {
  if (event.code === "Escape") {
    close();
  }

  if (!event.ctrlKey && !event.metaKey) {
    return;
  }

  if (event.key !== "s") {
    return;
  }

  event.preventDefault();
  if (canSave.value) save();
};

const handlePageChange = (event: BeforeUnloadEvent) => {
  if (!editor.value?.session.getUndoManager().isClean()) {
    event.preventDefault();
    // returnValue is now depecrated, though keeping in for legacy browser support
    // https://developer.mozilla.org/en-US/docs/Web/API/BeforeUnloadEvent/returnValue
    event.returnValue = true;
  }
};

const save = async (throwError?: boolean) => {
  if (!canSave.value) {
    return;
  }
  const button = "save";
  buttons.loading("save");

  try {
    await api.put(route.path, editor.value?.getValue());
    editor.value?.session.getUndoManager().markClean();
    // 保存成功后标记目录需要刷新：关闭返回列表时能立即看到新的修改时间/文件大小
    // （Files.vue watch(fileStore.reload) 会在下次进入目录时 fetchData 拉最新数据）
    fileStore.reload = true;
    buttons.success(button);
  } catch (e: any) {
    buttons.done(button);
    $showError(e);
    if (throwError) throw e;
  }
};

const increaseFontSize = () => {
  fontSize.value += 1;
  editor.value?.setFontSize(fontSize.value);
  localStorage.setItem("editorFontSize", fontSize.value.toString());
};

const decreaseFontSize = () => {
  if (fontSize.value > 1) {
    fontSize.value -= 1;
    editor.value?.setFontSize(fontSize.value);
    localStorage.setItem("editorFontSize", fontSize.value.toString());
  }
};

const close = () => {
  if (!editor.value?.session.getUndoManager().isClean()) {
    layoutStore.showHover({
      prompt: "discardEditorChanges",
      confirm: (event: Event) => {
        event.preventDefault();
        editor.value?.session.getUndoManager().reset();
        finishClose();
      },
      saveAction: async () => {
        try {
          await save(true);
          finishClose();
        } catch {}
      },
    });
    return;
  }
  finishClose();
};

const finishClose = () => {
  const uri = url.removeLastDir(route.path) + "/";
  router.push({ path: uri });
};

const preview = () => {
  isPreview.value = !isPreview.value;
};
</script>

<style scoped>
.editor-font-size {
  margin: 0 0.5em;
  color: var(--fg);
}

.editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.editor-header > div > button {
  background: transparent;
  color: var(--action);
  border: none;
  outline: none;
  opacity: 0.8;
  cursor: pointer;
}

.editor-header > div > button:hover:not(:disabled) {
  opacity: 1;
}

.editor-header > div > button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.editor-header > div > button > span > i {
  font-size: 1.2rem;
}
</style>
