<template>
  <div
    class="context-menu"
    ref="contextMenu"
    v-show="show"
    :style="menuStyle"
  >
    <slot />
  </div>
</template>

<script setup lang="ts">
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";

const emit = defineEmits(["hide"]);
interface Pos {
  x: number;
  y: number;
}
const props = defineProps<{ show: boolean; pos: Pos }>();

const contextMenu = ref<HTMLElement | null>(null);
/** 上次测量出的菜单真实尺寸（打开后 nextTick 中测量） */
const measuredSize = ref<{ w: number; h: number }>({ w: 220, h: 120 });
/** 视口安全边距，防止贴边太紧 */
const VIEWPORT_PADDING = 4;

/* ========== 尺寸测量（确保每次打开/菜单项变化时重新测量） ========== */
const measureSize = () => {
  const el = contextMenu.value;
  if (!el) return;
  // offsetWidth/offsetHeight 会触发 reflow，但上下文菜单尺寸很小（<10 项），可忽略
  if (el.offsetWidth > 0) measuredSize.value.w = el.offsetWidth;
  if (el.offsetHeight > 0) measuredSize.value.h = el.offsetHeight;
};

const remeasureNextTick = async () => {
  // 先让 Vue 渲染 slot 内容（菜单项是动态渲染的）
  await nextTick();
  // 下一帧再次确保布局完成（动态字体/主题色可能延迟高度）
  requestAnimationFrame(measureSize);
};

watch(
  () => [props.show, props.pos.x, props.pos.y] as const,
  ([show]) => {
    if (show) remeasureNextTick();
  },
  { flush: "post" }
);

onMounted(() => {
  if (props.show) remeasureNextTick();
});

/* ========== 边界校正：flip + clamp ========== */
const menuStyle = computed<Record<string, string>>(() => {
  const { x, y } = props.pos;
  const { w, h } = measuredSize.value;
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const pad = VIEWPORT_PADDING;

  // --- X 方向：优先放鼠标点右侧，放不下就 flip 到左侧，最后 clamp ---
  let left: number;
  if (x + w + pad <= vw) {
    left = x; // 右侧放得下 → 默认放右边
  } else {
    // flip 到鼠标点左边
    const flipped = x - w;
    left = flipped >= pad ? flipped : pad; // flip 后仍不越左边界；越左则贴边
  }
  // 最后保证不超出右边界（极端情况：视口比菜单还窄，贴左）
  left = Math.min(left, Math.max(pad, vw - w - pad));

  // --- Y 方向：优先放鼠标点下方，放不下就 flip 到上方，最后 clamp ---
  let top: number;
  if (y + h + pad <= vh) {
    top = y; // 下方放得下 → 默认放下面
  } else {
    // flip 到鼠标点上方
    const flipped = y - h;
    top = flipped >= pad ? flipped : pad; // flip 后仍不越上边界；越上则贴顶
  }
  // 最后保证不超出下边界
  top = Math.min(top, Math.max(pad, vh - h - pad));

  return {
    top: `${Math.round(top)}px`,
    left: `${Math.round(left)}px`,
  };
});

/* ========== 点击外部关闭 ========== */
const hideContextMenu = () => emit("hide");

watch(
  () => props.show,
  (val) => {
    if (val) document.addEventListener("click", hideContextMenu);
    else document.removeEventListener("click", hideContextMenu);
  }
);

onBeforeUnmount(() => {
  document.removeEventListener("click", hideContextMenu);
});
</script>
