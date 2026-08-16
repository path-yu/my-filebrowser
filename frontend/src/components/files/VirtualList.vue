<template>
  <div
    ref="containerRef"
    class="virtual-list"
    :class="{ 'virtual-list--self-scroll': mode === 'self' }"
    :style="containerStyle"
    @scroll="onScroll"
    @contextmenu="onContextMenu"
  >
    <!-- 占位层：撑起虚拟列表总高度（参与父级 / 自身滚动高度） -->
    <div
      class="virtual-list__spacer"
      data-clear-on-click="true"
      :style="{ height: totalHeight + 'px' }"
    >
      <!-- 可视层：translateY 定位到当前窗口起点，只渲染可视区间内的行 -->
      <div
        class="virtual-list__viewport"
        data-clear-on-click="true"
        :style="{ transform: `translateY(${offsetY}px)` }"
      >
        <div
          v-for="(item, i) in visibleItems"
          :key="getKey(item, startIndex + i)"
          class="virtual-list__row"
          :data-virtual-index="startIndex + i"
        >
          <slot :item="item" :index="startIndex + i" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * 虚拟滚动组件（可变行高）
 * - 支持可变行高：首次估算 itemHeight；渲染出可视行后通过 offsetHeight 真实测量并存入缓存；
 *   offsets[] 前缀和 + 二分查找 定位可视区间。行高不一致（如带产品编号 subtitle 的行）也完全准确。
 * - 两种模式：
 *    mode = "self"   (默认)：组件内部自己设 overflow-y，产生独立滚动条
 *    mode = "parent"         : 不接管滚动，依赖"父级滚动容器"的 scrollTop/clientHeight（通过
 *                              outerScrollTop / outerHeight + scrollContainerEl 传入）。
 *                              适合"header / section-title / dirs / files 共享同一个滚动条"场景。
 */

import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";

interface Props {
  items: any[];
  /** 未测量行的估算高度（px） */
  itemHeight?: number;
  /** 可视区上下缓冲行数 */
  buffer?: number;
  getKey?: (item: any, index: number) => string | number;
  /** 容器高度（px 或 CSS 值），仅 self 模式有效 */
  height?: number | string;
  /** self = 自己滚动，parent = 父级滚动驱动 */
  mode?: "self" | "parent";
  /** mode = parent 时必填：外部滚动容器当前 scrollTop */
  outerScrollTop?: number;
  /** mode = parent 时必填：外部滚动容器可视高度（clientHeight） */
  outerHeight?: number;
  /** mode = parent 时推荐：外部滚动容器 DOM 元素（#listing 等）。
   *  不传时将自动向上搜索"最近 overflow-y 不为 visible 的祖先"，可能不准。 */
  scrollContainerEl?: HTMLElement | null;
}

const props = withDefaults(defineProps<Props>(), {
  itemHeight: 48,
  buffer: 6,
  getKey: (_item: any, index: number) => index,
  height: undefined,
  mode: "self",
  outerScrollTop: 0,
  outerHeight: 0,
  scrollContainerEl: undefined,
});

const emit = defineEmits<{
  (e: "scroll", event: Event): void;
  (e: "contextmenu", event: MouseEvent): void;
}>();

const containerRef = ref<HTMLElement | null>(null);

/* ===== 自滚动模式状态 ===== */
const innerScrollTop = ref(0);
const innerHeight = ref(0);

/* ===== 父级驱动模式状态 =====
 * selfOffsetTop：虚拟列表起点相对【滚动容器(#listing)内容区原点】的固定距离
 *   = 滚动容器 scrollTop=0 时，虚拟列表 top 到滚动容器内边距 top 的距离
 *   这个值是常量（DOM 不变时），和当前滚动位置无关
 * measuredOnce：组件挂载 / 数据出现后，至少完成了一次自我定位测量。
 *   在 false 阶段：强行 startIndex=0 避免定位公式未完成时跳过前 N 行（即"上面元素消失"的典型 Bug）
 */
const selfOffsetTop = ref<number | null>(null);
const measuredOnce = ref(false);

/* ========== 行高缓存与偏移量（可变行高核心） ========== */
const measuredHeights: (number | undefined)[] = [];
const heightVersion = ref(0);

const estimateH = () => (props.itemHeight > 0 ? props.itemHeight : 48);

const offsets = computed<number[]>(() => {
  void heightVersion.value;
  const n = props.items.length;
  const arr = new Array<number>(n + 1);
  arr[0] = 0;
  for (let i = 0; i < n; i++) {
    arr[i + 1] = arr[i] + (measuredHeights[i] ?? estimateH());
  }
  return arr;
});

const totalHeight = computed(() => offsets.value[props.items.length] ?? 0);

