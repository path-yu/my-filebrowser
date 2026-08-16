import dayjs from "dayjs";
import { createI18n } from "vue-i18n";

import("dayjs/locale/ar");
import("dayjs/locale/bg");
import("dayjs/locale/ca");
import("dayjs/locale/cs");
import("dayjs/locale/de");
import("dayjs/locale/el");
import("dayjs/locale/en");
import("dayjs/locale/es");
import("dayjs/locale/fr");
import("dayjs/locale/he");
import("dayjs/locale/hr");
import("dayjs/locale/hu");
import("dayjs/locale/is");
import("dayjs/locale/it");
import("dayjs/locale/ja");
import("dayjs/locale/ko");
import("dayjs/locale/lv");
import("dayjs/locale/nb");
import("dayjs/locale/nl");
import("dayjs/locale/nl-be");
import("dayjs/locale/pl");
import("dayjs/locale/pt-br");
import("dayjs/locale/pt");
import("dayjs/locale/ro");
import("dayjs/locale/ru");
import("dayjs/locale/sk");
import("dayjs/locale/sv");
import("dayjs/locale/tr");
import("dayjs/locale/uk");
import("dayjs/locale/vi");
import("dayjs/locale/zh-cn");
import("dayjs/locale/zh-tw");

/* 语言包按需加载：
 * 之前通过 @intlify/unplugin-vue-i18n/messages 虚拟模块把全部 38 种语言
 * （~840KB / gzip ~190KB）静态打进首屏，而运行时同一时刻只用到一种语言。
 * 现在只内置默认语言 zh-cn，其余语言在 setLocale 切换时按需加载
 * （import.meta.glob 让 Vite 把每个语言文件拆成独立 chunk）。
 * 注意：vite.config.ts 中 manualChunks 的 "i18n/" 规则匹配的是 src/i18n
 * 目录下的运行时代码（本文件），语言 JSON chunk 仍按文件拆分。 */
import zhCN from "./zh-cn.json";

const localeModules = import.meta.glob("./*.json");

const loadedLocales = new Set<string>(["zh-cn"]);

const messages: Record<string, any> = {
  "zh-cn": zhCN,
};

async function ensureLocaleMessages(locale: string): Promise<boolean> {
  if (loadedLocales.has(locale)) return true;
  const loader = localeModules[`./${locale}.json`];
  if (!loader) return false;
  const mod = (await loader()) as { default: Record<string, any> };
  i18n.global.setLocaleMessage(locale, mod.default);
  loadedLocales.add(locale);
  return true;
}
export function detectLocale() {
  // 默认使用简体中文（文件管理系统默认语言）
  return "zh-cn";
}

// 保留多语言检测逻辑，供用户手动切换语言时使用
export function detectBrowserLocale() {
  // locale is an RFC 5646 language tag
  // https://developer.mozilla.org/en-US/docs/Web/API/Navigator/language
  let locale = navigator.language.toLowerCase();
  switch (true) {
    case /^ar\b/.test(locale):
      locale = "ar";
      break;
    case /^bg\b/.test(locale):
      locale = "bg";
      break;
    case /^cs\b/.test(locale):
      locale = "cs";
      break;
    case /^lv\b/.test(locale):
      locale = "lv";
      break;
    case /^he\b/.test(locale):
      locale = "he";
      break;
    case /^hr\b/.test(locale):
      locale = "hr";
      break;
    case /^hu\b/.test(locale):
      locale = "hu";
      break;
    case /^el.*/i.test(locale):
      locale = "el";
      break;
    case /^es\b/.test(locale):
      locale = "es";
      break;
    case /^en\b/.test(locale):
      locale = "en";
      break;
    case /^is\b/.test(locale):
      locale = "is";
      break;
    case /^it\b/.test(locale):
      locale = "it";
      break;
    case /^fr\b/.test(locale):
      locale = "fr";
      break;
    case /^pt-br\b/.test(locale):
      locale = "pt-br";
      break;
    case /^pt-pt\b/.test(locale):
    case /^pt\b/.test(locale):
      locale = "pt-pt";
      break;
    case /^ja\b/.test(locale):
      locale = "ja";
      break;
    case /^zh-tw\b/.test(locale):
      locale = "zh-tw";
      break;
    case /^zh-cn\b/.test(locale):
    case /^zh\b/.test(locale):
      locale = "zh-cn";
      break;
    case /^de\b/.test(locale):
      locale = "de";
      break;
    case /^ro\b/.test(locale):
      locale = "ro";
      break;
    case /^ru\b/.test(locale):
      locale = "ru";
      break;
    case /^pl\b/.test(locale):
      locale = "pl";
      break;
    case /^ko\b/.test(locale):
      locale = "ko";
      break;
    case /^sk\b/.test(locale):
      locale = "sk";
      break;
    case /^tr\b/.test(locale):
      locale = "tr";
      break;
    case /^uk\b/.test(locale):
      locale = "uk";
      break;
    case /^vi\b/.test(locale):
      locale = "vi";
      break;
    case /^sv-se\b/.test(locale):
    case /^sv\b/.test(locale):
      locale = "sv";
      break;
    case /^nl-be\b/.test(locale):
      locale = "nl-be";
      break;
    case /^nl\b/.test(locale):
      locale = "nl";
      break;
    case /^nb\b/.test(locale):
    case /^no\b/.test(locale):
      locale = "no";
      break;

    default:
      locale = "en";
  }

  return locale;
}

// TODO: was this really necessary?
// function removeEmpty(obj: Record<string, any>): void {
//   Object.keys(obj)
//     .filter((k) => obj[k] !== null && obj[k] !== undefined && obj[k] !== "") // Remove undef. and null and empty.string.
//     .reduce(
//       (newObj, k) =>
//         typeof obj[k] === "object"
//           ? Object.assign(newObj, { [k]: removeEmpty(obj[k]) }) // Recurse.
//           : Object.assign(newObj, { [k]: obj[k] }), // Copy value.
//       {}
//     );
// }

export const rtlLanguages = ["he", "ar"];

export const i18n = createI18n({
  locale: "zh-cn",
  fallbackLocale: "zh-cn",
  messages,
  // expose i18n.global for outside components
  legacy: true,
});

export const isRtl = (locale?: string) => {
  // see below
  // @ts-expect-error incorrect type when legacy
  return rtlLanguages.includes(locale || i18n.global.locale.value);
};

export function setLocale(locale: string) {
  // 语言包已加载（默认 zh-cn 内置）→ 同步切换；
  // 未加载 → 先拉取该语言的 chunk，成功后再切换，避免界面闪现 fallback 文案。
  if (loadedLocales.has(locale)) {
    dayjs.locale(locale);
    // according to doc u only need .value if legacy: false but they lied
    // https://vue-i18n.intlify.dev/guide/essentials/scope.html#local-scope-1
    // @ts-expect-error incorrect type when legacy
    i18n.global.locale.value = locale;
    return;
  }

  void ensureLocaleMessages(locale).then((ok) => {
    if (!ok) return;
    dayjs.locale(locale);
    // @ts-expect-error incorrect type when legacy
    i18n.global.locale.value = locale;
  });
}

export function setHtmlLocale(locale: string) {
  const html = document.documentElement;
  html.lang = locale;
  if (isRtl(locale)) html.dir = "rtl";
  else html.dir = "ltr";
}

export default i18n;
