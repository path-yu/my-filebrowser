<template>
  <!-- macOS Finder 风格：内联搜索框 + 下拉结果浮层
       支持「单选/单关键词」和「多选/多标签 OR 聚合」两种模式。
       多选模式下：输入一个词回车 -> 生成标签，输入下一个回车再新增标签，
       标签可单独点击编辑（改完回车保存），每个标签可以×单独删除。
       「单/多选」切换按钮直接放在搜索框同一行的右侧，不再占独立一行。-->
  <div
    id="search"
    v-bind:class="{ ongoing, open: showDropdown, focused: inputFocused }"
  >
    <!-- 输入框 + 单/多选切换按钮 + 关键词标签（三个元素同一行从左到右排列）
         顺序：搜索框 | 模式切换 | 关键词标签  （标签行和模式切换都在 #input 容器外面，都是兄弟节点，并排）
         用户要求："关键词列表也应该在切换菜单最右边" —— 即标签不要占搜索框上面的独立行，
         而是和模式切换放到同一行，紧贴其右侧，所有控件挤在一行。-->
    <div class="search-control-row">
      <div id="input" :class="{ focused: inputFocused, multi: multiMode }" @click.stop="focusInput">
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
          :placeholder="multiMode ? $t('search.multiPlaceholder') : $t('search.search')"
        />
        <!-- 清除 × 按钮（macOS Finder 风格） -->
        <button
          v-show="hasAnythingToClear"
          type="button"
          class="clear-btn"
          @mousedown.prevent.stop="clearPrompt"
          @click.stop.prevent="clearPrompt"
          :aria-label="$t('buttons.clear')"
          :title="$t('buttons.clear')"
        >
          <i class="material-icons">cancel</i>
        </button>
        <span
          class="count-badge"
          v-if="
            (results.length > 0 || fileStore.searchResults.length > 0) &&
            hasAnythingToClear
          "
        >
          {{
            fileStore.searchMode ? fileStore.searchResults.length : results.length
          }}
        </span>
      </div>
      <!-- ⚠ 单/多选模式切换（Finder segmented 风格）—— 在 #input 容器外面，和搜索框并排。-->
      <div class="finder-segmented finder-segmented--mode" role="tablist" :aria-label="$t('search.modeLabel')">
        <button
          type="button"
          role="tab"
          :class="{ active: !multiMode }"
          @mousedown.prevent.stop
          @click.stop.prevent="setMode('single')"
          :aria-pressed="!multiMode"
        >
          <i class="material-icons">search</i>
          <span>{{ $t("search.modeSingle") }}</span>
        </button>
        <button
          type="button"
          role="tab"
          :class="{ active: multiMode }"
          @mousedown.prevent.stop
          @click.stop.prevent="setMode('multi')"
          :aria-pressed="multiMode"
        >
          <i class="material-icons">label</i>
          <span>{{ $t("search.modeMulti") }}</span>
          <span v-if="tags.length > 0" class="mode-badge">{{ tags.length }}</span>
        </button>
      </div>
      <!-- ⚠ 多选模式：关键词标签列表——用户要求放到「切换菜单最右边」，所以和模式切换同一个 flex 行，
           放在分段控件右侧（紧挨着），不再占搜索框上面的独立一行。标签过多时横向滚动。
           v-if 条件保留：只有多选模式下且（已有标签 / 聚焦 / 正在输入）才显示空容器占位也可以 -->
      <ul
        v-if="multiMode && tags.length > 0"
        class="search-tags"
        aria-label="keywords"
      >
        <li
          v-for="(t, i) in tags"
          :key="i"
          class="search-tag"
          :class="{
            editing: editingTagIdx === i,
            searching: perKeywordProgress[i]?.finished === false,
          }"
        >
          <!-- 编辑态：把 tag 文本放回输入框里，但视觉上仍保留该 tag 高亮 -->
          <button
            v-if="editingTagIdx !== i"
            type="button"
            class="search-tag__text"
            @mousedown.prevent.stop
            @click.stop.prevent="startEditTag(i)"
            :title="$t('search.editTagTitle')"
          >
            <span v-if="perKeywordProgress[i]" class="search-tag__count">
              {{ perKeywordProgress[i].finished ? perKeywordProgress[i].count : `${perKeywordProgress[i].partialCount ?? 0}…` }}
            </span>
            <span class="search-tag__label">{{ t }}</span>
            <span v-if="perKeywordProgress[i]?.finished === false" class="search-tag__spinner">
              <i class="material-icons spin">autorenew</i>
            </span>
          </button>
          <span v-else class="search-tag__text search-tag__text--editing" aria-label="editing">
            <i class="material-icons" style="font-size: 14px">edit</i>
            <span class="search-tag__label">{{ t }}</span>
          </span>
          <button
            type="button"
            class="search-tag__remove"
            @mousedown.prevent.stop
            @click.stop.prevent="removeTag(i)"
            :aria-label="$t('buttons.delete') + ': ' + t"
            :title="$t('buttons.delete')"
          >
            <i class="material-icons">close</i>
          </button>
        </li>
      </ul>
      <!-- 上传 PDF 检索相似图纸：放在搜索框同一行的最右侧，图标按钮 -->
      <button
        type="button"
        class="upload-pdf-btn"
        :title="'上传 PDF / 图片 搜索相似图纸'"
        :aria-label="'上传 PDF / 图片 搜索相似图纸'"
        @mousedown.prevent.stop
        @click.stop.prevent="openSimilarPdfPrompt"
      >
        <i class="material-icons">perm_media</i>
      </button>
    </div>

    <!-- 内联下拉结果浮层（替代旧的全屏 overlay） -->
    <transition name="search-dropdown">
      <div id="result" ref="result" v-show="showDropdown" @mousedown.prevent>
        <div>
          <!-- Finder 工具条：范围分段 + 结果计数 + 多模式时的关键词状态 -->
          <div v-if="hasToolbar" class="finder-toolbar">
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
            <div class="finder-toolbar-right">
              <span v-if="ongoing" class="finder-count searching">
                {{
                  multiMode
                    ? $t("search.multiSearching", {
                        done: multiDoneCount,
                        total: tags.length,
                      })
                    : $t("search.searching")
                }}
              </span>
              <span v-else class="finder-count">
                {{ $t("search.foundCount", { count: results.length }) }}
              </span>
            </div>
          </div>

          <template v-if="isEmpty">
            <p class="result-tip">{{ text }}</p>

            <template v-if="prompt.length === 0 && (!multiMode || tags.length === 0)">
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
                    v-for="(seg, i) in highlightPathByKeywords(s.path)"
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
import { useLayoutStore } from "@/stores/layout";

