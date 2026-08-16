<template>
  <div :class="{ 'sidebar-collapsed': layoutStore.sidebarCollapsed }">
    <div v-if="uploadStore.totalBytes" class="progress">
      <div
        v-bind:style="{
          width: sentPercent + '%',
        }"
      ></div>
    </div>
    <sidebar></sidebar>
    <main>
      <router-view></router-view>
      <shell
        v-if="
          enableExec && authStore.isLoggedIn && authStore.user?.perm.execute
        "
      />
    </main>
    <prompts></prompts>
    <upload-files></upload-files>
  </div>
</template>

<script setup lang="ts">
import { useAuthStore } from "@/stores/auth";
import { useLayoutStore } from "@/stores/layout";
import { useFileStore } from "@/stores/file";
import { useUploadStore } from "@/stores/upload";
import Sidebar from "@/components/Sidebar.vue";
import Prompts from "@/components/prompts/Prompts.vue";
import Shell from "@/components/Shell.vue";
import UploadFiles from "@/components/prompts/UploadFiles.vue";
import { enableExec } from "@/utils/constants";
import { computed, watch } from "vue";
import { useRoute } from "vue-router";

const layoutStore = useLayoutStore();
const authStore = useAuthStore();
const fileStore = useFileStore();
const uploadStore = useUploadStore();
const route = useRoute();

const sentPercent = computed(() =>
  ((uploadStore.sentBytes / uploadStore.totalBytes) * 100).toFixed(2)
);

/* 只监听 route.path：query 变化（如列表选中同步的 ?sel=、CSV 的 ?edit=）
 * 不是页面切换，不应清空选中状态；否则"点击选中 → 写 URL → 被清空"形成死循环 */
watch(
  () => route.path,
  () => {
    fileStore.selected = [];
    fileStore.multiple = false;
    if (layoutStore.currentPromptName !== "success") {
      layoutStore.closeHovers();
    }
  }
);
</script>
