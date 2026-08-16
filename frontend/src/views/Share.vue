<template>
  <div>
    <header-bar showMenu showLogo>
      <title />

      <action
        v-if="fileStore.selectedCount"
        icon="file_download"
        :label="t('buttons.download')"
        @action="download"
        :counter="fileStore.selectedCount"
      />
      <button
        v-if="isSingleFile()"
        class="action copy-clipboard"
        :aria-label="t('buttons.copyDownloadLinkToClipboard')"
        :data-title="t('buttons.copyDownloadLinkToClipboard')"
        @click="copyToClipboard(linkSelected())"
      >
        <i class="material-icons">content_paste</i>
      </button>
      <action
        icon="check_circle"
        :label="t('buttons.selectMultiple')"
        @action="toggleMultipleSelection"
      />
    </header-bar>

    <breadcrumbs :base="'/share/' + hash" />

    <div v-if="layoutStore.loading">
      <h2 class="message delayed" style="padding-top: 3em !important">
        <div class="spinner">
          <div class="bounce1"></div>
          <div class="bounce2"></div>
          <div class="bounce3"></div>
        </div>
        <span>{{ t("files.loading") }}</span>
      </h2>
    </div>
    <div v-else-if="error">
      <div v-if="error.status === 401">
        <div class="card floating" id="password" style="z-index: 9999999">
          <div v-if="attemptedPasswordLogin" class="share__wrong__password">
            {{ t("login.wrongCredentials") }}
          </div>
          <div class="card-title">
            <h2>{{ t("login.password") }}</h2>
          </div>

          <div class="card-content">
            <input
              v-focus
              class="input input--block"
              type="password"
              :placeholder="t('login.password')"
              v-model="password"
              @keyup.enter="fetchData"
            />
          </div>
          <div class="card-action">
            <button
              class="button button--flat"
              @click="fetchData"
              :aria-label="t('buttons.submit')"
              :data-title="t('buttons.submit')"
            >
              {{ t("buttons.submit") }}
            </button>
          </div>
        </div>
        <div class="overlay" />
      </div>
      <errors v-else :errorCode="error.status" />
    </div>
    <div v-else-if="req !== null">
      <!--
        分享预览布局：
        - 单文件分享（!req.isDir）：顶部信息卡片（文件元信息+下载+二维码），下方是完整的 Preview 组件（PDF/Word/Excel/图片/音视频预览）
        - 目录分享（req.isDir）：两栏布局，左侧 info + 缩略预览，右侧文件列表；点击文件会路由跳转到 /share/<hash>/<filePath> 进入单文件预览
      -->
      <div class="share share--with-preview">
        <div
          class="share__box share__box__info"
          :style="req.isDir ? shareInfoStickyStyle : ''"
        >
          <div class="share__box__header" style="height: 3em">
            {{
              req.isDir
                ? t("download.downloadFolder")
                : t("download.downloadFile")
            }}
          </div>
          <div
            v-if="!req.isDir"
            class="share__box__element share__box__center share__box__icon share__box__icon--compact"
          >
            <i class="material-icons">{{ icon }}</i>
          </div>
          <div class="share__box__element" style="height: 3em">
            <strong>{{ $t("prompts.displayName") }}</strong> {{ req.name }}
          </div>
          <div v-if="!req.isDir" class="share__box__element" :title="modTime">
            <strong>{{ $t("prompts.lastModified") }}:</strong> {{ humanTime }}
          </div>
          <div class="share__box__element" style="height: 3em">
            <strong>{{ $t("prompts.size") }}:</strong> {{ humanSize }}
          </div>
          <div class="share__box__element share__box__center share__box__actions">
            <a
              target="_blank"
              :href="link"
              class="button button--flat"
              style="height: 4em"
            >
              <div>
                <i class="material-icons">file_download</i
                >{{ t("buttons.download") }}
              </div>
            </a>
            <a
              target="_blank"
              :href="inlineLink"
              class="button button--flat"
              v-if="!req.isDir"
            >
              <div>
                <i class="material-icons">open_in_new</i
                >{{ t("buttons.openFile") }}
              </div>
            </a>
            <qrcode-vue
              v-if="req.isDir"
              :value="link"
              :size="100"
              level="M"
            ></qrcode-vue>
          </div>
          <div v-if="!req.isDir" class="share__box__element share__box__center share__qrcode--compact">
            <qrcode-vue :value="link" :size="120" level="M"></qrcode-vue>
          </div>
          <!-- 目录分享才保留旧的侧边栏 12em 缩略预览 -->
          <template v-if="req.isDir">
            <div
              class="share__box__element share__box__header"
              style="height: 3em"
            >
              {{ $t("sidebar.preview") }}
            </div>
            <div
              class="share__box__element share__box__center share__box__icon"
              style="padding: 0em !important; height: 12em !important"
            >
              <a
                target="_blank"
                :href="raw"
                class="button button--flat"
                v-if="
                  !fileStore.multiple &&
                  fileStore.selectedCount === 1 &&
                  req.items[fileStore.selected[0]].type === 'image'
                "
                style="height: 12em; padding: 0; margin: 0"
              >
                <LazyImage eager :src="raw" style="height: 12em" />
              </a>
              <div
                v-else-if="
                  fileStore.multiple &&
                  fileStore.selectedCount === 1 &&
                  req.items[fileStore.selected[0]].type === 'audio'
                "
                style="height: 12em; padding-top: 1em; margin: 0"
              >
                <button
                  @click="play"
                  v-if="!tag"
                  style="
                    font-size: 6em !important;
                    border: 0px;
                    outline: none;
                    background: white;
                  "
                  class="material-icons"
                >
                  play_circle_filled
                </button>
                <button
                  @click="play"
                  v-if="tag"
                  style="
                    font-size: 6em !important;
                    border: 0px;
                    outline: none;
                    background: white;
                  "
                  class="material-icons"
                >
                  pause_circle_filled
                </button>
                <audio
                  id="myaudio"
                  ref="audio"
                  :src="raw"
                  controls
                  :autoplay="tag"
                ></audio>
              </div>
              <video
                v-else-if="
                  !fileStore.multiple &&
                  fileStore.selectedCount === 1 &&
                  req.items[fileStore.selected[0]].type === 'video'
                "
                style="height: 12em; padding: 0; margin: 0"
                :src="raw"
                controls
              >
                Sorry, your browser doesn't support embedded videos, but don't
                worry, you can <a :href="raw">download it</a>
                and watch it with your favorite video player!
              </video>
              <i
                v-else-if="
                  !fileStore.multiple &&
                  fileStore.selectedCount === 1 &&
                  req.items[fileStore.selected[0]].isDir
                "
                class="material-icons"
                >folder
              </i>
              <i v-else class="material-icons">call_to_action</i>
            </div>
          </template>
        </div>

        <!-- 单文件分享：右侧全宽显示 Preview 组件（PDF/Word/图片/音视频/Epub/CSV） -->
        <component
          v-if="currentView && !req.isDir"
          :is="currentView"
          class="share__preview-slot"
        ></component>

        <!-- 目录分享：右侧文件列表（点击文件跳转进入单文件预览） -->
        <div
          id="shareList"
          v-else-if="req.isDir && req.items.length > 0"
          class="share__box share__box__items"
        >
          <div class="share__box__header" v-if="req.isDir">
            {{ t("files.files") }}
          </div>
          <div id="listing" class="list file-icons">
            <item
              v-for="item in req.items.slice(0, showLimit)"
              :key="base64(item.name)"
              v-bind:index="item.index"
              v-bind:name="item.name"
              v-bind:isDir="item.isDir"
              v-bind:url="item.url"
              v-bind:modified="item.modified"
              v-bind:type="item.type"
              v-bind:size="item.size"
              readOnly
            >
            </item>
            <div
              v-if="req.items.length > showLimit"
              class="item"
              @click="showLimit += 100"
            >
              <div>
                <p class="name">+ {{ req.items.length - showLimit }}</p>
              </div>
            </div>

            <div
              :class="{ active: fileStore.multiple }"
              id="multiple-selection"
            >
              <p>{{ t("files.multipleSelectionEnabled") }}</p>
              <div
                @click="() => (fileStore.multiple = false)"
                tabindex="0"
                role="button"
                :data-title="t('buttons.clear')"
                :aria-label="t('buttons.clear')"
                class="action"
              >
                <i class="material-icons">clear</i>
              </div>
            </div>
          </div>
        </div>
        <div
          v-else-if="req.isDir && req.items.length === 0"
          class="share__box share__box__items"
        >
          <h2 class="message">
            <i class="material-icons">sentiment_dissatisfied</i>
            <span>{{ t("files.lonely") }}</span>
          </h2>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { pub as api } from "@/api";
