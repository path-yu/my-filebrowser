import { fetchURL, removePrefix } from "./utils";

export interface ProductCodeEntry {
  path: string;
  code: string;
  userID?: number;
  updatedAt?: number;
}

export interface ProductCodePutResult {
  path: string;
  code: string;
  pdfUpdated: boolean;
  pdfError?: string;
}

/** 归一化为后端存储的用户可见路径：
 *  - "/files/58/a.pdf"（前端 url）→ "/58/a.pdf"
 *  - "/58/a.pdf"（item.path / req.path）→ 原样返回 */
function normalizePath(p: string): string {
  if (p === "/files" || p.startsWith("/files/")) {
    return removePrefix(p);
  }
  if (!p.startsWith("/")) p = "/" + p;
  return p;
}

/** 查询单个 PDF 的产品编号 */
export async function get(path: string): Promise<ProductCodeEntry> {
  path = normalizePath(path);
  const res = await fetchURL(`/api/productcode${path}`, { method: "GET" });
  return (await res.json()) as ProductCodeEntry;
}

/** 设置/清除产品编号（code 传空字符串表示清除） */
export async function put(
  path: string,
  code: string
): Promise<ProductCodePutResult> {
  path = normalizePath(path);
  const res = await fetchURL(`/api/productcode${path}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),
  });
  return (await res.json()) as ProductCodePutResult;
}

/** 批量查询目录列表中 PDF 的产品编号：返回 path → code 映射 */
export async function batch(paths: string[]): Promise<Record<string, string>> {
  // 空数组：无需发请求，直接返回空映射，避免后端 400 或产生无意义网络往返。
  if (!Array.isArray(paths) || paths.length === 0) return {};

  // 后端单次限制 100k；前端按每批 50k 分片，避免极端目录（10 万文件）一次性 body 太大。
  // 各分片独立请求，结果合并到同一个 map；key 冲突时后到的不会覆盖（一般 code 相同）。
  const PER_BATCH = 50_000;
  const total = paths.length;
  const merged: Record<string, string> = {};
  const normalized = paths.map((p) => normalizePath(p));

  for (let offset = 0; offset < total; offset += PER_BATCH) {
    const slice = normalized.slice(offset, offset + PER_BATCH);
    const res = await fetchURL(`/api/productcode/batch`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ paths: slice }),
    });
    const part = (await res.json()) as Record<string, string>;
    for (const k of Object.keys(part)) {
      if (!(k in merged)) merged[k] = part[k];
    }
  }
  return merged;
}

/** 按产品编号前缀反查 PDF */
export async function search(
  query: string
): Promise<ProductCodeEntry[]> {
  const res = await fetchURL(
    `/api/productcode/search?query=${encodeURIComponent(query)}`,
    { method: "GET" }
  );
  return (await res.json()) as ProductCodeEntry[];
}
