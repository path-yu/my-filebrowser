/**
 * 用于本地验证「时间筛选」逻辑的 mock 数据生成器。
 *
 * 触发开关：
 *   1) 给任意 URL 加上 ?mockTime=1（例如 http://localhost:5173/files/?mockTime=1）
 *   2) 或者在 DevTools → Console 执行：localStorage.setItem("mockTime","1") 再刷新。
 *
 * 关闭：去掉 query 或 localStorage.removeItem("mockTime")。
 *
 * 数据分桶（全部覆盖各预设的边界，便于肉眼验证）：
 *   - 10 分钟前      → 「1小时内 / 12小时内 / 今天 / 3天内 / 1周内 / 1月内」 均应保留
 *   - 30 分钟前      → 同上
 *   - 2 小时前       → 1小时内会被剔除，其它保留
 *   - 今天 02:00     → 1小时 / 12小时 剔除，今天 / 3天 / 1周 / 1月 保留
 *   - 5 天前         → 1小时 / 12小时 / 今天 / 3天  剔除；1周 / 1月 保留
 *   - 2 周前         → 1小时 / 12小时 / 今天 / 3天 / 1周 剔除；1月保留
 *   - 40 天前        → 所有预设都剔除；只有「全部时间」保留
 *
 * 另外附赠 1 个子目录 "存档"（1个月前），时间筛选不会过滤目录。
 */

import router from "@/router";
import { useAuthStore } from "@/stores/auth";

// ---------------------------------------------------------------------------
// mock 模式下，自动注入一个虚拟的登录用户，避免被路由守卫重定向到 /login，
// 从而可以本地直接打开 /files/?mockTime=1 做 UI 验证，无需启动 Go 后端。
//
// 时序注意：main.ts 在 import 本模块时触发 installMockAuth()，但 router 的
// beforeResolve 会同步调用 initAuth() → validateLogin()，此时如果 localStorage
// 没有真实 JWT，authStore.isLoggedIn 在 guard 执行瞬间还是 false，守卫会
// 跳转到 /login。我们的注入在 DOMContentLoaded / 定时重试后才会写 store，
// 所以要多一层兜底：注入成功后若当前停在 /login，按 redirect query 回到目标页。
// ---------------------------------------------------------------------------
const installMockAuth = () => {
  try {
    const store = useAuthStore();
    if (store && !store.isLoggedIn) {
      store.setUser({
        id: 1,
        username: "mock-user",
        locale: "zh-cn",
        viewMode: "list",
        sorting: { by: "modified", asc: false, folderFirst: true },
        lockPassword: false,
        singleClick: false,
        perm: {
          admin: true,
          create: true,
          download: true,
          execute: true,
          modify: true,
          delete: true,
          rename: true,
          share: true,
        },
        commands: [],
      } as unknown as IUser);
      // 写一个合法的占位 JWT，阻止 /api/me / renew 失败引发的 logout 自动
      // 清空用户：exp 设 1 年，sub=mock-user，user 字段和 setUser 保持一致。
      const header = btoa(JSON.stringify({ alg: "none", typ: "JWT" }));
      const payload = btoa(
        JSON.stringify({
          sub: "mock-user",
          exp: Math.floor(Date.now() / 1000) + 365 * 24 * 3600,
          user: {
            id: 1,
            username: "mock-user",
            locale: "zh-cn",
            viewMode: "list",
            sorting: { by: "modified", asc: false, folderFirst: true },
            lockPassword: false,
            singleClick: false,
            perm: {
              admin: true,
              create: true,
              download: true,
              execute: true,
              modify: true,
              delete: true,
              rename: true,
              share: true,
            },
            commands: [],
          },
        })
      );
      const fakeToken = `${header}.${payload}.mock-signature`;
      try {
        localStorage.setItem("jwt", fakeToken);
      } catch { /* 忽略 storage 禁用 */ }
      if (store.jwt !== fakeToken) {
        try { store.jwt = fakeToken; } catch { /* 忽略只读 */ }
      }
    }
    // 注入成功：若当前停在 /login，回到 redirect query 指向的目标（默认 /files）
    if (window.location.pathname.endsWith("/login")) {
      const sp = new URLSearchParams(window.location.search);
      const redirect = decodeURIComponent(sp.get("redirect") || "/files/");
      const safeRedirect =
        redirect.startsWith("/") || redirect.startsWith(window.location.origin)
          ? redirect
          : "/files/";
      router.replace(safeRedirect).catch(() => {
        window.location.assign(safeRedirect);
      });
    }
  } catch {
    // Pinia 尚未安装，下次微任务再试
    return false;
  }
  return true;
};