/* ========== 查找滚动容器（兜底：向上找祖先 overflow-y != visible） ========== */
const isScrollable = (el: HTMLElement): boolean => {
  const s = window.getComputedStyle(el);
  const oy = s.overflowY ?? s.overflow;
  return (oy === "auto" || oy === "scroll") && s.display !== "contents";
};

const resolveScrollContainer = (): HTMLElement | null => {
  if (props.scrollContainerEl instanceof HTMLElement) return props.scrollContainerEl;
  let el = containerRef.value?.parentElement ?? null;
  while (el) {
    if (isScrollable(el)) return el;
    el = el.parentElement;
  }
  return document.scrollingElement instanceof HTMLElement
    ? document.scrollingElement
    : document.body;
};

/* ========== 当前滚动态 & 可视高度 ========== */
const currentScrollTop = computed(() => {
  if (props.mode === "parent") {
    // 【公式推导（关键点）】
    // 任意时刻：
    //   elRect.top - containerRect.top = (VirtualList 在视口 top) - (#listing 在视口 top)
    //     = (#listing 坐标系中 VirtualList 当前的相对 top)
    //     = selfOffsetTop - outerScrollTop
    // 所以：
    //   selfOffsetTop = (elRect.top - containerRect.top) + outerScrollTop
    // 这个值是常量（不随 scroll 变化）。每次 measure 时按上面公式重算即可。
    //
    // 计算"文件区当前已经滚过去多少像素"：
    const base = selfOffsetTop.value ?? 0;
    return Math.max(0, props.outerScrollTop - base);
  }
  return innerScrollTop.value;
});

const currentClientHeight = computed(() => {
  if (props.mode === "parent") {
    // 可视窗口：
    //  - 如果 #listing 顶部目前仍在 VirtualList 上方（还没滚到文件区），则
    //    文件区目前可见高度 = listingHeight - (selfOffsetTop - outerScrollTop)
    const base = selfOffsetTop.value ?? 0;
    const uncovered = Math.max(0, base - props.outerScrollTop);
    return Math.max(0, props.outerHeight - uncovered);
  }
  return innerHeight.value;
});

/* ========== 可视区间 ========== */
const findIndexAt = (pos: number): number => {
  const o = offsets.value;
  let lo = 0;
  let hi = o.length - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (o[mid] <= pos) lo = mid;
    else hi = mid - 1;
  }
  return lo;
};

const startIndex = computed(() => {
  // 未完成首次定位测量：安全 fallback 为 0，保证第一行起一定被渲染（不丢前 N 行）
  if (props.mode === "parent" && !measuredOnce.value) return 0;
  return Math.max(0, findIndexAt(currentScrollTop.value) - props.buffer);
});

const endIndex = computed(() => {
  const o = offsets.value;
  const limit =
    currentScrollTop.value +
    currentClientHeight.value +
    props.buffer * estimateH();
  let i = startIndex.value;
  while (i < props.items.length && o[i] < limit) i++;
  return Math.min(props.items.length, i);
});

const visibleItems = computed(() =>
  props.items.slice(startIndex.value, endIndex.value)
);

const offsetY = computed(() => offsets.value[startIndex.value] ?? 0);

const containerStyle = computed(() => {
  if (props.mode === "self" && props.height !== undefined) {
    return {
      height:
        typeof props.height === "number"
          ? `${props.height}px`
          : (props.height as string),
    };
  }
  return undefined;
});

/* ========== 滚动事件（仅 self 模式用） ========== */
let rafId: number | null = null;
let pendingEvent: Event | null = null;

const applyScroll = () => {
  rafId = null;
  if (props.mode === "self") {
    const el = containerRef.value;
    if (el) {
      innerScrollTop.value = el.scrollTop;
      innerHeight.value = el.clientHeight;
    }
  }
  if (pendingEvent) {
    emit("scroll", pendingEvent);
    pendingEvent = null;
  }
};

const scheduleScroll = (event: Event) => {
  pendingEvent = event;
  if (rafId == null) rafId = requestAnimationFrame(applyScroll);
};

const onScroll = (event: Event) => {
  if (props.mode === "self") scheduleScroll(event);
  else emit("scroll", event);
};

const onContextMenu = (event: MouseEvent) => emit("contextmenu", event);

