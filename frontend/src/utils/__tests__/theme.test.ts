import { beforeAll, describe, expect, it, vi } from "vitest";

/**
 * utils/theme.ts 测试：
 * getEditorTheme() 原先通过 `import { themesByName } from "ace-builds/..."`
 * 校验主题名，导致整个 Ace 编辑器（数百 KB）被打进首屏 bundle（theme.ts
 * 被 App.vue 同步引用）。现已内联 ACE_THEME_NAMES 常量名单做校验。
 * 本用例验证名单校验行为与浅/深色回退逻辑均与旧实现一致。
 *
 * 注意：constants.ts 在模块初始化时读取 window.FileBrowser / localStorage，
 * 因此必须先 stub 全局对象，再动态 import（不能顶层静态 import）。
 */

const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: (k: string) => (k in store ? store[k] : null),
    setItem: (k: string, v: string) => {
      store[k] = String(v);
    },
    removeItem: (k: string) => {
      delete store[k];
    },
    clear: () => {
      store = {};
    },
  };
})();

const documentMock = {
  documentElement: {
    className: "light",
    classList: {
      remove: () => {},
      add: () => {},
    },
  },
};

function installGlobals(theme: string) {
  vi.stubGlobal(
    "window",
    {
      FileBrowser: {
        Name: "test",
        Theme: theme,
        DisableExternal: false,
        DisableUsedPercentage: false,
        BaseURL: "",
        StaticURL: "",
        ReCaptcha: "",
        ReCaptchaKey: "",
        Signup: false,
        Version: "test",
        NoAuth: false,
        AuthMethod: "json",
        LogoutPage: "",
        LoginPage: false,
        EnableThumbs: false,
        ResizePreview: false,
        EnableExec: false,
        TusSettings: null,
        HideLoginButton: false,
      },
      location: { origin: "http://localhost" },
      matchMedia: () => ({ matches: false }),
    }
  );
  vi.stubGlobal("document", documentMock);
  vi.stubGlobal("localStorage", localStorageMock);
}

describe("getEditorTheme（Ace 主题名单内联校验）", () => {
  let themeMod: typeof import("@/utils/theme");

  beforeAll(async () => {
    localStorageMock.clear();
    installGlobals("light");
    themeMod = await import("@/utils/theme");
  });

  it("合法主题名：自动补 ace/theme/ 前缀后原样返回", () => {
    expect(themeMod.getEditorTheme("monokai")).toBe("ace/theme/monokai");
    expect(themeMod.getEditorTheme("tomorrow_night_blue")).toBe(
      "ace/theme/tomorrow_night_blue"
    );
  });

  it("已带前缀的合法主题名：原样返回", () => {
    expect(themeMod.getEditorTheme("ace/theme/twilight")).toBe(
      "ace/theme/twilight"
    );
  });

  it("浅色模式下非法主题名：回退到 chrome", () => {
    expect(themeMod.getEditorTheme("not_a_real_theme")).toBe(
      "ace/theme/chrome"
    );
  });

  it("深色模式下非法主题名：回退到 twilight", () => {
    // localStorage 保存的主题优先于 window.FileBrowser.Theme
    localStorageMock.setItem("fb_theme", "dark");
    expect(themeMod.getEditorTheme("not_a_real_theme")).toBe(
      "ace/theme/twilight"
    );
    localStorageMock.setItem("fb_theme", "light");
  });

  it("getTheme 优先级：localStorage > 后端配置 > DOM class", () => {
    localStorageMock.clear();
    installGlobals("dark");
    // window.FileBrowser.Theme = dark
    expect(themeMod.getTheme()).toBe("dark");
    // localStorage 手动选择覆盖后端配置
    localStorageMock.setItem("fb_theme", "light");
    expect(themeMod.getTheme()).toBe("light");
  });
});
