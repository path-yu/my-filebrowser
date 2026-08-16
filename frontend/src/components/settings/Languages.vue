<template>
  <MacOSSelect
    :id="$attrs.id"
    :model-value="locale"
    :options="localeOptions"
    @update:model-value="change"
  />
</template>

<script>
import { markRaw } from "vue";
import MacOSSelect from "../MacOSSelect.vue";

export default {
  name: "languages",
  components: { MacOSSelect },
  // 拦截父组件传入的 class="input input--block" fallthrough，仅透传 id
  inheritAttrs: false,
  props: ["locale"],
  data() {
    const dataObj = {};
    const locales = {
      ar: "العربية",
      bg: "български език",
      ca: "Català",
      cs: "Čeština",
      de: "Deutsch",
      el: "Ελληνικά",
      en: "English",
      es: "Español",
      fr: "Français",
      he: "עברית",
      hr: "Hrvatski",
      hu: "Magyar",
      is: "Icelandic",
      it: "Italiano",
      ja: "日本語",
      ko: "한국어",
      no: "Norsk",
      nl: "Nederlands (Nederland)",
      "nl-be": "Nederlands (België)",
      lv: "Latviešu",
      pl: "Polski",
      "pt-br": "Português (Brasil)",
      "pt-pt": "Português (Portugal)",
      ro: "Romanian",
      ru: "Русский",
      sk: "Slovenčina",
      "sv-se": "Swedish (Sweden)",
      tr: "Türkçe",
      uk: "Українська",
      vi: "Tiếng Việt",
      "zh-cn": "中文 (简体)",
      "zh-tw": "中文 (繁體)",
    };

    // Vue3 reactivity breaks with this configuration
    // so we need to use markRaw as a workaround
    // https://github.com/vuejs/core/issues/3024
    Object.defineProperty(dataObj, "locales", {
      value: markRaw(locales),
      configurable: false,
      writable: false,
    });

    return dataObj;
  },
  computed: {
    localeOptions() {
      return Object.entries(this.locales).map(([value, label]) => ({
        value,
        label,
      }));
    },
  },
  methods: {
    change(value) {
      this.$emit("update:locale", value);
    },
  },
};
</script>
