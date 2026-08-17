import { fetchJSON } from "./utils";

export interface SimilarPdfResult {
  path: string;
  name: string;
  similarity: number; // 0~1
  dir: boolean;
  size: number;
  modified: string; // RFC3339
}

export interface SimilarPdfResponse {
  query: string;
  totalInDB: number;
  results: SimilarPdfResult[];
  diagnosis: string;
  elapsed: string;
}

// 支持的图片扩展名（小写，含点）。与后端 drawingsearch.go 的 supportedImageExts 保持一致。
const SUPPORTED_IMAGE_EXTS = new Set([
  ".jpg", ".jpeg", ".png", ".webp", ".bmp", ".tif", ".tiff", ".gif",
]);

function isSupportedUploadExt(name: string): boolean {
  const n = (name || "").toLowerCase();
  if (n.endsWith(".pdf")) return true;
  const i = n.lastIndexOf(".");
  if (i < 0) return false;
  return SUPPORTED_IMAGE_EXTS.has(n.slice(i));
}

/**
 * 调用后端 POST /api/search/similar-pdf
 * 上传 PDF 或图片（JPG/PNG/WebP/BMP/TIFF/GIF）
 *   → PDF: 后端 pdftoppm 转 300DPI PNG
 *   → 图片: 直接解码（透明通道先贴到白底，避免黑背景）
 *   → ResNet18 特征 → 对 drawings.db 做 Top-K 余弦相似度
 *
 * @param file 用户选择的 PDF 或 图片 File 对象
 * @param topK 返回前 N 条（1~100，默认 10）
 */
export async function searchSimilarPdf(
  file: File,
  topK: number = 10,
  signal?: AbortSignal
): Promise<SimilarPdfResponse> {
  if (!file) {
    throw new Error("未选择 PDF / 图片文件");
  }
  if (!isSupportedUploadExt(file.name)) {
    const ext = file.name.includes(".")
      ? file.name.slice(file.name.lastIndexOf("."))
      : "(无扩展名)";
    throw new Error(
      `文件类型不支持 (${ext})：仅支持 PDF 或图片（JPG / PNG / WebP / BMP / TIFF / GIF）`
    );
  }

  const fd = new FormData();
  fd.append("file", file);
  fd.append("k", String(Math.min(100, Math.max(1, topK | 0 || 10))));

  return fetchJSON<SimilarPdfResponse>("/api/search/similar-pdf", {
    method: "POST",
    body: fd,
    // FormData 不要手动设置 Content-Type（让浏览器自动带 boundary）
    signal,
  });
}
