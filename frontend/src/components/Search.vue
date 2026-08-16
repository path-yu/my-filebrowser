<template>
  <!-- macOS Finder 风格：内联搜索框 + 下拉结果浮层
       不再使用旧版 layoutStore.showHover('search') 弹出的全屏 overlay。
       用户点一下就直接聚焦输入，有内容/结果时就地展开下拉浮层。 -->
  <div
    id="search"
    v-bind:class="{ ongoing, open: showDropdown, focused: inputFocused }"
  >
    <div id="input" :class="{ focused: inputFocused }" @click.stop="focusInput">
      <i class="material-icons search-icon">search</i>
      <input
        type="text"
        style="line-height: 2"
        @focus="
          inputFocused = true;
          maybeOpenDropdown();
        "
        @blur="onInputBlur"
        @input="onInput"
        @keyup.enter="submit"
        @keydown="keydown"
        ref="input"
        v-model.trim="prompt"
        :aria-label="$t('search.search')"
        :placeholder="$t('search.search')"
      />
      <!-- 清除 × 按钮（macOS Finder 风格） -->
      <button
        v-show="prompt.length > 0"
        type="button"
        class="clear-btn"
        @mousedown.prevent.stop="clearPrompt"
        @click.stop.prevent="clearPrompt"
        :aria-label="$t('buttons.clear')"
        :title="$t('buttons.clear')"
      >
        <i class="material-icons">cancel</i>
      </button>
      <!-- <i
        v-show="ongoing"
        class="material-icons spin"
        style="display: inline-block"
        >autorenew
      </i> -->
      <span
        class="count-badge"
        v-if="
          (results.length > 0 || fileStore.searchResults.length > 0) &&
          prompt.length > 0
        "
      >
        {{
          fileStore.searchMode ? fileStore.searchResults.length : results.length
        }}
      </span>
    </div>

    <!-- 内联下拉结果浮层（替代旧的全屏 overlay） -->
    <transition name="search-dropdown">
      <div id="result" ref="result" v-show="showDropdown" @mousedown.prevent>
        <div>
          <!-- Finder 工具条：范围分段 + 结果计数 -->
          <div v-if="prompt.length > 0" class="finder-toolbar">
            <div
              class="finder-segmented"
              role="tablist"
              :aria-label="$t('search.scope')"
            >
              <button
                type="button"
                role="tab"
                :class="{ active: scope === 'current' }"
                @mousedown.prevent.stop
                @click.stop.prevent="setScope('current')"
                :aria-pressed="scope === 'current'"
              >
                {{ $t("search.scopeCurrent") }}
              </button>
              <button
                type="button"
                role="tab"
                :class="{ active: scope === 'all' }"
                @mousedown.prevent.stop
                @click.stop.prevent="setScope('all')"
                :aria-pressed="scope === 'all'"
              >
                {{ $t("search.scopeAll") }}
              </button>
            </div>
            <div v-if="ongoing" class="finder-count searching">
              {{ $t("search.searching") }}
            </div>
            <div v-else class="finder-count">
              {{ $t("search.foundCount", { count: results.length }) }}
            </div>
          </div>

          <template v-if="isEmpty">
            <p class="result-tip">{{ text }}</p>

            <template v-if="prompt.length === 0">
              <div class="boxes">
                <h3>{{ $t("search.types") }}</h3>
                <div>
                  <div
                    tabindex="0"
                    v-for="(v, k) in boxes"
                    :key="k"
                    role="button"
                    @click.stop.prevent="init('type:' + k)"
                    :aria-label="$t('search.' + v.label)"
                  >
                    <i class="material-icons">{{ v.icon }}</i>
                    <p>{{ $t("search." + v.label) }}</p>
                  </div>
                </div>
              </div>
            </template>
          </template>
          <ul v-show="results.length > 0">
            <li v-for="(s, k) in filteredResults" :key="k">
              <router-link @click.stop.prevent="goResult(s)" :to="s.url">
                <i v-if="s.dir" class="material-icons">folder</i>
                <i v-else class="material-icons">insert_drive_file</i>
                <span>
                  <template
                    v-for="(seg, i) in splitByKeyword(s.path, prompt)"
                    :key="`${k}-${i}-${seg.text}`"
                  >
                    <span v-if="seg.match" class="search-match">{{
                      seg.text
                    }}</span>
                    <span v-else>{{ seg.text }}</span>
                  </template>
                </span>
              </router-link>
            </li>
          </ul>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { useFileStore } from "@/stores/file";

import url from "@/utils/url";
import { search } from "@/api";
import { splitByKeyword } from "@/utils/highlight";
import { resolveTimeFilter, timeFilter } from "@/composables/timeFilter";
import {
  computed,
  inject,
  onMounted,
  onBeforeUnmount,
  ref,
  watch,
  nextTick,
} from "vue";
import { useI18n } from "vue-i18n";
import { useRoute, useRouter } from "vue-router";
import { StatusError } from "@/api/utils";