import url from "@/utils/url";
import { search, searchMulti } from "@/api";
import { StatusError } from "@/api/utils";
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

const boxes = {
  image: { label: "images", icon: "insert_photo" },
  audio: { label: "music", icon: "volume_up" },
  video: { label: "video", icon: "movie" },
  pdf: { label: "pdf", icon: "picture_as_pdf" },
};

type SearchScope = "current" | "all";

const fileStore = useFileStore();
const layoutStore = useLayoutStore();
let searchAbortController = new AbortController();

// ------ 单/多选模式 + 标签状态 ------
const MODE_STORAGE_KEY = "fb.search.mode";
const TAGS_SEP = "|"; // URL query 里区分多关键词的分隔符（避免空格/逗号和用户输入冲突）

const multiMode = ref<boolean>(false);
const tags = ref<string[]>([]);
const editingTagIdx = ref<number | null>(null);
/** 每个关键词实时进度（正在搜多少条、是否完成） */
const perKeywordProgress = ref<
  { keyword: string; count: number; partialCount: number; finished: boolean }[]
>([]);

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

const hasAnythingToClear = computed(
  () =>
    prompt.value.length > 0 ||
    tags.value.length > 0 ||
    ongoing.value ||
    results.value.length > 0
);
const hasToolbar = computed(
  () =>
    prompt.value.length > 0 ||
    (multiMode.value && tags.value.length > 0) ||
    ongoing.value
);
const multiDoneCount = computed(() =>
  perKeywordProgress.value.filter((p) => p.finished).length
);

// --------- 工具：高亮路径（支持多关键词）---------
/** 把 path 里命中任意关键词的片段用 <search-match> 高亮 */
function highlightPathByKeywords(pathText: string): { text: string; match: boolean }[] {
  const kws = multiMode.value
    ? tags.value.filter((t) => t.length > 0)
    : [prompt.value.trim()].filter(Boolean);
  if (kws.length === 0) return [{ text: pathText, match: false }];

  // 为避免正则转义冲突，使用多关键词逐段扫描：每次找到最近的匹配位置
  const results_: { text: string; match: boolean }[] = [];
  let pos = 0;
  const lower = pathText.toLowerCase();
  const lowerKws = kws.map((k) => k.toLowerCase()).filter(Boolean);
  while (pos < pathText.length) {
    // 找第一个命中关键词的起始位置（最小索引）
    let hitIdx = -1;
    let hitLen = 0;
    for (const kw of lowerKws) {
      if (kw.length === 0) continue;
      const i = lower.indexOf(kw, pos);
      if (i === -1) continue;
      if (hitIdx === -1 || i < hitIdx) {
        hitIdx = i;
        hitLen = kw.length;
      }
    }
    if (hitIdx === -1) break;
    // 非命中前缀
    if (hitIdx > pos) {
      results_.push({ text: pathText.slice(pos, hitIdx), match: false });
    }
    // 命中段
    results_.push({ text: pathText.slice(hitIdx, hitIdx + hitLen), match: true });
    pos = hitIdx + hitLen;
  }
  if (pos < pathText.length) {
    results_.push({ text: pathText.slice(pos), match: false });
  }
  if (results_.length === 0) return [{ text: pathText, match: false }];
  return results_;
}

