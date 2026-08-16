import { ref, watch } from "vue";

export interface FileTypeCategory {
  id: string;
  label: string;
  extensions: string[] | null;
}

/** 十余类主流文件格式（id=other 为不属于任何已知类别的兜底） */
export const FILE_TYPE_CATEGORIES: FileTypeCategory[] = [
  { id: "all", label: "全部类型", extensions: [] },
  { id: "pdf", label: "PDF 文档", extensions: [".pdf"] },
  {
    id: "word",
    label: "Word 文档",
    extensions: [".doc", ".docx", ".rtf", ".odt"],
  },
  {
    id: "excel",
    label: "Excel 表格",
    extensions: [".xls", ".xlsx", ".csv", ".ods"],
  },
  {
    id: "ppt",
    label: "PPT 演示",
    extensions: [".ppt", ".pptx", ".odp"],
  },
  {
    id: "image",
    label: "图片",
    extensions: [
      ".jpg",
      ".jpeg",
      ".png",
      ".gif",
      ".bmp",
      ".webp",
      ".svg",
      ".ico",
      ".tif",
      ".tiff",
    ],
  },
  {
    id: "video",
    label: "视频",
    extensions: [
      ".mp4",
      ".avi",
      ".mkv",
      ".mov",
      ".wmv",
      ".flv",
      ".webm",
      ".m4v",
      ".mpg",
      ".mpeg",
    ],
  },
  {
    id: "audio",
    label: "音频",
    extensions: [".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a"],
  },
  { id: "cad", label: "CAD 图纸", extensions: [".dwg", ".dxf", ".dwt"] },
  {
    id: "archive",
    label: "压缩包",
    extensions: [".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".iso"],
  },
  { id: "text", label: "文本文件", extensions: [".txt", ".md", ".log", ".ini", ".cfg"] },
  {
    id: "code",
    label: "代码/网页",
    extensions: [
      ".html",
      ".htm",
      ".js",
      ".ts",
      ".jsx",
      ".tsx",
      ".vue",
      ".json",
      ".xml",
      ".css",
      ".py",
      ".go",
      ".java",
      ".c",
      ".cpp",
      ".h",
      ".sh",
      ".bat",
    ],
  },
  { id: "epub", label: "EPub 电子书", extensions: [".epub", ".mobi"] },
  { id: "other", label: "其他文件", extensions: null },
];

export const KNOWN_EXTENSIONS = new Set(
  FILE_TYPE_CATEGORIES.flatMap((c) => c.extensions ?? [])
);

export const FILE_TYPE_FILTER_OPTIONS = FILE_TYPE_CATEGORIES.map((c) => ({
  value: c.id,
  label: c.label,
}));

// 模块级单例：同一文件页的 Files.vue（面包屑）与 FileListing.vue（过滤）共享同一个 ref
export const fileTypeFilter = ref<string>(
  typeof localStorage !== "undefined"
    ? localStorage.getItem("fileTypeFilter") ?? "all"
    : "all"
);

watch(fileTypeFilter, (v) => {
  if (typeof localStorage !== "undefined") {
    localStorage.setItem("fileTypeFilter", v);
  }
});

/** 目录始终保留（保证导航）；文件按所选类别过滤
 *  但搜索模式（searchMode = true）下**不再二次过滤**：
 *  搜索命中本身就是用户想要的结果，若命中的恰好不是当前筛选类别也应展示，
 *  否则会出现"明明搜索到了 X 条（顶部徽标）但列表是空"的困惑。 */
export function matchesTypeFilter(
  item: { extension?: string; isDir?: boolean } | undefined,
  viewMode: string = "list",
  searchMode: boolean = false
): boolean {
  if (searchMode) return true;
  if (!item) return true;
  if (viewMode !== "list") return true;
  const cat = FILE_TYPE_CATEGORIES.find((c) => c.id === fileTypeFilter.value);
  if (!cat || cat.id === "all") return true;
  const ext = (item.extension || "").toLowerCase();
  if (cat.extensions === null) return !KNOWN_EXTENSIONS.has(ext);
  return cat.extensions.includes(ext);
}