if (typeof window !== "undefined" && isMockEnabled()) {
  let tries = 0;
  const tryInstall = () => {
    tries++;
    if (installMockAuth()) return;
    if (tries < 60) {
      setTimeout(tryInstall, 100);
    }
  };
  if (document.readyState === "loading") {
    window.addEventListener("DOMContentLoaded", tryInstall, { once: true });
  } else {
    tryInstall();
  }
  // 兜底：如果 1.5s 后还停在 /login，再强行尝试一次注入 + 跳转（某些时序下 router
  // guard 先跑完后 beforeResolve 不会再触发，必须主动 replace 一下）
  setTimeout(() => {
    tryInstall();
  }, 1500);
}

export interface MockBucket {
  id: string;
  name: string; // 文件名
  label: string; // 中文说明，写进 subtitle
  offsetMs: number; // 相对现在的负偏移
  ext: string; // 扩展名（含点）
  type?: ResourceType; // 默认为 pdf
}

export const MOCK_BUCKETS: MockBucket[] = [
  {
    id: "10min",
    name: "质量证明书(10分钟前).pdf",
    label: "10 分钟前 · 应出现在「1小时内」及更宽范围",
    offsetMs: -10 * 60 * 1000,
    ext: ".pdf",
    type: "pdf",
  },
  {
    id: "30min",
    name: "采购合同(30分钟前).pdf",
    label: "30 分钟前 · 应出现在「1小时内」及更宽范围",
    offsetMs: -30 * 60 * 1000,
    ext: ".pdf",
    type: "pdf",
  },
  {
    id: "2h",
    name: "设备点检记录(2小时前).pdf",
    label: "2 小时前 · 「1小时内」剔除，其它都在",
    offsetMs: -2 * 60 * 60 * 1000,
    ext: ".pdf",
    type: "pdf",
  },
  {
    id: "today",
    name: "日报(今天0200).pdf",
    label: "今天 02:00 · 「1小时/12小时」剔除，「今天」及更宽保留",
    offsetMs: 0, // 稍后根据"今天02:00"单独计算
    ext: ".pdf",
    type: "pdf",
  },
  {
    id: "5d",
    name: "供应商合同(5天前).pdf",
    label: "5 天前 · 「1周内」保留，更窄的选项都剔除",
    offsetMs: -5 * 24 * 60 * 60 * 1000,
    ext: ".pdf",
    type: "pdf",
  },
  {
    id: "2w",
    name: "质检扫描件(2周前).pdf",
    label: "2 周前 · 只有「1月内 / 全部时间」保留",
    offsetMs: -14 * 24 * 60 * 60 * 1000,
    ext: ".pdf",
    type: "pdf",
  },
  {
    id: "40d",
    name: "历史归档(40天前).pdf",
    label: "40 天前 · 所有预设时间选项都剔除，只有「全部时间」看得到",
    offsetMs: -40 * 24 * 60 * 60 * 1000,
    ext: ".pdf",
    type: "pdf",
  },
];

/** 构造某个 bucket 的 modified 时间（Date） */
export const bucketModified = (b: MockBucket, now = new Date): Date => {
  if (b.id === "today") {
    // 保证 today 桶同时落在「今天」与「12 小时内」两个预设的可见区间：
    //   today ∈ [start_of_today, now]              → 今天预设可见
    //   today ∈ [now - 12h, now]                   → 12 小时内预设可见
    // 合并即 today ∈ [max(start_of_today, now - 6h) + 5min, now - 1min]
    // 取区间内的一个点即可。
    const startOfToday = new Date(now);
    startOfToday.setHours(0, 0, 0, 0);
    const sixHoursAgo = new Date(now.getTime() - 6 * 60 * 60 * 1000);
    const lower = new Date(
      Math.max(startOfToday.getTime(), sixHoursAgo.getTime())
    );
    // 再往后推 5 分钟，避免整点边界问题
    return new Date(lower.getTime() + 5 * 60 * 1000);
  }
  return new Date(now.getTime() + b.offsetMs);
};

