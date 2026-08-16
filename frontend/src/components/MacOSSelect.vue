<script setup lang="ts">
/**
 * MacOSSelect — macOS 风格下拉选择器（原生 select 封装）
 * 使用原生 <select>（appearance: none + 自定义样式），
 * 保持 macOS bezel 外观与亮/暗色主题。
 * 对外保留原有 props / events API，使用方无需改动。
 */
import { computed, useAttrs, type StyleValue } from "vue";

export interface SelectOption<T = string> {
  value: T;
  label: string;
  icon?: string; // 兼容旧 API，原生 option 不渲染图标
  description?: string;
  group?: string;
  badge?: string;
  badgeColor?: "green" | "amber" | "red" | "purple" | "neutral";
  shortcut?: string;
  disabled?: boolean;
}

const props = withDefaults(
  defineProps<{
    id?: string;
    options: SelectOption[];
    modelValue?: unknown;
    placeholder?: string;
    multiple?: boolean;
    disabled?: boolean;
    size?: "sm" | "md" | "lg";
    accentColor?: "blue" | "green" | "orange" | "red" | "purple";
    variant?: "glass" | "subtle" | "default";
    width?: string;
    ariaLabel?: string;
    className?: string;
  }>(),
  {
    placeholder: "请选择...",
    multiple: false,
    disabled: false,
    size: "md",
    accentColor: "blue",
    variant: "glass",
    width: "",
    ariaLabel: "",
    className: "",
  }
);

const emit = defineEmits<{
  (e: "update:modelValue", value: unknown): void;
  (e: "change", value: unknown): void;
}>();

// class/style 落在根容器，其余透传属性（tabindex、aria-* 等）落在 select 上
const attrs = useAttrs();
const rootClasses = computed(() => [
  `macos-select__accent--${props.accentColor}`,
  props.className,
  attrs.class as string,
]);
const selectAttrs = computed(() => {
  const { class: _c, style: _s, ...rest } = attrs;
  return rest;
});

// 原生 select 的 value 只能是字符串，双向映射回原始 option.value
const innerValue = computed(() =>
  props.modelValue === undefined || props.modelValue === null
    ? ""
    : String(props.modelValue)
);

const findOriginal = (v: string) =>
  props.options.find((o) => String(o.value) === v)?.value;

const onChange = (e: Event) => {
  const el = e.target as HTMLSelectElement;
  if (props.multiple) {
    const values = Array.from(el.selectedOptions)
      .map((o) => findOriginal(o.value))
      .filter((v) => v !== undefined);
    emit("update:modelValue", values);
    emit("change", values);
    return;
  }
  const value = findOriginal(el.value) ?? null;
  emit("update:modelValue", value);
  emit("change", value);
};

// 分组：未分组选项在前，分组选项用 <optgroup>
const grouped = computed(() => {
  const map = new Map<string, SelectOption[]>();
  const ungrouped: SelectOption[] = [];
  props.options.forEach((opt) => {
    if (opt.group) {
      const list = map.get(opt.group) || [];
      list.push(opt);
      map.set(opt.group, list);
    } else {
      ungrouped.push(opt);
    }
  });
  return { groups: Array.from(map.entries()), ungrouped };
});
</script>

<template>
  <div
    class="macos-select"
    :class="rootClasses"
    :style="[width ? { width } : undefined, attrs.style as StyleValue]"
  >
    <select
      :id="id"
      class="macos-select__trigger"
      :class="[
        `macos-select__trigger--${variant}`,
        `macos-select__size--${size}`,
      ]"
      :value="innerValue"
      :disabled="disabled"
      :multiple="multiple"
      :aria-label="ariaLabel || undefined"
      v-bind="selectAttrs"
      @change="onChange"
    >
      <option
        v-if="placeholder && !multiple"
        class="macos-select__placeholder-option"
        value=""
        disabled
        hidden
      >
        {{ placeholder }}
      </option>
      <option
        v-for="o in grouped.ungrouped"
        :key="String(o.value)"
        :value="String(o.value)"
        :disabled="o.disabled"
      >
        {{ o.label }}
      </option>
      <optgroup
        v-for="[groupName, items] in grouped.groups"
        :key="groupName"
        :label="groupName"
      >
        <option
          v-for="o in items"
          :key="String(o.value)"
          :value="String(o.value)"
          :disabled="o.disabled"
        >
          {{ o.label }}
        </option>
      </optgroup>
    </select>
    <i
      v-if="!multiple"
      class="material-icons macos-select__chevrons-icon"
      aria-hidden="true"
      >unfold_more</i
    >
  </div>