const boxes = {
  image: { label: "images", icon: "insert_photo" },
  audio: { label: "music", icon: "volume_up" },
  video: { label: "video", icon: "movie" },
  pdf: { label: "pdf", icon: "picture_as_pdf" },
};

type SearchScope = "current" | "all";

const fileStore = useFileStore();
let searchAbortController = new AbortController();

const prompt = ref<string>("");
const ongoing = ref<boolean>(false);
const inputFocused = ref<boolean>(false);
const results = ref<any[]>([]);
const resultsCount = ref<number>(50);
const scope = ref<SearchScope>("current");
// 是否展开下拉结果浮层（替代旧版全屏 overlay 的 active）
const showDropdown = ref<boolean>(false);

const $showError = inject<IToastError>("$showError")!;

const input = ref<HTMLInputElement | null>(null);
const result = ref<HTMLElement | null>(null);

const { t } = useI18n();
const route = useRoute();
const router = useRouter();

// debounce：macOS Finder 风格，输入停顿 250ms 后自动搜索
let debounceTimer: number | null = null;
const DEBOUNCE_MS = 250;

// scope 变化自动重搜
watch(scope, () => {
  if (prompt.value.trim().length > 0) {
    debouncedSubmit();
  }
});

// 时间筛选变化时：若正在搜索则重新跑，让搜索结果也遵守时间限制
watch(timeFilter, () => {
  if (prompt.value.trim().length > 0) {
    debouncedSubmit();
  }
});

/** 路由切换（进入子目录 / 返回上一级）时，立即中止本组件侧的搜索任务与防抖等待。
 *  FileListing.vue 那边会触发 fileStore.clearSearch() 退出搜索模式，
 *  但这里的 debounceTimer 若未清理，会在 250ms 后"延迟补发"一次搜索请求，
 *  表现为：切目录后列表"先正常 → 闪一下变回搜索结果"。（经验 1385816） */
watch(
  () => route.path,
  (newPath, oldPath) => {
    if (newPath !== oldPath) {
      if (debounceTimer) {
        window.clearTimeout(debounceTimer);
        debounceTimer = null;
      }
      abortLastSearch();
      ongoing.value = false;
    }
  }
);

// 当关键字/结果变化时，自动展开浮层（便于用户看到搜索结果）
watch(
  [prompt, results, ongoing],
  () => {
    if (inputFocused.value) {
      maybeOpenDropdown();
    }
  },
  { flush: "post" }
);

const isEmpty = computed(() => results.value.length === 0);
const text = computed(() => {
  if (ongoing.value) return t("search.searching");
  return prompt.value === "" ? t("search.typeToSearch") : t("search.noMatches");
});
const filteredResults = computed(() =>
  results.value.slice(0, resultsCount.value)
);

onMounted(() => {
  if (result.value) {
    result.value.addEventListener("scroll", (event: Event) => {
      const tgt = event.target as HTMLElement;
      if (tgt.offsetHeight + tgt.scrollTop >= tgt.scrollHeight - 100) {
        resultsCount.value += 50;
      }
    });
  }
  document.addEventListener("click", onDocClick, true);
  document.addEventListener("keydown", onGlobalKeydown, true);
});

const onDocClick = (event: MouseEvent) => {
  // 点击到外面（既不在 #search 里，也不在结果浮层里）→ 收起浮层
  const el = document.getElementById("search");
  if (!el) return;
  const target = event.target as Node;
  if (!el.contains(target)) {
    closeDropdown();
  }
};

onBeforeUnmount(() => {
  document.removeEventListener("click", onDocClick, true);
  document.removeEventListener("keydown", onGlobalKeydown, true);
  abortLastSearch();
  if (debounceTimer) {
    window.clearTimeout(debounceTimer);
    debounceTimer = null;
  }
});

/** 点击搜索框/放大镜 → 只做聚焦，不弹全屏 overlay */
const focusInput = () => {
  nextTick(() => input.value?.focus());
  maybeOpenDropdown();
};

/** 输入框失焦：先判断点击是否落到浮层上（链接、分段控件、清除按钮等）
 *  是的话就继续保持浮层，否则延迟收起（让链接能先响应 click） */
const onInputBlur = () => {
  inputFocused.value = false;
  // 如果浮层里有结果或快捷入口，允许用户继续操作，下一次 doc 点击外再收起
};

/** 有内容或类型快捷入口时才展开浮层 */
const maybeOpenDropdown = () => {
  showDropdown.value = true;
};

const closeDropdown = () => {
  showDropdown.value = false;
};

