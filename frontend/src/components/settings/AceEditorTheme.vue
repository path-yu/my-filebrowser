<template>
  <MacOSSelect
    :id="($attrs.id as string | undefined)"
    :model-value="aceEditorTheme"
    :options="editorThemes"
    @update:model-value="change"
  />
</template>

<script setup lang="ts">
import { computed } from "vue";
// ext-themelist.js 首行调用全局 ace.define —— 必须先导入 ace 核心
// （其尾部副作用会挂载 window.ace）。原先该副作用由 utils/theme.ts 的
// "ace-builds" 静态导入提供；theme.ts 为首屏瘦身移除后，这里需自行导入。
// 本组件仅在设置页（懒加载路由）中使用，ace 归入独立 "ace" chunk 按需加载，
// 不影响首屏体积。
import "ace-builds";
import { themes } from "ace-builds/src-noconflict/ext-themelist";
import MacOSSelect from "../MacOSSelect.vue";

// 拦截父组件传入的 class="input input--block" fallthrough，仅透传 id
defineOptions({ inheritAttrs: false });

defineProps<{
  aceEditorTheme: string;
}>();

const emit = defineEmits<{
  (e: "update:aceEditorTheme", val: string | null): void;
}>();

const editorThemes = computed(() =>
  themes.map((theme) => ({ value: theme.theme, label: theme.name })),
);

const change = (val: unknown) => {
  emit("update:aceEditorTheme", (val as string) ?? null);
};
</script>
