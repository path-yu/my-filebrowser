import { fetchURL, removePrefix, StatusError } from "./utils";
import { encodePath } from "../utils/url";
import {
  isMockEnabled,
  mockSearchResultSet,
} from "./mockTimeFilter";

/** 已知图片 / 视频 / 音频扩展名集合（小写，不带点），
 *  用于给 /api/search 返回的精简条目补齐 extension / type。
 *  后端 files/file.go 的 detectType 用 MIME + 嗅探更准确，
 *  但搜索结果为了流式性能只返回最小字段，所以前端用扩展名
 *  静态映射兜底；无法归类时统一归为 blob。 */
const IMAGE_EXTS = new Set([
  "jpg", "jpeg", "png", "gif", "webp", "bmp", "svg", "tif", "tiff",
  "heic", "heif", "avif", "ico", "cur",
]);
const VIDEO_EXTS = new Set([
  "mp4", "m4v", "mov", "avi", "mkv", "webm", "flv", "wmv", "mpeg",
  "mpg", "3gp", "ts", "m2ts", "rmvb", "ogv",
]);
const AUDIO_EXTS = new Set([
  "mp3", "wav", "flac", "aac", "m4a", "ogg", "oga", "opus", "wma",
  "aiff", "aif", "alac",
]);
const TEXT_EXTS = new Set([
  "txt", "md", "markdown", "log", "csv", "json", "xml", "yaml", "yml",
  "toml", "ini", "conf", "cfg", "sh", "bat", "cmd", "ps1", "bash",
  "zsh", "js", "jsx", "ts", "tsx", "vue", "css", "scss", "sass", "less",
  "html", "htm", "c", "h", "cpp", "cc", "cxx", "hpp", "hh", "rs", "go",
  "py", "rb", "php", "java", "kt", "swift", "cs", "sql", "lua", "r",
  "pl", "dart", "ex", "exs", "erl", "hs", "ml", "fs", "vim",
]);

/** 按文件名补齐 ResourceItem 的 extension + type（用于搜索结果精简响应） */
function enrichSearchItem(item: ResourceItem): ResourceItem {
  const anyItem = item as any;
  // 目录直接定 type=dir
  if (anyItem.isDir) {
    anyItem.extension = "";
    anyItem.type = "dir";
    anyItem.isSymlink = anyItem.isSymlink ?? false;
    anyItem.mode = anyItem.mode ?? 0o644;
    return item;
  }
  const name = (item.name ?? "").replace(/\s+$/g, "");
  let ext = "";
  const dotIdx = name.lastIndexOf(".");
  if (dotIdx >= 0) {
    ext = name.slice(dotIdx);
  }
  const extNoDot = ext.length > 1 ? ext.slice(1).toLowerCase() : "";
  anyItem.extension = ext;
  let type: "pdf" | "image" | "video" | "audio" | "text" | "blob" = "blob";
  if (extNoDot === "pdf") {
    type = "pdf";
  } else if (IMAGE_EXTS.has(extNoDot)) {
    type = "image";
  } else if (VIDEO_EXTS.has(extNoDot)) {
    type = "video";
  } else if (AUDIO_EXTS.has(extNoDot)) {
    type = "audio";
  } else if (TEXT_EXTS.has(extNoDot)) {
    type = "text";
  }
  anyItem.type = type;
  anyItem.isSymlink = anyItem.isSymlink ?? false;
  anyItem.mode = anyItem.mode ?? 0o644;
  return item;
}

export interface SearchQueryParams {
  modified_after?: number;
  modified_before?: number;
  [k: string]: string | number | undefined;
}

