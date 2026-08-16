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
  const list = paths.map((p) => normalizePath(p));
  const res = await fetchURL(`/api/productcode/batch`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ paths: list }),
  });
  return (await res.json()) as Record<string, string>;
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