/** 把搜索模式 + 标签/关键字写入 URL query（刷新后自动回填）。
 *  - 单选：?q=foo&scope=current
 *  - 多选：?q=tag1|tag2|tag3&sm=1&scope=all
 *  空值会把对应参数从 URL 上移除。*/
const persistQueryInUrl = (s: SearchScope) => {
  try {
    const next = { ...route.query } as Record<string, any>;
    delete next.q;
    delete next.scope;
    delete next.sm;

    if (multiMode.value) {
      if (tags.value.length > 0) {
        next.q = tags.value.join(TAGS_SEP);
        next.sm = "1";
      }
    } else {
      const trimmed = prompt.value.trim();
      if (trimmed) next.q = trimmed;
    }
    if (s !== "current") next.scope = s;

    router.replace({ path: route.path, query: next }).catch(() => {
      /* ignore NavigationDuplicated / aborted navigations */
    });
  } catch {
    /* ignore */
  }
};

// debounce：macOS Finder 风格，输入停顿 250ms 后自动搜索
let debounceTimer: number | null = null;
const DEBOUNCE_MS = 250;

// scope 变化自动重搜
watch(scope, () => {
  if (hasAnythingToSearch()) {
    debouncedSubmit();
  }
});

// 时间筛选变化时：若正在搜索则重新跑，让搜索结果也遵守时间限制
watch(timeFilter, () => {
  if (hasAnythingToSearch()) {
    debouncedSubmit();
  }
});

/** 路由切换（进入子目录 / 返回上一级）时，立即中止搜索任务与防抖 */
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

// ---- 监听 URL 中的 q / sm / scope：刷新 / 分享链接 / 手动改地址栏 时回填 ----
watch(
  () => [route.query.q, route.query.scope, route.query.sm] as const,
  ([newQ, newScope, newSm], [oldQ, oldScope, oldSm]) => {
    if (newQ === oldQ && newScope === oldScope && newSm === oldSm) return;

    const sc = typeof newScope === "string" ? newScope : "current";
    const needChangeScope =
      (sc === "current" || sc === "all") && sc !== scope.value;
    if (needChangeScope) scope.value = sc as SearchScope;

    // 用和 onMounted 完全一致的规则解析 wantMulti / tagsArr / singleStr
    const parsed = parseQueryForMode(newSm, newQ);
    // wantMulti 为 null 时：不强制改模式，沿用当前 UI 已选择的（不改写 localStorage 偏好）
    const forceMulti = parsed.wantMulti;
    const wantMulti: boolean =
      forceMulti === null ? multiMode.value : forceMulti;
    const tagsArr: string[] = parsed.wantMulti ? parsed.tagsArr : [];
    const singleStr: string = parsed.wantMulti ? "" : parsed.singleStr;

    if (forceMulti !== null && forceMulti !== multiMode.value) {
      multiMode.value = forceMulti;
      saveModePref();
    }
    // 空 q 但 sm key 存在强制多模式：仍切多模式、不自动搜、直接 return
    if (
      wantMulti &&
      tagsArr.length === 0 &&
      (typeof newSm === "string" || (typeof newQ === "string" && newQ.includes(TAGS_SEP)))
    ) {
      tags.value = [];
      if (!multiMode.value) {
        multiMode.value = true;
        saveModePref();
      }
      editingTagIdx.value = null;
      return;
    }

    if (wantMulti) {
      // 标签完全一致则跳过，避免自己写入后触发死循环
      const sameAsTags =
        tags.value.length === tagsArr.length &&
        tags.value.every((v, i) => v === tagsArr[i]);
      if (!sameAsTags) {
        tags.value = tagsArr;
        editingTagIdx.value = null;
      }
    } else {
      if (prompt.value.trim() !== singleStr) prompt.value = singleStr;
    }

    const hasQuery = wantMulti ? tagsArr.length > 0 : singleStr.length > 0;
    if (hasQuery) {
      nextTick(() => {
        maybeOpenDropdown();
        // URL 来的稳定值：跳过 debounce 直接搜
        doSearch();
      });
    } else if (!wantMulti && !singleStr) {
      abortLastSearch();
      ongoing.value = false;
      results.value = [];
      resultsCount.value = 50;
      fileStore.clearSearch();
    }
  },
  { flush: "post" }
);

