import { describe, expect, it, vi } from "vitest";

/**
 * i18n 按需加载测试：
 * 原先通过 @intlify/unplugin-vue-i18n/messages 虚拟模块把全部 34 种
 * 语言包（~840KB）静态打进首屏。现在只内置默认语言 zh-cn，其余语言
 * 在 setLocale 时通过 import.meta.glob 按需加载。
 * 本用例验证：
 *   1. 默认语言 zh-cn 内置，切换走同步路径
 *   2. 非内置语言（en）首次切换触发异步 chunk 加载，成功后才切换 locale
 *   3. 语言包加载后写入 i18n messages，二次切换不再需要网络
 *   4. detectBrowserLocale 语言标签映射逻辑保持不变
 */

const { detectBrowserLocale, i18n, setLocale } = await import("@/i18n");

describe("i18n 按需语言包加载", () => {
  it("默认语言为 zh-cn 且已内置（无需异步加载）", () => {
    // @ts-expect-error - legacy 模式下 locale 是 ref
    expect(i18n.global.locale.value).toBe("zh-cn");
    expect(Object.keys(i18n.global.getLocaleMessage("zh-cn"))).not.toHaveLength(
      0
    );
  });

  it("初始状态不包含非内置语言包（en 未随首屏打包）", () => {
    // en.json 存在于源码但不应在首屏 messages 中
    expect(Object.keys(i18n.global.getLocaleMessage("en"))).toHaveLength(0);
  });

  it("切换到 en：异步加载语言包成功后才切换 locale", async () => {
    setLocale("en");
    // setLocale 内部 void promise，不返回值 —— 用 waitFor 轮询结果
    await vi.waitFor(() => {
      // @ts-expect-error - legacy 模式下 locale 是 ref
      expect(i18n.global.locale.value).toBe("en");
    });
    expect(Object.keys(i18n.global.getLocaleMessage("en"))).not.toHaveLength(0);
  });

  it("切换回已加载的 zh-cn：同步完成", () => {
    setLocale("zh-cn");
    // @ts-expect-error - legacy 模式下 locale 是 ref
    expect(i18n.global.locale.value).toBe("zh-cn");
  });

  it("再次切换 en：走已加载缓存，locale 立即生效", async () => {
    setLocale("en");
    await Promise.resolve();
    // @ts-expect-error - legacy 模式下 locale 是 ref
    expect(i18n.global.locale.value).toBe("en");
    // 还原默认语言，避免影响其他用例
    setLocale("zh-cn");
  });
});

describe("detectBrowserLocale 语言标签映射", () => {
  const cases: Array<[string, string]> = [
    ["zh-CN", "zh-cn"],
    ["zh-TW", "zh-tw"],
    ["en-US", "en"],
    ["pt-BR", "pt-br"],
    ["pt-PT", "pt-pt"],
    ["nl-BE", "nl-be"],
    ["fr-FR", "fr"],
    ["ja-JP", "ja"],
    ["xx-XX", "en"], // 未知语言回退 en
  ];

  it.each(cases)("navigator.language=%s → %s", (input, expected) => {
    vi.stubGlobal("navigator", { language: input });
    expect(detectBrowserLocale()).toBe(expected);
  });
});