import { filesize } from "@/utils";
import dayjs from "dayjs";
import { Base64 } from "js-base64";
import { createURL } from "@/api/utils";
import HeaderBar from "@/components/header/HeaderBar.vue";
import Action from "@/components/header/Action.vue";
import Breadcrumbs from "@/components/Breadcrumbs.vue";
import Errors from "@/views/Errors.vue";
import QrcodeVue from "qrcode.vue";
import Item from "@/components/files/ListingItem.vue";
import LazyImage from "@/components/files/LazyImage.vue";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";
import {
  computed,
  defineAsyncComponent,
  inject,
  onMounted,
  onBeforeUnmount,
  ref,
  watch,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { StatusError } from "@/api/utils";
import { copy } from "@/utils/clipboard";

// 与 Files.vue 一致：异步加载预览/编辑器，减小首屏包体积
const Editor = defineAsyncComponent(() => import("@/views/files/Editor.vue"));
const Preview = defineAsyncComponent(() => import("@/views/files/Preview.vue"));

// 目录分享时 info 卡片 sticky 定位（与旧行为一致）
const shareInfoStickyStyle = {
  position: "sticky",
  top: "-20.6em",
  zIndex: 999,
} as const;

// 参考 Files.vue 的视图路由：单文件时决定渲染 Preview / Editor，目录时为空
const currentView = computed(() => {
  if (!req.value) return null;
  if (req.value.isDir) return null;
  if (req.value.extension.toLowerCase() === ".csv") {
    // CSV 默认预览表格视图，?edit=true 时走 Editor
    if (route.query.edit === "true") return Editor;
    return Preview;
  }
  if (req.value.type === "text" || req.value.type === "textImmutable") {
    return Editor;
  }
  return Preview;
});

const error = ref<StatusError | null>(null);
const showLimit = ref<number>(100);
const password = ref<string>("");
const attemptedPasswordLogin = ref<boolean>(false);
const hash = ref<string>("");
const token = ref<string>("");
const audio = ref<HTMLAudioElement>();
const tag = ref<boolean>(false);

const $showError = inject<IToastError>("$showError")!;
const $showSuccess = inject<IToastSuccess>("$showSuccess")!;

const { t } = useI18n({});

const route = useRoute();
const router = useRouter();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

watch(route, () => {
  showLimit.value = 100;
  fetchData();
});

const req = computed(() => fileStore.req);

// 目录分享时：单击选中单个非目录文件 → 自动导航到单文件预览 URL，
// 与双击 <item> 的 open() 行为一致。这样 <component :is="currentView">
// （preview/editor）才会有机会渲染，否则 req.isDir === true 时预览组件
// 被 v-else-if="req.isDir" 互斥分支挡住，永远显示不出来。
watch(
  () => [fileStore.selected, fileStore.selectedCount, req.value] as const,
  () => {
    if (!req.value?.isDir) return; // 非目录分享场景（单文件分享）无需处理
    if (fileStore.selectedCount !== 1) return;
    const idx = fileStore.selected[0];
    const item = req.value.items?.[idx];
    if (!item || item.isDir) return; // 目录条目：交给双击 open() 导航到子目录
    if (typeof item.url !== "string" || !item.url) return;
    if (route.fullPath === item.url || route.path === item.url) return; // 已在目标路径，避免循环
    router.push({ path: item.url });
  }
);

// Define computes

const icon = computed(() => {
  if (req.value === null) return "insert_drive_file";
  if (req.value.isDir) return "folder";
  if (req.value.type === "image") return "insert_photo";
  if (req.value.type === "audio") return "volume_up";
  if (req.value.type === "video") return "movie";
  return "insert_drive_file";
});

const link = computed(() => (req.value ? api.getDownloadURL(req.value) : ""));
const raw = computed(() => {
  if (!req.value || !req.value.items[fileStore.selected[0]]) return "";
  return createURL(
    `api/public/dl/${hash.value}${req.value.items[fileStore.selected[0]].path}`,
    { token: token.value }
  );
});
const inlineLink = computed(() =>
  req.value ? api.getDownloadURL(req.value, true) : ""
);
const humanSize = computed(() => {
  if (req.value) {
    return req.value.isDir
      ? req.value.items.length
      : filesize(req.value.size ?? 0);
  } else {
    return "";
  }
});
const humanTime = computed(() => dayjs(req.value?.modified).fromNow());
const modTime = computed(() =>
  req.value
    ? new Date(Date.parse(req.value.modified)).toLocaleString()
    : new Date().toLocaleString()
);

// Functions
const base64 = (name: any) => Base64.encodeURI(name);
const play = () => {
  if (tag.value) {
    audio.value?.pause();
    tag.value = false;
  } else {
    audio.value?.play();
    tag.value = true;
  }
};
const fetchData = async () => {
  fileStore.reload = false;
  fileStore.selected = [];
  fileStore.multiple = false;
  layoutStore.closeHovers();

  // Set loading to true and reset the error.
  layoutStore.loading = true;
  error.value = null;
  if (password.value !== "") {
    attemptedPasswordLogin.value = true;
  }

  let url = route.path;
  if (url === "") url = "/";
  if (url[0] !== "/") url = "/" + url;

  try {
    const file = await api.fetch(url, password.value);
    file.hash = hash.value;

    token.value = file.token || "";

    fileStore.updateRequest(file);
    document.title = `${file.name} - ${document.title}`;
  } catch (err) {
    if (err instanceof Error) {
      error.value = err;
    }
  } finally {
    layoutStore.loading = false;
  }
};

const keyEvent = (event: KeyboardEvent) => {
  if (event.key === "Escape") {
    // If we're on a listing, unselect all
    // files and folders.
    if (fileStore.selectedCount > 0) {
      fileStore.selected = [];
    }
  }
};

const toggleMultipleSelection = () => {
  fileStore.toggleMultiple();
};

const isSingleFile = () =>
  fileStore.selectedCount === 1 &&
  !req.value?.items[fileStore.selected[0]].isDir;

const download = () => {
  if (!req.value) return false;

  if (isSingleFile()) {
    api.download(
      null,
      hash.value,
      token.value,
      req.value.items[fileStore.selected[0]].path
    );
    return true;
  }

  layoutStore.showHover({
    prompt: "download",
    confirm: (format: DownloadFormat) => {
      if (req.value === null) return false;
      layoutStore.closeHovers();

      const files: string[] = [];

      for (const i of fileStore.selected) {
        files.push(req.value.items[i].path);
      }

      api.download(format, hash.value, token.value, ...files);
      return true;
    },
  });

  return true;
};

const linkSelected = () => {
  return isSingleFile() && req.value
    ? api.getDownloadURL({
        ...req.value,
        hash: hash.value,
        path: req.value.items[fileStore.selected[0]].path,
      })
    : "";
};

const copyToClipboard = (text: string) => {
  copy({ text }).then(
    () => {
      // clipboard successfully set
      $showSuccess(t("success.linkCopied"));
    },
    () => {
      // clipboard write failed
      copy({ text }, { permission: true }).then(
        () => {
          // clipboard successfully set
          $showSuccess(t("success.linkCopied"));
        },
        (e) => {
          // clipboard write failed
          $showError(e);
        }
      );
    }
  );
};

onMounted(async () => {
  // Created
  hash.value = route.params.path[0];
  window.addEventListener("keydown", keyEvent);
  await fetchData();
});

onBeforeUnmount(() => {
  // Destroyed
  window.removeEventListener("keydown", keyEvent);
});
</script>

<style scoped>
#listing.list {
  height: auto;
}

