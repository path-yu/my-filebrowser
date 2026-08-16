import { theme } from "./constants";

/**
 * ace-builds 1.44.0 内置编辑器主题名单（src-noconflict/ext-themelist.js 的
 * name 字段，按字母排序）。此处以常量内联，避免 utils/theme.ts →
 * "ace-builds" 的静态导入把整个 Ace 编辑器打进首屏 bundle —— theme.ts 被
 * App.vue 同步引用，而它只需要校验主题名是否合法。
 * 升级 ace-builds 时同步更新此名单（可用脚本从 ext-themelist.js 提取）。
 */
const ACE_THEME_NAMES: ReadonlySet<string> = new Set([
  "ambiance",
  "chaos",
  "chrome",
  "cloud_editor",
  "cloud_editor_dark",
  "clouds",
  "clouds_midnight",
  "cobalt",
  "crimson_editor",
  "dawn",
  "dracula",
  "dreamweaver",
  "eclipse",
  "github",
  "github_dark",
  "github_light_default",
  "gob",
  "gruvbox",
  "idle_fingers",
  "iplastic",
  "katzenmilch",
  "kr_theme",
  "kuroir",
  "merbivore",
  "merbivore_soft",
  "mono_industrial",
  "monokai",
  "nord_dark",
  "one_dark",
  "pastel_on_dark",
  "solarized_dark",
  "solarized_light",
  "sqlserver",
  "terminal",
  "textmate",
  "tomorrow",
  "tomorrow_night",
  "tomorrow_night_blue",
  "tomorrow_night_bright",
  "tomorrow_night_eighties",
  "twilight",
  "vibrant_ink",
  "xcode",
]);

const LS_KEY = "fb_theme";

// 从 localStorage 读用户手动保存的主题
const readSavedTheme = (): UserTheme | "" => {
  try {
    const v = localStorage.getItem(LS_KEY);
    if (v === "light" || v === "dark") return v as UserTheme;
  } catch {
    /* ignore */
  }
  return "";
};

// 写主题到 localStorage 持久化（刷新后优先读取）
const writeSavedTheme = (t: UserTheme) => {
  try {
    localStorage.setItem(LS_KEY, t);
  } catch {
    /* ignore */
  }
};

/**
 * 读取当前主题 —— 修复后的优先级：
 *  1) localStorage['fb_theme']     （用户在 App 里手动切换过的，最高优先级）
 *  2) window.FileBrowser.Theme     （后端注入的全局配置）
 *  3) document.documentElement.className （兜底）
 *  4) 常量 theme（默认 "light" 出厂默认）
 *
 *  ❌ 旧实现只读 html.className，会被 index.html 渲染前的预赋值
 *     （例如系统 dark 时）“固化” 成 dark，不管后端配置是浅。
 */
export const getTheme = (): UserTheme => {
  // 1) 用户手动保存
  const saved = readSavedTheme();
  if (saved) return saved;

  // 2) 后端配置
  try {
    const cfg = (window.FileBrowser &&
      (window.FileBrowser as any).Theme) as string;
    if (cfg === "light" || cfg === "dark") return cfg as UserTheme;
  } catch {
    /* ignore */
  }

  // 3) DOM class 兜底
  const cls = document.documentElement.className.trim();
  if (cls === "light" || cls === "dark") return cls as UserTheme;

  // 4) 出厂默认浅色
  return theme || "light";
};

/**
 * 写入主题：
 *  - 先移除 light / dark 两个 class，避免同时存在导致样式冲突
 *  - 添加目标 class
 *  - 同步写入 localStorage，保证刷新后仍为用户所选
 */
export const setTheme = (target: UserTheme) => {
  const html = document.documentElement;
  const chosen: UserTheme =
    target === "light" || target === "dark" ? target : getMediaPreference();
  html.classList.remove("light", "dark");
  html.classList.add(chosen);
  writeSavedTheme(chosen);
};

export const toggleTheme = (): void => {
  const activeTheme = getTheme();
  if (activeTheme === "light") {
    setTheme("dark");
  } else {
    setTheme("light");
  }
};

export const getMediaPreference = (): UserTheme => {
  const hasDarkPreference = window.matchMedia(
    "(prefers-color-scheme: dark)"
  ).matches;
  return hasDarkPreference ? "dark" : "light";
};

export const getEditorTheme = (themeName: string) => {
  if (!themeName.startsWith("ace/theme/")) {
    themeName = `ace/theme/${themeName}`;
  }
  const themeKey = themeName.replace("ace/theme/", "");
  if (ACE_THEME_NAMES.has(themeKey)) {
    return themeName;
  } else if (getTheme() === "dark") {
    return "ace/theme/twilight";
  } else {
    return "ace/theme/chrome";
  }
};