/** 键盘 Esc：两段式（对齐 Spotlight） */
const keydown = (event: KeyboardEvent) => {
  if (event.key === "Escape") {
    event.preventDefault();
    event.stopPropagation();
    const hasContent =
      prompt.value.length > 0 || ongoing.value || results.value.length > 0;
    if (hasContent) {
      clearPrompt();
    } else {
      closeDropdown();
      input.value?.blur();
    }
  }
};

const onGlobalKeydown = (event: KeyboardEvent) => {
  if (event.key !== "Escape") return;
  if (!showDropdown.value && !inputFocused.value) return;
  if (event.defaultPrevented) return;
  event.preventDefault();
  event.stopPropagation();
  const hasContent =
    prompt.value.length > 0 || ongoing.value || results.value.length > 0;
  if (hasContent) {
    clearPrompt();
  } else {
    closeDropdown();
    input.value?.blur();
  }
};

const onInput = () => {
  debouncedSubmit();
};

const init = (s: string) => {
  prompt.value = `${s} `;
  nextTick(() => input.value?.focus());
  debouncedSubmit();
};

const clearPrompt = () => {
  prompt.value = "";
  abortLastSearch();
  ongoing.value = false;
  results.value = [];
  resultsCount.value = 50;
  fileStore.clearSearch();
  if (debounceTimer) {
    window.clearTimeout(debounceTimer);
    debounceTimer = null;
  }
  nextTick(() => input.value?.focus());
};

const abortLastSearch = () => {
  try {
    searchAbortController.abort();
  } catch {
    /* ignore */
  }
};

/** debounce 自动搜索 */
const debouncedSubmit = () => {
  if (debounceTimer) {
    window.clearTimeout(debounceTimer);
    debounceTimer = null;
  }
  const q = prompt.value.trim();
  if (!q) {
    results.value = [];
    ongoing.value = false;
    fileStore.clearSearch();
    return;
  }
  debounceTimer = window.setTimeout(() => {
    debounceTimer = null;
    doSearch();
  }, DEBOUNCE_MS);
};

const setScope = (s: SearchScope) => {
  if (scope.value !== s) scope.value = s;
  nextTick(() => input.value?.focus());
};

const doSearch = async () => {
  const q = prompt.value.trim();
  if (!q) {
    results.value = [];
    fileStore.clearSearch();
    return;
  }

  let path: string;
  if (scope.value === "all") {
    path = "/";
  } else {
    path = route.path;
    if (!fileStore.isListing) {
      path = url.removeLastDir(path) + "/";
    }
  }

  // ① 先中止上一次还在进行的搜索（否则旧结果晚回来可能覆盖新结果）
  abortLastSearch();
  searchAbortController = new AbortController();

  // ② 标记"正在搜索"（显示 spinner 与「搜索中」文案）
  // ❗ 注意：此处**故意不再做**「results.value = []」和「fileStore.setSearchResults(q, [])」。
  //    之前的实现在请求开始时先把旧结果清空 → 主列表区域整片先白一下 →
  //    流式搜索回调一条一条 push 回来，列表从空一行一行"逐行显现" → 用户感知为"文件列表闪烁"，
  //    同时浮层 results.length 在 0/N 之间抖动导致浮层高度反复开合 → 输入框被挤得轻微抖动闪烁。
  //    修复策略：搜索过程中保持旧结果显示（视觉稳定），把新命中暂存在本地临时数组，
  //    等流式搜索全部收齐后「一次性原子替换」 results / searchResults（经验 1385816：只触发一次状态变更 = 只重渲染一次，零闪烁）。
  ongoing.value = true;

  const batchItems: any[] = [];

  try {
    const r = resolveTimeFilter(timeFilter.value);
    const extra = {
      modified_after: r.modifiedAfter,
      modified_before: r.modifiedBefore,
    };
    await search(
      path,
      prompt.value,
      searchAbortController.signal,
      (item) => {
        // 仅累积到本地，不写响应式 store，不在中途触发视图更新
        batchItems.push(item);
      },
      extra
    );

    // 「一次性原子替换」：流式搜索完成，把全部命中一次性写入。
    // 避免多次 set / push 造成逐帧重绘（闪烁 / 打字机效果）。
    results.value = batchItems;
    fileStore.setSearchResults(q, batchItems as unknown as ResourceItem[]);
  } catch (error: any) {
    if (error instanceof StatusError && error.is_canceled) {
      // 被 AbortController 取消（用户继续输入 / 切目录），保留旧结果即可
      return;
    }
    $showError(error);
  } finally {
    ongoing.value = false;
  }
};

const submit = async (event?: Event) => {
  if (event) event.preventDefault();
  if (debounceTimer) {
    window.clearTimeout(debounceTimer);
    debounceTimer = null;
  }
  return doSearch();
};

/** 用户点结果链接 → 先收浮层再跳转（保持 Finder 利落感） */
const goResult = (s: any) => {
  closeDropdown();
  router.push(s.url);
};
</script>
