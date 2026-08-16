import { fetchURL, removePrefix, StatusError } from "./utils";
import { encodePath } from "../utils/url";
import {
  isMockEnabled,
  mockSearchResultSet,
} from "./mockTimeFilter";

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