/** 是否启用 mock */
export function isMockEnabled(): boolean {
  if (typeof window === "undefined") return false;
  try {
    const url = new URL(window.location.href);
    if (url.searchParams.get("mockTime") === "1") return true;
    if (localStorage.getItem("mockTime") === "1") return true;
  } catch {
    /* ignore */
  }
  return false;
}

/**
 * 模拟后端 resourceGetHandler 的 modified_after / modified_before 过滤。
 * 后端行为：目录始终保留；文件用 [after, before] 区间（端点包含）。
 */
export function mockResourceResponse(
  dirPath: string,
  now = new Date,
  opts: { modified_after?: number; modified_before?: number } = {}
): Resource {
  const after =
    typeof opts.modified_after === "number"
      ? new Date(opts.modified_after)
      : new Date(0);
  const before =
    typeof opts.modified_before === "number"
      ? new Date(opts.modified_before)
      : new Date(8640000000000000); // Max

  if (!dirPath.endsWith("/")) dirPath += "/";
  const rootName = dirPath === "/" ? "" : dirPath.slice(0, -1);

  const items: any[] = [];

  // 1. 子目录：固定一个「存档」目录，保证即使不在时间范围也不被过滤
  const archiveName = "存档";
  const archiveMod = new Date(now.getTime() - 200 * 24 * 60 * 60 * 1000);
  items.push({
    index: 0,
    path: dirPath + archiveName,
    name: archiveName,
    size: 0,
    extension: "",
    modified: archiveMod.toISOString(),
    mode: 16895,
    isDir: true,
    isSymlink: false,
    type: "dir" as ResourceType,
    url: `/files${dirPath}${encodeURIComponent(archiveName)}/`,
  });

  // 2. 文件：按 bucket 生成，然后应用 modified_after / modified_before 过滤
  const filesOnly: any[] = [];
  MOCK_BUCKETS.forEach((b, idx) => {
    const mod = bucketModified(b, now);
    filesOnly.push({
      index: idx + 1,
      path: dirPath + b.name,
      name: b.name,
      size: 1024 * (idx + 1),
      extension: b.ext,
      modified: mod.toISOString(),
      mode: 33206,
      isDir: false,
      isSymlink: false,
      type: (b.type ?? "pdf") as ResourceType,
      url: `/files${dirPath}${encodeURIComponent(b.name)}`,
      // 调试字段：DevTools 里展开文件对象可见哪个 bucket、预期出现在什么预设里
      _bucket: b.label,
    } as any);
  });

  for (const f of filesOnly) {
    const t = new Date(f.modified).getTime();
    if (t < after.getTime() || t > before.getTime()) {
      continue; // 模拟后端过滤
    }
    items.push(f);
  }

  // 最终 index 重新编号，保证与真实 fetch 里 post-process 行为一致
  items.forEach((it, i) => (it.index = i));

  const numDirs = items.filter((i) => i.isDir).length;
  const numFiles = items.length - numDirs;

  return {
    path: dirPath,
    name: rootName,
    size: 0,
    extension: "",
    modified: new Date().toISOString(),
    mode: 16895,
    isDir: true,
    isSymlink: false,
    type: "dir" as ResourceType,
    url: `/files${dirPath}`,
    items,
    numDirs,
    numFiles,
    sorting: { by: "modified", asc: false, folderFirst: true },
  } as unknown as Resource;
}

/**
 * 模拟后端 search handler 的流式输出，输出所有 bucket 文件（包含子目录名命中 query 的部分）。
 * 调用方负责在 callback 前再跑一遍 modified_after / before 过滤；这里故意产出全量，
 * 验证「搜索 API 传入 extra 参数」路径是否真的在后端生效。
 */