// 当关键字/结果变化时，自动展开浮层
watch(
  [prompt, results, ongoing, multiMode, () => tags.value.length],
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
  if (!hasAnythingToSearch()) return t("search.typeToSearch");
  return t("search.noMatches");
});
const filteredResults = computed(() =>
  results.value.slice(0, resultsCount.value)
);

function hasAnythingToSearch(): boolean {
  if (multiMode.value) return tags.value.length > 0;
  return prompt.value.trim().length > 0;
}

// --------- 模式切换 + 标签操作 ---------
function saveModePref() {
  try {
    localStorage.setItem(MODE_STORAGE_KEY, multiMode.value ? "multi" : "single");
  } catch {
    /* private mode etc. */
  }
}

function setMode(m: "single" | "multi") {
  const nowMulti = m === "multi";
  if (nowMulti === multiMode.value) return;
  multiMode.value = nowMulti;
  saveModePref();
  editingTagIdx.value = null;

  // 切换模式时：把当前已有内容同步一下，避免输入丢失
  if (nowMulti) {
    // 单选 -> 多选：如果 prompt 非空，拆成初始标签
    const t = prompt.value.trim();
    if (t) {
      // 先尝试按常见分隔符（逗号、全角逗号、空格、分号）拆，用户也可以单个编辑
      const initial = splitToKeywords(t).filter(Boolean);
      tags.value = Array.from(new Set(initial));
      prompt.value = "";
      if (tags.value.length > 0) {
        persistQueryInUrl(scope.value);
        debouncedSubmit();
      }
    }
  } else {
    // 多选 -> 单选：如果有标签，把第一个标签回填到 prompt（或者用空格拼接全部显示在输入框）
    if (tags.value.length > 0) {
      prompt.value = tags.value.join(" ");
    }
    tags.value = [];
    persistQueryInUrl(scope.value);
    debouncedSubmit();
  }

  nextTick(() => input.value?.focus());
}

/** 把用户输入的一段字符串拆成多个关键词（用于单 -> 多自动转换 / 粘贴多词场景）
 *  分隔符：逗号 ,  全角逗号 ， 分号 ;  全角分号 ； 换行 \n / \r / \t  连续空白 */
function splitToKeywords(input: string): string[] {
  return input
    .split(/[,，;；\n\r\t]+|\s{2,}/)
    .map((s) => s.trim())
    .filter(Boolean);
}

/** 统一解析 URL 中的 sm/q → 判定是否多选模式 + 拆出关键词数组 / 单关键词字符串
 *  规则（按优先级）：
 *   1) sm="0" / sm="false" → 强制单选
 *   2) sm key 存在（哪怕是 `sm=` 空串，或者 sm=1 / sm=true / sm=any）→ 多选
 *   3) sm key 完全缺失，但 q 本身含 TAGS_SEP(|) → 识别为多选 URL 编码形式，按多词拆
 *   4) 其余 → wantMulti=null 由调用方决定是否沿用 localStorage 偏好
 *  返回：{ wantMulti: boolean | null; tagsArr: string[]; singleStr: string }
 *
 *  ⚠ 放在模块级（非 onMounted 内部），因为 watch route.query 也要引用它。*/
function parseQueryForMode(
  rawSm: unknown,
  rawQ: unknown
): { wantMulti: boolean | null; tagsArr: string[]; singleStr: string } {
  const smVal = typeof rawSm === "string" ? rawSm : undefined;
  const qStr = typeof rawQ === "string" ? rawQ : "";

  // 1) 显式单模式
  if (smVal === "0" || smVal === "false") {
    return { wantMulti: false, tagsArr: [], singleStr: qStr.trim() };
  }
  // 2) sm key 存在（含空串） → 多选
  if (typeof smVal !== "undefined") {
    const tagsArr = Array.from(
      new Set(qStr.split(TAGS_SEP).map((x) => x.trim()).filter(Boolean))
    );
    return { wantMulti: true, tagsArr, singleStr: "" };
  }
  // 3) 完全没 sm，但 q 内含 TAGS_SEP(|) → 自动按多词 URL 识别
  if (qStr.includes(TAGS_SEP)) {
    const tagsArr = Array.from(
      new Set(qStr.split(TAGS_SEP).map((x) => x.trim()).filter(Boolean))
    );
    return { wantMulti: true, tagsArr, singleStr: "" };
  }
  // 4) 其余：返回 null 让调用方自己决定（沿用 localStorage 偏好等）
  return { wantMulti: null, tagsArr: [], singleStr: qStr.trim() };
}

function startEditTag(idx: number) {
  editingTagIdx.value = idx;
  prompt.value = tags.value[idx] ?? "";
  nextTick(() => {
    input.value?.focus();
    // 光标移到末尾
    const el = input.value;
    if (el && typeof el.setSelectionRange === "function") {
      const len = el.value.length;
      try { el.setSelectionRange(len, len); } catch { /* ignore */ }
    }
  });
}