#shareList {
  overflow-y: scroll;
}

/* 单文件分享：左侧信息卡片紧凑化 + 右侧 Preview 占满主区域 */
.share.share--with-preview {
  display: flex;
  flex-direction: row;
  align-items: flex-start;
  gap: 16px;
}
.share.share--with-preview > .share__box__info {
  /* 目录两栏时保持原最大宽度；单文件时更紧凑，给 Preview 更大空间 */
  flex: 0 0 320px;
  max-width: 360px;
  width: 320px;
  min-width: 0;
  border-radius: 12px;
  overflow: hidden;
  background: var(--card-bg);
  border: 1px solid var(--card-border);
}
.share.share--with-preview .share__preview-slot {
  flex: 1 1 auto;
  min-width: 0;
  min-height: calc(100vh - 9.8em);
  max-height: calc(100vh - 9.8em);
  overflow: auto;
  border-radius: 12px;
  border: 1px solid var(--card-border);
  background: var(--card-bg);
}
/* 单文件 info 卡片：图标/二维码更紧凑，避免挤压 Preview */
.share__box__icon--compact {
  padding: 0.8em 0 !important;
  height: auto !important;
}
.share__box__icon--compact .material-icons {
  font-size: 3em !important;
}
.share__qrcode--compact {
  padding: 0.6em 0 !important;
}
.share__box__actions {
  gap: 8px;
  padding: 0.4em 0.8em !important;
  flex-wrap: wrap;
}
.share__box__actions .button--flat {
  border-radius: 980px;
}

@media (max-width: 930px) {
  .share.share--with-preview {
    flex-direction: column;
    gap: 12px;
  }
  .share.share--with-preview > .share__box__info {
    flex: none;
    width: 100%;
    max-width: 100%;
    position: static !important;
  }
  .share.share--with-preview .share__preview-slot {
    min-height: 60vh;
    max-height: 70vh;
    width: 100%;
  }
}

@media (min-width: 930px) {
  #shareList {
    height: calc(100vh - 9.8em);
    overflow-y: auto;
  }
}
</style>