export function mockSearchResultSet(
  basePath = "/",
  now = new Date
): ResourceItem[] {
  const base = basePath.endsWith("/") ? basePath : basePath + "/";
  const results: ResourceItem[] = [];

  MOCK_BUCKETS.forEach((b, idx) => {
    const mod = bucketModified(b, now);
    results.push({
      index: idx,
      path: base + b.name,
      name: b.name,
      size: 1024 * (idx + 1),
      extension: b.ext,
      modified: mod.toISOString(),
      mode: 33206,
      isDir: false,
      isSymlink: false,
      type: (b.type ?? "pdf") as ResourceType,
      url: `/files${base}${encodeURIComponent(b.name)}`,
    });
  });

  // 加一个子目录命中项（搜索 "存档"）
  const archiveMod = new Date(now.getTime() - 200 * 24 * 60 * 60 * 1000);
  results.push({
    index: results.length,
    path: base + "存档",
    name: "存档",
    size: 0,
    extension: "",
    modified: archiveMod.toISOString(),
    mode: 16895,
    isDir: true,
    isSymlink: false,
    type: "dir" as ResourceType,
    url: `/files${base}存档/`,
  });

  return results;
}

/**
 * 把 MOCK_BUCKETS + 过滤条件 → 生成一段方便复制的说明文案，
 * 可用来做自动化单元测试时作为"预期对照表"。
 */
export function mockExpectationTable(now = new Date) {
  const presets = [
    { id: "all", label: "全部时间" },
    { id: "hour1", label: "1小时内" },
    { id: "hour12", label: "12小时内" },
    { id: "today", label: "今天" },
    { id: "days3", label: "3天内" },
    { id: "week1", label: "1周内" },
    { id: "month1", label: "1月内" },
  ];

  const presetRange = (
    id: string
  ): { after?: Date; before?: Date } => {
    switch (id) {
      case "hour1":
        return {
          after: new Date(now.getTime() - 60 * 60 * 1000),
          before: now,
        };
      case "hour12":
        return {
          after: new Date(now.getTime() - 12 * 60 * 60 * 1000),
          before: now,
        };
      case "today": {
        const d = new Date(now);
        d.setHours(0, 0, 0, 0);
        return { after: d, before: now };
      }
      case "days3":
        return {
          after: new Date(now.getTime() - 3 * 24 * 60 * 60 * 1000),
          before: now,
        };
      case "week1":
        return {
          after: new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000),
          before: now,
        };
      case "month1": {
        const d = new Date(now);
        d.setMonth(d.getMonth() - 1);
        return { after: d, before: now };
      }
      default:
        return {};
    }
  };

  const rows: string[] = [];
  for (const p of presets) {
    const { after, before } = presetRange(p.id);
    const visible = MOCK_BUCKETS.filter((b) => {
      const t = bucketModified(b, now).getTime();
      if (after && t < after.getTime()) return false;
      if (before && t > before.getTime()) return false;
      return true;
    }).map((b) => b.id);
    rows.push(`- ${p.label}：${visible.length} 个文件（${visible.join("、") || "无"}）`);
  }
  return rows.join("\n");
}

// dev 友好：若启用 mock，则把期望对照表暴露到 window.__MOCK_TIME__，
// 用户打开 DevTools 直接看 / 复制。
if (typeof window !== "undefined" && isMockEnabled()) {
  try {
    (window as any).__MOCK_TIME__ = {
      buckets: MOCK_BUCKETS.map((b) => ({
        id: b.id,
        name: b.name,
        label: b.label,
        modifiedISO: bucketModified(b, new Date()).toISOString(),
      })),
      printExpectation() {
        console.log(
          "%c[时间筛选 Mock] 各预设预期可见文件数：\n" +
            mockExpectationTable(new Date()),
          "color:#007aff;font-weight:600;"
        );
      },
    };
    // 页面加载后自动打印一次，便于一打开 DevTools 就看到
    window.addEventListener("load", () => {
      try {
        (window as any).__MOCK_TIME__.printExpectation();
      } catch {
        /* ignore */
      }
    });
  } catch {
    /* ignore */
  }
}