/* ========== 测量：selfOffsetTop 定位 + 各行真实高度 ========== */
const measureSelfPosition = () => {
  if (props.mode !== "parent") return;
  const el = containerRef.value;
  if (!el) return;
  const container = resolveScrollContainer();
  if (!container) return;

  const elRect = el.getBoundingClientRect();
  const containerRect = container.getBoundingClientRect();

  // 推导：selfOffsetTop 是「滚动容器内容坐标系（scrollTop=0 原点）下
  // VirtualList 顶部的 Y 值」，这是一个不随滚动改变的常量
  //   当前相对 (elRect.top - containerRect.top) = selfOffsetTop - container.scrollTop
  //   => selfOffsetTop = (elRect.top - containerRect.top) + container.scrollTop
  let computedOffset =
    elRect.top - containerRect.top + (container.scrollTop ?? 0);

  // fallback：如果上面算出来为负（VirtualList 在滚动容器外上方），就用 offsetTop
  if (computedOffset < -0.5) {
    // 寻找相对于同一个 scroll container 的累计 offsetTop
    let cur: HTMLElement | null = el;
    let sum = 0;
    const stopAt = container;
    while (cur && cur !== stopAt) {
      sum += cur.offsetTop;
      cur = cur.offsetParent as HTMLElement | null;
    }
    computedOffset = Math.max(0, sum);
  }

  selfOffsetTop.value = Math.max(0, computedOffset);
  measuredOnce.value = true;
};

const measureVisible = () => {
  const el = containerRef.value;
  if (!el) return;
  if (props.mode === "self") innerHeight.value = el.clientHeight;

  let changed = false;
  const rows = el.querySelectorAll<HTMLElement>(".virtual-list__row");
  rows.forEach((row) => {
    const idx = Number(row.dataset.virtualIndex);
    if (Number.isNaN(idx)) return;
    const h = row.offsetHeight;
    if (h > 0 && Math.abs((measuredHeights[idx] ?? 0) - h) > 0.5) {
      measuredHeights[idx] = h;
      changed = true;
    }
  });
  if (changed) heightVersion.value++;
};

const runMeasures = () => {
  measureSelfPosition();
  measureVisible();
};

/* 数据源变化 → 行高缓存失效 + 重新定位（可能前面 dirs 数变了导致 files section 上下移动） */
watch(
  () => props.items,
  () => {
    measuredHeights.length = 0;
    heightVersion.value++;
    measuredOnce.value = false;
    // 下一帧再测量（等 v-if 真实挂载，dirs/section 重排完成）
    requestAnimationFrame(() => {
      requestAnimationFrame(runMeasures);
    });
  },
  { deep: false }
);

/* 可视区间变化后测量新进入窗口的行 */
watch(
  () => [props.items.length, startIndex.value, endIndex.value],
  () => {
    nextTick(() => {
      if (rafId == null) {
        rafId = requestAnimationFrame(() => {
          rafId = null;
          runMeasures();
        });
      } else {
        runMeasures();
      }
    });
  }
);

/* parent 模式：外部 scrollTop / height / scrollContainer 变化时重算位置 */
watch(
  () => [props.mode, props.outerScrollTop, props.outerHeight, props.scrollContainerEl],
  () => {
    if (props.mode === "parent") {
      if (rafId == null) {
        rafId = requestAnimationFrame(() => {
          rafId = null;
          runMeasures();
        });
      }
    }
  }
);

/* ========== 容器尺寸监听 ========== */
let ro: ResizeObserver | null = null;

onMounted(() => {
  // 双 rAF 兜底：等 listing 内 header / section / dirs 等相邻元素布局稳定再测量
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      runMeasures();
    });
  });
  if (containerRef.value && typeof ResizeObserver !== "undefined") {
    ro = new ResizeObserver(() => runMeasures());
    ro.observe(containerRef.value);
  }
});

onBeforeUnmount(() => {
  if (rafId != null) cancelAnimationFrame(rafId);
  ro?.disconnect();
  ro = null;
});

/* ========== 对外 API ========== */
const scrollToIndex = (
  index: number,
  behavior: ScrollBehavior = "auto"
): number | null => {
  const n = props.items.length;
  if (n === 0) return null;
  const clamped = Math.max(0, Math.min(index, n - 1));
  const itemTop = offsets.value[clamped] ?? clamped * estimateH();

  if (props.mode === "parent") {
    // 父级需要 scrollTop = selfOffsetTop + itemTop - 轻微居中
    const base = selfOffsetTop.value ?? 0;
    const desired =
      base + itemTop - Math.max(0, currentClientHeight.value / 4);
    return Math.max(0, desired);
  }

  const el = containerRef.value;
  if (!el) return null;
  el.scrollTo({ top: itemTop, behavior });
  return null;
};

const scrollToTop = () => {
  if (props.mode === "self") containerRef.value?.scrollTo({ top: 0 });
};

defineExpose({ scrollToIndex, scrollToTop });
</script>

<style scoped>
.virtual-list {
  position: relative;
  width: 100%;
}

.virtual-list--self-scroll {
  overflow-y: auto;
  overflow-x: hidden;
  -webkit-overflow-scrolling: touch;
}

.virtual-list__spacer {
  position: relative;
  width: 100%;
}

.virtual-list__viewport {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  will-change: transform;
}
</style>