</template>

<style scoped>
/* ---------- 根容器与强调色板 ---------- */
.macos-select {
  position: relative;
  display: inline-flex;
  vertical-align: middle;
  max-width: 100%;
  --macos-accent: #007aff;
}

.macos-select__accent--blue   { --macos-accent: #007aff; }
.macos-select__accent--green  { --macos-accent: #34c759; }
.macos-select__accent--orange { --macos-accent: #ff9500; }
.macos-select__accent--red    { --macos-accent: #ff3b30; }
.macos-select__accent--purple { --macos-accent: #af52de; }

/* ---------- 原生 select 触发器（macOS bezel 风格） ---------- */
.macos-select__trigger {
  appearance: none;
  -webkit-appearance: none;
  width: 100%;
  min-width: 0;
  padding: 0 30px 0 10px;
  border: 1px solid rgba(0, 0, 0, 0.14);
  border-radius: 8px;
  background: var(--surfacePrimary, #ffffff);
  color: var(--textPrimary, #1d1d1f);
  font-family: inherit;
  font-size: 13px;
  line-height: 1;
  cursor: default;
  transition:
    background 0.12s ease,
    border-color 0.12s ease,
    box-shadow 0.12s ease;
  -webkit-tap-highlight-color: transparent;
}

/* 尺寸三档 */
.macos-select__trigger.macos-select__size--sm { height: 24px; font-size: 11px; padding: 0 26px 0 8px; border-radius: 6px; }
.macos-select__trigger.macos-select__size--md { height: 28px; font-size: 13px; }
.macos-select__trigger.macos-select__size--lg { height: 36px; font-size: 14px; padding: 0 32px 0 12px; border-radius: 9px; }

/* Variant */
.macos-select__trigger--glass {
  background: color-mix(in srgb, var(--surfacePrimary, #fff) 92%, transparent);
  -webkit-backdrop-filter: blur(8px);
  backdrop-filter: blur(8px);
}

.macos-select__trigger--subtle {
  background: var(--surfaceSecondary, #f2f2f7);
  border-color: transparent;
}

.macos-select__trigger--default {
  background: var(--surfacePrimary, #fff);
}

html.dark .macos-select__trigger {
  border-color: rgba(255, 255, 255, 0.18);
  background: rgba(44, 44, 46, 0.9);
  color: var(--textPrimary, #f2f2f7);
}

html.dark .macos-select__trigger--subtle {
  background: rgba(255, 255, 255, 0.08);
}

.macos-select__trigger:hover:not(:disabled) {
  background: var(--surfaceSecondary, #f2f2f7);
}

html.dark .macos-select__trigger:hover:not(:disabled) {
  background: rgba(58, 58, 60, 0.95);
}

.macos-select__trigger:focus-visible,
.macos-select__trigger:focus {
  outline: none;
  border-color: var(--macos-accent);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--macos-accent) 30%, transparent);
}

.macos-select__trigger:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* 多选：原生列表框形态 */
.macos-select__trigger[multiple] {
  height: auto;
  padding: 4px;
}

/* placeholder 态文字弱化 */
.macos-select__trigger:has(.macos-select__placeholder-option:checked) {
  color: var(--textSecondary, #8e8e93);
}

/* ---------- macOS 上下微箭头 ---------- */
.macos-select__chevrons-icon {
  position: absolute;
  right: 8px;
  top: 50%;
  translate: 0 -50%;
  font-size: 15px;
  line-height: 1;
  color: var(--iconSecondary, #8e8e93);
  opacity: 0.75;
  pointer-events: none;
  transition: opacity 0.15s ease;
}

.macos-select:hover .macos-select__chevrons-icon,
.macos-select__trigger:focus ~ .macos-select__chevrons-icon {
  opacity: 1;
  color: var(--macos-accent);
}
</style>
