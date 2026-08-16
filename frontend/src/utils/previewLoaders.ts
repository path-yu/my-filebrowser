/* ================== 重型预览库：共享惰性加载器 ==================
 * pdfjs-dist / marked / dompurify / docx-preview 每个体积数百 KB，只在
 * 用户真正预览对应类型文件时才需要。FileListing（首屏组件）与 Preview
 * 都会用到它们，此模块统一提供"首次使用时动态 import + 模块级缓存"
 * 的加载入口：
 *   - 避免重库进入首屏 bundle（首次渲染速度）
 *   - 两个视图共享同一实例与 worker 配置（不重复初始化）
 *   - vite.config.ts 的 manualChunks 会把这些库归入独立 chunk，
 *     FileListing/Preview 不会各打包一份。
 */

// pdfjs-dist legacy build 没有单独类型声明，但 Vite 运行时可解析路径
// @ts-expect-error - TS2307: legacy build 类型并入主入口
const importPdfjs = () => import("pdfjs-dist/legacy/build/pdf");

let _pdfjsPromise: Promise<any> | null = null;

/* pdf.js worker 初始化（每个使用方仅执行一次）
   根因分析：
     pdfjs-dist 6.x main 与 worker 都使用 ES Module（.mjs），且 #pagesNumber / #apiVersion
     等私有字段名称的编译结果与"同一份构建产物"强绑定——worker 与 main 只要有任意字节
     差异（例如 Vite ?worker&inline 把 worker 再次打包/压缩/转译），就会引发
     "Private element is not present on this object" 或
     "Cannot read private member #pagesNumber from an object whose class did not declare it"。
   最稳策略（按优先级）：
     1) 通过 new Worker(new URL('/pdfjs/pdf.worker.min.mjs', base).href, {type:'module'})
        创建 ES Module worker，并使用 GlobalWorkerOptions.workerPort 直接把
        Worker 实例注入 pdfjs — 完全跳过 pdfjs 默认 classic workerSrc 路径。
        public/pdfjs/pdf.worker.min.mjs 是从 node_modules 字节级拷贝，100% 与 main
        模块版本 (6.2.108 legacy build) 一致。
     2) 国内镜像兜底：jsdelivr / 腾讯云镜像 legacy build 的 .mjs（.mjs 后缀，非旧 .js）。
     3) 最后留空避免经典路径出错。 */
const initPdfWorker = (pdfjs: any) => {
  // 浏览器专属逻辑：SSR / Node 测试环境（无 DOM、无 Worker）直接跳过，
  // pdfjs 会回退到主线程解析，不影响 API 可用性。
  if (typeof document === "undefined" || typeof window === "undefined") return;
  const BASE_HREF =
    (document.querySelector &&
      document.querySelector("base") &&
      (document.querySelector("base") as HTMLBaseElement).href) ||
    (typeof window !== "undefined" ? window.location.origin + "/" : "/");
  const PUBLIC_WORKER = new URL("/pdfjs/pdf.worker.min.mjs", BASE_HREF).href;
  const VERSION = "6.2.108";
  const LEGACY_MJS = "legacy/build/pdf.worker.min.mjs";
  // 使用纯 JS 数组对象字面量（不显式声明 TS 类型），避免 rolldown 在 IIFE 中误解析类型
  const candidates = [
    { src: PUBLIC_WORKER, asPort: true },
    {
      src: `https://fastly.jsdelivr.net/npm/pdfjs-dist@${VERSION}/${LEGACY_MJS}`,
      asPort: true,
    },
    {
      src: `https://mirrors.tencent.com/npm/pdfjs-dist@${VERSION}/${LEGACY_MJS}`,
      asPort: true,
    },
    { src: PUBLIC_WORKER, asPort: false },
    {
      src: `https://fastly.jsdelivr.net/npm/pdfjs-dist@${VERSION}/${LEGACY_MJS}`,
      asPort: false,
    },
    {
      src: `https://mirrors.tencent.com/npm/pdfjs-dist@${VERSION}/${LEGACY_MJS}`,
      asPort: false,
    },
  ];
  let i: number;
  let c: { src: string; asPort?: boolean };
  let w: Worker | null = null;
  for (i = 0; i < candidates.length; i++) {
    c = candidates[i];
    try {
      if (c.asPort) {
        if (typeof Worker === "undefined") continue;
        w = null;
        try {
          w = new Worker(c.src, {
            type: "module",
            name: "pdfjs-worker",
          } as WorkerOptions);
        } catch {
          continue;
        }
        try {
          pdfjs.GlobalWorkerOptions.workerPort = w;
          return;
        } catch {
          try {
            w.terminate();
          } catch {
            /* ignore */
          }
        }
      } else {
        pdfjs.GlobalWorkerOptions.workerSrc = c.src;
        return;
      }
    } catch {
      /* 继续试下一个 */
    }
  }
  try {
    pdfjs.GlobalWorkerOptions.workerSrc = "";
  } catch {
    /* ignore */
  }
};

export const ensurePdfLib = async (): Promise<any> => {
  if (!_pdfjsPromise) {
    _pdfjsPromise = importPdfjs().then((m) => {
      const pdfjs = m as any;
      initPdfWorker(pdfjs);
      return pdfjs;
    });
  }
  return _pdfjsPromise;
};

let _mdPromise: Promise<any> | null = null;

/** marked + dompurify：Markdown 预览/编辑按需加载（返回命名空间模块本身） */
export const ensureMarkdownLibs = () => {
  if (!_mdPromise) {
    _mdPromise = Promise.all([import("marked"), import("dompurify")]);
  }
  return _mdPromise as Promise<
    [typeof import("marked"), typeof import("dompurify")]
  >;
};

let _docxPromise: Promise<typeof import("docx-preview")> | null = null;

/** docx-preview：.docx 保真渲染按需加载 */
export const ensureDocxLib = () => {
  if (!_docxPromise) {
    _docxPromise = import("docx-preview");
  }
  return _docxPromise;
};
