import { describe, expect, it, vi } from "vitest";

/**
 * utils/previewLoaders.ts 测试：
 * 重型预览库（pdfjs-dist / marked / dompurify / docx-preview）已从
 * FileListing.vue / Preview.vue 的静态导入改为本模块的"首次使用时
 * 动态 import + 模块级缓存"。本用例验证：
 *   1. 缓存语义 —— 多次调用返回同一实例（Promise 复用，不重复加载）
 *   2. 返回结构 —— ensureMarkdownLibs 返回 [marked, dompurify] 元组
 *   3. 无 DOM 环境安全性 —— initPdfWorker 在 Node 下自动跳过
 */

vi.mock("pdfjs-dist/legacy/build/pdf", () => ({
  GlobalWorkerOptions: { workerPort: null, workerSrc: "" },
  getDocument: vi.fn(),
  version: "test-mock",
}));

vi.mock("marked", () => ({
  marked: { parse: vi.fn(async () => "<p>ok</p>") },
}));

vi.mock("dompurify", () => ({
  default: { sanitize: vi.fn((html: string) => html) },
}));

vi.mock("docx-preview", () => ({
  renderAsync: vi.fn(async () => undefined),
}));

import {
  ensureDocxLib,
  ensureMarkdownLibs,
  ensurePdfLib,
} from "@/utils/previewLoaders";

describe("ensurePdfLib", () => {
  it("重复调用返回同一 pdfjs 实例（模块级缓存，仅 import 一次）", async () => {
    const a = await ensurePdfLib();
    const b = await ensurePdfLib();
    expect(a).toBe(b);
    expect(a.version).toBe("test-mock");
    expect(typeof a.getDocument).toBe("function");
  });

  it("并发调用共享同一个 Promise", async () => {
    const [a, b] = await Promise.all([ensurePdfLib(), ensurePdfLib()]);
    expect(a).toBe(b);
  });

  it("无 DOM 环境下不抛错（initPdfWorker 自动跳过）", async () => {
    // vitest 默认 node 环境没有 document/window，
    // ensurePdfLib 应正常 resolve 而不是在 worker 初始化中崩溃
    await expect(ensurePdfLib()).resolves.toBeTruthy();
  });
});

describe("ensureMarkdownLibs", () => {
  it("返回 [marked, dompurify] 模块元组", async () => {
    const [markedMod, dompurifyMod] = await ensureMarkdownLibs();
    expect(markedMod.marked).toBeDefined();
    expect(dompurifyMod.default.sanitize).toBeDefined();
  });

  it("重复调用返回同一 Promise 结果（缓存）", async () => {
    const first = await ensureMarkdownLibs();
    const second = await ensureMarkdownLibs();
    expect(first[0]).toBe(second[0]);
    expect(first[1]).toBe(second[1]);
  });
});

describe("ensureDocxLib", () => {
  it("重复调用返回同一 docx-preview 模块（缓存）", async () => {
    const a = await ensureDocxLib();
    const b = await ensureDocxLib();
    expect(a).toBe(b);
    expect(typeof a.renderAsync).toBe("function");
  });
});
