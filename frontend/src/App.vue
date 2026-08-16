<template>
  <div>
    <router-view></router-view>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from "vue";
import { useI18n } from "vue-i18n";
import { setHtmlLocale } from "./i18n";
import { getTheme, setTheme } from "./utils/theme";

const { locale } = useI18n();

// 默认亮色主题（不跟随系统深色偏好），可在设置中切换
const userTheme = ref<UserTheme>(getTheme() || "light");

onMounted(() => {
  setTheme(userTheme.value);
  setHtmlLocale(locale.value);
  // this might be null during HMR
  const loading = document.getElementById("loading");
  loading?.classList.add("done");

  setTimeout(function () {
    loading?.parentNode?.removeChild(loading);
  }, 200);
});

// handles ltr/rtl changes
watch(locale, (newValue) => {
  newValue && setHtmlLocale(newValue);
});
</script>