export default async function search(
  base: string,
  query: string,
  signal: AbortSignal,
  callback: (item: ResourceItem) => void,
  extra?: SearchQueryParams
) {
  base = removePrefix(base);
  query = encodeURIComponent(query);

  if (!base.endsWith("/")) {
    base += "/";
  }

  const parts = [`query=${query}`];
  if (extra) {
    for (const [k, v] of Object.entries(extra)) {
      if (v === undefined || v === null || v === "") continue;
      parts.push(`${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`);
    }
  }

  // ------------------------------------------------------------------
  // Mock：?mockTime=1 或 localStorage.mockTime==="1" 时，不走真实后端。
  // 生成一份全量结果 + 按 query 关键字过滤，再按 modified_after/before 过滤
  // （模拟后端 http/search.go 里的 inModifiedRange 分支）
  // ------------------------------------------------------------------
  if (isMockEnabled()) {
    await new Promise((r) => setTimeout(r, 100));
    if (signal.aborted) throw new StatusError("000 No connection", 0, true);
    const q = decodeURIComponent(query).trim().toLowerCase();
    const after = extra?.modified_after
      ? new Date(extra.modified_after).getTime()
      : -Infinity;
    const before = extra?.modified_before
      ? new Date(extra.modified_before).getTime()
      : Infinity;
    const full = mockSearchResultSet(base, new Date());
    for (const it of full) {
      if (signal.aborted) throw new StatusError("000 No connection", 0, true);
      if (q && !it.name.toLowerCase().includes(q)) continue;
      const t = new Date(it.modified).getTime();
      if (t < after || t > before) continue;
      // 模拟"流式"逐批产出：每条间隔 8ms，便于观察 UI
      await new Promise((r) => setTimeout(r, 8));
      callback(it);
    }
    return;
  }

  const res = await fetchURL(`/api/search${base}?${parts.join("&")}`, { signal });
  if (!res.body) {
    throw new StatusError("000 No connection", 0);
  }
  try {
    // Try streaming approach first (modern browsers)
    if (res.body && typeof res.body.pipeThrough === "function") {
      const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
      let buffer = "";
      while (true) {
        const { done, value } = await reader.read();
        if (value) {
          buffer += value;
        }
        const lines = buffer.split(/\n/);
        let lastLine = lines.pop();
        // Save incomplete last line
        if (!lastLine) {
          lastLine = "";
        }
        buffer = lastLine;

        for (const line of lines) {
          if (line) {
            const raw: any = JSON.parse(line);
            const item = raw as ResourceItem;
            // 兼容后端 http/search.go 实际返回字段名：dir / path / name / size / modified
            (item as any).isDir =
              typeof raw.isDir === "boolean" ? raw.isDir : !!raw.dir;
            // 后端 path 是用户可见路径（以 "/" 开头，相对 FB_ROOT）。
            // 老版 removePrefix() 会错误地切掉 "/my-wiki/wiki/media/..." → "/wiki/media/..."。
            // 所以直接用 raw.path（保证仍以 "/" 开头）拼接 /files + encodePath。
            const rawPath: string = raw.path ?? "";
            const visiblePath = rawPath.startsWith("/")
              ? rawPath
              : "/" + rawPath;
            item.url = "/files" + encodePath(visiblePath);
            if ((item as any).isDir && !item.url.endsWith("/")) {
              item.url += "/";
            }
            // 搜索接口只返回最小字段，补齐 extension / type / mode 等基础字段，
            // 否则 productCodeTarget / 图标渲染 / 预览入口 / loadProductCodes
            // 等依赖 item.type 的分支会全部跳过，造成"搜索结果右键无编辑产品编号"。
            enrichSearchItem(item);
            callback(item);
          }
        }
        if (done) break;
      }
    } else {
      // Fallback for browsers without streaming support (e.g., Safari)
      const text = await res.text();
      const lines = text.split(/\n/);
      for (const line of lines) {
        if (line) {
          const raw: any = JSON.parse(line);
          const item = raw as ResourceItem;
          (item as any).isDir =
            typeof raw.isDir === "boolean" ? raw.isDir : !!raw.dir;
          const rawPath: string = raw.path ?? "";
          const visiblePath = rawPath.startsWith("/") ? rawPath : "/" + rawPath;
          item.url = "/files" + encodePath(visiblePath);
          if ((item as any).isDir && !item.url.endsWith("/")) {
            item.url += "/";
          }
          enrichSearchItem(item);
          callback(item);
        }
      }
    }
  } catch (e) {
    // Check if the error is an intentional cancellation
    if (e instanceof Error && e.name === "AbortError") {
      throw new StatusError("000 No connection", 0, true);
    }
    throw e;
  }
}