function removeTag(idx: number) {
  tags.value.splice(idx, 1);
  perKeywordProgress.value.splice(idx, 1);
  if (editingTagIdx.value === idx) {
    editingTagIdx.value = null;
    prompt.value = "";
  } else if (editingTagIdx.value !== null && editingTagIdx.value > idx) {
    editingTagIdx.value -= 1;
  }
  persistQueryInUrl(scope.value);
  if (!hasAnythingToSearch()) {
    abortLastSearch();
    ongoing.value = false;
    results.value = [];
    fileStore.clearSearch();
    resultsCount.value = 50;
    return;
  }
  debouncedSubmit();
}

/** 回车（@keyup.enter）时：
 *  - 多选模式下：若正在编辑某个 tag → 保存编辑；否则若 prompt 非空 → 新增 tag */
function submitOrAddTag() {
  const raw = prompt.value.trim();
  if (!multiMode.value) {
    // 单选模式：照旧
    return doSearch();
  }
  // 多选模式
  if (editingTagIdx.value !== null) {
    // 保存编辑：如果输入为空 -> 视为删除
    const idx = editingTagIdx.value;
    if (!raw) {
      removeTag(idx);
      return;
    }
    // 如果新内容和原 tag 一样，什么都不做
    if (tags.value[idx] === raw) {
      editingTagIdx.value = null;
      prompt.value = "";
      return;
    }
    // 去重：如果已存在同名 tag，删掉当前（等价于合并）
    const existingIdx = tags.value.indexOf(raw);
    tags.value[idx] = raw;
    // 双去重（如果已经有重复，把后面重复的去掉）
    const seen = new Set<string>();
    const newTags: string[] = [];
    const newProg: typeof perKeywordProgress.value = [];
    for (let i = 0; i < tags.value.length; i++) {
      const t = tags.value[i];
      if (seen.has(t)) continue;
      seen.add(t);
      newTags.push(t);
      newProg.push(perKeywordProgress.value[i] ?? { keyword: t, count: 0, partialCount: 0, finished: true });
    }
    tags.value = newTags;
    perKeywordProgress.value = newProg;
    // 如果 editingTagIdx 因为去重被挤没了，置 null
    if (editingTagIdx.value >= tags.value.length) editingTagIdx.value = null;
    const finalEditingIdx = editingTagIdx.value;
    // 编辑完成后，清除 editing 状态并清空 prompt
    editingTagIdx.value = null;
    prompt.value = "";
    void existingIdx; void finalEditingIdx;
    persistQueryInUrl(scope.value);
    debouncedSubmit();
    return;
  }
  // 新增 tag
  if (!raw) return;
  // 按分隔符拆：支持用户一次粘贴多个词（"CQG20，ZKG2 80"）一起导入
  const toAdd = splitToKeywords(raw).filter(Boolean);
  if (toAdd.length === 0) return;
  const seen = new Set(tags.value);
  for (const kw of toAdd) {
    if (!seen.has(kw)) {
      seen.add(kw);
      tags.value.push(kw);
      perKeywordProgress.value.push({ keyword: kw, count: 0, partialCount: 0, finished: true });
    }
  }
  prompt.value = "";
  persistQueryInUrl(scope.value);
  debouncedSubmit();
}

// --------- 生命周期 ---------
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

  // 恢复模式偏好
  try {
    const savedMode = localStorage.getItem(MODE_STORAGE_KEY);
    if (savedMode === "multi") {
      multiMode.value = true;
    }
  } catch { /* ignore */ }

  // 从 URL query params 读取上一次的搜索条件
  const savedQ = route.query.q;
  const savedScope = route.query.scope;
  const savedSm = route.query.sm;
  if (typeof savedScope === "string" && (savedScope === "current" || savedScope === "all")) {
    scope.value = savedScope;
  }
  const parsed = parseQueryForMode(savedSm, savedQ);
  if (parsed.wantMulti === true) {
    multiMode.value = true;
    saveModePref();
    tags.value = parsed.tagsArr;
    if (tags.value.length > 0) {
      nextTick(() => {
        maybeOpenDropdown();
        doSearch();
      });
      return;
    }
  } else if (parsed.wantMulti === false) {
    multiMode.value = false;
    saveModePref();
    if (parsed.singleStr.length > 0) {
      prompt.value = parsed.singleStr;
      nextTick(() => {
        maybeOpenDropdown();
        doSearch();
      });
      return;
    }
  }
  // wantMulti === null：sm / q 没给强信号，按 localStorage 偏好 + q 字面量回退
  if (parsed.singleStr.length > 0) {
    if (multiMode.value) {
      tags.value = Array.from(new Set(splitToKeywords(parsed.singleStr)));
    } else {
      prompt.value = parsed.singleStr;
    }
    nextTick(() => {
      maybeOpenDropdown();
      doSearch();
    });
  }
});

