<template>
  <MacOSSelect
    :id="($attrs.id as string | undefined)"
    :model-value="theme"
    :options="themeOptions"
    @update:model-value="change"
  />
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import MacOSSelect from "../MacOSSelect.vue";

// 父组件会传入 class="input input--block"（面向原生 select 的样式），
// 拦截 fallthrough 避免污染 MacOSSelect，仅手动透传 id
defineOptions({ inheritAttrs: false });

const { t } = useI18n();

defineProps<{
  theme: UserTheme;
}>();

const emit = defineEmits<{
  (e: "update:theme", val: string | null): void;
}>();

const themeOptions = computed(() => [
  { value: "", label: t("settings.themes.default") },
  { value: "light", label: t("settings.themes.light") },
  { value: "dark", label: t("settings.themes.dark") },
]);

const change = (val: unknown) => {
  emit("update:theme", (val as string) ?? null);
};
</script>
