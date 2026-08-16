<template>
  <div class="breadcrumbs">
    <component
      :is="element"
      :to="base || ''"
      :aria-label="t('files.home')"
      :title="t('files.home')"
    >
      <i class="material-icons">home</i>
    </component>

    <span v-for="(link, index) in items" :key="index">
      <span class="chevron"
        ><i class="material-icons">keyboard_arrow_right</i></span
      >
      <component :is="element" :to="link.url">{{ link.name }}</component>
    </span>

    <!-- 产品编号编辑按钮（最右侧）：单选 PDF 或正在预览 PDF 时可用 -->
    <button
      v-if="productCodeAvailable"
      class="breadcrumbs-product-code"
      type="button"
      :aria-label="t('buttons.productCode')"
      :title="t('buttons.productCode')"
      @click.stop="openProductCode"
    >
      <i class="material-icons">sell</i>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRoute } from "vue-router";
import { useAuthStore } from "@/stores/auth";
import { useFileStore } from "@/stores/file";
import { useLayoutStore } from "@/stores/layout";

const { t } = useI18n();

const route = useRoute();
const authStore = useAuthStore();
const fileStore = useFileStore();
const layoutStore = useLayoutStore();

const props = defineProps<{
  base: string;
  noLink?: boolean;
}>();

const items = computed(() => {
  const relativePath = route.path.replace(props.base, "");
  const parts = relativePath.split("/");

  if (parts[0] === "") {
    parts.shift();
  }

  if (parts[parts.length - 1] === "") {
    parts.pop();
  }

  const breadcrumbs: BreadCrumb[] = [];

  for (let i = 0; i < parts.length; i++) {
    if (i === 0) {
      breadcrumbs.push({
        name: decodeURIComponent(parts[i]),
        url: props.base + "/" + parts[i] + "/",
      });
    } else {
      breadcrumbs.push({
        name: decodeURIComponent(parts[i]),
        url: breadcrumbs[i - 1].url + parts[i] + "/",
      });
    }
  }

  if (breadcrumbs.length > 3) {
    while (breadcrumbs.length !== 4) {
      breadcrumbs.shift();
    }

    breadcrumbs[0].name = "...";
  }

  return breadcrumbs;
});

const element = computed(() => {
  if (props.noLink) {
    return "span";
  }

  return "router-link";
});

/** 编辑产品编号的目标是否存在：
 *  - 列表视图：恰好单选一个 PDF；
 *  - 预览/编辑器视图：当前打开的是 PDF。
 *  仅登录的 /files 上下文启用（公开分享页无对应 API 权限）。 */
const productCodeAvailable = computed(() => {
  if (!fileStore.isFiles || !authStore.jwt || !authStore.user?.perm?.modify) {
    return false;
  }
  const req = fileStore.req;
  if (!req) return false;
  if (req.isDir) {
    if (fileStore.selectedCount !== 1) return false;
    const item = req.items[fileStore.selected[0]];
    return !!item && !item.isDir && item.type === "pdf";
  }
  return req.type === "pdf";
});

const openProductCode = () => {
  layoutStore.showHover("productCode");
};
</script>

<style scoped>
.breadcrumbs-product-code {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  margin-left: auto;
  padding: 0;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--textPrimary, #000);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.breadcrumbs-product-code:hover {
  background: rgba(0, 122, 255, 0.12);
  color: var(--blue, #007aff);
}

.breadcrumbs-product-code i {
  font-size: 17px;
  line-height: 1;
}
</style>