const onDocClick = (event: MouseEvent) => {
  const el = document.getElementById("search");
  if (!el) return;
  const target = event.target as Node;
  if (!el.contains(target)) {
    closeDropdown();
    // 失焦时也取消编辑态，避免下次再点输入框还带着旧的编辑值
    if (editingTagIdx.value !== null) {
      // 如果 prompt 有改 -> 保存；否则只取消编辑
      const idx = editingTagIdx.value;
      const raw = prompt.value.trim();
      if (raw && tags.value[idx] !== raw) {
        tags.value[idx] = raw;
        persistQueryInUrl(scope.value);
        debouncedSubmit();
      }
      editingTagIdx.value = null;
      prompt.value = "";
    }
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

/** 点击搜索框 → 聚焦并展开浮层 */
const focusInput = () => {
  nextTick(() => input.value?.focus());
  maybeOpenDropdown();
};

const onInputBlur = () => {
  inputFocused.value = false;
};

const maybeOpenDropdown = () => {
  showDropdown.value = true;
};

const closeDropdown = () => {
  showDropdown.value = false;
};

/** 键盘 Esc / Backspace（多选且 prompt 空、没在编辑 -> 把最后一个 tag 回拉到编辑） */
const keydown = (event: KeyboardEvent) => {
  if (event.key === "Escape") {
    event.preventDefault();
    event.stopPropagation();
    if (hasAnythingToClear) {
      clearPrompt();
    } else {
      closeDropdown();
      input.value?.blur();
    }
    return;
  }
  // 多选模式下 Backspace：若输入框为空且有 tags，把最后一个 tag 拉回编辑
  if (multiMode.value && event.key === "Backspace" && !event.defaultPrevented) {
    if (prompt.value.length === 0 && editingTagIdx.value === null && tags.value.length > 0) {
      event.preventDefault();
      startEditTag(tags.value.length - 1);
    }
  }
  // 多选模式下：逗号 / 分号（中英）也直接触发"新增标签"，用户不用非要按回车
  if (multiMode.value && editingTagIdx.value === null && !event.ctrlKey && !event.metaKey) {
    const keys: Record<string, boolean> = {
      ",": true,
      "，": true,
      ";": true,
      "；": true,
    };
    if (keys[event.key] && prompt.value.trim().length > 0) {
      event.preventDefault();
      submitOrAddTag();
    }
  }
};

const onGlobalKeydown = (event: KeyboardEvent) => {
  if (event.key !== "Escape") return;
  if (!showDropdown.value && !inputFocused.value) return;
  if (event.defaultPrevented) return;
  event.preventDefault();
  event.stopPropagation();
  if (hasAnythingToClear) {
    clearPrompt();
  } else {
    closeDropdown();
    input.value?.blur();
  }
};

const onInput = () => {
  if (multiMode.value && editingTagIdx.value !== null) {
    // 编辑某个 tag 时：不触发 debounced 搜索（等回车保存时再搜）
    return;
  }
  debouncedSubmit();
};

const init = (s: string) => {
  prompt.value = `${s} `;
  nextTick(() => input.value?.focus());
  debouncedSubmit();
};

const clearPrompt = () => {
  prompt.value = "";
  tags.value = [];
  perKeywordProgress.value = [];
  editingTagIdx.value = null;
  abortLastSearch();
  ongoing.value = false;
  results.value = [];
  resultsCount.value = 50;
  fileStore.clearSearch();
  if (debounceTimer) {
    window.clearTimeout(debounceTimer);
    debounceTimer = null;
  }
  persistQueryInUrl(scope.value);
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
  if (!hasAnythingToSearch()) {
    results.value = [];
    ongoing.value = false;
    fileStore.clearSearch();
    persistQueryInUrl(scope.value);
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

// ------ 核心搜索入口 ------
const doSearch = async () => {
  if (multiMode.value) {
    return doMultiSearch();
  }
  return doSingleSearch();
};

const doSingleSearch = async () => {
  const q = prompt.value.trim();
  if (!q) {
    results.value = [];
    fileStore.clearSearch();
    persistQueryInUrl(scope.value);
    return;
  }
  persistQueryInUrl(scope.value);

  let path: string;
  if (scope.value === "all") {
    path = "/";
  } else {
    path = route.path;
    if (!fileStore.isListing) {
      path = url.removeLastDir(path) + "/";
    }
  }

  abortLastSearch();
  searchAbortController = new AbortController();
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
        batchItems.push(item);
      },
      extra
    );
    results.value = batchItems;
    fileStore.setSearchResults(q, batchItems as unknown as ResourceItem[]);
  } catch (error: any) {
    if (error instanceof StatusError && error.is_canceled) return;
    $showError(error);
  } finally {
    ongoing.value = false;
  }
};

const doMultiSearch = async () => {
  // 先去重（startEditTag / removeTag 里也做过，但这里再做一次以防御并发）
  const uniq: string[] = [];
  const seen_ = new Set<string>();
  for (const t of tags.value) {
    const tt = t.trim();
    if (!tt || seen_.has(tt)) continue;
    seen_.add(tt);
    uniq.push(tt);
  }
  if (!seen_.size) {
    results.value = [];
    fileStore.clearSearch();
    persistQueryInUrl(scope.value);
    return;
  }
  tags.value = uniq;
  perKeywordProgress.value = uniq.map((kw) => ({
    keyword: kw,
    count: 0,
    partialCount: 0,
    finished: false,
  }));
  persistQueryInUrl(scope.value);

  let path: string;
  if (scope.value === "all") {
    path = "/";
  } else {
    path = route.path;
    if (!fileStore.isListing) {
      path = url.removeLastDir(path) + "/";
    }
  }

  abortLastSearch();
  searchAbortController = new AbortController();
  ongoing.value = true;

  const batchItems: any[] = [];
  try {
    const r = resolveTimeFilter(timeFilter.value);
    const extra = {
      modified_after: r.modifiedAfter,
      modified_before: r.modifiedBefore,
    };
    await searchMulti(
      path,
      uniq,
      searchAbortController.signal,
      (item) => {
        batchItems.push(item);
      },
      extra,
      (prog) => {
        const p = perKeywordProgress.value[prog.queryIdx];
        if (!p) return;
        p.partialCount = prog.partialCount;
        if (prog.finished) {
          p.finished = true;
        }
      }
    );
    // 每个关键词最终计数（成功的）：从结果集再数一遍最准
    // 但 searchMulti 返回值里 perKeyword 已经给我们准备好了，这里同步显示
    // （searchMulti 的 perKeyword 计数 = 去重后“真正贡献给最终集合”的条目数，所以最贴合用户预期）
    // 注意：这里我们直接用 batchItems 来渲染，
    // perKeyword 的 count 就用 searchMulti 返回结果去回填。
    results.value = batchItems;
    fileStore.setSearchResults(uniq, batchItems as unknown as ResourceItem[]);
  } catch (error: any) {
    if (error instanceof StatusError && error.is_canceled) return;
    $showError(error);
  } finally {
    ongoing.value = false;
    perKeywordProgress.value = perKeywordProgress.value.map((p) => ({ ...p, finished: true }));
  }
};

const submit = async (event?: Event) => {
  if (event) event.preventDefault();
  if (debounceTimer) {
    window.clearTimeout(debounceTimer);
    debounceTimer = null;
  }
  return submitOrAddTag();
};

/** 用户点结果链接 → 先收浮层再跳转 */
const goResult = (s: any) => {
  closeDropdown();
  router.push(s.url);
};

/** 点击右上角"上传 PDF / 图片"图标 → 打开 SimilarPdf.vue 弹窗（共享全局 prompt 系统） */
const openSimilarPdfPrompt = () => {
  // 用户要求："点击上传弹窗提示用户上传文件，支持拖拽上传"
  // 用 layoutStore 弹出 SimilarPdf 组件，组件自己负责拖拽 + 点击选择 + 调用后端接口
  layoutStore.showHover({
    prompt: "similarPdf",
    confirm: null,
    action: undefined,
    saveAction: undefined,
    props: null,
    close: null,
  });
};
</script>

<style scoped>
/* ---- 搜索控件 + 模式切换 + 关键词标签：三兄弟同一行横向排列 ----
   优先把空间让给输入框：#input (flex 2.5) | finder-segmented (flex:none) | search-tags (flex 1 自动收缩) */
.search-control-row {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: flex-start;
  gap: 10px;
  width: 100%;
  flex-wrap: nowrap;
  /* 单行，不换行，标签多时允许横向 scroll，避免把行高撑大 */
  min-height: 32px;
}
/* 搜索框占大头（自适应缩小放大），输入框最小 320px，保证用户输入时看得清 */
.search-control-row #input {
  flex: 2.5 1 320px;
  min-width: 320px;
  max-width: 100%;
}
.finder-segmented--mode {
  flex: none;
  display: inline-flex;
  background: var(--surface-raised, rgba(0, 0, 0, 0.05));
  border: 1px solid var(--border, rgba(0, 0, 0, 0.1));
  border-radius: 8px;
  padding: 2px;
  gap: 2px;
  align-self: center;
}
.finder-segmented--mode button {
  all: unset;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 12px;
  color: var(--theme-text, #333);
  transition: background 0.15s ease;
}
.finder-segmented--mode button .material-icons {
  font-size: 16px;
  opacity: 0.8;
}
.finder-segmented--mode button:hover {
  background: var(--surface-hover, rgba(0, 0, 0, 0.06));
}
.finder-segmented--mode button.active {
  background: var(--theme-color, #1976d2);
  color: #fff;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.15);
}
.finder-segmented--mode button.active .material-icons {
  opacity: 1;
}
.mode-badge {
  display: inline-block;
  min-width: 18px;
  padding: 0 6px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.25);
  color: inherit;
  font-size: 11px;
  line-height: 16px;
  text-align: center;
  margin-left: 2px;
}

/* ---- 标签行（现在和搜索框/模式同一行，在「切换菜单最右边」）---- */
.search-tags {
  list-style: none;
  padding: 0;
  margin: 0;
  /* 不允许换行，单行显示；标签多时横向滚动 */
  display: flex;
  flex-direction: row;
  flex-wrap: nowrap;
  align-items: center;
  gap: 6px;
  /* 标签占用不超过行宽的 30%，优先把空间留给输入框（用户核心诉求）*/
  flex: 1 1 30%;
  max-width: 30%;
  min-width: 0;
  overflow-x: auto;
  overflow-y: hidden;
  /* 细滚动条 */
  scrollbar-width: thin;
  scrollbar-color: rgba(0, 0, 0, 0.2) transparent;
}
.search-tags::-webkit-scrollbar {
  height: 6px;
}
.search-tags::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.15);
  border-radius: 3px;
}
/* 多选刚切进来 / 没聚焦 / 没输入 / 没标签时：完全隐藏，不占用空间 */
.search-tags.is-empty {
  display: none;
}
.search-tag {
  display: inline-flex;
  align-items: center;
  background: var(--theme-color, #1976d2);
  color: #fff;
  border-radius: 999px;
  padding: 1px 4px 1px 10px;
  font-size: 12px;
  line-height: 22px;
  max-width: 200px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.15);
  transition: background 0.12s ease, transform 0.08s ease;
  /* 防止标签本身被横向滚动挤扁（作为滚动容器内部 item，不 shrink）*/
  flex: 0 0 auto;
}
.search-tag:hover {
  background: var(--theme-color-dark, #1565c0);
}
.search-tag.editing {
  background: var(--theme-color-dark, #1565c0);
  outline: 2px dashed rgba(255, 255, 255, 0.5);
  outline-offset: -2px;
}
.search-tag.searching {
  background: linear-gradient(
    135deg,
    var(--theme-color, #1976d2),
    var(--theme-color-light, #42a5f5)
  );
}
.search-tag__text {
  all: unset;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 220px;
}
.search-tag__count {
  display: inline-block;
  min-width: 18px;
  padding: 0 5px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.22);
  font-size: 11px;
  line-height: 16px;
  text-align: center;
  flex: none;
}
.search-tag__label {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 150px;
}
.search-tag__spinner {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
}
.search-tag__spinner .material-icons {
  font-size: 14px;
  animation: spin 900ms linear infinite;
}
.search-tag__remove {
  all: unset;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  margin-left: 4px;
  color: #fff;
  opacity: 0.8;
}
.search-tag__remove:hover {
  background: rgba(255, 255, 255, 0.22);
  opacity: 1;
}
.search-tag__remove .material-icons {
  font-size: 15px;
}

/* ---- 输入框 ---- */
#input.multi {
  /* 多选模式下输入框更扁，给标签留空间 */
  min-height: 32px;
}
.finder-toolbar-right {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

/* ---------- 搜索框最右侧：上传 PDF 图标按钮 ----------
   位置：关键词标签右侧 → 即搜索框控制行的最末端（用户要求）
   外观：Finder style 圆形胶囊图标按钮，hover 变蓝色 */
.upload-pdf-btn {
  all: unset;
  flex: none;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 32px;
  border-radius: 8px;
  background: var(--surface-raised, rgba(0, 0, 0, 0.04));
  border: 1px solid var(--border, rgba(0, 0, 0, 0.1));
  color: #b71c1c;
  box-sizing: border-box;
  transition: background 0.15s ease, color 0.15s ease, transform 0.08s ease, box-shadow 0.15s ease;
  align-self: center;
  margin-left: 4px;
}
.upload-pdf-btn .material-icons {
  font-size: 20px;
}
.upload-pdf-btn:hover {
  background: rgba(183, 28, 28, 0.1);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.14);
}
.upload-pdf-btn:active {
  transform: translateY(1px);
}
/* 多选模式下标签多时：上传按钮保持最右端，不参与横向 scroll（在容器外）*/
</style>
